package test

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"file-ledger/internal/fsx"
)

// driveItem / driveResp 是契约规定的响应形状（黑盒断言用）
type driveItem struct {
	Drive  string `json:"drive"`
	Online bool   `json:"online"`
	Reason string `json:"reason"`
}

type driveResp struct {
	Items []driveItem `json:"items"`
	Total int         `json:"total"`
}

// currentDrive 返回测试临时目录所在盘符（形如 "D:"）
func currentDrive(t *testing.T) string {
	t.Helper()
	v := filepath.VolumeName(t.TempDir())
	if len(v) != 2 || v[1] != ':' {
		t.Fatalf("拿不到盘符（VolumeName=%q）", v)
	}
	return strings.ToUpper(v)
}

// pickOfflineDrive 找一个当前确实不通的盘符；26 个字母都在线则跳过该断言
func pickOfflineDrive(t *testing.T) string {
	t.Helper()
	for c := byte('A'); c <= 'Z'; c++ {
		d := string(c) + ":"
		if _, err := os.Stat(d + string(filepath.Separator)); err != nil {
			return d
		}
	}
	t.Skip("26 个盘符全部在线，跳过离线断言")
	return ""
}

// parseDriveResp 解析响应体，顺带校验 items 与 total 一致
func parseDriveResp(t *testing.T, body string) driveResp {
	t.Helper()
	var got driveResp
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("解析响应失败: %v, body=%s", err, body)
	}
	if len(got.Items) != got.Total {
		t.Fatalf("items 长度 %d 与 total %d 不一致: %s", len(got.Items), got.Total, body)
	}
	return got
}

func TestParseDrives(t *testing.T) {
	// 归一化：裸盘符补冒号、小写转大写
	got, err := fsx.ParseDrives(" d ,Z:")
	if err != nil {
		t.Fatalf("ParseDrives 失败: %v", err)
	}
	if strings.Join(got, ",") != "D:,Z:" {
		t.Fatalf("归一化 = %v, 期望 [D: Z:]", got)
	}

	// 去重保序
	got, err = fsx.ParseDrives("C:,D:,C:,d")
	if err != nil {
		t.Fatalf("ParseDrives 失败: %v", err)
	}
	if strings.Join(got, ",") != "C:,D:" {
		t.Fatalf("去重保序 = %v, 期望 [C: D:]", got)
	}

	// 非法：空/空元素/数字/多加冒号/带路径/中文
	for _, bad := range []string{"", "   ", ",", "D:,", ",D:", "1", "D::", "DD", `D:\x`, "D:x", "D:+", "盘符"} {
		if _, err := fsx.ParseDrives(bad); !errors.Is(err, fsx.ErrInvalidDrive) {
			t.Fatalf("ParseDrives(%q) err = %v, 期望 ErrInvalidDrive", bad, err)
		}
	}

	// 上限：27 个元素（含重复）→ 非法
	many := strings.TrimSuffix(strings.Repeat("A:,", fsx.MaxDrives+1), ",")
	if _, err := fsx.ParseDrives(many); !errors.Is(err, fsx.ErrInvalidDrive) {
		t.Fatalf("ParseDrives(27 个元素) err = %v, 期望 ErrInvalidDrive", err)
	}
	// 26 个元素（A-Z 各一次）是允许的
	var all strings.Builder
	for c := byte('A'); c <= 'Z'; c++ {
		all.WriteString(string(c) + ":,")
	}
	ok26 := strings.TrimSuffix(all.String(), ",")
	if got, err := fsx.ParseDrives(ok26); err != nil || len(got) != fsx.MaxDrives {
		t.Fatalf("ParseDrives(26 个元素) = %v, %v, 期望 26 个且无错", got, err)
	}
}

func TestFSStatusHTTP(t *testing.T) {
	cur := currentDrive(t)
	ts := setupFSServer(t)

	// 等价写 + 去重：裸盘符、小写、重复都归一为同一个 "D:"
	_, body := fsGet(t, ts.URL+"/api/fs/status?drives="+cur[:1]+","+strings.ToLower(cur))
	got := parseDriveResp(t, body)
	if got.Total != 1 || len(got.Items) != 1 {
		t.Fatalf("%q 与 %q 应去重成 1 个: %s", cur[:1], strings.ToLower(cur), body)
	}
	if got.Items[0].Drive != cur {
		t.Fatalf("drive = %q, 期望归一化 %q", got.Items[0].Drive, cur)
	}
	if !got.Items[0].Online {
		t.Fatalf("当前盘符 %q 应在线: %s", cur, body)
	}
	if strings.Contains(body, `"reason"`) {
		t.Fatalf("在线时不应出现 reason 字段: %s", body)
	}
	for _, key := range []string{`"items":[`, `"total":1`, `"online":true`} {
		if !strings.Contains(body, key) {
			t.Fatalf("响应缺少关键字段 %s: %s", key, body)
		}
	}

	// 离线：挑一个当前不通的盘符，必须 online=false 且带 reason
	off := pickOfflineDrive(t)
	_, body = fsGet(t, ts.URL+"/api/fs/status?drives="+cur+","+off)
	got = parseDriveResp(t, body)
	if got.Total != 2 {
		t.Fatalf("两个盘符 total = %d, 期望 2: %s", got.Total, body)
	}
	if got.Items[0].Drive != cur || got.Items[1].Drive != off {
		t.Fatalf("顺序 = %q,%q, 期望 %q,%q", got.Items[0].Drive, got.Items[1].Drive, cur, off)
	}
	if !got.Items[0].Online || got.Items[1].Online {
		t.Fatalf("在线状态 = %v/%v, 期望 true/false: %s", got.Items[0].Online, got.Items[1].Online, body)
	}
	if got.Items[1].Reason != fsx.ReasonOffline && got.Items[1].Reason != fsx.ReasonTimeout {
		t.Fatalf("离线 reason = %q, 期望「%s」或「%s」", got.Items[1].Reason, fsx.ReasonOffline, fsx.ReasonTimeout)
	}
	if !strings.Contains(body, `"reason"`) {
		t.Fatalf("离线时应出现 reason 字段: %s", body)
	}
}

func TestFSStatusParamErrors(t *testing.T) {
	ts := setupFSServer(t)

	many := strings.TrimSuffix(strings.Repeat("A:,", fsx.MaxDrives+1), ",")
	bad := []string{
		"",                // 缺省
		"?drives=",        // 空串
		"?drives=%20%20",  // 全空白
		"?drives=1",       // 不是字母
		"?drives=DD",      // 两个字母
		"?drives=D::",     // 多冒号
		"?drives=D:x",     // 冒号后还有字符
		"?drives=D:,",     // 空元素
		"?drives=,D:",     // 空元素
		"?drives=" + many, // 超过 26 个元素
	}
	for _, qs := range bad {
		resp, body := fsGet(t, ts.URL+"/api/fs/status"+qs)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("query %q 状态码 = %d, 期望 400, body=%s", qs, resp.StatusCode, body)
		}
		var e struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal([]byte(body), &e); err != nil || e.Error != "盘符参数不合法" {
			t.Fatalf("query %q 错误响应异常: err=%v body=%s", qs, err, body)
		}
	}
}
