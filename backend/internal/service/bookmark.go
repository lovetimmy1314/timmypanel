// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package service

import (
	"io"
	"strings"

	"golang.org/x/net/html"
)

// Bookmark 是从书签文件里解析出来的一条记录。
type Bookmark struct {
	Title  string
	URL    string
	Icon   string // 书签文件里内嵌的 base64 图标（data: URI），可能为空
	Folder string // 所在目录，多级用 / 拼接
}

// ParseBookmarks 解析 Chrome / Edge / Firefox 导出的 Netscape 书签 HTML。
// 这类文件结构是 <DL><DT><H3>目录名</H3><DL>...</DL><DT><A HREF>书签</A></DL>，
// 嵌套很深且经常缺闭合标签，所以直接顺序扫 token 并用一个栈跟踪当前目录。
func ParseBookmarks(r io.Reader) ([]Bookmark, error) {
	z := html.NewTokenizer(io.LimitReader(r, 20<<20))

	var (
		out       []Bookmark
		folders   []string // 目录栈
		depth     []int    // 每层目录对应的 <DL> 嵌套深度
		dlDepth   int
		pendingH3 bool
		current   *Bookmark
	)

	for {
		switch z.Next() {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				return out, nil
			}
			// 书签文件常有不规范之处，读到什么算什么。
			return out, nil

		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			switch strings.ToLower(t.Data) {
			case "dl":
				dlDepth++
			case "h3":
				pendingH3 = true
			case "a":
				href := strings.TrimSpace(attr(t, "href"))
				if href == "" || !isImportableLink(href) {
					current = nil
					continue
				}
				current = &Bookmark{
					URL:    href,
					Icon:   strings.TrimSpace(attr(t, "icon")),
					Folder: strings.Join(folders, "/"),
				}
			}

		case html.EndTagToken:
			t := z.Token()
			switch strings.ToLower(t.Data) {
			case "dl":
				// 一层 <DL> 结束，弹出在这一层打开的目录。
				for len(depth) > 0 && depth[len(depth)-1] >= dlDepth {
					depth = depth[:len(depth)-1]
					folders = folders[:len(folders)-1]
				}
				if dlDepth > 0 {
					dlDepth--
				}
			case "a":
				if current != nil {
					if current.Title == "" {
						current.Title = hostFromURL(current.URL)
					}
					out = append(out, *current)
					current = nil
				}
			}

		case html.TextToken:
			text := strings.TrimSpace(html.UnescapeString(z.Token().Data))
			if text == "" {
				continue
			}
			switch {
			case pendingH3:
				folders = append(folders, sanitizeFolder(text))
				depth = append(depth, dlDepth+1)
				pendingH3 = false
			case current != nil:
				current.Title = text
			}
		}
	}
}

// isImportableLink 过滤掉 javascript: 书签小工具和浏览器内部页。
func isImportableLink(href string) bool {
	l := strings.ToLower(href)
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://")
}

// sanitizeFolder 清掉目录名里的路径分隔符，避免破坏多级拼接。
// 长度按**字符**截：书签目录名常是中文，按字节切会把最后一个字切成半截，
// 存进库就是坏 UTF-8（序列化后显示成替换字符）。
func sanitizeFolder(s string) string {
	s = strings.ReplaceAll(s, "/", "-")
	if r := []rune(s); len(r) > 48 {
		s = string(r[:48])
	}
	return strings.TrimSpace(s)
}

func hostFromURL(raw string) string {
	s := raw
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	return s
}
