// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package service

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

const (
	maxHTMLBytes = 2 << 20 // 页面最多读 2MB，够解析 <head> 了
	maxIconBytes = 1 << 20 // 图标最大 1MB
	maxRedirects = 5       // 限制重定向跳数。http→https→www→地区路径 很容易就三跳
	// maxIconAttempts 限制一个站点最多试几个图标候选。候选是按质量排过序的，
	// 前几个都失败基本就没戏了，再试下去只是让请求一直挂着。
	maxIconAttempts = 4

	// 抓取用主流浏览器 UA。原先自报 "Timmypanel/1.0" 的写法很礼貌，但 Cloudflare、
	// 知乎、B 站这类站点见到非浏览器 UA 会直接 403 或返回验证码页 —— 实测同一个地址
	// 不带 UA 就是 403。图标抓不到时用户无从判断是站点拦了还是程序坏了，
	// 所以这里选择伪装（决策 016）。
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

// Meta 是从目标页面抓到的元信息。
type Meta struct {
	Title       string
	Description string
	// IconURLs 是按质量降序排好的图标候选，末尾兜底 /favicon.ico。
	// 只留一个候选的话，HTML 里首选图标 404 就整条失败 —— 而那恰恰很常见
	// （站点改版删了 apple-touch-icon 却忘了改 HTML）。
	IconURLs []string
}

// lookupFunc 把主机名解析成 IP。生产用系统解析器；测试里换成假数据，
// 才能在不碰真 DNS 的情况下盯住 rebinding。
type lookupFunc func(ctx context.Context, host string) ([]net.IP, error)

func systemLookupIP(ctx context.Context, host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, "ip", host)
}

// Fetcher 负责抓取站点标题和图标。默认拒绝访问内网地址以防 SSRF：
// 这个接口是登录用户可控的任意 URL 出站请求，公网部署时不设防等于把
// 内网当跳板对外开放。
type Fetcher struct {
	client       *http.Client
	allowPrivate bool
	lookupIP     lookupFunc
	// dial 是通过校验之后真正建连的那一下。测试里换成假实现，用来断言
	// 拨出去的是 IP:port 而不是原来的主机名。
	dial func(ctx context.Context, network, addr string) (net.Conn, error)
}

// NewFetcher 构造抓取器。allowPrivate 为 true 时允许访问内网（自建服务场景手动开启）。
func NewFetcher(allowPrivate bool, timeout time.Duration) *Fetcher {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	f := &Fetcher{
		allowPrivate: allowPrivate,
		lookupIP:     systemLookupIP,
		dial:         dialer.DialContext,
	}
	transport := &http.Transport{
		// 必须自己解析再拨 IP：Go 交给 DialContext 的 addr 对域名是
		// "example.com:443"，不是解析后的 IP。只对 ParseIP 成功的值做检查，
		// 主机名整段会被跳过，决策 003 要防的 rebinding 窗口还在。见决策 017。
		DialContext:           f.dialContext,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		DisableKeepAlives:     false,
		MaxIdleConnsPerHost:   4,
	}
	f.client = &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("重定向次数过多")
			}
			if !allowPrivate {
				if err := checkHostAllowed(req.URL.Hostname()); err != nil {
					return err
				}
			}
			return nil
		},
	}
	return f
}

func (f *Fetcher) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	dialAddr, err := f.safeDialAddr(ctx, addr)
	if err != nil {
		return nil, err
	}
	return f.dial(ctx, network, dialAddr)
}

