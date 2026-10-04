package test

import (
	"strings"
	"testing"

	"file-ledger/internal/registry"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		e    registry.Entry
		want string // 期望错误子串；空 = 期望通过
	}{
		{"合法多路径", registry.Entry{Alias: " 文档 ", Paths: []registry.Path{
			{Type: "local", Path: " D:\\docs "},
			{Type: "virtual", Path: "ali:docs"},
		}}, ""},
		{"别名空白", registry.Entry{Alias: "  ", Paths: []registry.Path{{Type: "local", Path: "D:\\x"}}}, "别名"},
		{"文件无路径", registry.Entry{Alias: "x", Paths: nil}, "至少"},
		{"虚拟文件夹无路径", registry.Entry{Alias: "游戏", IsDir: true, Paths: nil}, ""},
		{"类型非法", registry.Entry{Alias: "x", Paths: []registry.Path{{Type: "ftp", Path: "D:\\x"}}}, "类型非法"},
		{"路径空白", registry.Entry{Alias: "x", Paths: []registry.Path{{Type: "local", Path: " "}}}, "不能为空"},
		{"挂载类型合法", registry.Entry{Alias: "x", Paths: []registry.Path{{Type: "mount", Path: "Z:"}}}, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.e.Validate()
			if c.want == "" {
				if err != nil {
					t.Fatalf("期望通过, 实际报错: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("期望报错含 %q, 实际通过", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("错误 %q 不含 %q", err.Error(), c.want)
			}
		})
	}
}

func TestValidateTrims(t *testing.T) {
	e := registry.Entry{Alias: "  文档  ", Paths: []registry.Path{{Type: " local ", Path: " D:\\docs "}}}
	if err := e.Validate(); err != nil {
		t.Fatalf("Validate 失败: %v", err)
	}
	if e.Alias != "文档" {
		t.Fatalf("alias 未 trim: %q", e.Alias)
	}
	if e.Paths[0].Type != "local" || e.Paths[0].Path != "D:\\docs" {
		t.Fatalf("paths 未 trim: %+v", e.Paths[0])
	}
}
