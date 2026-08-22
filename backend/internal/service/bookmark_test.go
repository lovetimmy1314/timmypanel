package service

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// 一份精简过的 Chrome 导出书签，含多级目录、根级书签和 javascript 书签。
const sample = `<!DOCTYPE NETSCAPE-Bookmark-file-1>
<META HTTP-EQUIV="Content-Type" CONTENT="text/html; charset=UTF-8">
<TITLE>Bookmarks</TITLE>
<H1>Bookmarks</H1>
<DL><p>
    <DT><H3>书签栏</H3>
    <DL><p>
        <DT><A HREF="https://github.com" ICON="data:image/png;base64,AAA">GitHub</A>
        <DT><H3>开发/工具</H3>
        <DL><p>
            <DT><A HREF="https://stackoverflow.com">Stack Overflow</A>
            <DT><A HREF="javascript:void(0)">书签小工具</A>
        </DL><p>
        <DT><A HREF="https://news.ycombinator.com">Hacker News</A>
    </DL><p>
    <DT><A HREF="https://example.com">根级书签</A>
</DL><p>`

func TestParseBookmarks(t *testing.T) {
	items, err := ParseBookmarks(strings.NewReader(sample))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("期望 4 条书签（javascript: 应被过滤），实际 %d 条: %+v", len(items), items)
	}

	byURL := map[string]Bookmark{}
	for _, it := range items {
		byURL[it.URL] = it
	}

	if got := byURL["https://github.com"]; got.Title != "GitHub" || got.Folder != "书签栏" {
		t.Errorf("GitHub 解析错误: %+v", got)
	}
	// 目录名里的 / 会被换成 -，避免破坏多级路径拼接
	if got := byURL["https://stackoverflow.com"]; got.Folder != "书签栏/开发-工具" {
		t.Errorf("嵌套目录解析错误，实际 %q", got.Folder)
	}
	// 子目录的 </DL> 结束后应该回到父目录，而不是继续留在子目录里
	if got := byURL["https://news.ycombinator.com"]; got.Folder != "书签栏" {
		t.Errorf("子目录结束后未回退到父目录，实际 %q", got.Folder)
	}
	if got := byURL["https://example.com"]; got.Folder != "" {
		t.Errorf("根级书签不应有目录，实际 %q", got.Folder)
	}
	if byURL["https://github.com"].Icon == "" {
		t.Error("应保留书签里内嵌的图标")
	}
}

// 书签目录名常是中文。长度上限按字节算的话，48 字节切在第 17 个字中间，
// 存进库就是坏 UTF-8——和 sanitizeIconValue 是同一个坑。
func TestSanitizeFolderTruncatesByRune(t *testing.T) {
	got := sanitizeFolder(strings.Repeat("目", 100))
	if !utf8.ValidString(got) {
		t.Errorf("截断后不是合法 UTF-8: % x", got)
	}
	if n := utf8.RuneCountInString(got); n != 48 {
		t.Errorf("截断后 %d 个字符，期望 48", n)
	}
	// 分隔符替换和两端去空白仍然照旧
	if got := sanitizeFolder("  开发/工具  "); got != "开发-工具" {
		t.Errorf("sanitizeFolder 基本行为变了: %q", got)
	}
}

// TestIsBlockedIP 在 fetcher_test.go —— 它测的是 fetcher.go 的函数。
