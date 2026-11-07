package test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"file-ledger/internal/api"
	"file-ledger/internal/logx"
)

func TestWithLogging(t *testing.T) {
	var buf bytes.Buffer
	logx.SetOutput(&buf)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if logx.RIDFrom(r.Context()) == "" {
			t.Error("context 里没有 rid")
		}
		w.WriteHeader(http.StatusCreated)
	})

	ts := httptest.NewServer(api.WithLogging(inner))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/registry")
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	resp.Body.Close()

	line := buf.String()
	if !strings.Contains(line, "http.request") {
		t.Fatalf("日志缺少请求记录: %q", line)
	}
	if !strings.Contains(line, "status=201") {
		t.Fatalf("日志缺少状态码: %q", line)
	}
	if !strings.Contains(line, "rid=") {
		t.Fatalf("日志缺少 rid: %q", line)
	}
}

func TestWithLoggingSkipsStatic(t *testing.T) {
	var buf bytes.Buffer
	logx.SetOutput(&buf)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	ts := httptest.NewServer(api.WithLogging(inner))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	resp.Body.Close()
	if buf.Len() != 0 {
		t.Fatalf("静态请求不应记日志: %q", buf.String())
	}
}
