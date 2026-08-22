package service

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// isBlockedIP 是 SSRF 防护的判定核心（决策 003），挂在 DialContext 上，
// 放错一个网段就等于把内网当跳板对外开放。
func TestIsBlockedIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1",        // 环回
		"::1",              // IPv6 环回
		"::ffff:127.0.0.1", // IPv4-mapped 的环回，必须也认出来
		"10.0.0.5",         // 私网
		"172.16.3.4",       // 私网
		"172.31.255.255",   // 私网上界
		"192.168.1.1",      // 私网
		"169.254.169.254",  // 云厂商元数据端点
		"100.64.0.1",       // CGNAT
		"0.0.0.0",
		"fc00::1",   // ULA
		"fe80::1",   // 链路本地
		"224.0.0.1", // 组播
		"240.0.0.1", // 保留
	}
	for _, s := range blocked {
		if !isBlockedIP(net.ParseIP(s)) {
			t.Errorf("isBlockedIP(%s) = false，期望拦截", s)
		}
	}

	allowed := []string{
		"1.1.1.1",
		"8.8.8.8",
		"93.184.216.34",
		"172.32.0.1", // 刚好在 172.16/12 之外
		"2606:4700:4700::1111",
	}
	for _, s := range allowed {
		if isBlockedIP(net.ParseIP(s)) {
			t.Errorf("isBlockedIP(%s) = true，期望放行", s)
		}
	}

	// 解析失败的地址一律当作不可信。
	if !isBlockedIP(nil) {
		t.Error("isBlockedIP(nil) = false，期望拦截")
	}
}

// 决策 017：DialContext 必须自己解析并拨已校验的 IP。
// 只对 ParseIP 成功的值做检查，主机名整段会被跳过，rebinding 窗口还在。
func TestSafeDialAddrResolvesAndDialsIP(t *testing.T) {
	f := NewFetcher(false, time.Second)
	f.lookupIP = func(_ context.Context, host string) ([]net.IP, error) {
		if host != "safe.example" {
			t.Errorf("lookup 收到意外主机名 %q", host)
		}
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}

	got, err := f.safeDialAddr(context.Background(), "safe.example:443")
	if err != nil {
		t.Fatalf("公网解析应放行: %v", err)
	}
	if got != "93.184.216.34:443" {
		t.Errorf("应拨解析后的 IP:port，得到 %q", got)
	}
}

func TestSafeDialAddrRejectsHostnameResolvingToPrivate(t *testing.T) {
	f := NewFetcher(false, time.Second)
	f.lookupIP = func(_ context.Context, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("169.254.169.254")}, nil
	}
	if _, err := f.safeDialAddr(context.Background(), "meta.example:80"); err == nil {
		t.Fatal("解析到元数据地址应当拒绝")
	}

	// 多地址里混进一条内网也拒，不能挑公网那条拨。
	f.lookupIP = func(_ context.Context, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("1.1.1.1"), net.ParseIP("127.0.0.1")}, nil
	}
	if _, err := f.safeDialAddr(context.Background(), "mixed.example:443"); err == nil {
		t.Fatal("部分解析到环回应拒绝")
	}
}

func TestSafeDialAddrIPLiteral(t *testing.T) {
	f := NewFetcher(false, time.Second)
	if _, err := f.safeDialAddr(context.Background(), "127.0.0.1:80"); err == nil {
		t.Error("环回字面量应拒绝")
	}
	got, err := f.safeDialAddr(context.Background(), "1.1.1.1:443")
	if err != nil {
		t.Fatalf("公网字面量应放行: %v", err)
	}
	if got != "1.1.1.1:443" {
		t.Errorf("字面量应原样返回，得到 %q", got)
	}
}

func TestSafeDialAddrLookupFailure(t *testing.T) {
	f := NewFetcher(false, time.Second)
	f.lookupIP = func(_ context.Context, host string) ([]net.IP, error) {
		return nil, errors.New("nxdomain")
	}
	if _, err := f.safeDialAddr(context.Background(), "gone.example:443"); err == nil {
		t.Fatal("解析失败应返回错误")
	}
}

func TestPickDialIPPrefersIPv4(t *testing.T) {
	v6 := net.ParseIP("2606:4700:4700::1111")
	v4 := net.ParseIP("1.1.1.1")
	got := pickDialIP([]net.IP{v6, v4})
	if !got.Equal(v4) {
		t.Errorf("应优先 IPv4，得到 %v", got)
	}
	if got := pickDialIP([]net.IP{v6}); !got.Equal(v6) {
		t.Errorf("只有 IPv6 时应返回它，得到 %v", got)
	}
}

func eqStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 只留一个图标候选是老代码抓不到图标的主因：打分最高的那个 404，整条就失败了。
// 排序必须稳定 —— 同分时保持文档顺序，否则每次跑出来的优先级都不一样。
func TestRankIconCandidates(t *testing.T) {
	got := rankIconCandidates([]iconCandidate{
		{url: "/a.svg", score: 1},
		{url: "/b-32.png", score: 1},
		{url: "/favicon.ico", score: 2},
		{url: "/apple.png", score: 5},
		{url: "   ", score: 7}, // 只有空白的地址丢掉，别占尝试名额
	})
	want := []string{"/apple.png", "/favicon.ico", "/a.svg", "/b-32.png"}
	if !eqStrings(got, want) {
		t.Errorf("rankIconCandidates =\n  %v\n期望\n  %v", got, want)
	}

	// 同一个文件被 rel="icon" 和 rel="apple-touch-icon" 同时声明很常见。
	// 这时按它拿到的**最高分**排，并且只出现一次 —— 重复地址重试两遍没有意义。
	got = rankIconCandidates([]iconCandidate{
		{url: "/x.png", score: 1},
		{url: "/y.png", score: 3},
		{url: "/x.png", score: 5},
	})
	if want = []string{"/x.png", "/y.png"}; !eqStrings(got, want) {
		t.Errorf("重复地址去重 =\n  %v\n期望\n  %v", got, want)
	}

	if len(rankIconCandidates(nil)) != 0 {
		t.Error("空输入应返回空切片")
	}
}

func TestAbsoluteIconURLs(t *testing.T) {
	base, _ := url.Parse("https://example.com/a/b")
	got := absoluteIconURLs(base, []string{
		"/icon.png",                     // 根相对
		"sub/i.svg",                     // 相对当前路径
		"https://cdn.example.net/x.ico", // 绝对且跨域
		"data:image/png;base64,AAAA",    // 非 http(s)，白占尝试名额
		"",                              // 空串：解析出来会是页面自己
	})
	want := []string{
		"https://example.com/icon.png",
		"https://example.com/a/sub/i.svg",
		"https://cdn.example.net/x.ico",
		"https://example.com/favicon.ico", // 末尾兜底
	}
	if !eqStrings(got, want) {
		t.Errorf("absoluteIconURLs =\n  %v\n期望\n  %v", got, want)
	}

	// HTML 里已经声明了 /favicon.ico 时，兜底不该再追加一份。
	got = absoluteIconURLs(base, []string{"/favicon.ico"})
	if !eqStrings(got, []string{"https://example.com/favicon.ico"}) {
		t.Errorf("兜底重复了：%v", got)
	}

	// 一个候选都没有时，也要能靠兜底拿到 /favicon.ico。
	got = absoluteIconURLs(base, nil)
	if !eqStrings(got, []string{"https://example.com/favicon.ico"}) {
		t.Errorf("无候选时应只有兜底，实际 %v", got)
	}
}

