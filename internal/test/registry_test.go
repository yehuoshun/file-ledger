// Package test 开发期自检测试（沙箱用，黑盒：只测导出 API）。
// 与主代码分目录存放，不影响日常浏览；go test ./... 仍会执行。
package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"file-ledger/internal/api"
	"file-ledger/internal/registry"
)

// mockStore 内存版 RegistryStore（黑盒实现导出接口）
type mockStore struct {
	items map[string]registry.Entry
	next  int
}

func newMockStore() *mockStore {
	return &mockStore{items: map[string]registry.Entry{}}
}

func (m *mockStore) List(_ context.Context) ([]registry.Entry, error) {
	out := make([]registry.Entry, 0, len(m.items))
	for _, e := range m.items {
		out = append(out, e)
	}
	return out, nil
}

func (m *mockStore) Create(_ context.Context, e *registry.Entry) (*registry.Entry, error) {
	if _, exists := m.items[e.Alias]; exists {
		return nil, registry.ErrAliasTaken
	}
	m.next++
	e.ID = string(rune('a' + m.next))
	m.items[e.Alias] = *e
	cp := *e
	return &cp, nil
}

func (m *mockStore) Update(_ context.Context, id string, e *registry.Entry) (*registry.Entry, error) {
	for alias, item := range m.items {
		if item.ID == id {
			if alias != e.Alias {
				if _, exists := m.items[e.Alias]; exists {
					return nil, registry.ErrAliasTaken
				}
				delete(m.items, alias)
			}
			e.ID = id
			m.items[e.Alias] = *e
			cp := *e
			return &cp, nil
		}
	}
	return nil, registry.ErrNotFound
}

func (m *mockStore) Delete(_ context.Context, id string) error {
	for alias, item := range m.items {
		if item.ID == id {
			delete(m.items, alias)
			return nil
		}
	}
	return registry.ErrNotFound
}

// mockRegistrar 内存版 TagRegistrar：记录 Ensure 每次收到的标签名，便于断言归一结果
type mockRegistrar struct {
	calls [][]string
}

func newMockRegistrar() *mockRegistrar {
	return &mockRegistrar{}
}

func (m *mockRegistrar) Ensure(_ context.Context, names []string) error {
	m.calls = append(m.calls, append([]string(nil), names...))
	return nil
}

func setup(t *testing.T, store api.RegistryStore) *httptest.Server {
	t.Helper()
	return setupWith(t, store, newMockRegistrar())
}

// setupWith 可注入自定义标签登记 mock（默认用记录型 mockRegistrar）
func setupWith(t *testing.T, store api.RegistryStore, tags api.TagRegistrar) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	api.RegisterRegistryRoutes(mux, store, tags)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func doJSON(t *testing.T, method, url, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	return resp
}

func validEntry() string {
	return `{"alias":"文档","isDir":true,"size":"1.2 GB","note":"项目文档","tags":["工作"],"paths":[{"type":"local","path":"D:\\docs"}]}`
}

