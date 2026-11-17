// Package fsx 只读文件系统查询：步骤4「目录浏览」所需的目录列表能力。
// 本包只读：不创建、不改写、不删除任何文件，也不读取文件内容（只做目录项枚举）。
package fsx

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"file-ledger/internal/logx"
)

// log fsx 模块日志器（目录项 Info 失败等非致命问题记录在此）
var log = logx.New("fsx")

// DefaultLimit 单次列表最多返回的条目数；超出则截断 Entries，Total 仍为实际总数。
const DefaultLimit = 1000

// 导出错误：调用方用 errors.Is 判断，api 层据此映射 400/500。
var (
	// ErrInvalidPath 路径参数为空、非绝对路径或含 ".." 段（防穿越校验不通过）。
	ErrInvalidPath = errors.New("路径非法")
	// ErrNotExist 路径不存在，或其所在盘符/网络位置不在线。
	ErrNotExist = errors.New("路径不存在或盘符不在线")
)

// Entry 目录项（JSON 字段与前端 DirBrowser 对齐）。
// Ext 只对文件有意义（目录恒为空串），Size 对目录恒为 0。
type Entry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
	Ext     string `json:"ext"`
}

// Listing 目录列表结果。
// Path 为实际列出的目录（入参指向文件时为其所在目录）；Parent 为上级目录，
// 已在盘符根/UNC 共享根时为空串，前端据此停止回退。
type Listing struct {
	Path      string  `json:"path"`
	Parent    string  `json:"parent"`
	Entries   []Entry `json:"entries"`
	Total     int     `json:"total"`
	Truncated bool    `json:"truncated"`
}

// List 列出 path 指定目录的直接子项（最多 limit 条，limit <= 0 时用 DefaultLimit）。
// path 指向文件时自动改为列出其所在目录。
// 只读：不做任何写入，也不打开文件内容。
// 参数非法返回 ErrInvalidPath；路径不存在/盘符离线返回 ErrNotExist；其余为内部错误。
func List(path string, limit int) (*Listing, error) {
	clean, err := normalize(path)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = DefaultLimit
	}

	info, err := os.Stat(clean)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return nil, fmt.Errorf("读取 %q 状态失败: %w", clean, err)
		}
		// 不存在、非法名字、盘符离线等都归为「路径不可用」，api 层映射 400
		return nil, fmt.Errorf("%w: %s", ErrNotExist, clean)
	}
	dir := clean
	if !info.IsDir() {
		dir = filepath.Dir(clean) // 指向文件 → 列其所在目录
	}

	items, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrNotExist, dir)
		}
		return nil, fmt.Errorf("读取目录 %q 失败: %w", dir, err)
	}

	entries := make([]Entry, 0, len(items))
	for _, it := range items {
		entries = append(entries, toEntry(it))
	}
	sortEntries(entries)

	total := len(entries)
	truncated := false
	if total > limit {
		entries = entries[:limit]
		truncated = true
	}

	return &Listing{
		Path:      dir,
		Parent:    parentOf(dir),
		Entries:   entries,
		Total:     total,
		Truncated: truncated,
	}, nil
}

// normalize 校验并归一化入参：必须绝对路径、不含 ".." 段；返回 Clean 后的路径。
// 防穿越的三道关：绝对路径 → 无 .. 段 → Clean 后仍无 .. 段（防御性复查）。
func normalize(path string) (string, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return "", fmt.Errorf("%w: 路径为空", ErrInvalidPath)
	}
	// 容错：裸盘符 "D:" 补上分隔符归一化为盘符根（结果仍是绝对路径，不放开任何穿越口子）
	if len(p) == 2 && p[1] == ':' && isAlpha(p[0]) {
		p += string(filepath.Separator)
	}
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("%w: %q 不是绝对路径", ErrInvalidPath, path)
	}
	if hasDotDot(p) {
		return "", fmt.Errorf("%w: %q 含 .. 段", ErrInvalidPath, path)
	}
	clean := filepath.Clean(p)
	if hasDotDot(clean) {
		return "", fmt.Errorf("%w: %q 清理后仍含 .. 段", ErrInvalidPath, clean)
	}
	return clean, nil
}

// hasDotDot 判断路径里是否存在 ".." 段（按 / 与 \ 两种分隔符切分，兼容两种写法）。
func hasDotDot(p string) bool {
	for _, seg := range strings.FieldsFunc(p, isSeparator) {
		if seg == ".." {
			return true
		}
	}
	return false
}

// isSeparator 判断是否路径分隔符（Windows 上两种写法都可能出现在入参里）。
func isSeparator(r rune) bool {
	return r == '/' || r == '\\'
}

// isAlpha 判断是否 ASCII 字母（用于识别裸盘符）。
func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// toEntry 把 os.DirEntry 转成响应条目；Info 失败时退化为仅名字+类型，不影响整次列表。
func toEntry(d os.DirEntry) Entry {
	e := Entry{Name: d.Name(), IsDir: d.IsDir()}
	info, err := d.Info()
	if err != nil {
		log.Warn("entry.info", "name", e.Name, "err", err)
		return e
	}
	e.ModTime = info.ModTime().Format(time.RFC3339)
	if !e.IsDir {
		e.Size = info.Size()
		e.Ext = filepath.Ext(e.Name)
	}
	return e
}

// sortEntries 就地排序：目录优先，再按名字不区分大小写升序（同名不同大小写按原串兜底，保证确定性）。
func sortEntries(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		a, b := strings.ToLower(entries[i].Name), strings.ToLower(entries[j].Name)
		if a != b {
			return a < b
		}
		return entries[i].Name < entries[j].Name
	})
}

// parentOf 返回上级目录；已在根（盘符根、UNC 共享根）时返回空串，前端据此停止回退。
func parentOf(dir string) string {
	p := filepath.Dir(dir)
	if p == dir {
		return ""
	}
	return p
}