// safeDialAddr 把 Transport 给的 addr 收成可以安全拨的 IP:port。
// 主机名必须在这里解析并按解析结果拦截，再返回字面量 IP，不能把原主机名
// 交给底层 Dialer —— 那会再解析一次，rebinding 窗口还在。
func (f *Fetcher) safeDialAddr(ctx context.Context, addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}
	if ip := net.ParseIP(host); ip != nil {
		if !f.allowPrivate && isBlockedIP(ip) {
			return "", fmt.Errorf("拒绝访问内网地址 %s", host)
		}
		return addr, nil
	}
	if f.allowPrivate {
		return addr, nil
	}
	lookup := f.lookupIP
	if lookup == nil {
		lookup = systemLookupIP
	}
	ips, err := lookup(ctx, host)
	if err != nil {
		return "", fmt.Errorf("域名解析失败: %w", err)
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("域名解析失败: %s 没有地址", host)
	}
	// 任一地址落在黑名单就拒，不「挑一个公网拨」——部分解析成内网就是攻击面。
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return "", fmt.Errorf("%s 解析到内网地址，已拒绝", host)
		}
	}
	chosen := pickDialIP(ips)
	return net.JoinHostPort(chosen.String(), port), nil
}

// pickDialIP 在已经确认全是公网的地址里挑一个拨。IPv4 优先：个人站点抓
// favicon 不值得上 Happy Eyeballs，而不少目标 IPv6 比 IPv4 更容易不通。
func pickDialIP(ips []net.IP) net.IP {
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			return v4
		}
	}
	return ips[0]
}

// Fetch 抓取页面并解析标题、描述和图标地址。
func (f *Fetcher) Fetch(rawURL string) (*Meta, error) {
	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, errors.New("只支持 http/https")
	}
	if !f.allowPrivate {
		if err := checkHostAllowed(target.Hostname()); err != nil {
			return nil, err
		}
	}

	resp, err := f.get(rawURL, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("目标站点返回 %d", resp.StatusCode)
	}

	meta := &Meta{}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "html") {
		parseHTMLMeta(io.LimitReader(resp.Body, maxHTMLBytes), meta)
	}
	// base 取 resp.Request.URL 而不是入参：跟完重定向后，相对地址要相对**最终**地址解析。
	meta.IconURLs = absoluteIconURLs(resp.Request.URL, meta.IconURLs)
	// 有些站点的 <title> 是一整句 SEO 文案，截一下免得撑爆卡片和数据库。
	meta.Title = truncateRunes(meta.Title, 80)
	meta.Description = truncateRunes(meta.Description, 160)
	return meta, nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n])) + "…"
}

// iconCandidate 是 HTML 里的一个图标声明，score 越大越优先。
type iconCandidate struct {
	url   string
	score int
}

// rankIconCandidates 按得分降序排出候选顺序并去重。
// 用稳定排序：同分时保持文档顺序，这样结果可预测（也才好写测试）。
func rankIconCandidates(in []iconCandidate) []string {
	sorted := make([]iconCandidate, len(in))
	copy(sorted, in)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].score > sorted[j].score })

	out := make([]string, 0, len(sorted))
	seen := map[string]bool{}
	for _, c := range sorted {
		u := strings.TrimSpace(c.url)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}

// absoluteIconURLs 把候选转成绝对地址，丢掉不是 http(s) 的（data: 之类进不了
// SaveIcon，留着只会白占一个尝试名额），并在末尾补上 /favicon.ico 兜底 ——
// 站点没在 HTML 里声明图标、或声明的那些全挂了，它往往还在。
func absoluteIconURLs(base *url.URL, refs []string) []string {
	out := make([]string, 0, len(refs)+1)
	seen := map[string]bool{}
	add := func(raw string) {
		if raw == "" || seen[raw] {
			return
		}
		seen[raw] = true
		out = append(out, raw)
	}
	for _, ref := range refs {
		trimmed := strings.TrimSpace(ref)
		// 空串必须挡掉：ResolveReference("") 返回的是页面自己的地址，
		// 那会把 HTML 页面本身当成图标候选去下载。
		if trimmed == "" {
			continue
		}
		u, err := url.Parse(trimmed)
		if err != nil {
			continue
		}
		abs := base.ResolveReference(u)
		switch strings.ToLower(abs.Scheme) {
		case "http", "https":
			add(abs.String())
		}
	}
	if base.Host != "" {
		add(base.Scheme + "://" + base.Host + "/favicon.ico")
	}
	return out
}

