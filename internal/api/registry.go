// Package api REST 处理器与路由
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"file-ledger/internal/logx"
	"file-ledger/internal/registry"
)

// 台账列表参数：GET /api/registry 一页最多 200 条（与 tags 的分页口径一致）
const (
	defaultEntryLimit = 50
	maxEntryLimit     = 200
)

// RegistryStore 台账存储接口（api 依赖接口，便于单元测试 mock）
type RegistryStore interface {
	List(ctx context.Context) ([]registry.Entry, error)
	ListPaged(ctx context.Context, params registry.ListParams) ([]registry.Entry, int, error)
	ListFolders(ctx context.Context) ([]registry.FolderRef, error)
	Create(ctx context.Context, e *registry.Entry) (*registry.Entry, error)
	Update(ctx context.Context, id string, e *registry.Entry) (*registry.Entry, error)
	Delete(ctx context.Context, id string) error
}

// TagRegistrar 标签登记接口：条目保存前把新标签补进 tag 字典（接口便于 mock；nil = 不登记）
type TagRegistrar interface {
	Ensure(ctx context.Context, names []string) error
}

// entryList 台账列表响应：items 为当页数据，total 为匹配总数（前端据此判断是否还有更多）
type entryList struct {
	Items []registry.Entry `json:"items"`
	Total int              `json:"total"`
}

// folderList 文件夹轻量列表响应：items 只带 id/alias/parentId，total 为文件夹总数
type folderList struct {
	Items []registry.FolderRef `json:"items"`
	Total int                  `json:"total"`
}

