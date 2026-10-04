package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"file-ledger/internal/logx"
)

var log = logx.New("api")

// WithLogging 请求日志中间件：
// - 只记录 /api/* 请求（静态资源/页面不刷屏）
// - 生成 rid 写入 context（业务层 logx 可带出）
// - 记录方法/路径/状态/耗时
func WithLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rid := newRID()
		ctx := logx.WithRID(r.Context(), rid)
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r.WithContext(ctx))
		log.Info("http.request", "rid", rid, "method", r.Method, "path", r.URL.Path,
			"status", sw.status, "dur", time.Since(start).Round(time.Millisecond))
	})
}

// statusWriter 捕获响应状态码
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// newRID 6 字节随机请求号（16 hex）
func newRID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