// SavedIcon 描述一个已经落地的图标文件。调用方拿它去补 model.Upload 记录，
// 让自动抓来的图标和手动上传的在备份打包、账号清理时受到同等对待。
type SavedIcon struct {
	Path string // 形如 /uploads/1/icons/xxxx.png，可直接用于 <img>
	Mime string
	Size int64
}

// SaveIcon 下载图标存到 uploads/{uid}/icons/ 下。
// 本地化存储有两个好处：不依赖第三方 favicon 服务，也不会把你的收藏列表
// 通过图标请求泄露给别人。
func (f *Fetcher) SaveIcon(uploadRoot string, uid uint, iconURL string) (*SavedIcon, error) {
	if iconURL == "" {
		return nil, errors.New("图标地址为空")
	}
	u, err := url.Parse(iconURL)
	if err != nil {
		return nil, err
	}
	if !f.allowPrivate {
		if err := checkHostAllowed(u.Hostname()); err != nil {
			return nil, err
		}
	}
	resp, err := f.get(iconURL, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("图标返回 %d", resp.StatusCode)
	}

	// 多读一个字节：正好读满上限说明后面还有内容，属于超限而不是刚好读完。
	// 老写法是 LimitReader(body, maxIconBytes)，超限时**静默截断**，把一个坏文件
	// 存进 uploads —— 卡片上就是一张破图，而且没有任何错误可查。
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxIconBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxIconBytes {
		return nil, fmt.Errorf("图标超过 %dKB", maxIconBytes>>10)
	}

	headerCT := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	return f.SaveIconData(uploadRoot, uid, data, headerCT)
}

// SaveIconData 把已经拿到的图标字节按 SaveIcon 同一套规则落盘：按内容嗅探类型、
// 按内容哈希命名。给「浏览器抓到了字节、反向上传回来」的 ingest 端点用 ——
// 那条链路在服务器侧没有可下载的 URL，只有字节。headerCT 只用于拼错误信息。
func (f *Fetcher) SaveIconData(uploadRoot string, uid uint, data []byte, headerCT string) (*SavedIcon, error) {
	if len(data) > maxIconBytes {
		return nil, fmt.Errorf("图标超过 %dKB", maxIconBytes>>10)
	}
	ext, err := sniffIconExt(data, headerCT)
	if err != nil {
		return nil, err
	}

	sum := sha1.Sum(data)
	name := hex.EncodeToString(sum[:]) + ext
	dir := filepath.Join(uploadRoot, strconv.FormatUint(uint64(uid), 10), "icons")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		return nil, err
	}
	return &SavedIcon{
		Path: fmt.Sprintf("/uploads/%d/icons/%s", uid, name),
		// mime 按嗅探出的扩展名归一，不用响应头里那个可能不规范的值。
		Mime: mimeByExt[ext],
		Size: int64(len(data)),
	}, nil
}

