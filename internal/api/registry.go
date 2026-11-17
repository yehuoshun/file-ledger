// Package api REST 处理器与路由
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"file-ledger/internal/logx"
	"file-ledger/internal/registry"
)

// RegistryStore 台账存储接口（api 依赖接口，便于单元测试 mock）
type RegistryStore interface {
	List(ctx context.Context) ([]registry.Entry, error)
	Create(ctx context.Context, e *registry.Entry) (*registry.Entry, error)
	Update(ctx context.Context, id string, e *registry.Entry) (*registry.Entry, error)
	Delete(ctx context.Context, id string) error
}

// TagRegistrar 标签登记接口：条目保存前把新标签补进 tag 字典（接口便于 mock；nil = 不登记）
type TagRegistrar interface {
	Ensure(ctx context.Context, names []string) error
}

// RegisterRegistryRoutes 挂载台账 CRUD 路由（Go 1.22+ 方法+路径模式）；
// tags 非 nil 时，POST/PUT 会先把条目标签登记进字典，再落库。
func RegisterRegistryRoutes(mux *http.ServeMux, store RegistryStore, tags TagRegistrar) {
	mux.HandleFunc("GET /api/registry", func(w http.ResponseWriter, r *http.Request) {
		items, err := store.List(r.Context())
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "查询台账失败", err)
			return
		}
		writeJSON(w, http.StatusOK, items)
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
