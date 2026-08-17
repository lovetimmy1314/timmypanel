package api

import (
	"strings"
	"testing"
	"unicode/utf8"

	"timmypanel/internal/model"
)

// isOwnUploadPath 决定一个值能不能被免登录端点吐出去，属于安全判定。
func TestIsOwnUploadPath(t *testing.T) {
	good := []string{
		"/uploads/1/icons/abc.png",
		"/uploads/42/bg/a-b_c.jpg",
	}
	for _, v := range good {
		if !isOwnUploadPath(v) {
			t.Errorf("isOwnUploadPath(%q) = false，期望放行", v)
		}
	}

	bad := []string{
		"",
		"/uploads/1/icons/../../config.yaml", // 穿越
		"/uploads/1/icons",                   // 少一段
		"/uploads/1/icons/a/b.png",           // 多一段
		"/uploads/x/icons/a.png",             // uid 不是数字
		"/uploads/1/other/a.png",             // kind 不认
		"/data/1/icons/a.png",                // 不在 uploads 下
		"uploads/1/icons/a.png",              // 少了开头的斜杠，形状对不上
		"https://evil.example/x.png",
		`/uploads/1/icons/a".png`,
	}
	for _, v := range bad {
		if isOwnUploadPath(v) {
			t.Errorf("isOwnUploadPath(%q) = true，期望拒绝", v)
		}
	}
}

func TestNormalizeSiteConfig(t *testing.T) {
	// 空标题回落到默认值，中文标题按字符截断。
	in := model.SiteConfigData{SiteTitle: "   "}
	normalizeSiteConfig(&in)
	if in.SiteTitle != model.DefaultSiteConfig().SiteTitle {
		t.Errorf("空标题应回落到默认值，得到 %q", in.SiteTitle)
	}

	in = model.SiteConfigData{SiteTitle: strings.Repeat("导", 50)}
	normalizeSiteConfig(&in)
	if !utf8.ValidString(in.SiteTitle) || utf8.RuneCountInString(in.SiteTitle) != 32 {
		t.Errorf("标题应截到 32 个字符且是合法 UTF-8，得到 %q", in.SiteTitle)
	}

	// 图标只收自家上传的图，外链和 data: 一律清掉。
	for _, v := range []string{"https://evil.example/f.ico", "data:image/svg+xml;base64,PHN2Zz4=", "javascript:alert(1)"} {
		in = model.SiteConfigData{SiteIcon: v}
		normalizeSiteConfig(&in)
		if in.SiteIcon != "" {
			t.Errorf("siteIcon=%q 应被清空，得到 %q", v, in.SiteIcon)
		}
	}
	in = model.SiteConfigData{SiteIcon: "/uploads/1/icons/a.png"}
	normalizeSiteConfig(&in)
	if in.SiteIcon != "/uploads/1/icons/a.png" {
		t.Errorf("自家上传的图标被清掉了: %q", in.SiteIcon)
	}

	// 登录背景比图标多放行 http(s)，但引号之类会越出 CSS 属性值的字符要挡掉。
	cases := map[string]string{
		"https://cdn.example/bg.jpg":  "https://cdn.example/bg.jpg",
		"/uploads/2/bg/x.jpg":         "/uploads/2/bg/x.jpg",
		`https://a.example/x.jpg");}`: "",
		"javascript:alert(1)":         "",
		"/etc/passwd":                 "",
	}
	for v, want := range cases {
		in = model.SiteConfigData{LoginBackground: v}
		normalizeSiteConfig(&in)
		if in.LoginBackground != want {
			t.Errorf("loginBackground=%q 归一化成 %q，期望 %q", v, in.LoginBackground, want)
		}
	}
}
