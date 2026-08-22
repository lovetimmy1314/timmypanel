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

// 设置整块存一行 JSON 且每次进首页都全量拉回来，所以引擎清单必须有上限。
// icon 前端连编辑入口都没有，形状对不上直接清空。
func TestNormalizeSearchEngineLimits(t *testing.T) {
	in := model.DefaultSettings()
	in.Search.Engines = nil
	for i := 0; i < 100; i++ {
		in.Search.Engines = append(in.Search.Engines, model.SearchEngine{
			Name: "引擎", URL: "https://e.example.com/s?q=%s", Icon: "mdi:magnify",
		})
	}
	normalizeSettings(&in)
	if n := len(in.Search.Engines); n != maxSearchEngines {
		t.Errorf("引擎数 = %d，期望截到 %d", n, maxSearchEngines)
	}

	// 名字按字符截，URL 过长整条丢掉，icon 形状不对的清空
	in = model.DefaultSettings()
	in.Search.Engines = []model.SearchEngine{
		{Name: strings.Repeat("名", 80), URL: "https://a.example.com/s?q=%s", Icon: "mdi:google"},
		{Name: "太长", URL: "https://b.example.com/s?q=%s&pad=" + strings.Repeat("x", maxEngineURLBytes), Icon: ""},
		{Name: "脏图标", URL: "https://c.example.com/s?q=%s", Icon: "url(https://evil/x)"},
		{Name: "别的图标集", URL: "https://d.example.com/s?q=%s", Icon: "fa:github"},
	}
	normalizeSettings(&in)
	if len(in.Search.Engines) != 3 {
		t.Fatalf("过长 URL 那条应被丢掉，剩下 %d 条: %+v", len(in.Search.Engines), in.Search.Engines)
	}
	first := in.Search.Engines[0]
	if !utf8.ValidString(first.Name) || utf8.RuneCountInString(first.Name) != maxEngineNameRunes {
		t.Errorf("引擎名截断不对: %q", first.Name)
	}
	if first.Icon != "mdi:google" {
		t.Errorf("合法的 mdi 图标名被改动了: %q", first.Icon)
	}
	for _, e := range in.Search.Engines[1:] {
		if e.Icon != "" {
			t.Errorf("%q 的图标 %q 形状不对，应被清空", e.Name, e.Icon)
		}
	}
}
