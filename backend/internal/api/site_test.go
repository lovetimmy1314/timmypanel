package api

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"github.com", "https://github.com", false},
		{"https://github.com/a?b=1", "https://github.com/a?b=1", false},
		// 内网地址补 http，否则内外网切换里的链接会打不开
		{"192.168.1.5:3000", "http://192.168.1.5:3000", false},
		{"10.0.0.2", "http://10.0.0.2", false},
		{"localhost:8080", "http://localhost:8080", false},
		{"nas.local", "http://nas.local", false},
		{"javascript:alert(1)", "", true},
		{"ftp://example.com", "", true},
		{"file:///etc/passwd", "", true},
		{"", "", false},
	}
	for _, c := range cases {
		got, err := normalizeURL(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("normalizeURL(%q) 期望报错，实际返回 %q", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizeURL(%q) 意外报错: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("normalizeURL(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
}

func TestCanonicalURL(t *testing.T) {
	// 去重时这几组应该被认为是同一个站点
	same := [][2]string{
		{"https://github.com", "http://github.com"},
		{"https://www.github.com/", "https://github.com"},
		{"https://example.com/path/", "https://example.com/path"},
	}
	for _, p := range same {
		if canonicalURL(p[0]) != canonicalURL(p[1]) {
			t.Errorf("%q 和 %q 应视为同一 URL，实际 %q vs %q",
				p[0], p[1], canonicalURL(p[0]), canonicalURL(p[1]))
		}
	}
	if canonicalURL("https://example.com/a") == canonicalURL("https://example.com/b") {
		t.Error("不同路径不应被去重")
	}
}

func TestSanitizeIconValue(t *testing.T) {
	cases := []struct {
		iconType, in, want string
	}{
		{"url", "/uploads/1/icons/a.png", "/uploads/1/icons/a.png"},
		{"url", "https://cdn.example.com/i.png", "https://cdn.example.com/i.png"},
		{"url", "data:image/png;base64,AAAA", "data:image/png;base64,AAAA"},
		// SVG 能内嵌脚本，必须挡掉
		{"url", "data:image/svg+xml;base64,AAAA", ""},
		{"url", "javascript:alert(1)", ""},
		{"iconify", "mdi:github", "mdi:github"},
		{"text", "GH", "GH"},
	}
	for _, c := range cases {
		if got := sanitizeIconValue(c.iconType, c.in); got != c.want {
			t.Errorf("sanitizeIconValue(%q, %q) = %q，期望 %q", c.iconType, c.in, got, c.want)
		}
	}
}

// 文字图标可以是中文。长度上限按字节算的话，一个 3 字节的字会被切成半截，
// 存进库就是坏 UTF-8，序列化时换成 U+FFFD，卡片上显示一个替换字符。
func TestSanitizeIconValueTruncatesByRune(t *testing.T) {
	for _, iconType := range []string{"text", "iconify"} {
		in := strings.Repeat("文", 100)
		got := sanitizeIconValue(iconType, in)
		if !utf8.ValidString(got) {
			t.Errorf("%s: 截断后不是合法 UTF-8: % x", iconType, got)
		}
		if n := utf8.RuneCountInString(got); n != 64 {
			t.Errorf("%s: 截断后 %d 个字符，期望 64", iconType, n)
		}
	}
	// 没超上限的原样放行，别顺手改了短值
	if got := sanitizeIconValue("text", "文"); got != "文" {
		t.Errorf("短值被改动了: %q", got)
	}
}

func TestSanitizeIconBg(t *testing.T) {
	ok := []string{"#0f172a", "hsl(210, 55%, 45%)", "rgba(30,58,138,0.5)"}
	for _, v := range ok {
		if got := sanitizeIconBg(v); got != v {
			t.Errorf("sanitizeIconBg(%q) = %q，期望原样放行", v, got)
		}
	}
	if got := sanitizeIconBg("  #abc  "); got != "#abc" {
		t.Errorf("sanitizeIconBg 应去掉首尾空白，得到 %q", got)
	}
	if got := sanitizeIconBg(""); got != "" {
		t.Errorf("空串应保留为空，得到 %q", got)
	}
	bad := []string{
		"url(https://evil.example.com/x.png)",
		"red; position:fixed",
		"#fff}</style><script>",
		"expression(alert(1))",
	}
	for _, v := range bad {
		if got := sanitizeIconBg(v); got != "" {
			t.Errorf("sanitizeIconBg(%q) = %q，期望丢掉", v, got)
		}
	}
	long := "#0123456789abcdef0123456789abcdef0" // 33 字节，超过列宽
	if got := sanitizeIconBg(long); got != "" {
		t.Errorf("超长值应丢掉，得到 %q", got)
	}
}
