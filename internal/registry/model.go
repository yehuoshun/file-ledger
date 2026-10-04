// Package registry 台账模型与 PostgreSQL 存储
package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Path 单条路径：type ∈ local|mount|virtual，path 为实际路径/指向
type Path struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

// Entry 台账条目（JSON 字段与前端 web/src/views/Cloud.vue 对齐）
type Entry struct {
	ID        string   `json:"id"`
	Alias     string   `json:"alias"`
	IsDir     bool     `json:"isDir"`
	Size      string   `json:"size"`
	Note      string   `json:"note"`
	Tags      []string `json:"tags"`
	Paths     []Path   `json:"paths"`
	CreatedAt string   `json:"createdAt,omitempty"`
	UpdatedAt string   `json:"updatedAt,omitempty"`
}

// 错误定义
var (
	ErrNotFound   = errors.New("条目不存在")
	ErrAliasTaken = errors.New("别名已存在")
)

// Validate 业务校验：别名必填唯一（唯一由 DB 约束兜底）、路径数组非空、每项类型合法且路径非空
func (e *Entry) Validate() error {
	e.Alias = strings.TrimSpace(e.Alias)
	if e.Alias == "" {
		return fmt.Errorf("别名不能为空")
	}

	if len(e.Paths) == 0 {
		return fmt.Errorf("至少需要一条路径")
	}
	for i, p := range e.Paths {
		p.Type = strings.TrimSpace(p.Type)
		p.Path = strings.TrimSpace(p.Path)
		switch p.Type {
		case "local", "mount", "virtual":
		default:
			return fmt.Errorf("第 %d 条路径类型非法: %q（仅支持 local/mount/virtual）", i+1, p.Type)
		}
		if p.Path == "" {
			return fmt.Errorf("第 %d 条路径不能为空", i+1)
		}
		e.Paths[i] = p
	}
	return nil
}

// marshalJSONB 把切片转 jsonb 字节（入库用）
func marshalJSONB(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("序列化失败: %w", err)
	}
	return b, nil
}
