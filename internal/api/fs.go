package api

import (
	"errors"
	"net/http"

	"file-ledger/internal/fsx"
)

// RegisterFSRoutes 挂载只读目录浏览路由：GET /api/fs/list?path=<绝对路径>。
// 纯只读接口，不含任何写入动作。
func RegisterFSRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/fs/list", func(w http.ResponseWriter, r *http.Request) {
		listing, err := fsx.List(r.URL.Query().Get("path"), fsx.DefaultLimit)
		if err != nil {
			writeFSErr(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, listing)
	})
}

// writeFSErr 把 fsx 错误映射为 HTTP 响应：路径类问题 400，其余 500。
func writeFSErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, fsx.ErrInvalidPath):
		writeError(w, r, http.StatusBadRequest, err.Error(), nil)
	case errors.Is(err, fsx.ErrNotExist):
		writeError(w, r, http.StatusBadRequest, "路径不存在或盘符不在线", nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "读取目录失败", err)
	}
}
