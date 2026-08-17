package model

import "testing"

func TestValidUsername(t *testing.T) {
	valid := []string{"admin", "ab", "timmy_2026", "a.b-c", "User1"}
	for _, s := range valid {
		if !ValidUsername(s) {
			t.Errorf("ValidUsername(%q) = false，期望通过", s)
		}
	}

	invalid := []string{
		"",
		"a",                                 // 太短
		"012345678901234567890123456789012", // 33 字节，超长
		"..",                                // 会拼进备份文件名，必须挡掉
		"../../tmp/x",                       // 路径穿越
		`a\b`,                               // Windows 分隔符
		".hidden",                           // 以点开头
		"_leading",                          // 首字符必须是字母或数字
		"has space",
		"中文名",
	}
	for _, s := range invalid {
		if ValidUsername(s) {
			t.Errorf("ValidUsername(%q) = true，期望拒绝", s)
		}
	}
}

func TestValidPassword(t *testing.T) {
	if ValidPassword("short") {
		t.Error("少于 8 字节应拒绝")
	}
	if !ValidPassword("12345678") {
		t.Error("刚好 8 字节应通过")
	}
	if !ValidPassword(string(make([]byte, 72))) {
		t.Error("刚好 72 字节应通过")
	}
	if ValidPassword(string(make([]byte, 73))) {
		t.Error("超过 72 字节应拒绝")
	}
}
