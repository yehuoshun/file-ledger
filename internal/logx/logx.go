// Package logx 轻量结构化日志（标准库实现，零第三方）
// 格式：时间 [级别] 模块 key=value ... 消息
// 目标：每行日志能回答「哪个请求、干了啥、成败、卡哪」，grep rid 即可串起一次操作全链路。
package logx

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Level 日志级别
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO "
	case LevelWarn:
		return "WARN "
	case LevelError:
		return "ERROR"
	default:
		return "?????"
	}
}

// ── 请求号（rid）：一次请求的日志用同一 rid 串起来 ──

type ridKey struct{}

// WithRID 把 rid 写入 context
func WithRID(ctx context.Context, rid string) context.Context {
	return context.WithValue(ctx, ridKey{}, rid)
}

// RIDFrom 从 context 取 rid；无则空串
func RIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(ridKey{}).(string); ok {
		return v
	}
	return ""
}

var (
	mu       sync.Mutex
	out      io.Writer = os.Stderr
	minLevel           = LevelInfo
)

// SetOutput 重定向输出（默认 stderr；测试用）
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	out = w
}

// SetLevel 设置最低输出级别（默认 INFO）
func SetLevel(l Level) {
	mu.Lock()
	defer mu.Unlock()
	minLevel = l
}

// Logger 带模块名的日志器。
// 用法：logx.New("api").Info("registry.create", "rid", rid, "alias", alias, "err", err)
// 键值对必须成对；单独出现的 error 值自动转为 err=xxx。
type Logger struct {
	module string
}

// New 创建模块日志器，如 logx.New("registry")
func New(module string) *Logger {
	return &Logger{module: module}
}

// Debug 输出 DEBUG 级别日志。
func (l *Logger) Debug(msg string, kv ...any) { l.log(LevelDebug, msg, kv...) }

// Info 输出 INFO 级别日志。
func (l *Logger) Info(msg string, kv ...any) { l.log(LevelInfo, msg, kv...) }

// Warn 输出 WARN 级别日志。
func (l *Logger) Warn(msg string, kv ...any) { l.log(LevelWarn, msg, kv...) }

// Error 输出 ERROR 级别日志。
func (l *Logger) Error(msg string, kv ...any) { l.log(LevelError, msg, kv...) }

func (l *Logger) log(lv Level, msg string, kv ...any) {
	mu.Lock()
	defer mu.Unlock()
	if lv < minLevel {
		return
	}
	var b strings.Builder
	write(&b, time.Now().Format("2006-01-02 15:04:05.000"))
	write(&b, fmt.Sprintf(" [%s] %-10s %s", lv, l.module, msg))
	for i := 0; i < len(kv); i++ {
		key := fmt.Sprintf("%v", kv[i])
		var val any = "?"
		if i+1 < len(kv) {
			val = kv[i+1]
			i++
		}
		if err, ok := val.(error); ok && err != nil {
			val = err.Error()
		}
		write(&b, " ")
		write(&b, key)
		write(&b, "=")
		write(&b, fmt.Sprintf("%v", val))
	}
	write(&b, "\n")
	_, _ = io.WriteString(out, b.String())
}

// write strings.Builder 写入封装：Builder 写入永不返回错误（签名如此），统一在此忽略
func write(b *strings.Builder, s string) {
	_, _ = b.WriteString(s)
}
