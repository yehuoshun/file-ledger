package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"file-ledger/internal/api"
	"file-ledger/internal/tag"
)

// mockTagStore 内存版 TagStore（黑盒实现导出接口）
type mockTagStore struct {
	items map[string]tag.Tag // key = id
	seq   int
}

func newMockTagStore() *mockTagStore {
	return &mockTagStore{items: map[string]tag.Tag{}}
}

func (m *mockTagStore) List(_ context.Context) ([]tag.Tag, error) {
	out := make([]tag.Tag, 0, len(m.items))
	for _, t := range m.items {
		out = append(out, t)
	}
	return out, nil
}

func (m *mockTagStore) Create(_ context.Context, t *tag.Tag) (*tag.Tag, error) {
	for _, exist := range m.items {
		if exist.Name == t.Name {
			return nil, tag.ErrNameTaken
		}
	}
	m.seq++
	t.ID = fmt.Sprintf("tag-%d", m.seq)
	if t.BG == "" {
		t.BG = "#fae3c8"
	}
	if t.FG == "" {
		t.FG = "#b35d0a"
	}
	t.CreatedAt = "2026-11-17T00:00:00Z"
	m.items[t.ID] = *t
	cp := *t
	return &cp, nil
}

func (m *mockTagStore) Update(_ context.Context, id string, t *tag.Tag) (*tag.Tag, error) {
	old, ok := m.items[id]
	if !ok {
		return nil, tag.ErrNotFound
	}
	for _, exist := range m.items {
		if exist.ID != id && exist.Name == t.Name {
			return nil, tag.ErrNameTaken
		}
	}
	t.ID = id
	t.CreatedAt = old.CreatedAt
	if t.BG == "" {
		t.BG = old.BG
	}
	if t.FG == "" {
		t.FG = old.FG
	}
	m.items[id] = *t
	cp := *t
	return &cp, nil
}

func (m *mockTagStore) Delete(_ context.Context, id string) error {
	if _, ok := m.items[id]; !ok {
		return tag.ErrNotFound
	}
	delete(m.items, id)
	return nil
}

func setupTags(t *testing.T, store api.TagStore) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	api.RegisterTagRoutes(mux, store)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// doJSONErr 发请求并解析 {"error": msg}（非 2xx 响应体）
func doJSONErr(t *testing.T, method, url, body string, wantStatus int) string {
	t.Helper()
	resp := doJSON(t, method, url, body)
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s = %d, 期望 %d", method, url, resp.StatusCode, wantStatus)
	}
	var out map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("解析错误响应失败: %v", err)
	}
	return out["error"]
}

// TestTagListShape GET /api/tags 返回数组，字段齐全（前端标签色板直接消费）
func TestTagListShape(t *testing.T) {
	store := newMockTagStore()
	if _, err := store.Create(context.Background(), &tag.Tag{Name: "工作"}); err != nil {
		t.Fatalf("预置标签失败: %v", err)
	}
	ts := setupTags(t, store)

	resp := doJSON(t, "GET", ts.URL+"/api/tags", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET = %d, 期望 200", resp.StatusCode)
	}
	var list []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("解析列表失败: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("列表长度 = %d, 期望 1", len(list))
	}
	for _, key := range []string{"id", "name", "bg", "fg", "createdAt"} {
		if _, ok := list[0][key]; !ok {
			t.Fatalf("返回缺少字段 %q: %v", key, list[0])
		}
	}
	if list[0]["name"] != "工作" {
		t.Fatalf("name = %v, 期望 工作", list[0]["name"])
	}
}

// TestTagCreate POST 201，名字 trim，未给颜色时由存储层补齐
func TestTagCreate(t *testing.T) {
	ts := setupTags(t, newMockTagStore())

	resp := doJSON(t, "POST", ts.URL+"/api/tags", `{"name":"  工作  "}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST = %d, 期望 201", resp.StatusCode)
	}
	var created tag.Tag
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("解析新增响应失败: %v", err)
	}
	if created.ID == "" || created.Name != "工作" {
		t.Fatalf("新增返回异常: %+v", created)
	}
	if created.BG == "" || created.FG == "" {
		t.Fatalf("配色未补齐: %+v", created)
	}
}

// TestTagConflict 重名 409
func TestTagConflict(t *testing.T) {
	ts := setupTags(t, newMockTagStore())

	resp := doJSON(t, "POST", ts.URL+"/api/tags", `{"name":"工作","bg":"#d4eaff","fg":"#1565c0"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("首次新增 = %d, 期望 201", resp.StatusCode)
	}
	resp.Body.Close()

	if msg := doJSONErr(t, "POST", ts.URL+"/api/tags", `{"name":"工作"}`, http.StatusConflict); msg != "标签已存在" {
		t.Fatalf("409 响应 = %q, 期望 标签已存在", msg)
	}
}

// TestTagNameValidation 空名 / 超 32 字符 → 400
func TestTagNameValidation(t *testing.T) {
	ts := setupTags(t, newMockTagStore())

	doJSONErr(t, "POST", ts.URL+"/api/tags", `{"name":"   "}`, http.StatusBadRequest)
	doJSONErr(t, "POST", ts.URL+"/api/tags",
		`{"name":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, http.StatusBadRequest)
}

// TestTagUpdate 改名成功 200；不存在 404
func TestTagUpdate(t *testing.T) {
	store := newMockTagStore()
	created, err := store.Create(context.Background(), &tag.Tag{Name: "工作"})
	if err != nil {
		t.Fatalf("预置标签失败: %v", err)
	}
	ts := setupTags(t, store)

	resp := doJSON(t, "PUT", ts.URL+"/api/tags/"+created.ID, `{"name":"上班","bg":"#d4eaff","fg":"#1565c0"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT = %d, 期望 200", resp.StatusCode)
	}
	var updated tag.Tag
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("解析更新响应失败: %v", err)
	}
	if updated.Name != "上班" || updated.BG != "#d4eaff" {
		t.Fatalf("更新返回异常: %+v", updated)
	}

	if msg := doJSONErr(t, "PUT", ts.URL+"/api/tags/not-exist", `{"name":"x"}`, http.StatusNotFound); msg != "标签不存在" {
		t.Fatalf("404 响应 = %q, 期望 标签不存在", msg)
	}
}

// TestTagDelete DELETE 204，重复删除 404
func TestTagDelete(t *testing.T) {
	store := newMockTagStore()
	created, err := store.Create(context.Background(), &tag.Tag{Name: "备份"})
	if err != nil {
		t.Fatalf("预置标签失败: %v", err)
	}
	ts := setupTags(t, store)

	resp := doJSON(t, "DELETE", ts.URL+"/api/tags/"+created.ID, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE = %d, 期望 204", resp.StatusCode)
	}
	resp.Body.Close()

	doJSONErr(t, "DELETE", ts.URL+"/api/tags/"+created.ID, "", http.StatusNotFound)
}
