// file-ledger 入口
// 步骤1-2：HTTP 服务 :18765 + 台账 CRUD（PostgreSQL 存储）+ config.json 自动生成

package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"file-ledger/internal/api"
	"file-ledger/internal/db"
	"file-ledger/internal/logx"
	"file-ledger/internal/registry"
)

//go:embed web/dist
var webFS embed.FS

// DatabaseConfig 数据库连接配置（本机 PgSQL）
type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
}

// Config 运行配置
type Config struct {
	Host         string         `json:"host"`
	Port         int            `json:"port"`
	DefaultDrive string         `json:"default_drive"`
	ThumbCache   string         `json:"thumb_cache"`
	Database     DatabaseConfig `json:"database"`
}

func defaultConfig() Config {
	return Config{
		Host:         "127.0.0.1",
		Port:         18765,
		DefaultDrive: "Z:",
		ThumbCache:   "data/thumbs",
		Database: DatabaseConfig{
			Host:     "127.0.0.1",
			Port:     5432,
			User:     "postgres",
			Password: "root",
			DBName:   "file_ledger",
		},
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

// connString 拼接 pgx 连接串
func (d DatabaseConfig) connString() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		d.Host, d.Port, d.User, d.Password, d.DBName)
}

func main() {
	if exe, err := os.Executable(); err == nil {
		_ = os.Chdir(filepath.Dir(exe))
	}

	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. 确保数据库存在（连 admin 库检查/创建）
	adminConn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=disable",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password)
	if err := db.EnsureDatabase(ctx, adminConn, cfg.Database.DBName); err != nil {
		log.Fatalf("数据库初始化失败: %v\n请确认 PostgreSQL 已启动且 config.json 的 database 段正确", err)
	}

	// 2. 连接业务库 + 初始化表结构（幂等）
	conn, err := pgx.Connect(ctx, cfg.Database.connString())
	if err != nil {
		log.Fatalf("连接数据库 %s 失败: %v", cfg.Database.DBName, err)
	}
	defer conn.Close(ctx)
	if err := db.EnsureSchema(ctx, conn); err != nil {
		log.Fatalf("表结构初始化失败: %v", err)
	}
	logx.New("main").Info("database.ready", "dbname", cfg.Database.DBName)

	// 3. 挂路由
	store := registry.NewStore(conn)
	mux := http.NewServeMux()
	api.RegisterRegistryRoutes(mux, store)
	mux.Handle("/", spaHandler())

	// 请求日志中间件（只记 /api/*）
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	logx.New("main").Info("server.start", "addr", addr, "db", cfg.Database.DBName,
		"db_host", cfg.Database.Host, "db_port", cfg.Database.Port)

	if err := http.ListenAndServe(addr, api.WithLogging(mux)); err != nil {
		log.Fatal(err)
	}
}

// spaHandler 服务内嵌前端：命中文件返回文件，否则回退 index.html（history 路由刷新），/api/* 未命中返回 404。
func spaHandler() http.Handler {
	distFS, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		log.Fatalf("内嵌前端读取失败: %v", err)
	}
	fileServer := http.FileServer(http.FS(distFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if strings.HasPrefix(p, "api/") {
			writeNotFound(w)
			return
		}
		if _, err := fs.Stat(distFS, p); err != nil {
			r.URL.Path = "/" // SPA fallback：history 路由刷新不 404
		}
		fileServer.ServeHTTP(w, r)
	})
}

func writeNotFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"error":"接口不存在"}`))
}
