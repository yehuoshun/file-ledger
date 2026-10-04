// Package api REST 处理器与路由
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"file-ledger/internal/registry"
)

// RegistryStore 台账存储接口（api 依赖接口，便于单元测试 mock）
type RegistryStore interface {
	List(ctx context.Context) ([]registry.Entry, error)
	Create(ctx context.Context, e *registry.Entry) (*registry.Entry, error)
	Update(ctx context.Context, id string, e *registry.Entry) (*registry.Entry, error)
	Delete(ctx context.Context, id string) error
}

// RegisterRegistryRoutes 挂载台账 CRUD 路由（Go 1.22+ 方法+路径模式）
func RegisterRegistryRoutes(mux *http.ServeMux, store RegistryStore) {
	mux.HandleFunc("GET /api/registry", func(w http.ResponseWriter, r *http.Request) {
		items, err := store.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "查询台账失败", err)
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
			writeError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		created, err := store.Create(r.Context(), &e)
		if err != nil {
			writeStoreErr(w, err)
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
			writeError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		updated, err := store.Update(r.Context(), r.PathValue("id"), &e)
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	})

	mux.HandleFunc("DELETE /api/registry/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := store.Delete(r.Context(), r.PathValue("id")); err != nil {
			writeStoreErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// decodeBody 解析 JSON 请求体；失败时已写好 400 响应
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "请求体不是合法 JSON", err)
		return false
	}
	return true
}

// writeStoreErr 把存储层错误映射为 HTTP 响应
func writeStoreErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, registry.ErrNotFound):
		writeError(w, http.StatusNotFound, "条目不存在", nil)
	case errors.Is(err, registry.ErrAliasTaken):
		writeError(w, http.StatusConflict, "别名已存在", nil)
	default:
		writeError(w, http.StatusInternalServerError, "存储操作失败", err)
	}
}

// writeJSON 统一 JSON 响应
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("写响应失败: %v", err)
	}
}

// writeError 统一错误 JSON：{"error": msg}
func writeError(w http.ResponseWriter, status int, msg string, err error) {
	if err != nil {
		log.Printf("%s: %v", msg, err)
	}
	writeJSON(w, status, map[string]string{"error": msg})
}