// RegisterRegistryRoutes 挂载台账 CRUD 路由（Go 1.22+ 方法+路径模式）；
// tags 非 nil 时，POST/PUT 会先把条目标签登记进字典，再落库。
func RegisterRegistryRoutes(mux *http.ServeMux, store RegistryStore, tags TagRegistrar) {
	// GET /api/registry：分页 + 搜索（云盘列表滚动加载）
	//   ?parent_id=<uuid> 缺省/空 = 顶层（parent_id IS NULL）；给 uuid = 该文件夹的直接子条目
	//                     （uuid 不存在不报错，返回空 items、total=0）
	//   ?q=<alias / note / tags 任一包含匹配，大小写不敏感，% _ \ 按字面>
	//   ?is_dir=true|false（缺省不过滤；其他值 400）
	//   ?limit=<默认 50，最大 200>&offset=<默认 0>
	// 响应恒为 {"items":[...],"total":N}（旧的全量 []Entry 形状已废弃）
	mux.HandleFunc("GET /api/registry", func(w http.ResponseWriter, r *http.Request) {
		params, ok := parseEntryListParams(w, r)
		if !ok {
			return
		}
		items, total, err := store.ListPaged(r.Context(), params)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "查询台账失败", err)
			return
		}
		if items == nil {
			items = []registry.Entry{}
		}
		writeJSON(w, http.StatusOK, entryList{Items: items, Total: total})
	})

	// GET /api/registry/folders：全部文件夹的轻量列表（前端建树/面包屑/子条目计数）
	// 只含 is_dir=true 的条目，扁平按 alias 升序；
	// 与 PUT/DELETE /api/registry/{id} 是不同方法+不同路径，路由不冲突
	mux.HandleFunc("GET /api/registry/folders", func(w http.ResponseWriter, r *http.Request) {
		items, err := store.ListFolders(r.Context())
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "查询文件夹列表失败", err)
			return
		}
		if items == nil {
			items = []registry.FolderRef{}
		}
		writeJSON(w, http.StatusOK, folderList{Items: items, Total: len(items)})
	})

	mux.HandleFunc("POST /api/registry", func(w http.ResponseWriter, r *http.Request) {
		var e registry.Entry
		if !decodeBody(w, r, &e) {
			return
		}
		if err := e.Validate(); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error(), nil)
			return
		}
		// 先登记标签再落库：字典写不进去就不保存条目，避免出现字典里没有的标签
		if err := ensureTags(r.Context(), tags, e.Tags); err != nil {
			writeError(w, r, http.StatusInternalServerError, "登记标签失败", err)
			return
		}
		created, err := store.Create(r.Context(), &e)
		if err != nil {
			writeStoreErr(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	})

	mux.HandleFunc("PUT /api/registry/{id}", func(w http.ResponseWriter, r *http.Request) {
		var e registry.Entry
		if !decodeBody(w, r, &e) {
			return
		}
		if err := e.Validate(); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error(), nil)
			return
		}
		if err := ensureTags(r.Context(), tags, e.Tags); err != nil {
			writeError(w, r, http.StatusInternalServerError, "登记标签失败", err)
			return
		}
		updated, err := store.Update(r.Context(), r.PathValue("id"), &e)
		if err != nil {
			writeStoreErr(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	})

	mux.HandleFunc("DELETE /api/registry/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := store.Delete(r.Context(), r.PathValue("id")); err != nil {
			writeStoreErr(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// parseEntryListParams 解析台账列表查询参数；参数不合法时已写好 400 响应。
// q/parent_id 做 trim，空串按「未传」处理（空 parent_id 与缺省同义：查顶层）。
func parseEntryListParams(w http.ResponseWriter, r *http.Request) (registry.ListParams, bool) {
	query := r.URL.Query()

	limit, offset, ok := parseEntryPage(w, r, query)
	if !ok {
		return registry.ListParams{}, false
	}
	params := registry.ListParams{
		Q:      strings.TrimSpace(query.Get("q")),
		Limit:  limit,
		Offset: offset,
	}

	if raw := strings.TrimSpace(query.Get("parent_id")); raw != "" {
		parentID := raw
		params.ParentID = &parentID
	}

	if raw := strings.TrimSpace(query.Get("is_dir")); raw != "" {
		switch raw {
		case "true":
			isDir := true
			params.IsDir = &isDir
		case "false":
			isDir := false
			params.IsDir = &isDir
		default:
			writeError(w, r, http.StatusBadRequest, "is_dir 参数不合法（仅支持 true/false）", nil)
			return registry.ListParams{}, false
		}
	}
	return params, true
}

// parseEntryPage 解析 limit/offset（空值走默认）；不合法时已写好 400 响应。
// 口径与 tags 的 parseTagPage 一致：limit ∈ [1,200]，offset ≥ 0。
func parseEntryPage(w http.ResponseWriter, r *http.Request, query url.Values) (int, int, bool) {
	limit, offset := defaultEntryLimit, 0
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
	if limit <= 0 || limit > maxEntryLimit || offset < 0 {
		writeError(w, r, http.StatusBadRequest, "分页参数不合法", nil)
		return 0, 0, false
	}
	return limit, offset, true
}

// ensureTags 保存条目之前把标签登记进字典（tags 为 nil 表示不登记，旧调用照常编译）
func ensureTags(ctx context.Context, tags TagRegistrar, names []string) error {
	if tags == nil {
		return nil
	}
	return tags.Ensure(ctx, names)
}

// decodeBody 解析 JSON 请求体；失败时已写好 400 响应
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, r, http.StatusBadRequest, "请求体不是合法 JSON", err)
		return false
	}
	return true
}

// writeStoreErr 把存储层错误映射为 HTTP 响应
func writeStoreErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, registry.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "条目不存在", nil)
	case errors.Is(err, registry.ErrAliasTaken):
		writeError(w, r, http.StatusConflict, "别名已存在", nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "存储操作失败", err)
	}
}

// writeJSON 统一 JSON 响应
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error("json.encode", "err", err)
	}
}

// writeError 统一错误 JSON：{"error": msg}；错误详情进日志（带 rid）
func writeError(w http.ResponseWriter, r *http.Request, status int, msg string, err error) {
	if err != nil {
		log.Error("handler.error", "rid", logx.RIDFrom(r.Context()), "path", r.URL.Path,
			"status", status, "err", err)
	}
	writeJSON(w, status, map[string]string{"error": msg})
}
