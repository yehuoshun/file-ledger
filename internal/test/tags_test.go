package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
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

// sorted 按 name 升序返回全部标签（与存储层 ORDER BY name 对齐）
func (m *mockTagStore) sorted() []tag.Tag {
	out := make([]tag.Tag, 0, len(m.items))
	for _, t := range m.items {
		out = append(out, t)
	}
	slices.SortFunc(out, func(a, b tag.Tag) int { return strings.Compare(a.Name, b.Name) })
	return out
}

func (m *mockTagStore) List(_ context.Context) ([]tag.Tag, error) {
	return m.sorted(), nil
}

// ListPaged 名字包含匹配（大小写不敏感）+ 分页；返回当页数据与匹配总数
func (m *mockTagStore) ListPaged(_ context.Context, q string, limit, offset int) ([]tag.Tag, int, error) {
	needle := strings.ToLower(q)
	matched := make([]tag.Tag, 0, len(m.items))
	for _, t := range m.sorted() {
		if strings.Contains(strings.ToLower(t.Name), needle) {
			matched = append(matched, t)
		}
	}
	total := len(matched)
	if offset >= total {
		return []tag.Tag{}, total, nil
	}
	end := offset + limit
	if end > total || end < offset {
		end = total
	}
	return matched[offset:end], total, nil
}

// ListByNames 名字批量精确取；没登记的名字忽略
func (m *mockTagStore) ListByNames(_ context.Context, names []string) ([]tag.Tag, error) {
	want := make(map[string]struct{}, len(names))
	for _, name := range names {
		want[name] = struct{}{}
	}
	out := make([]tag.Tag, 0, len(names))
	for _, t := range m.sorted() {
		if _, ok := want[t.Name]; ok {
			out = append(out, t)
		}
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

// mustCreateTags 预置标签（顺序无关，返回按 name 排好的结果由接口决定）
func mustCreateTags(t *testing.T, store *mockTagStore, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := store.Create(context.Background(), &tag.Tag{Name: name}); err != nil {
			t.Fatalf("预置标签 %q 失败: %v", name, err)
		}
	}
}

// tagPage GET /api/tags 响应体：items 保留原始 map 以便校验字段齐全
type tagPage struct {
	Items []map[string]any `json:"items"`
	Total int              `json:"total"`
	raw   string
}

// names 当页标签名（保持返回顺序）
func (p tagPage) names() []string {
	out := make([]string, 0, len(p.Items))
	for _, item := range p.Items {
		name, _ := item["name"].(string)
		out = append(out, name)
	}
	return out
}

// getTagList 请求 GET /api/tags?<rawQuery>，断言状态码并解析 {items,total}
func getTagList(t *testing.T, ts *httptest.Server, rawQuery string, wantStatus int) tagPage {
	t.Helper()
	resp := doJSON(t, "GET", ts.URL+"/api/tags"+rawQuery, "")
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		t.Fatalf("GET /api/tags%s = %d, 期望 %d", rawQuery, resp.StatusCode, wantStatus)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}
	var page tagPage
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("解析 %s 响应失败: %v，原文 %s", rawQuery, err, body)
	}
	page.raw = string(body)
	return page
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

// TestTagListShape GET /api/tags 返回 {items,total}，字段齐全（前端标签色板直接消费）
func TestTagListShape(t *testing.T) {
	store := newMockTagStore()
	mustCreateTags(t, store, "工作")
	ts := setupTags(t, store)

	page := getTagList(t, ts, "", http.StatusOK)
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("total = %d, items = %d, 期望各 1", page.Total, len(page.Items))
	}
	for _, key := range []string{"id", "name", "bg", "fg", "createdAt"} {
		if _, ok := page.Items[0][key]; !ok {
			t.Fatalf("返回缺少字段 %q: %v", key, page.Items[0])
		}
	}
	if page.Items[0]["name"] != "工作" {
		t.Fatalf("name = %v, 期望 工作", page.Items[0]["name"])
	}
}

