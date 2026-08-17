package api

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeFooterHTML(t *testing.T) {
	ok := `<div class="flex justify-center text-slate-300" style="margin-top:100px">Powered By <a href="https://github.com/hslr-s/sun-panel" target="_blank" class="ml-[5px]">Sun-Panel</a></div>`
	got := sanitizeFooterHTML(ok)
	if !strings.Contains(got, "Powered By") || !strings.Contains(got, "https://github.com/hslr-s/sun-panel") {
		t.Fatalf("合法示例被改坏了: %s", got)
	}
	if !strings.Contains(got, `rel="noopener noreferrer"`) {
		t.Fatalf("target=_blank 应强制 rel=noopener: %s", got)
	}

	cases := []struct {
		in      string
		banned  []string
		wantEmp bool
	}{
		{`<script>alert(1)</script>`, []string{"script", "alert"}, false},
		{`<a href="javascript:alert(1)">x</a>`, []string{"javascript", "href"}, false},
		{`<div style="background:url(https://evil.example/x)">x</div>`, []string{"url(", "evil"}, false},
		{`<img src=x onerror=alert(1)>`, []string{"img", "onerror"}, false},
		{`<a href="https://ok.example" onclick="alert(1)">x</a>`, []string{"onclick"}, false},
	}
	for _, tc := range cases {
		out := sanitizeFooterHTML(tc.in)
		for _, b := range tc.banned {
			if strings.Contains(strings.ToLower(out), strings.ToLower(b)) {
				t.Errorf("sanitizeFooterHTML(%q) = %q，仍含 %q", tc.in, out, b)
			}
		}
	}

	// 用户自己写的 rel 要留着，不能因为「没有 target=_blank」就整个丢掉。
	if got := sanitizeFooterHTML(`<a href="/help" rel="nofollow">帮助</a>`); !strings.Contains(got, `rel="nofollow"`) {
		t.Errorf("用户写的 rel 被丢了: %s", got)
	}
	// 留着的同时，_blank 仍然必须补 noopener。
	got = sanitizeFooterHTML(`<a href="https://ok.example" target="_blank" rel="nofollow">x</a>`)
	if !strings.Contains(got, "nofollow") || !strings.Contains(got, "noopener") {
		t.Errorf("_blank 链接的 rel 应同时含 nofollow 和 noopener: %s", got)
	}

	if got := sanitizeFooterHTML(""); got != "" {
		t.Errorf("空字符串应保持为空，得到 %q", got)
	}
	long := `<p>` + strings.Repeat("a", 4000) + `</p>`
	if len(sanitizeFooterHTML(long)) > maxFooterBytes+32 {
		t.Error("超长页脚没有被截断")
	}
	// 按字节截会切在多字节序列中间，截完必须是合法 UTF-8。
	zh := `<p>` + strings.Repeat("字", 2000) + `</p>`
	if got := sanitizeFooterHTML(zh); !utf8.ValidString(got) {
		t.Error("截断后的中文页脚不是合法 UTF-8")
	}
}
