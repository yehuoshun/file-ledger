// Package tag 标签字典：标签由数据库管理（不再写死在前端），条目只存标签名。
package tag

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Tag 标签字典项（JSON 字段与前端标签色板对齐）
type Tag struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	BG        string `json:"bg"`
	FG        string `json:"fg"`
	CreatedAt string `json:"createdAt"`
}

// MaxNameRunes 标签名长度上限（按 rune 计，一个汉字算 1 个字符）
const MaxNameRunes = 32

// 错误定义：HTTP 层据此映射 400 / 404 / 409
var (
	ErrNotFound    = errors.New("标签不存在")
	ErrNameTaken   = errors.New("标签已存在")
	ErrInvalidName = errors.New("标签名不合法")
)

// palette 自动配色调色板（4 组 bg/fg）：新建标签没给颜色时按 tag 表行数轮转取用
var palette = [][2]string{
	{"#fae3c8", "#b35d0a"},
	{"#d9f0d9", "#2e7d32"},
	{"#fce4ec", "#c2185b"},
	{"#fee2e2", "#b71c1c"},
}

// NormalizeName 归一标签名并校验：trim 后非空、且不超过 MaxNameRunes 个字符。
// api 与 store 共用同一套规则，避免两处判定不一致。
func NormalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%w: 不能为空", ErrInvalidName)
	}
	if utf8.RuneCountInString(name) > MaxNameRunes {
		return "", fmt.Errorf("%w: 最长 %d 个字符", ErrInvalidName, MaxNameRunes)
	}
	return name, nil
}

// TruncateName 把超长标签名截断到 MaxNameRunes 个字符（批量登记用，不因脏数据中断条目保存）
func TruncateName(name string) string {
	if utf8.RuneCountInString(name) <= MaxNameRunes {
		return name
	}
	return string([]rune(name)[:MaxNameRunes])
}