// TestTagListPaged limit/offset 分页，total 始终是匹配总数，越界返回空数组
func TestTagListPaged(t *testing.T) {
	store := newMockTagStore()
	mustCreateTags(t, store, "a5", "a3", "a1", "a4", "a2")
	ts := setupTags(t, store)

	// 不给参数：默认 limit=50，一次拿全
	page := getTagList(t, ts, "", http.StatusOK)
	if page.Total != 5 || !slices.Equal(page.names(), []string{"a1", "a2", "a3", "a4", "a5"}) {
		t.Fatalf("默认分页 = total %d %v, 期望 total 5 按 name 升序", page.Total, page.names())
	}

	// 第二页：offset=2&limit=2 → 第 3、4 条
	page = getTagList(t, ts, "?limit=2&offset=2", http.StatusOK)
	if page.Total != 5 || !slices.Equal(page.names(), []string{"a3", "a4"}) {
		t.Fatalf("分页 = total %d %v, 期望 total 5 [a3 a4]", page.Total, page.names())
	}

	// 尾页不足一页
	page = getTagList(t, ts, "?limit=2&offset=4", http.StatusOK)
	if page.Total != 5 || !slices.Equal(page.names(), []string{"a5"}) {
		t.Fatalf("尾页 = total %d %v, 期望 total 5 [a5]", page.Total, page.names())
	}

	// 越界：items 是 [] 而不是 null（前端直接 for 循环）
	page = getTagList(t, ts, "?limit=2&offset=10", http.StatusOK)
	if page.Total != 5 || len(page.Items) != 0 {
		t.Fatalf("越界分页 = total %d, items %d, 期望 5 / 0", page.Total, len(page.Items))
	}
	if !strings.Contains(page.raw, `"items":[]`) {
		t.Fatalf("越界分页 items 应为空数组，实际 %s", page.raw)
	}

	// limit/offset 传空串 = 走默认值
	page = getTagList(t, ts, "?limit=&offset=", http.StatusOK)
	if page.Total != 5 || len(page.Items) != 5 {
		t.Fatalf("空参数 = total %d, items %d, 期望 5 / 5", page.Total, len(page.Items))
	}
}

// TestTagListSearch q 名字包含匹配、大小写不敏感、% 与 _ 按字面匹配
func TestTagListSearch(t *testing.T) {
	store := newMockTagStore()
	mustCreateTags(t, store, "工作", "工作台", "BACKUP", "100%进度", "a_b", "axb")
	ts := setupTags(t, store)

	// 大小写不敏感
	page := getTagList(t, ts, "?q=backup", http.StatusOK)
	if page.Total != 1 || !slices.Equal(page.names(), []string{"BACKUP"}) {
		t.Fatalf("q=backup = total %d %v, 期望 total 1 [BACKUP]", page.Total, page.names())
	}

	// 名字包含匹配（不是前缀）
	page = getTagList(t, ts, "?q="+url.QueryEscape("工作"), http.StatusOK)
	if page.Total != 2 || !slices.Equal(page.names(), []string{"工作", "工作台"}) {
		t.Fatalf("q=工作 = total %d %v, 期望 total 2 [工作 工作台]", page.Total, page.names())
	}

	// % 当字面量，不是通配符
	page = getTagList(t, ts, "?q="+url.QueryEscape("%"), http.StatusOK)
	if page.Total != 1 || !slices.Equal(page.names(), []string{"100%进度"}) {
		t.Fatalf("q=%% = total %d %v, 期望 total 1 [100%%进度]", page.Total, page.names())
	}

	// _ 当字面量，不匹配 axb
	page = getTagList(t, ts, "?q=a_b", http.StatusOK)
	if page.Total != 1 || !slices.Equal(page.names(), []string{"a_b"}) {
		t.Fatalf("q=a_b = total %d %v, 期望 total 1 [a_b]", page.Total, page.names())
	}

	// 搜索 + 分页组合：total 是匹配总数，不是当页条数
	page = getTagList(t, ts, "?q="+url.QueryEscape("工作")+"&limit=1&offset=1", http.StatusOK)
	if page.Total != 2 || !slices.Equal(page.names(), []string{"工作台"}) {
		t.Fatalf("搜索分页 = total %d %v, 期望 total 2 [工作台]", page.Total, page.names())
	}

	// 搜不到 → 空数组
	page = getTagList(t, ts, "?q="+url.QueryEscape("不存在"), http.StatusOK)
	if page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("搜不到 = total %d, items %d, 期望 0 / 0", page.Total, len(page.Items))
	}
}

