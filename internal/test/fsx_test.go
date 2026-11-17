package test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"file-ledger/internal/api"
	"file-ledger/internal/fsx"
)

// makeFSTree 在 t.TempDir() 下造一棵最小目录树：
// <root>/aa（目录）、<root>/zz（目录）、<root>/A.txt、<root>/b.TXT（12345 字节）。
func makeFSTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"aa", "zz"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatalf("建目录 %s 失败: %v", d, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "A.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("写 A.txt 失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.TXT"), make([]byte, 12345), 0o644); err != nil {
		t.Fatalf("写 b.TXT 失败: %v", err)
	}
	return root
}

// namesOf 取出条目名序列（断言顺序用）
func namesOf(l *fsx.Listing) []string {
	out := make([]string, 0, len(l.Entries))
	for _, e := range l.Entries {
		out = append(out, e.Name)
	}
	return out
}

func TestFSListShape(t *testing.T) {
	root := makeFSTree(t)

	l, err := fsx.List(root, fsx.DefaultLimit)
	if err != nil {
		t.Fatalf("List(%q) 失败: %v", root, err)
	}
	if l.Path != root {
		t.Fatalf("Path = %q, 期望 %q", l.Path, root)
	}
	if want := filepath.Dir(root); l.Parent != want {
		t.Fatalf("Parent = %q, 期望 %q", l.Parent, want)
	}
	if l.Total != 4 || len(l.Entries) != 4 || l.Truncated {
		t.Fatalf("total/entries/truncated = %d/%d/%v, 期望 4/4/false", l.Total, len(l.Entries), l.Truncated)
	}

	byName := make(map[string]fsx.Entry, len(l.Entries))
	for _, e := range l.Entries {
		byName[e.Name] = e
	}
	if !byName["aa"].IsDir || !byName["zz"].IsDir {
		t.Fatalf("目录项 IsDir 未置位: %+v", l.Entries)
	}
	if byName["b.TXT"].Size != 12345 {
		t.Fatalf("b.TXT size = %d, 期望 12345", byName["b.TXT"].Size)
	}
	if byName["b.TXT"].Ext != ".TXT" || byName["b.TXT"].IsDir {
		t.Fatalf("b.TXT 的 ext/isDir = %q/%v, 期望 .TXT/false", byName["b.TXT"].Ext, byName["b.TXT"].IsDir)
	}
	if byName["aa"].Ext != "" || byName["aa"].Size != 0 {
		t.Fatalf("目录项 ext/size = %q/%d, 期望空串/0", byName["aa"].Ext, byName["aa"].Size)
	}
	if _, err := time.Parse(time.RFC3339, byName["A.txt"].ModTime); err != nil {
		t.Fatalf("modTime %q 不是 RFC3339: %v", byName["A.txt"].ModTime, err)
	}
}

func TestFSListSorting(t *testing.T) {
	root := makeFSTree(t)
	for _, n := range []string{"Zebra.txt", "apple.txt"} {
		if err := os.WriteFile(filepath.Join(root, n), []byte("x"), 0o644); err != nil {
			t.Fatalf("写 %s 失败: %v", n, err)
		}
	}

	l, err := fsx.List(root, 0)
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	// 目录优先，再按名字不区分大小写升序：Zebra.txt 排末尾而不是按 ASCII 排最前
	want := []string{"aa", "zz", "A.txt", "apple.txt", "b.TXT", "Zebra.txt"}
	if got := namesOf(l); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("排序 = %v, 期望 %v", got, want)
	}
}

func TestFSListFilePath(t *testing.T) {
	root := makeFSTree(t)
	l, err := fsx.List(filepath.Join(root, "A.txt"), 0)
	if err != nil {
		t.Fatalf("List(文件) 失败: %v", err)
	}
	if l.Path != root {
		t.Fatalf("指向文件时 Path = %q, 期望所在目录 %q", l.Path, root)
	}
	if l.Total != 4 {
		t.Fatalf("指向文件时 Total = %d, 期望 4", l.Total)
	}
}

func TestFSListTruncate(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(root, n), []byte("x"), 0o644); err != nil {
			t.Fatalf("写 %s 失败: %v", n, err)
		}
	}

	small, err := fsx.List(root, 2)
	if err != nil {
		t.Fatalf("List(limit=2) 失败: %v", err)
	}
	if len(small.Entries) != 2 || small.Total != 3 || !small.Truncated {
		t.Fatalf("limit=2 → entries/total/truncated = %d/%d/%v, 期望 2/3/true",
			len(small.Entries), small.Total, small.Truncated)
	}

	full, err := fsx.List(root, 10)
	if err != nil {
		t.Fatalf("List(limit=10) 失败: %v", err)
	}
	if len(full.Entries) != 3 || full.Total != 3 || full.Truncated {
		t.Fatalf("limit=10 → entries/total/truncated = %d/%d/%v, 期望 3/3/false",
			len(full.Entries), full.Total, full.Truncated)
	}

	def, err := fsx.List(root, 0) // limit <= 0 回落 DefaultLimit
	if err != nil {
		t.Fatalf("List(limit=0) 失败: %v", err)
	}
	if def.Total != 3 || def.Truncated {
		t.Fatalf("limit=0 → total/truncated = %d/%v, 期望 3/false", def.Total, def.Truncated)
	}
}