// sniffIconExt 按内容判类型。响应头两个方向都错得起，所以不能信它。
func TestSniffIconExt(t *testing.T) {
	pad := strings.Repeat("\x00", 64)
	ok := []struct {
		name string
		data string
		want string
	}{
		{"png", "\x89PNG\x0D\x0A\x1A\x0A" + pad, ".png"},
		{"jpeg", "\xFF\xD8\xFF" + pad, ".jpg"},
		{"gif", "GIF89a" + pad, ".gif"},
		{"webp", "RIFF\x00\x00\x00\x00WEBPVP8 " + pad, ".webp"},
		{"ico", "\x00\x00\x01\x00" + pad, ".ico"},
		// 下面三条 Go 的嗅探表都不认（只有 ico/bmp/gif/webp/png/jpeg），
		// 靠 looksLikeSVG 兜住 —— 决策 004 明确要收 SVG favicon。
		{"svg 裸标签", `<svg xmlns="http://www.w3.org/2000/svg"><path/></svg>`, ".svg"},
		{"svg 带 XML 声明", `<?xml version="1.0"?><svg xmlns="x"></svg>`, ".svg"},
		{"svg 带 BOM 和换行", "\xEF\xBB\xBF\n  <svg xmlns=\"x\"></svg>", ".svg"},
	}
	for _, c := range ok {
		t.Run(c.name, func(t *testing.T) {
			ext, err := sniffIconExt([]byte(c.data), "application/octet-stream")
			if err != nil {
				t.Fatalf("意外失败: %v", err)
			}
			if ext != c.want {
				t.Errorf("ext = %q，期望 %q", ext, c.want)
			}
			if mimeByExt[ext] == "" {
				t.Errorf("%q 在 mimeByExt 里没有对应 mime，会写出空 mime 的 Upload 记录", ext)
			}
		})
	}

	bad := []struct {
		name string
		data string
		ct   string
	}{
		{"空内容", "", "image/png"},
		// 站点把 404 页面标成 image/png 是常见的，老代码会把这坨 HTML 存成 .png。
		{"HTML 冒充 png", "<!doctype html><html><body>404 Not Found</body></html>", "image/png"},
		// 只用 Contains 判 SVG 的话，这种内嵌了图标的 HTML 页会被误收。
		{"HTML 里内嵌 svg", `<!doctype html><html><body><svg viewBox="0 0 1 1"></svg></body></html>`, "image/svg+xml"},
		{"纯文本", "not an image at all", "image/x-icon"},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			if ext, err := sniffIconExt([]byte(c.data), c.ct); err == nil {
				t.Errorf("期望被拒，却判成了 %q", ext)
			}
		})
	}
}

// 用 claude.ai 实际的图标声明做回归：老代码只取 apple-touch-icon 一个，
// 它挂了就整条失败；现在要拿到全部候选，且 /favicon.ico 排在前面当兜底。
func TestParseHTMLMetaCollectsAllIcons(t *testing.T) {
	doc := `<html><head><title>Claude</title>
<meta name="description" content="Talk to Claude">
<link rel="icon" type="image/svg+xml" href="https://cdn.example.com/a.svg">
<link rel="icon" type="image/png" sizes="32x32" href="https://cdn.example.com/b32.png">
<link rel="icon" type="image/png" sizes="16x16" href="https://cdn.example.com/c16.png">
<link rel="shortcut icon" href="/favicon.ico">
<link rel="apple-touch-icon" href="https://cdn.example.com/d.png">
</head><body></body></html>`

	var meta Meta
	parseHTMLMeta(strings.NewReader(doc), &meta)

	if meta.Title != "Claude" {
		t.Errorf("Title = %q", meta.Title)
	}
	if meta.Description != "Talk to Claude" {
		t.Errorf("Description = %q", meta.Description)
	}
	want := []string{
		"https://cdn.example.com/d.png", // apple-touch-icon，5 分
		"/favicon.ico",                  // shortcut，2 分
		"https://cdn.example.com/a.svg", // 以下同为 1 分，按文档顺序
		"https://cdn.example.com/b32.png",
		"https://cdn.example.com/c16.png",
	}
	if !eqStrings(meta.IconURLs, want) {
		t.Errorf("IconURLs =\n  %v\n期望\n  %v", meta.IconURLs, want)
	}
}

// sizes 大的图标更清晰，应该排在小图前面。
func TestIconRelScorePrefersLargeAndAppleTouch(t *testing.T) {
	if iconRelScore("apple-touch-icon", "") <= iconRelScore("icon", "") {
		t.Error("apple-touch-icon 应优于普通 icon")
	}
	if iconRelScore("icon", "180x180") <= iconRelScore("icon", "16x16") {
		t.Error("大尺寸应优于小尺寸")
	}
}

// pngBytes 造一个指定大小、签名合法的假 PNG。
func pngBytes(n int) []byte {
	b := make([]byte, n)
	copy(b, []byte("\x89PNG\x0D\x0A\x1A\x0A"))
	return b
}

// 超限的图标必须报错。老写法用 LimitReader(body, maxIconBytes) 会**静默截断**，
// 把一个坏文件存进 uploads —— 卡片上是破图，日志里什么都没有。
func TestSaveIconRejectsOversized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes(maxIconBytes + 1024))
	}))
	defer srv.Close()

	// allowPrivate=true：httptest 跑在 127.0.0.1 上，否则会被 SSRF 防护挡掉。
	f := NewFetcher(true, 10*time.Second)
	dir := t.TempDir()
	if _, err := f.SaveIcon(dir, 1, srv.URL+"/big.png"); err == nil {
		t.Fatal("超过上限的图标应当报错，而不是截断后存成坏文件")
	}

	entries, _ := os.ReadDir(filepath.Join(dir, "1", "icons"))
	if len(entries) != 0 {
		t.Errorf("失败时不该留下文件，却写出了 %d 个", len(entries))
	}
}

