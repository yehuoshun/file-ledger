package api

import (
	"errors"
	"net/http"

	"file-ledger/internal/fsx"
)

// RegisterFSRoutes 挂载只读文件系统路由：
//
//	GET /api/fs/list?path=<绝对路径>   列目录
//	GET /api/fs/status?drives=D:,Z:   盘符在线/离线探测
//
// 纯只读接口，不含任何写入动作，也不改任何文件。
func RegisterFSRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/fs/list", func(w http.ResponseWriter, r *http.Request) {
		listing, err := fsx.List(r.URL.Query().Get("path"), fsx.DefaultLimit)
		if err != nil {
			writeFSErr(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, listing)
	})

	// GET /api/fs/status?drives=D:,Z:：盘符在线/离线探测（列表「离线」徽标用）
	//   drives 逗号分隔，元素接受 "D:" 或 "D"（统一归一为 "D:"），去重保序，最多 26 个；
	//          缺省/空/含非法元素一律 400 {"error":"盘符参数不合法"}
	//   响应恒为 {"items":[{"drive":"D:","online":true},...],"total":N}，
	//   逐盘并发探测、单盘 2s 超时；reason 仅离线条目出现（"盘符不在线" / "探测超时"）
	mux.HandleFunc("GET /api/fs/status", func(w http.ResponseWriter, r *http.Request) {
		drives, err := fsx.ParseDrives(r.URL.Query().Get("drives"))
		if err != nil {
			// 参数问题属预期分支，不写错误日志（详情随 400 返回）
			writeError(w, r, http.StatusBadRequest, "盘符参数不合法", nil)
			return
		}
		items := fsx.ProbeDrives(r.Context(), drives)
		writeJSON(w, http.StatusOK, driveStatusList{Items: items, Total: len(items)})
	})
}

// driveStatusList 盘符在线状态响应：items 与入参去重后一一对应，total 等于 len(items)。
type driveStatusList struct {
	Items []fsx.DriveStatus `json:"items"`
	Total int               `json:"total"`
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