func TestFSListInvalidPath(t *testing.T) {
	root := makeFSTree(t)
	sep := string(filepath.Separator)
	cases := []string{
		"",
		"   ",
		"relative",
		`aa\zz`,
		"..",
		`..\..\windows`,
		root + sep + ".." + sep + "x",
		root + sep + "..",
	}
	for _, c := range cases {
		if _, err := fsx.List(c, 0); !errors.Is(err, fsx.ErrInvalidPath) {
			t.Fatalf("List(%q) err = %v, 期望 ErrInvalidPath", c, err)
		}
	}
}

func TestFSListNotExist(t *testing.T) {
	root := makeFSTree(t)
	missing := filepath.Join(root, "nope", "deeper")
	if _, err := fsx.List(missing, 0); !errors.Is(err, fsx.ErrNotExist) {
		t.Fatalf("List(%q) err = %v, 期望 ErrNotExist", missing, err)
	}
}

func TestFSListRootParent(t *testing.T) {
	root := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	l, err := fsx.List(root, 5)
	if err != nil {
		t.Skipf("盘符根 %q 不可读，跳过: %v", root, err)
	}
	if l.Parent != "" {
		t.Fatalf("盘符根 %q 的 parent = %q, 期望空串", root, l.Parent)
	}
}

// setupFSServer 起一个只挂 fs 路由的测试服务器
func setupFSServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	api.RegisterFSRoutes(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// fsGet 发 GET 并读回状态码与响应体
func fsGet(t *testing.T, u string) (*http.Response, string) {
	t.Helper()
	resp, err := http.Get(u)
	if err != nil {
		t.Fatalf("GET %s 失败: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var b strings.Builder
	if _, err := io.Copy(&b, resp.Body); err != nil {
		t.Fatalf("读响应体失败: %v", err)
	}
	return resp, b.String()
}

func TestFSListHTTP(t *testing.T) {
	root := makeFSTree(t)
	ts := setupFSServer(t)

	resp, body := fsGet(t, ts.URL+"/api/fs/list?path="+url.QueryEscape(root))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200, body=%s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, 期望 application/json 前缀", ct)
	}
	var got struct {
		Path      string      `json:"path"`
		Parent    string      `json:"parent"`
		Entries   []fsx.Entry `json:"entries"`
		Total     int         `json:"total"`
		Truncated bool        `json:"truncated"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("解析响应失败: %v, body=%s", err, body)
	}
	if got.Path != root || got.Parent != filepath.Dir(root) {
		t.Fatalf("path/parent = %q/%q, 期望 %q/%q", got.Path, got.Parent, root, filepath.Dir(root))
	}
	if got.Total != 4 || len(got.Entries) != 4 || got.Truncated {
		t.Fatalf("total/entries/truncated = %d/%d/%v, 期望 4/4/false", got.Total, len(got.Entries), got.Truncated)
	}
	// 原始报文里的键名必须是契约规定的小写形式（json 解码大小写不敏感，这里补一道线上格式断言）
	for _, key := range []string{`"path":"`, `"parent":"`, `"entries":[`, `"total":4`, `"truncated":false`} {
		if !strings.Contains(body, key) {
			t.Fatalf("响应缺少关键字段 %s: %s", key, body)
		}
	}

	found := false
	for _, e := range got.Entries {
		if e.Name == "b.TXT" {
			found = true
			if e.Size != 12345 || e.Ext != ".TXT" || e.IsDir {
				t.Fatalf("b.TXT 字段异常: %+v", e)
			}
		}
	}
	if !found {
		t.Fatalf("响应里没有 b.TXT: %s", body)
	}

	bad := []string{
		"",
		"?path=",
		"?path=" + url.QueryEscape("relative"),
		"?path=" + url.QueryEscape(`..\x`),
		"?path=" + url.QueryEscape(filepath.Join(root, "nope")),
	}
	for _, q := range bad {
		resp, body := fsGet(t, ts.URL+"/api/fs/list"+q)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("query %q 状态码 = %d, 期望 400, body=%s", q, resp.StatusCode, body)
		}
		var e struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal([]byte(body), &e); err != nil || e.Error == "" {
			t.Fatalf("query %q 错误响应异常: err=%v body=%s", q, err, body)
		}
	}
}
