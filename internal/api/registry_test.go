package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"file-ledger/internal/registry"
)

// mockStore 内存版 RegistryStore，只测 handler 行为
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

// setup 起一个带路由的测试服务器
func setup(t *testing.T, store RegistryStore) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	RegisterRegistryRoutes(mux, store)
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

func decodeEntry(t *testing.T, resp *http.Response) registry.Entry {
	t.Helper()
	defer resp.Body.Close()
	var e registry.Entry
	if err := json.NewDecoder(resp.Body).Decode(&e); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	return e
}

func validEntry() string {
	return `{"alias":"文档","isDir":true,"size":"1.2 GB","note":"项目文档","tags":["工作"],"paths":[{"type":"local","path":"D:\\docs"},{"type":"virtual","path":"ali:docs"}]}`
}

func TestCRUDFlow(t *testing.T) {
	ts := setup(t, newMockStore())

	// 1. 空列表
	resp := doJSON(t, "GET", ts.URL+"/api/registry", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET 空列表状态码 = %d, 期望 200", resp.StatusCode)
	}
	var list []registry.Entry
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("解析列表失败: %v", err)
	}
	resp.Body.Close()
	if len(list) != 0 {
		t.Fatalf("空列表期望 0 条, 实际 %d", len(list))
	}

	// 2. 新增
	resp = doJSON(t, "POST", ts.URL+"/api/registry", validEntry())
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST 状态码 = %d, 期望 201", resp.StatusCode)
	}
	created := decodeEntry(t, resp)
	if created.ID == "" || created.Alias != "文档" || len(created.Paths) != 2 {
		t.Fatalf("新增返回异常: %+v", created)
	}

	// 3. 更新
	resp = doJSON(t, "PUT", ts.URL+"/api/registry/"+created.ID,
		`{"alias":"文档改","isDir":false,"size":"2 GB","note":"","tags":[],"paths":[{"type":"mount","path":"Z:"}]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT 状态码 = %d, 期望 200", resp.StatusCode)
	}
	updated := decodeEntry(t, resp)
	if updated.Alias != "文档改" || updated.IsDir || len(updated.Paths) != 1 {
		t.Fatalf("更新返回异常: %+v", updated)
	}

	// 4. 删除
	resp = doJSON(t, "DELETE", ts.URL+"/api/registry/"+created.ID, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE 状态码 = %d, 期望 204", resp.StatusCode)
	}
	resp.Body.Close()

	// 5. 删除后再查 → 404
	resp = doJSON(t, "DELETE", ts.URL+"/api/registry/"+created.ID, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("重复删除状态码 = %d, 期望 404", resp.StatusCode)
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
		{"路径为空", `{"alias":"x","paths":[]}`, 400},
		{"类型非法", `{"alias":"x","paths":[{"type":"ftp","path":"D:\\x"}]}`, 400},
		{"路径为空串", `{"alias":"x","paths":[{"type":"local","path":""}]}`, 400},
		{"非 JSON", `not-json`, 400},
		{"合法最小", `{"alias":"x","paths":[{"type":"local","path":"D:\\x"}]}`, 201},
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
		t.Fatalf("首次新增状态码 = %d, 期望 201", resp.StatusCode)
	}
	resp.Body.Close()

	resp = doJSON(t, "POST", ts.URL+"/api/registry", validEntry())
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("重名状态码 = %d, 期望 409", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestUpdateNotFound(t *testing.T) {
	ts := setup(t, newMockStore())
	resp := doJSON(t, "PUT", ts.URL+"/api/registry/no-such-id", validEntry())
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("PUT 不存在状态码 = %d, 期望 404", resp.StatusCode)
	}
	resp.Body.Close()
}

// 编译期断言：*registry.Store 满足 RegistryStore
var _ RegistryStore = (*registry.Store)(nil)