// TestTagListNames names 批量精确取名：trim 去空、未登记忽略、按 name 升序、优先于 q/limit/offset
func TestTagListNames(t *testing.T) {
	store := newMockTagStore()
	mustCreateTags(t, store, "工作", "工作台", "备份")
	ts := setupTags(t, store)

	// 乱序 + 前后空格 + 空项 + 未登记的名字
	page := getTagList(t, ts, "?names="+url.QueryEscape(" 工作台 , 工作 ,, 不存在 "), http.StatusOK)
	if page.Total != 2 || !slices.Equal(page.names(), []string{"工作", "工作台"}) {
		t.Fatalf("names = total %d %v, 期望 total 2 [工作 工作台]", page.Total, page.names())
	}

	// names 优先：q 与 limit/offset 被忽略（limit=1 也不会截断）
	page = getTagList(t, ts, "?names="+url.QueryEscape("工作,备份")+"&q=zzz&limit=1&offset=9", http.StatusOK)
	// 排序按 name 升序（UTF-8 字节序：备 < 工），与存储层 ORDER BY name 一致
	if page.Total != 2 || !slices.Equal(page.names(), []string{"备份", "工作"}) {
		t.Fatalf("names 优先 = total %d %v, 期望 total 2 [备份 工作]", page.Total, page.names())
	}
	// names 优先时非法 limit 也不报错
	page = getTagList(t, ts, "?names="+url.QueryEscape("备份")+"&limit=abc", http.StatusOK)
	if page.Total != 1 || !slices.Equal(page.names(), []string{"备份"}) {
		t.Fatalf("names + 非法 limit = total %d %v, 期望 total 1 [备份]", page.Total, page.names())
	}

	// 全是空项 → 200 + 空数组
	page = getTagList(t, ts, "?names="+url.QueryEscape(", ,"), http.StatusOK)
	if page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("空 names = total %d, items %d, 期望 0 / 0", page.Total, len(page.Items))
	}
}

// TestTagListBadParams 分页参数越界 400；names 超过 100 个 400
func TestTagListBadParams(t *testing.T) {
	store := newMockTagStore()
	mustCreateTags(t, store, "工作")
	ts := setupTags(t, store)

	for _, query := range []string{"?limit=0", "?limit=-1", "?limit=201", "?offset=-1", "?limit=abc", "?offset=x"} {
		if msg := doJSONErr(t, "GET", ts.URL+"/api/tags"+query, "", http.StatusBadRequest); msg != "分页参数不合法" {
			t.Fatalf("%s 响应 = %q, 期望 分页参数不合法", query, msg)
		}
	}

	// 边界：limit=200 合法
	getTagList(t, ts, "?limit=200", http.StatusOK)

	// 边界：100 个 names 合法，101 个 400
	names := make([]string, 100)
	for i := range names {
		names[i] = "n" + strconv.Itoa(i)
	}
	getTagList(t, ts, "?names="+url.QueryEscape(strings.Join(names, ",")), http.StatusOK)

	tooMany := append(slices.Clone(names), "n100")
	msg := doJSONErr(t, "GET", ts.URL+"/api/tags?names="+url.QueryEscape(strings.Join(tooMany, ",")), "", http.StatusBadRequest)
	if msg != "names 数量过多" {
		t.Fatalf("101 个 names 响应 = %q, 期望 names 数量过多", msg)
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