func TestCRUDFlow(t *testing.T) {
	ts := setup(t, newMockStore())

	resp := doJSON(t, "GET", ts.URL+"/api/registry", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET 空列表 = %d, 期望 200", resp.StatusCode)
	}
	var list []registry.Entry
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("解析列表失败: %v", err)
	}
	resp.Body.Close()
	if len(list) != 0 {
		t.Fatalf("空列表期望 0 条, 实际 %d", len(list))
	}

	resp = doJSON(t, "POST", ts.URL+"/api/registry", validEntry())
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST = %d, 期望 201", resp.StatusCode)
	}
	var created registry.Entry
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("解析新增响应失败: %v", err)
	}
	resp.Body.Close()
	if created.ID == "" || created.Alias != "文档" {
		t.Fatalf("新增返回异常: %+v", created)
	}

	resp = doJSON(t, "PUT", ts.URL+"/api/registry/"+created.ID,
		`{"alias":"文档改","isDir":true,"size":"","note":"","tags":[],"paths":[{"type":"mount","path":"Z:"}]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT = %d, 期望 200", resp.StatusCode)
	}
	resp.Body.Close()

	resp = doJSON(t, "DELETE", ts.URL+"/api/registry/"+created.ID, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE = %d, 期望 204", resp.StatusCode)
	}
	resp.Body.Close()

	resp = doJSON(t, "DELETE", ts.URL+"/api/registry/"+created.ID, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("重复删除 = %d, 期望 404", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestValidation(t *testing.T) {
	ts := setup(t, newMockStore())

	cases := []struct {
		name string
		body string
		want int
	}{
		{"别名空", `{"alias":"","paths":[{"type":"local","path":"D:\\x"}]}`, 400},
		{"文件无路径", `{"alias":"x","isDir":false,"paths":[]}`, 400},
		{"类型非法", `{"alias":"x","paths":[{"type":"ftp","path":"D:\\x"}]}`, 400},
		{"路径空串", `{"alias":"x","paths":[{"type":"local","path":""}]}`, 400},
		{"非 JSON", `not-json`, 400},
		{"文件夹无路径合法", `{"alias":"游戏","isDir":true,"paths":[]}`, 201},
		{"标签超长", `{"alias":"x","isDir":true,"tags":["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"],"paths":[]}`, 400},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp := doJSON(t, "POST", ts.URL+"/api/registry", c.body)
			if resp.StatusCode != c.want {
				t.Fatalf("状态码 = %d, 期望 %d", resp.StatusCode, c.want)
			}
			resp.Body.Close()
		})
	}
}

func TestAliasConflict(t *testing.T) {
	ts := setup(t, newMockStore())

	resp := doJSON(t, "POST", ts.URL+"/api/registry", validEntry())
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("首次新增 = %d, 期望 201", resp.StatusCode)
	}
	resp.Body.Close()

	resp = doJSON(t, "POST", ts.URL+"/api/registry", validEntry())
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("重名 = %d, 期望 409", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestEntrySaveEnsuresTags 保存条目时自动登记标签，且收到的是归一后的标签（trim/去空/去重）
func TestEntrySaveEnsuresTags(t *testing.T) {
	reg := newMockRegistrar()
	ts := setupWith(t, newMockStore(), reg)

	body := `{"alias":"项目文档","isDir":true,"tags":[" 工作 ","","工作","文档"," 文档 "],"paths":[{"type":"local","path":"D:\\docs"}]}`
	resp := doJSON(t, "POST", ts.URL+"/api/registry", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST = %d, 期望 201", resp.StatusCode)
	}
	var created registry.Entry
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("解析新增响应失败: %v", err)
	}
	resp.Body.Close()

	want := []string{"工作", "文档"}
	if len(reg.calls) != 1 {
		t.Fatalf("Ensure 调用次数 = %d, 期望 1", len(reg.calls))
	}
	if !slices.Equal(reg.calls[0], want) {
		t.Fatalf("Ensure 收到 %v, 期望 %v", reg.calls[0], want)
	}
	if !slices.Equal(created.Tags, want) {
		t.Fatalf("新增返回的 tags = %v, 期望 %v", created.Tags, want)
	}

	// PUT 同样先登记标签
	resp = doJSON(t, "PUT", ts.URL+"/api/registry/"+created.ID,
		`{"alias":"项目文档","isDir":true,"tags":[" 重要 ","重要","  "],"paths":[{"type":"mount","path":"Z:"}]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT = %d, 期望 200", resp.StatusCode)
	}
	resp.Body.Close()

	if len(reg.calls) != 2 {
		t.Fatalf("PUT 后 Ensure 调用次数 = %d, 期望 2", len(reg.calls))
	}
	if !slices.Equal(reg.calls[1], []string{"重要"}) {
		t.Fatalf("PUT 后 Ensure 收到 %v, 期望 [重要]", reg.calls[1])
	}
}
