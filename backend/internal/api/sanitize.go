// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package api

import (
	"strings"

	"golang.org/x/net/html"
)

const maxFooterBytes = 2048

var footerAllowedTags = map[string]bool{
	"div": true, "span": true, "a": true, "p": true, "br": true, "strong": true, "em": true,
}

// 整棵丢掉、连子节点也不留。脚本和能换文档上下文的标签走这里。
var footerDropTags = map[string]bool{
	"script": true, "style": true, "iframe": true, "object": true, "embed": true,
	"link": true, "meta": true, "base": true, "form": true, "input": true,
	"textarea": true, "button": true, "svg": true, "math": true,
}

// sanitizeFooterHTML 把自定义页脚收成白名单 HTML。消毒必须发生在写入层，
// 前端 v-html 只渲染这里的产出；漏掉这一步，页脚就是 XSS。
func sanitizeFooterHTML(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if len(raw) > maxFooterBytes {
		// 上限按字节算（这是防膨胀，不是防字数），但切完要把末尾切碎的
		// 多字节序列丢掉，否则存进去的是坏 UTF-8。
		raw = strings.ToValidUTF8(raw[:maxFooterBytes], "")
	}
	// ParseFragment 要求 context 带 DataAtom，漏了会直接报 inconsistent Node。
	// 走完整文档解析再抽 body 子节点，少踩这个坑。
	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return ""
	}
	body := findHTMLBody(doc)
	if body == nil {
		return ""
	}
	var b strings.Builder
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		if cleaned := filterFooterNode(c); cleaned != nil {
			if err := html.Render(&b, cleaned); err != nil {
				return ""
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func findHTMLBody(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && strings.EqualFold(n.Data, "body") {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findHTMLBody(c); found != nil {
			return found
		}
	}
	return nil
}

func filterFooterNode(n *html.Node) *html.Node {
	switch n.Type {
	case html.TextNode:
		return &html.Node{Type: html.TextNode, Data: n.Data}
	case html.ElementNode:
		name := strings.ToLower(n.Data)
		if footerDropTags[name] {
			return nil
		}
		if !footerAllowedTags[name] {
			return unwrapFooterChildren(n)
		}
		out := &html.Node{Type: html.ElementNode, Data: name}
		out.Attr = filterFooterAttrs(name, n.Attr)
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if child := filterFooterNode(c); child != nil {
				out.AppendChild(child)
			}
		}
		return out
	default:
		return unwrapFooterChildren(n)
	}
}

func unwrapFooterChildren(n *html.Node) *html.Node {
	var kept []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if child := filterFooterNode(c); child != nil {
			kept = append(kept, child)
		}
	}
	if len(kept) == 0 {
		return nil
	}
	if len(kept) == 1 {
		return kept[0]
	}
	wrap := &html.Node{Type: html.ElementNode, Data: "span"}
	for _, c := range kept {
		wrap.AppendChild(c)
	}
	return wrap
}

func filterFooterAttrs(tag string, attrs []html.Attribute) []html.Attribute {
	var out []html.Attribute
	var href, target, rel string
	for _, a := range attrs {
		key := strings.ToLower(a.Key)
		val := strings.TrimSpace(a.Val)
		switch key {
		case "class":
			if safeFooterClass(val) {
				out = append(out, html.Attribute{Key: "class", Val: val})
			}
		case "style":
			if safeInlineStyle(val) {
				out = append(out, html.Attribute{Key: "style", Val: val})
			}
		case "href":
			if tag == "a" && safeFooterHref(val) {
				href = val
				out = append(out, html.Attribute{Key: "href", Val: val})
			}
		case "target":
			if tag == "a" && (val == "_blank" || val == "_self") {
				target = val
				out = append(out, html.Attribute{Key: "target", Val: val})
			}
		case "rel":
			if tag == "a" && safeRelTokens(val) {
				rel = val
			}
		}
	}
	// 链接一律带 rel：用户自己写了就留着，target=_blank 时再补上 noopener，
	// 避免打开的页面通过 window.opener 回指本站。
	if tag == "a" && href != "" {
		if target == "_blank" && !strings.Contains(strings.ToLower(rel), "noopener") {
			rel = strings.TrimSpace(rel + " noopener noreferrer")
		}
		if rel == "" {
			rel = "noopener noreferrer"
		}
		out = append(out, html.Attribute{Key: "rel", Val: rel})
	}
	return out
}

// safeRelTokens 只放行 rel 该有的形状：空格分隔的字母/连字符词。
func safeRelTokens(v string) bool {
	if v == "" || len(v) > 128 {
		return false
	}
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case r == ' ', r == '-':
		default:
			return false
		}
	}
	return true
}

func safeFooterClass(v string) bool {
	if v == "" || len(v) > 256 {
		return false
	}
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == ' ', r == '-', r == '_', r == '/', r == '[', r == ']', r == ':':
		default:
			return false
		}
	}
	return true
}

func safeFooterHref(v string) bool {
	if v == "" || len(v) > 512 {
		return false
	}
	lower := strings.ToLower(v)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return !strings.ContainsAny(v, " \t\r\n\"'<>")
	}
	// 相对路径可以，协议相对 //evil 和 javascript: 不行。
	return strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "//") &&
		!strings.ContainsAny(v, " \t\r\n\"'<>")
}

// safeInlineStyle 比 safeCSSColor 多放行 : ;，因为 style 属性本来就是声明列表。
// 分号只允许出现在属性值内部，url()/expression 仍然挡掉。
func safeInlineStyle(v string) bool {
	if v == "" || len(v) > 256 {
		return false
	}
	lower := strings.ToLower(v)
	for _, bad := range []string{"url", "expression", "image", "element", "attr", "javascript", "behavior", "import"} {
		if strings.Contains(lower, bad) {
			return false
		}
	}
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '#', r == '%', r == '.', r == ',', r == '(', r == ')', r == ' ', r == '-', r == ':', r == ';':
		default:
			return false
		}
	}
	return true
}
