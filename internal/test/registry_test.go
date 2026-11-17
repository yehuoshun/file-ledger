// Package test 开发期自检测试（沙箱用，黑盒：只测导出 API）。
// 与主代码分目录存放，不影响日常浏览；go test ./... 仍会执行。
package test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sort"
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

// ListPaged 内存版分页：父级/关键字/类型过滤与真库同口径（文件夹优先 + alias 升序）
func (m *mockStore) ListPaged(_ context.Context, p registry.ListParams) ([]registry.Entry, int, error) {
	q := strings.ToLower(p.Q)
	matched := make([]registry.Entry, 0, len(m.items))
	for _, e := range m.items {
		if p.ParentID == nil || *p.ParentID == "" {
			if e.ParentID != nil && *e.ParentID != "" {
				continue
			}
		} else if e.ParentID == nil || *e.ParentID != *p.ParentID {
			continue
		}
		if p.IsDir != nil && e.IsDir != *p.IsDir {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(e.Alias), q) &&
			!strings.Contains(strings.ToLower(e.Note), q) && !tagsContain(e.Tags, q) {
			continue
		}
		matched = append(matched, e)
	}
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].IsDir != matched[j].IsDir {
			return matched[i].IsDir
		}
		return matched[i].Alias < matched[j].Alias
	})

	total := len(matched)
	start := p.Offset
	if start > total {
		start = total
	}
	end := total
	if p.Limit > 0 && start+p.Limit < end {
		end = start + p.Limit
	}
	page := make([]registry.Entry, 0, end-start)
	page = append(page, matched[start:end]...)
	return page, total, nil
}

// tagsContain 标签数组任一元素包含 needle（已小写）即命中，与真库 jsonb_array_elements_text 口径一致
func tagsContain(tags []string, needle string) bool {
	for _, name := range tags {
		if strings.Contains(strings.ToLower(name), needle) {
			return true
		}
	}
	return false
}

// ListFolders 内存版文件夹列表：只取 is_dir=true，按 alias 升序
func (m *mockStore) ListFolders(_ context.Context) ([]registry.FolderRef, error) {
	out := make([]registry.FolderRef, 0, len(m.items))
	for _, e := range m.items {
		if !e.IsDir {
			continue
		}
		out = append(out, registry.FolderRef{ID: e.ID, Alias: e.Alias, ParentID: e.ParentID})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Alias < out[j].Alias })
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

// postEntry 新增条目并返回服务端返回的条目（含 id）
func postEntry(t *testing.T, baseURL, body string) registry.Entry {
	t.Helper()
	resp := doJSON(t, "POST", baseURL+"/api/registry", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST = %d, 期望 201", resp.StatusCode)
	}
	var created registry.Entry
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("解析新增响应失败: %v", err)
	}
	return created
}

// listEntry 列表响应 {"items":[...],"total":N}
type listEntry struct {
	Items []registry.Entry `json:"items"`
	Total int              `json:"total"`
}

// folderRef 文件夹轻量列表响应里的单条
type folderRef struct {
	ID       string  `json:"id"`
	Alias    string  `json:"alias"`
	ParentID *string `json:"parentId"`
}

// getList 请求台账列表并断言 200，返回当页条目与 total
func getList(t *testing.T, rawURL string) ([]registry.Entry, int) {
	t.Helper()
	resp := doJSON(t, "GET", rawURL, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d, 期望 200", rawURL, resp.StatusCode)
	}
	var list listEntry
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("解析列表失败: %v", err)
	}
	return list.Items, list.Total
}

// aliasesOf 取条目别名序列，便于断言排序
func aliasesOf(items []registry.Entry) []string {
	out := make([]string, 0, len(items))
	for _, e := range items {
		out = append(out, e.Alias)
	}
	return out
}