// SaveFirstIcon 按顺序试候选，第一个成功的就返回。全失败时返回最后一个错误
// （候选已排过序，最后一个通常是 /favicon.ico，它的错误对用户最有解释力）。
func (f *Fetcher) SaveFirstIcon(uploadRoot string, uid uint, urls []string) (*SavedIcon, error) {
	if len(urls) == 0 {
		return nil, errors.New("没有可用的图标地址")
	}
	var lastErr error
	for i, u := range urls {
		if i >= maxIconAttempts {
			break
		}
		saved, err := f.SaveIcon(uploadRoot, uid, u)
		if err == nil {
			return saved, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// get 发一个请求。forImage 只影响 Accept 和 Sec-Fetch-Dest —— 抓页面和抓图标
// 在真实浏览器里本来就是两种请求，headers 对不上反而更像机器人。
func (f *Fetcher) get(rawURL string, forImage bool) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	// 不要手动设 Accept-Encoding：Go 的 Transport 只会对自己加的那个 gzip 头
	// 做透明解压，手动设了就得自己解，白白多一份出错的地方。
	if forImage {
		req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
		req.Header.Set("Sec-Fetch-Dest", "image")
		req.Header.Set("Sec-Fetch-Mode", "no-cors")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
	} else {
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Site", "none")
		req.Header.Set("Upgrade-Insecure-Requests", "1")
	}
	return f.client.Do(req)
}

// iconExtByMIME 把嗅探出的 mime 映射到落盘扩展名。SVG 不在这里 —— Go 的嗅探表
// 不认它，单独由 looksLikeSVG 判定。
var iconExtByMIME = map[string]string{
	"image/png":                ".png",
	"image/jpeg":               ".jpg",
	"image/webp":               ".webp",
	"image/gif":                ".gif",
	"image/x-icon":             ".ico",
	"image/vnd.microsoft.icon": ".ico",
}

// mimeByExt 用于把落盘扩展名反查回规范 mime，存进 model.Upload。
var mimeByExt = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".webp": "image/webp",
	".gif": "image/gif", ".ico": "image/x-icon", ".svg": "image/svg+xml",
}

// sniffIconExt 按下载到的**内容**判定图标类型，返回落盘扩展名。
// 不信任响应头，两个方向都错得起：有站点给 favicon 发 application/octet-stream
// （本该收却被拒），也有站点给 404 页面发 image/png（本该拒却被当图片存下来）。
// 这和 api/upload.go 对用户上传的处理是同一个思路：按文件实际内容判类型。
// headerCT 只用于拼错误信息。
// SniffStoredImage 按内容判定图片类型，给备份还原这类「用户给的文件」用。
// 和 SaveIcon 同一套规则：不信任文件名和声明的类型。
func SniffStoredImage(data []byte) (ext, mime string, err error) {
	ext, err = sniffIconExt(data, "")
	if err != nil {
		return "", "", err
	}
	return ext, mimeByExt[ext], nil
}

func sniffIconExt(data []byte, headerCT string) (string, error) {
	if len(data) == 0 {
		return "", errors.New("图标内容为空")
	}
	sniffed := strings.TrimSpace(strings.Split(http.DetectContentType(data), ";")[0])
	if ext, ok := iconExtByMIME[strings.ToLower(sniffed)]; ok {
		return ext, nil
	}
	// Go 的嗅探表只认 ico/bmp/gif/webp/png/jpeg，**不认 SVG**，而 SVG favicon 很常见
	// （vuejs.org、news.ycombinator.com 都是），决策 004 特意要收。所以单独判一次。
	if looksLikeSVG(data) {
		return ".svg", nil
	}
	return "", fmt.Errorf("不是支持的图片格式（嗅探为 %q，响应头 %q）", sniffed, headerCT)
}

// looksLikeSVG 判断内容是不是 SVG。要求**开头**就是 <svg / XML 声明 / SVG DOCTYPE，
// 不能只用 Contains —— 那样任何内嵌了 <svg> 的 HTML 页面（比如 404 页）都会被
// 当成图标存下来。
func looksLikeSVG(data []byte) bool {
	// 有些站点的 SVG 带 UTF-8 BOM（EF BB BF），不剥掉就匹配不到开头的 <svg。
	// 用字节切片剥而不是把 BOM 写进 TrimLeft 的字符集——源码里直接放那个字符，
	// Go 会报 illegal byte order mark。
	head := bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	head = bytes.TrimLeft(head, " \t\r\n")
	if len(head) > 1024 {
		head = head[:1024]
	}
	lower := bytes.ToLower(head)
	if bytes.HasPrefix(lower, []byte("<svg")) {
		return true
	}
	// 带 XML 声明或 DOCTYPE 的 SVG，<svg> 在后面几行。
	if bytes.HasPrefix(lower, []byte("<?xml")) || bytes.HasPrefix(lower, []byte("<!doctype svg")) {
		return bytes.Contains(lower, []byte("<svg"))
	}
	return false
}

// parseHTMLMeta 扫一遍 HTML，取标题、描述和**全部**图标候选。
// 循环里有多处 return（读完、出错、遇到 <body>），所以用 defer 统一收口。
func parseHTMLMeta(r io.Reader, meta *Meta) {
	var icons []iconCandidate
	defer func() { meta.IconURLs = rankIconCandidates(icons) }()

	z := html.NewTokenizer(r)
	for {
		switch z.Next() {
		case html.ErrorToken:
			return
		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			switch t.Data {
			case "title":
				if meta.Title == "" && z.Next() == html.TextToken {
					meta.Title = strings.TrimSpace(z.Token().Data)
				}
			case "meta":
				name := strings.ToLower(attr(t, "name"))
				prop := strings.ToLower(attr(t, "property"))
				content := strings.TrimSpace(attr(t, "content"))
				if content == "" {
					continue
				}
				switch {
				case prop == "og:title" && meta.Title == "":
					meta.Title = content
				case name == "description" || prop == "og:description":
					if meta.Description == "" {
						meta.Description = content
					}
				case name == "msapplication-tileimage":
					icons = append(icons, iconCandidate{url: content, score: 1})
				}
			case "link":
				rel := strings.ToLower(attr(t, "rel"))
				href := strings.TrimSpace(attr(t, "href"))
				if href == "" || !strings.Contains(rel, "icon") {
					continue
				}
				icons = append(icons, iconCandidate{url: href, score: iconRelScore(rel, attr(t, "sizes"))})
			case "body":
				// <head> 已经结束，没必要继续读了。
				return
			}
		}
	}
}

// iconRelScore 给图标候选打分，优先取尺寸大的、apple-touch-icon 这类清晰图标。
func iconRelScore(rel, sizes string) int {
	score := 1
	if strings.Contains(rel, "apple-touch-icon") {
		score = 5
	}
	if strings.Contains(rel, "shortcut") {
		score = 2
	}
	if s := strings.ToLower(sizes); s != "" {
		if idx := strings.Index(s, "x"); idx > 0 {
			if n, err := strconv.Atoi(s[:idx]); err == nil && n >= 64 {
				score += 3
			}
		}
	}
	return score
}

func attr(t html.Token, key string) string {
	for _, a := range t.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

// ---- SSRF 防护 ----

// blockedNets 是禁止出站访问的网段：环回、私网、链路本地、共享地址等。
var blockedNets = func() []*net.IPNet {
	cidrs := []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8",
		"169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24",
		"192.168.0.0/16", "198.18.0.0/15", "224.0.0.0/4", "240.0.0.0/4",
		"::1/128", "fc00::/7", "fe80::/10", "ff00::/8", "::/128",
	}
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, s := range cidrs {
		if _, n, err := net.ParseCIDR(s); err == nil {
			out = append(out, n)
		}
	}
	return out
}()

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	for _, n := range blockedNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// checkHostAllowed 在发请求前先解析域名做一次拦截，给出更友好的错误信息；
// 真正的兜底仍然是 DialContext 里的检查。
func checkHostAllowed(host string) error {
	if host == "" {
		return errors.New("地址缺少主机名")
	}
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return errors.New("拒绝访问本机地址")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("拒绝访问内网地址 %s", host)
		}
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("域名解析失败: %w", err)
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("%s 解析到内网地址，已拒绝", host)
		}
	}
	return nil
}

// ForEachLimited 以最多 limit 个并发跑 n 次 fn(i)，用于批量抓取时控制并发。
func ForEachLimited(n, limit int, fn func(i int)) {
	if limit <= 0 {
		limit = 1
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			fn(i)
		}(i)
	}
	wg.Wait()
}
