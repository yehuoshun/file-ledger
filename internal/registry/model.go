// Package registry 台账模型与 PostgreSQL 存储
package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"file-ledger/internal/tag"
)

// Path 单条路径：type ∈ local|mount|virtual，path 为实际路径/指向
type Path struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

// Entry 台账条目（JSON 字段与前端 web/src/views/Cloud.vue 对齐）
// ParentID 为空 = 顶层；非空 = 挂在某个虚拟文件夹下（Windows 式层级）
type Entry struct {
	ID        string   `json:"id"`
	Alias     string   `json:"alias"`
	ParentID  *string  `json:"parentId,omitempty"`
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

// Validate 业务校验：别名必填唯一（唯一由 DB 约束兜底）、路径数组非空、每项类型合法且路径非空。
// 注意：文件夹（isDir=true）允许无路径——虚拟文件夹只是分类容器，路径可选。
func (e *Entry) Validate() error {
	e.Alias = strings.TrimSpace(e.Alias)
	if e.Alias == "" {
		return fmt.Errorf("别名不能为空")
	}

	if !e.IsDir && len(e.Paths) == 0 {
		return fmt.Errorf("文件条目至少需要一条路径")
	}

	// 标签归一：trim、丢空串、按首次出现去重、单个不超过 tag.MaxNameRunes 个字符
	normalized := make([]string, 0, len(e.Tags))
	seen := make(map[string]struct{}, len(e.Tags))
	for _, raw := range e.Tags {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if utf8.RuneCountInString(name) > tag.MaxNameRunes {
			return fmt.Errorf("标签 %q 过长（最长 %d 个字符）", name, tag.MaxNameRunes)
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}
	e.Tags = normalized

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
