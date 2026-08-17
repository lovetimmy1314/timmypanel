package api

import (
	"strings"
	"testing"
	"unicode/utf8"

	"timmypanel/internal/model"
)

// 品牌文案按字符截断。按字节截会把中文切碎成坏 UTF-8，序列化后变成一串 U+FFFD。
func TestNormalizeLogoTextTruncation(t *testing.T) {
	in := model.DefaultSettings()
	in.Layout.LogoText = strings.Repeat("字", 50)
	normalizeSettings(&in)
	if !utf8.ValidString(in.Layout.LogoText) {
		t.Fatalf("截断后不是合法 UTF-8: %q", in.Layout.LogoText)
	}
	if n := utf8.RuneCountInString(in.Layout.LogoText); n != 32 {
		t.Fatalf("期望截到 32 个字符，得到 %d", n)
	}
	if in.Layout.SiteName != in.Layout.LogoText {
		t.Error("siteName 应与 logoText 保持同值")
	}
}

// safeCSSColor 决定用户提交的背景值能否直通 style，属于安全判定。
func TestSafeCSSColor(t *testing.T) {
	ok := []string{
		"#0f172a",
		"rgba(30,58,138,0.5)",
		"hsl(210, 55%, 45%)",
		"linear-gradient(135deg,#1e3a8a 0%,#0f172a 60%,#312e81 100%)",
		"repeating-linear-gradient(45deg, #222, #333 10px)",
		"conic-gradient(from 90deg, #fff, #000)",
	}
	for _, v := range ok {
		if !safeCSSColor(v) {
			t.Errorf("safeCSSColor(%q) = false，期望放行", v)
		}
	}

	bad := []string{
		"",
		"url(https://evil.example.com/track.png)", // 会让背景去外站发请求
		"red; position:fixed; top:0",              // 想越出属性值
		"#fff}</style><script>alert(1)</script>",
		"image-set('a.png')",
		"-moz-element(#x)",
		"expression(alert(1))",
		"var(--x)/**/",
	}
	for _, v := range bad {
		if safeCSSColor(v) {
			t.Errorf("safeCSSColor(%q) = true，期望拒绝", v)
		}
	}

	// 超长值直接拒，避免把整段 CSS 塞进设置里。
	long := make([]byte, 513)
	for i := range long {
		long[i] = 'a'
	}
	if safeCSSColor(string(long)) {
		t.Error("safeCSSColor(超长值) = true，期望拒绝")
	}
}

// 语言是白名单字段：前端只有 zh/en 两份词典，别的值必须回落到 zh，
// 否则界面上显示的是查不到的词条 key。
func TestNormalizeLanguage(t *testing.T) {
	cases := map[string]string{
		"zh":    "zh",
		"en":    "en",
		"":      "zh",
		"ja":    "zh",
		"zh-CN": "zh",
	}
	for in, want := range cases {
		s := model.DefaultSettings()
		s.Language = in
		normalizeSettings(&s)
		if s.Language != want {
			t.Errorf("normalizeSettings(language=%q) = %q，期望 %q", in, s.Language, want)
		}
	}
}

func TestNormalizeSearchDefaultFallsBack(t *testing.T) {
	in := model.DefaultSettings()
	in.Search.Default = "Google"
	in.Search.Engines = []model.SearchEngine{
		{Name: "Bing", URL: "https://www.bing.com/search?q=%s"},
	}
	normalizeSettings(&in)
	if in.Search.Default != "local" {
		t.Fatalf("删掉默认引擎后 default = %q，期望 local", in.Search.Default)
	}

	in = model.DefaultSettings()
	in.Search.Default = "Bing"
	in.Search.Engines = []model.SearchEngine{
		{Name: "Bing", URL: "https://www.bing.com/search?q=%s"},
	}
	normalizeSettings(&in)
	if in.Search.Default != "Bing" {
		t.Fatalf("仍在清单里的默认引擎被改成了 %q", in.Search.Default)
	}
}
