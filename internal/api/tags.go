package api

import (
	"context"
	"errors"
	"net/http"

	"file-ledger/internal/tag"
)

// TagStore 标签字典存储接口（api 依赖接口，便于单元测试 mock）
type TagStore interface {
	List(ctx context.Context) ([]tag.Tag, error)
	Create(ctx context.Context, t *tag.Tag) (*tag.Tag, error)
	Update(ctx context.Context, id string, t *tag.Tag) (*tag.Tag, error)
	Delete(ctx context.Context, id string) error
}

// tagBody 标签请求体：只收 name/bg/fg（id、createdAt 由服务端管理）
type tagBody struct {
	Name string `json:"name"`
	BG   string `json:"bg"`
	FG   string `json:"fg"`
}

// RegisterTagRoutes 挂载标签字典路由（Go 1.22+ 方法+路径模式）
func RegisterTagRoutes(mux *http.ServeMux, store TagStore) {
	mux.HandleFunc("GET /api/tags", func(w http.ResponseWriter, r *http.Request) {
		items, err := store.List(r.Context())
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "查询标签失败", err)
			return
		}
		writeJSON(w, http.StatusOK, items)
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
