package registry

import "testing"

// TestEscapeLike 搜索词里的 LIKE 通配符必须按字面转义。
// 这段逻辑只有真库 SQL 才会用到（api 的 httptest 走内存 mock），因此在包内单测兜住。
func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"":       "",
		"文档":     "文档",
		"steam":  "steam",
		"100%":   `100\%`,
		"a_b":    `a\_b`,
		`C:\tmp`: `C:\\tmp`,
		`\%_`:    `\\\%\_`,
	}
	for in, want := range cases {
		if got := escapeLike(in); got != want {
			t.Fatalf("escapeLike(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

// TestIsUUIDText parent_id 的 UUID 粗校验：合法形式放行，脏值按「查不到」处理（不送进 SQL）
func TestIsUUIDText(t *testing.T) {
	valid := []string{
		"00000000-0000-7000-8000-000000000000",
		"0193A9F0-1111-7ABC-8DEF-0123456789AB",
		"0193a9f011117abc8def0123456789ab",
	}
	for _, s := range valid {
		if !isUUIDText(s) {
			t.Fatalf("isUUIDText(%q) = false, 期望 true", s)
		}
	}

	invalid := []string{
		"",
		"abc",
		"not-a-uuid",
		"00000000-0000-7000-8000-00000000000",
		"00000000-0000-7000-8000-0000000000000",
		"00000000_0000_7000_8000_000000000000",
		"00000000-0000-7000-8000-00000000000g",
	}
	for _, s := range invalid {
		if isUUIDText(s) {
			t.Fatalf("isUUIDText(%q) = true, 期望 false", s)
		}
	}
}
