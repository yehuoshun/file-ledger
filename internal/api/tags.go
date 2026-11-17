package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"file-ledger/internal/tag"
)

// 标签列表参数：GET /api/tags 一页最多 200 条，批量取名一次最多 100 个
const (
	defaultTagLimit = 50
	maxTagLimit     = 200
	maxTagNames     = 100
)

// TagStore 标签字典存储接口（api 依赖接口，便于单元测试 mock）
type TagStore interface {
	List(ctx context.Context) ([]tag.Tag, error)
	ListPaged(ctx context.Context, q string, limit, offset int) ([]tag.Tag, int, error)
	ListByNames(ctx context.Context, names []string) ([]tag.Tag, error)
	Create(ctx context.Context, t *tag.Tag) (*tag.Tag, error)
	Update(ctx context.Context, id string, t *tag.Tag) (*tag.Tag, error)
	Delete(ctx context.Context, id string) error
}

// tagList 标签列表响应：items 为当页数据，total 为匹配总数（前端据此算页数/是否还有更多）
type tagList struct {
	Items []tag.Tag `json:"items"`
	Total int       `json:"total"`
}

// tagBody 标签请求体：只收 name/bg/fg（id、createdAt 由服务端管理）
type tagBody struct {
	Name string `json:"name"`
	BG   string `json:"bg"`
	FG   string `json:"fg"`
}

// RegisterTagRoutes 挂载标签字典路由（Go 1.22+ 方法+路径模式）
func RegisterTagRoutes(mux *http.ServeMux, store TagStore) {
	// GET /api/tags：
	//   ?q=<包含匹配，大小写不敏感>&limit=<默认 50，最大 200>&offset=<默认 0>
	//   ?names=a,b,c 批量精确取名（≤100 个，优先于 q/limit/offset）
	mux.HandleFunc("GET /api/tags", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		if query.Has("names") {
			names := splitTagNames(query.Get("names"))
			if len(names) > maxTagNames {
				writeError(w, r, http.StatusBadRequest, "names 数量过多", nil)
				return
			}
			items, err := store.ListByNames(r.Context(), names)
			if err != nil {
				writeError(w, r, http.StatusInternalServerError, "查询标签失败", err)
				return
			}
			writeJSON(w, http.StatusOK, tagList{Items: items, Total: len(items)})
			return
		}

		limit, offset, ok := parseTagPage(w, r, query)
		if !ok {
			return
		}
		items, total, err := store.ListPaged(r.Context(), strings.TrimSpace(query.Get("q")), limit, offset)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "查询标签失败", err)
			return
		}
		writeJSON(w, http.StatusOK, tagList{Items: items, Total: total})
	})

	mux.HandleFunc("POST /api/tags", func(w http.ResponseWriter, r *http.Request) {
		var body tagBody
		if !decodeBody(w, r, &body) {
			return
		}
		t, ok := parseTag(w, r, body)
		if !ok {
			return
		}
		created, err := store.Create(r.Context(), t)
		if err != nil {
			writeTagErr(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	})

	mux.HandleFunc("PUT /api/tags/{id}", func(w http.ResponseWriter, r *http.Request) {
		var body tagBody
		if !decodeBody(w, r, &body) {
			return
		}
		t, ok := parseTag(w, r, body)
		if !ok {
			return
		}
		updated, err := store.Update(r.Context(), r.PathValue("id"), t)
		if err != nil {
			writeTagErr(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	})

	mux.HandleFunc("DELETE /api/tags/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := store.Delete(r.Context(), r.PathValue("id")); err != nil {
			writeTagErr(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// parseTagPage 解析 limit/offset（空值走默认）；不合法时已写好 400 响应。
func parseTagPage(w http.ResponseWriter, r *http.Request, query url.Values) (int, int, bool) {
	limit, offset := defaultTagLimit, 0
	if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "分页参数不合法", err)
			return 0, 0, false
		}
		limit = n
	}
	if raw := strings.TrimSpace(query.Get("offset")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "分页参数不合法", err)
			return 0, 0, false
		}
		offset = n
	}
	if limit <= 0 || limit > maxTagLimit || offset < 0 {
		writeError(w, r, http.StatusBadRequest, "分页参数不合法", nil)
		return 0, 0, false
	}
	return limit, offset, true
}

// splitTagNames 逗号分隔的标签名：trim 后丢空（去重与排序交给 SQL）
func splitTagNames(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if name := strings.TrimSpace(part); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// parseTag 校验并组装标签；名字为空/超 32 字符时已写好 400 响应
func parseTag(w http.ResponseWriter, r *http.Request, body tagBody) (*tag.Tag, bool) {
	name, err := tag.NormalizeName(body.Name)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error(), nil)
		return nil, false
	}
	return &tag.Tag{Name: name, BG: body.BG, FG: body.FG}, true
}

// writeTagErr 把标签存储层错误映射为 HTTP 响应
func writeTagErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, tag.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "标签不存在", nil)
	case errors.Is(err, tag.ErrNameTaken):
		writeError(w, r, http.StatusConflict, "标签已存在", nil)
	case errors.Is(err, tag.ErrInvalidName):
		writeError(w, r, http.StatusBadRequest, err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "存储操作失败", err)
	}
}