// SaveIconData 是 ingest（浏览器反向上传）那条链路的落盘入口：
// 没有 URL 可下载，只有字节。它必须和 SaveIcon 按同一套规则判类型。
func TestSaveIconData(t *testing.T) {
	f := NewFetcher(true, 10*time.Second)

	// 正常 PNG：落盘、按内容哈希命名、mime 归一。
	saved, err := f.SaveIconData(t.TempDir(), 1, pngBytes(128), "")
	if err != nil {
		t.Fatalf("合法 PNG 被拒: %v", err)
	}
	if !strings.HasSuffix(saved.Path, ".png") || saved.Mime != "image/png" {
		t.Errorf("Path=%q Mime=%q，期望 .png / image/png", saved.Path, saved.Mime)
	}

	// SVG 字节（Go 嗅探表不认，靠 looksLikeSVG 兜住，决策 004）。
	svg, err := f.SaveIconData(t.TempDir(), 1, []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), "")
	if err != nil || !strings.HasSuffix(svg.Path, ".svg") {
		t.Errorf("SVG 字节应落盘为 .svg: err=%v path=%v", err, svg)
	}

	// HTML 404 页和超限内容都必须拒，不能存成破图。
	if _, err := f.SaveIconData(t.TempDir(), 1, []byte("<!doctype html><html></html>"), ""); err == nil {
		t.Error("HTML 内容应被拒")
	}
	if _, err := f.SaveIconData(t.TempDir(), 1, pngBytes(maxIconBytes+1), ""); err == nil {
		t.Error("超限内容应被拒")
	}
}

// 这条是本次改动的核心：首选图标挂了要能退到下一个候选，而不是整条失败。
func TestSaveFirstIconSkipsBadCandidates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apple.png": // 首选：HTML 里还写着，文件其实已经删了
			w.WriteHeader(http.StatusNotFound)
		case "/broken.png": // 次选：200，但响应头骗人，内容是 HTML 404 页
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("<!doctype html><html><body>not found</body></html>"))
		case "/favicon.ico": // 兜底：内容没问题，但 Content-Type 不规范
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(append([]byte("\x00\x00\x01\x00"), make([]byte, 64)...))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	f := NewFetcher(true, 10*time.Second)
	saved, err := f.SaveFirstIcon(t.TempDir(), 1, []string{
		srv.URL + "/apple.png",
		srv.URL + "/broken.png",
		srv.URL + "/favicon.ico",
	})
	if err != nil {
		t.Fatalf("应当退到第三个候选，却整条失败了：%v", err)
	}
	if !strings.HasSuffix(saved.Path, ".ico") {
		t.Errorf("Path = %q，期望以 .ico 结尾（说明按内容判了类型，没被 octet-stream 劝退）", saved.Path)
	}
	if saved.Mime != "image/x-icon" {
		t.Errorf("Mime = %q，期望 image/x-icon", saved.Mime)
	}

	// 全都不可用时要如实报错，不能返回一个空的 SavedIcon。
	if _, err := f.SaveFirstIcon(t.TempDir(), 1, []string{srv.URL + "/a", srv.URL + "/b"}); err == nil {
		t.Error("候选全失败时应返回错误")
	}
}

// ForEachLimited 的 worker 跑在请求 goroutine 之外，gin.Recovery() 够不着——
// 少了那层 recover，一条 panic 就是整个进程退出（这个测试会直接崩掉，
// 而不是报 FAIL）。其余任务必须照跑完。
func TestForEachLimitedRecoversPanic(t *testing.T) {
	var mu sync.Mutex
	done := make([]bool, 5)
	ForEachLimited(5, 2, func(i int) {
		if i == 2 {
			panic("故意的")
		}
		mu.Lock()
		defer mu.Unlock()
		done[i] = true
	})
	for i, ok := range done {
		if i == 2 {
			continue
		}
		if !ok {
			t.Errorf("第 %d 条没跑完，panic 把整批带偏了", i)
		}
	}
}