func TestCRUDFlow(t *testing.T) {
	ts := setup(t, newMockStore())

	resp := doJSON(t, "GET", ts.URL+"/api/registry", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET 空列表 = %d, 期望 200", resp.StatusCode)
	}
	var empty listEntry
	if err := json.NewDecoder(resp.Body).Decode(&empty); err != nil {
		t.Fatalf("解析列表失败: %v", err)
	}
	resp.Body.Close()
	if len(empty.Items) != 0 || empty.Total != 0 {
		t.Fatalf("空列表 = %+v, 期望 items 空、total 0", empty)
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
	if len(reg.calls[0]) != len(want) || reg.calls[0][0] != want[0] || reg.calls[0][1] != want[1] {
		t.Fatalf("Ensure 收到 %v, 期望 %v", reg.calls[0], want)
	}
	if len(created.Tags) != len(want) || created.Tags[0] != want[0] || created.Tags[1] != want[1] {
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
	if len(reg.calls[1]) != 1 || reg.calls[1][0] != "重要" {
		t.Fatalf("PUT 后 Ensure 收到 %v, 期望 [重要]", reg.calls[1])
	}
}

// TestRegistryListPaging 分页 + 排序：默认 limit 50，文件夹优先，其次 alias 升序
func TestRegistryListPaging(t *testing.T) {
	ts := setup(t, newMockStore())

	postEntry(t, ts.URL, `{"alias":"zeta","isDir":false,"paths":[{"type":"local","path":"D:\\z"}]}`)
	postEntry(t, ts.URL, `{"alias":"alpha","isDir":false,"paths":[{"type":"local","path":"D:\\a"}]}`)
	postEntry(t, ts.URL, `{"alias":"box","isDir":true,"paths":[]}`)
	postEntry(t, ts.URL, `{"alias":"beta","isDir":false,"paths":[{"type":"local","path":"D:\\b"}]}`)

	items, total := getList(t, ts.URL+"/api/registry")
	if total != 4 || len(items) != 4 {
		t.Fatalf("默认页 total/len = %d/%d, 期望 4/4", total, len(items))
	}
	wantOrder := []string{"box", "alpha", "beta", "zeta"}
	if got := aliasesOf(items); !slices.Equal(got, wantOrder) {
		t.Fatalf("排序 = %v, 期望 %v（文件夹优先 + alias 升序）", got, wantOrder)
	}

	// 第二页：limit=2&offset=2 → beta/zeta，total 仍为匹配总数 4
	items, total = getList(t, ts.URL+"/api/registry?limit=2&offset=2")
	if total != 4 {
		t.Fatalf("第二页 total = %d, 期望 4", total)
	}
	if got := aliasesOf(items); !slices.Equal(got, []string{"beta", "zeta"}) {
		t.Fatalf("第二页 = %v, 期望 [beta zeta]", got)
	}

	// offset 超出范围 → 空页，total 不变
	items, total = getList(t, ts.URL+"/api/registry?offset=99")
	if total != 4 || len(items) != 0 {
		t.Fatalf("超范围页 total/len = %d/%d, 期望 4/0", total, len(items))
	}

	// limit 上限 200 合法
	if _, total = getList(t, ts.URL+"/api/registry?limit=200"); total != 4 {
		t.Fatalf("limit=200 total = %d, 期望 4", total)
	}
}

// TestRegistryListParent 顶层与子级：parent_id 缺省 = 顶层，给 uuid = 直接子条目，未知 uuid = 空
func TestRegistryListParent(t *testing.T) {
	ts := setup(t, newMockStore())

	folder := postEntry(t, ts.URL, `{"alias":"proj","isDir":true,"paths":[]}`)
	childFile := postEntry(t, ts.URL, `{"alias":"a-child","isDir":false,"parentId":"`+folder.ID+`","paths":[{"type":"local","path":"D:\\a"}]}`)
	postEntry(t, ts.URL, `{"alias":"z-child","isDir":true,"parentId":"`+folder.ID+`","paths":[]}`)
	postEntry(t, ts.URL, `{"alias":"top","isDir":false,"paths":[{"type":"local","path":"D:\\t"}]}`)

	// 缺省 / 空串 = 顶层：只有 proj 和 top
	for _, query := range []string{"", "?parent_id="} {
		items, total := getList(t, ts.URL+"/api/registry"+query)
		if total != 2 || len(items) != 2 {
			t.Fatalf("顶层%s total/len = %d/%d, 期望 2/2", query, total, len(items))
		}
		if got := aliasesOf(items); !slices.Equal(got, []string{"proj", "top"}) {
			t.Fatalf("顶层%s = %v, 期望 [proj top]", query, got)
		}
	}

	// 子级：两个子条目，文件夹 z-child 优先于文件 a-child
	items, total := getList(t, ts.URL+"/api/registry?parent_id="+folder.ID)
	if total != 2 {
		t.Fatalf("子级 total = %d, 期望 2", total)
	}
	if got := aliasesOf(items); !slices.Equal(got, []string{"z-child", "a-child"}) {
		t.Fatalf("子级 = %v, 期望 [z-child a-child]", got)
	}
	if items[1].ID != childFile.ID {
		t.Fatalf("子条目 id = %q, 期望 %q", items[1].ID, childFile.ID)
	}

	// 不存在的 uuid：不报错，空 items + total 0
	items, total = getList(t, ts.URL+"/api/registry?parent_id=00000000-0000-7000-8000-000000000000")
	if total != 0 || len(items) != 0 {
		t.Fatalf("未知 parent_id total/len = %d/%d, 期望 0/0", total, len(items))
	}

	// 非法 uuid 文本：同样按查不到处理（不 500）
	items, total = getList(t, ts.URL+"/api/registry?parent_id=not-a-uuid")
	if total != 0 || len(items) != 0 {
		t.Fatalf("非法 parent_id total/len = %d/%d, 期望 0/0", total, len(items))
	}
}

// TestRegistryListSearch q：alias / note / tags 包含匹配、大小写不敏感、通配符按字面
func TestRegistryListSearch(t *testing.T) {
	ts := setup(t, newMockStore())

	postEntry(t, ts.URL, `{"alias":"项目文档","note":"季度报告","isDir":true,"paths":[]}`)
	postEntry(t, ts.URL, `{"alias":"Game","note":"Steam 游戏","isDir":false,"paths":[{"type":"local","path":"D:\\g"}]}`)
	postEntry(t, ts.URL, `{"alias":"100_备份","note":"","isDir":true,"paths":[]}`)
	// 只靠标签才能命中：alias/note 都不含 archive 或 归档
	postEntry(t, ts.URL, `{"alias":"old-project","note":"历史","isDir":true,"tags":["Archive","归档"],"paths":[]}`)

	// 命中 alias
	items, total := getList(t, ts.URL+"/api/registry?q="+url.QueryEscape("文档"))
	if total != 1 || items[0].Alias != "项目文档" {
		t.Fatalf("q=文档 total/items = %d/%v, 期望命中 项目文档", total, aliasesOf(items))
	}

	// 命中 note，且大小写不敏感（steam → Steam）
	items, total = getList(t, ts.URL+"/api/registry?q=steam")
	if total != 1 || items[0].Alias != "Game" {
		t.Fatalf("q=steam total/items = %d/%v, 期望命中 Game（note 命中）", total, aliasesOf(items))
	}

	// 命中 tags（jsonb 数组任一元素），且大小写不敏感（archive → Archive）
	items, total = getList(t, ts.URL+"/api/registry?q=archive")
	if total != 1 || items[0].Alias != "old-project" {
		t.Fatalf("q=archive total/items = %d/%v, 期望命中 old-project（标签命中）", total, aliasesOf(items))
	}

	// 命中中文标签
	items, total = getList(t, ts.URL+"/api/registry?q="+url.QueryEscape("归档"))
	if total != 1 || items[0].Alias != "old-project" {
		t.Fatalf("q=归档 total/items = %d/%v, 期望命中 old-project（标签命中）", total, aliasesOf(items))
	}

	// 下划线按字面匹配（不当通配符）：q=_ 只命中 alias 含 _ 的
	items, total = getList(t, ts.URL+"/api/registry?q="+url.QueryEscape("_"))
	if total != 1 || items[0].Alias != "100_备份" {
		t.Fatalf("q=_ total/items = %d/%v, 期望只命中 100_备份", total, aliasesOf(items))
	}

	// 百分号按字面匹配：没有别名含 % → 0 条
	if _, total = getList(t, ts.URL+"/api/registry?q="+url.QueryEscape("%")); total != 0 {
		t.Fatalf("q=%% total = %d, 期望 0（通配符应按字面）", total)
	}

	// 空白 q 视为不过滤
	if _, total = getList(t, ts.URL+"/api/registry?q="+url.QueryEscape("   ")); total != 4 {
		t.Fatalf("q=空白 total = %d, 期望 4", total)
	}
}

// TestRegistryListIsDir is_dir 过滤：true/false，非法值 400
func TestRegistryListIsDir(t *testing.T) {
	ts := setup(t, newMockStore())

	postEntry(t, ts.URL, `{"alias":"folder","isDir":true,"paths":[]}`)
	postEntry(t, ts.URL, `{"alias":"file","isDir":false,"paths":[{"type":"local","path":"D:\\f"}]}`)

	items, total := getList(t, ts.URL+"/api/registry?is_dir=true")
	if total != 1 || items[0].Alias != "folder" || !items[0].IsDir {
		t.Fatalf("is_dir=true total/items = %d/%v, 期望只有 folder", total, aliasesOf(items))
	}
	items, total = getList(t, ts.URL+"/api/registry?is_dir=false")
	if total != 1 || items[0].Alias != "file" || items[0].IsDir {
		t.Fatalf("is_dir=false total/items = %d/%v, 期望只有 file", total, aliasesOf(items))
	}

	for _, bad := range []string{"yes", "1", "TRUE"} {
		resp := doJSON(t, "GET", ts.URL+"/api/registry?is_dir="+bad, "")
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("is_dir=%s = %d, 期望 400", bad, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// TestRegistryListBadParams 分页参数不合法统一 400 + {"error":"分页参数不合法"}
func TestRegistryListBadParams(t *testing.T) {
	ts := setup(t, newMockStore())

	for _, query := range []string{"limit=0", "limit=-1", "limit=201", "limit=abc", "offset=-1", "offset=abc"} {
		resp := doJSON(t, "GET", ts.URL+"/api/registry?"+query, "")
		if resp.StatusCode != http.StatusBadRequest {
			resp.Body.Close()
			t.Fatalf("%s = %d, 期望 400", query, resp.StatusCode)
		}
		var body map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			resp.Body.Close()
			t.Fatalf("%s 解析错误响应失败: %v", query, err)
		}
		resp.Body.Close()
		if body["error"] != "分页参数不合法" {
			t.Fatalf("%s error = %q, 期望 分页参数不合法", query, body["error"])
		}
	}
}

// TestRegistryFolders folders 轻量列表：只有文件夹、形状 id/alias/parentId、按 alias 升序
func TestRegistryFolders(t *testing.T) {
	ts := setup(t, newMockStore())

	parent := postEntry(t, ts.URL, `{"alias":"alpha-folder","isDir":true,"paths":[]}`)
	postEntry(t, ts.URL, `{"alias":"beta-folder","isDir":true,"parentId":"`+parent.ID+`","paths":[]}`)
	postEntry(t, ts.URL, `{"alias":"z-file.txt","isDir":false,"paths":[{"type":"local","path":"D:\\z"}]}`)

	resp := doJSON(t, "GET", ts.URL+"/api/registry/folders", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/registry/folders = %d, 期望 200", resp.StatusCode)
	}
	var list struct {
		Items []folderRef `json:"items"`
		Total int         `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		resp.Body.Close()
		t.Fatalf("解析文件夹列表失败: %v", err)
	}
	resp.Body.Close()

	if list.Total != 2 || len(list.Items) != 2 {
		t.Fatalf("folders total/len = %d/%d, 期望 2/2（不含文件）", list.Total, len(list.Items))
	}
	if list.Items[0].Alias != "alpha-folder" || list.Items[1].Alias != "beta-folder" {
		t.Fatalf("folders 排序 = %q,%q, 期望 alpha-folder,beta-folder", list.Items[0].Alias, list.Items[1].Alias)
	}
	if list.Items[0].ID == "" || list.Items[1].ID == "" {
		t.Fatalf("folders id 为空: %+v", list.Items)
	}
	if list.Items[0].ParentID != nil {
		t.Fatalf("顶层文件夹 parentId = %v, 期望 null", *list.Items[0].ParentID)
	}
	if list.Items[1].ParentID == nil || *list.Items[1].ParentID != parent.ID {
		t.Fatalf("子文件夹 parentId = %v, 期望 %q", list.Items[1].ParentID, parent.ID)
	}

	// 原始 JSON 必须显式带 parentId:null，前端靠它判断根级
	resp = doJSON(t, "GET", ts.URL+"/api/registry/folders", "")
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		t.Fatalf("读取原始响应失败: %v", err)
	}
	resp.Body.Close()
	if !strings.Contains(string(raw), `"parentId":null`) {
		t.Fatalf("响应里缺少 parentId:null: %s", raw)
	}
}
