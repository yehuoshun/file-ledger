package main

import (
	"io/fs"
	"net/http/httptest"
	"strings"
	"testing"
)

// firstAsset 从内嵌 dist 里找一个真实存在的静态资源路径（用于静态命中测试）
func firstAsset(t *testing.T) string {
	t.Helper()
	var found string
	err := fs.WalkDir(webFS, "web/dist/assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".js") && found == "" {
			found = strings.TrimPrefix(path, "web/dist")
		}
		return nil
	})
	if err != nil || found == "" {
		t.Fatalf("内嵌 dist 里找不到 js 资源: %v", err)
	}
	return found
}

func TestSPAHandler(t *testing.T) {
	h := spaHandler()

	cases := []struct {
		name       string
		path       string
		wantStatus int
		wantCT     string // 期望 Content-Type 前缀；空 = 不检查
	}{
		{"根路径", "/", 200, "text/html"},
		{"history 路由 /cloud", "/cloud", 200, "text/html"},
		{"history 路由 /settings", "/settings", 200, "text/html"},
		{"静态资源", firstAsset(t), 200, "text/javascript"},
		{"API 未注册路径 404", "/api/unknown", 404, "application/json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", c.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != c.wantStatus {
				t.Fatalf("状态码 = %d, 期望 %d", rec.Code, c.wantStatus)
			}
			if c.wantCT != "" && !strings.HasPrefix(rec.Header().Get("Content-Type"), c.wantCT) {
				t.Fatalf("Content-Type = %q, 期望前缀 %q", rec.Header().Get("Content-Type"), c.wantCT)
			}
		})
	}
}
