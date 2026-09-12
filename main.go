// file-ledger 入口：配置加载 → 启动 HTTP 服务 → 打开浏览器
//
// 开发计划步骤 1（空壳）：
//   - HTTP 服务监听 127.0.0.1:18765
//   - GET /api/registry 返回空台账数组
//   - GET /healthz 存活探测
//   - 首页（后续批次由内嵌前端接管，本步骤先给个占位页）
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// Entry 台账条目（结构先定死，后续步骤 2 落盘复用）
type Entry struct {
	ID        string `json:"id"`
	Alias     string `json:"alias"`
	Type      string `json:"type"` // local / mount / virtual
	Root      string `json:"root"`
	RelPath   string `json:"rel_path"`
	JumpURL   string `json:"jump_url"`
	Note      string `json:"note"`
	ReadOnly  bool   `json:"read_only"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Config 运行配置（config.json，exe 旁）
type Config struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	DefaultDrive string `json:"default_drive"`
	DataFile     string `json:"data_file"`
	ThumbCache   string `json:"thumb_cache"`
	OpenBrowser  bool   `json:"open_browser"`
}

// defaultConfig 配置默认值；config.json 不存在时生成
func defaultConfig() Config {
	return Config{
		Host:         "127.0.0.1",
		Port:         18765,
		DefaultDrive: "Z:",
		DataFile:     "data/registry.json",
		ThumbCache:   "data/thumbs",
		OpenBrowser:  true,
	}
}

// loadConfig 读取 config.json；不存在则落盘默认配置
func loadConfig() (Config, error) {
	cfg := defaultConfig()
	path := "config.json"
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 首次运行：写默认配置，方便用户改端口/盘符
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
	// 工作目录 = exe 所在目录（保证双击运行时数据落在 exe 旁）
	if exe, err := os.Executable(); err == nil {
		_ = os.Chdir(filepath.Dir(exe))
	}

	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()

	// 台账列表：步骤 1 先返回空数组（步骤 2 接 JSON 存储）
	mux.HandleFunc("GET /api/registry", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, []Entry{})
	})

	// 存活探测：给前端/脚本判断后端是否在跑
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// 首页占位（批次 1-3 结束后由 go:embed 内嵌前端替换）
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, "<h1>文件台账 file-ledger</h1><p>服务运行中：<a href=\"/api/registry\">/api/registry</a></p>")
	})

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	log.Printf("file-ledger 服务启动: http://%s", addr)

	// 单实例复用：端口被占说明已有实例在跑，直接开浏览器指向它
	if cfg.OpenBrowser {
		go openBrowser(fmt.Sprintf("http://%s", addr))
	}

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

// writeJSON 统一 JSON 输出（错误格式 {"error": "中文消息"} 后续步骤统一）
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
