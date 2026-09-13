// file-ledger 入口
// 步骤1：HTTP 服务 :18765 + /api/registry 返回空数组 + 首页占位 + config.json 自动生成

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// Config 运行配置
type Config struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	DefaultDrive string `json:"default_drive"`
	DataFile     string `json:"data_file"`
	ThumbCache   string `json:"thumb_cache"`
}

func defaultConfig() Config {
	return Config{
		Host:         "127.0.0.1",
		Port:         18765,
		DefaultDrive: "Z:",
		DataFile:     "data/registry.json",
		ThumbCache:   "data/thumbs",
	}
}

func loadConfig() (Config, error) {
	cfg := defaultConfig()
	path := "config.json"
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			raw, _ := json.MarshalIndent(cfg, "", "  ")
			if werr := os.WriteFile(path, raw, 0o644); werr != nil {
				return cfg, fmt.Errorf("写默认配置失败: %w", werr)
			}
			return cfg, nil
		}
		return cfg, fmt.Errorf("读配置失败: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("解析配置失败: %w", err)
	}
	return cfg, nil
}

func main() {
	if exe, err := os.Executable(); err == nil {
		_ = os.Chdir(filepath.Dir(exe))
	}

	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/registry", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, []any{})
	})

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, "<h1>file-ledger</h1><p>Service running: <a href=\"/api/registry\">/api/registry</a></p>")
	})

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	log.Printf("file-ledger 启动: http://%s", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}