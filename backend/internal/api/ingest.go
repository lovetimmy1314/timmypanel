// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"timmypanel/internal/middleware"
	"timmypanel/internal/model"
	"timmypanel/internal/service"
)

// 这个文件实现「浏览器书签反向上传」（决策 026）：后端直连抓不动的站点
// （Cloudflare 按 TLS 指纹拦机器人，UA 伪装到不了那一层），让用户的浏览器
// 在自己已过反爬的会话里抓到图标字节，POST 回这个端点落盘。
//
// 鉴权不走会话 Cookie，走的是书签里内嵌的专属令牌 —— 书签是在别人的站点
// （chatgpt.com 之类）的上下文里执行的，跨站 POST 带不上 SameSite=Lax 的
// Cookie，而且也不该带：令牌泄露的爆炸半径只有「能提交收藏」这一件事。
//
// 因此 POST /api/v1/ingest 注册在 CSRF 组之外（书签发的是 simple request，
// 没有自定义头可设），并回 Access-Control-Allow-Origin: * 让书签 JS 读得到
// 结果（要拿 next 跳下一个待补站点）。放开 CORS 在这里是安全的：
// 没有任何 ambient 凭证会跟着请求走，令牌就是全部鉴权。

// ingestTokenPrefix 让令牌在日志和书签栏里一眼可辨，也方便误提交时排查。
const ingestTokenPrefix = "tpk_"

// maxIngestQueue 是「浏览器逐个补」队列的长度上限。队列在内存里，本意是
// 兜底批量补全的失败长尾，真到这个量级该考虑别的方式了。
const maxIngestQueue = 100

// ingestQueues 按 uid 存待补 URL 队列（canonical 形式）。进程内存即可：
// 队列是「这一轮补全」的临时状态，重启丢了只是少个便利，不丢数据。
type ingestQueues struct {
	mu sync.Mutex
	m  map[uint][]string
}

func (q *ingestQueues) set(uid uint, urls []string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.m == nil {
		q.m = map[uint][]string{}
	}
	if len(urls) == 0 {
		delete(q.m, uid)
		return
	}
	q.m[uid] = urls
}

func (q *ingestQueues) list(uid uint) []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]string, len(q.m[uid]))
	copy(out, q.m[uid])
	return out
}

// popCanonical 把刚提交的 URL 从队列里划掉，返回下一个待补地址（空串=队列空了）。
// 匹配用 canonicalURL：书签提交的是 location.href，和卡片 URL 常有
// 协议/www/末尾斜杠的差异，直接字符串比较会划不掉。
func (q *ingestQueues) popCanonical(uid uint, canon string) (next string, remaining int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	list := q.m[uid]
	out := list[:0]
	for _, u := range list {
		if canonicalURL(u) != canon {
			out = append(out, u)
		}
	}
	if len(out) == 0 {
		delete(q.m, uid)
		return "", 0
	}
	q.m[uid] = out
	return out[0], len(out)
}

// newIngestToken 生成令牌原文。32 字节随机数 hex 后 64 位，暴力枚举不现实，
// 再加上错误令牌按 IP 限流（见 handleIngest），不需要更复杂的结构。
func newIngestToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return ingestTokenPrefix + hex.EncodeToString(b), nil
}

// hashIngestToken 与 session 同理：库里只存 SHA-256，原文只存在于用户的书签里。
func hashIngestToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// lookupIngestUID 用令牌原文换用户 ID。令牌带前缀校验，免得把别的系统
// 误 POST 过来的字段当令牌查一遍库。
func (s *Server) lookupIngestUID(token string) (uint, bool) {
	if !strings.HasPrefix(token, ingestTokenPrefix) {
		return 0, false
	}
	var row model.IngestToken
	res := s.db.Where("token_hash = ?", hashIngestToken(token)).Limit(1).Find(&row)
	if res.Error != nil || res.RowsAffected == 0 {
		return 0, false
	}
	return row.UserID, true
}

// ---- 令牌管理（走会话，CSRF 组内）----

// handleGetIngestToken 只回答「有没有」，永不返回令牌原文 —— 原文只在
// 生成那一刻出现一次，之后泄露面就只有用户自己的书签。
func (s *Server) handleGetIngestToken(c *gin.Context) {
	var row model.IngestToken
	res := s.scoped(c).Limit(1).Find(&row)
	if res.Error != nil {
		serverError(c, res.Error)
		return
	}
	if res.RowsAffected == 0 {
		ok(c, gin.H{"exists": false})
		return
	}
	ok(c, gin.H{"exists": true, "createdAt": row.CreatedAt})
}

// handleCreateIngestToken 生成（或重新生成）令牌。重新生成即吊销旧令牌：
// 每个用户只有一行，原文只返回这一次。
func (s *Server) handleCreateIngestToken(c *gin.Context) {
	token, err := newIngestToken()
	if err != nil {
		serverError(c, err)
		return
	}
	uid := middleware.UserID(c)
	row := model.IngestToken{UserID: uid, TokenHash: hashIngestToken(token)}
	// uniqueIndex 在 user_id 上，先删后插比 upsert 直白，单行写入不需要事务。
	if err := s.db.Where("user_id = ?", uid).Delete(&model.IngestToken{}).Error; err != nil {
		serverError(c, err)
		return
	}
	if err := s.db.Create(&row).Error; err != nil {
		serverError(c, err)
		return
	}
	ok(c, gin.H{"token": token, "createdAt": row.CreatedAt})
}

func (s *Server) handleDeleteIngestToken(c *gin.Context) {
	if err := s.scoped(c).Delete(&model.IngestToken{}).Error; err != nil {
		serverError(c, err)
		return
	}
	ok(c, gin.H{"ok": true})
}

// handlePutIngestQueue 覆盖当前用户的「浏览器逐个补」队列。
// 前端在批量补全跑完后，把失败卡片的 URL 塞进来；书签每成功提交一个，
// handleIngest 会划掉它并返回下一个。
func (s *Server) handlePutIngestQueue(c *gin.Context) {
	var req struct {
		URLs []string `json:"urls"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求格式错误")
		return
	}
	urls := make([]string, 0, len(req.URLs))
	seen := map[string]bool{}
	for _, raw := range req.URLs {
		u, err := normalizeURL(raw)
		if err != nil || u == "" {
			continue
		}
		canon := canonicalURL(u)
		if seen[canon] {
			continue
		}
		seen[canon] = true
		urls = append(urls, u)
		if len(urls) >= maxIngestQueue {
			break
		}
	}
	s.queues.set(middleware.UserID(c), urls)
	ok(c, gin.H{"ok": true, "count": len(urls)})
}

// ---- 反向上传（走令牌，CSRF 组外）----

// ingestCORS 让书签 JS 能读到响应（要拿 next 跳下一个站点）。
// 只放在这两个端点上：令牌是唯一鉴权，没有 Cookie 会被跨站带走。
func ingestCORS(c *gin.Context) {
	h := c.Writer.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	h.Set("Access-Control-Allow-Headers", "Content-Type")
}

func (s *Server) handleIngestOptions(c *gin.Context) {
	ingestCORS(c)
	c.Status(204)
}

// handleIngest 接收浏览器书签提交的收藏。表单字段：
//   - token：书签里内嵌的令牌（必填）
//   - url：当前页地址（必填）
//   - title：当前页标题（可空，空则回退域名）
//   - icon：图标文件（可空，书签同源抓到的字节）
//   - iconUrl：图标地址（可空，书签没抓到字节时的后备 —— 有的站页面被拦
//     但图标 CDN 是开放的，服务端可以再试一次；出站走 s.fetcher，SSRF 防护不变）
//
// URL 已存在时按「补」处理：只填空的标题、按 shouldReplaceIcon 的口径换图标；
// 不存在则新建卡片，落到第一个分组。两种都返回 next —— 队列里下一个待补地址，
// 书签靠它实现「点一下、自动跳下一个、再点一下」的半自动逐个补。
func (s *Server) handleIngest(c *gin.Context) {
	ingestCORS(c)

	ip := c.ClientIP()
	if locked, _ := s.ingestLimiter.Locked(ip); locked {
		fail(c, 429, "尝试次数过多，请稍后再试")
		return
	}
	uid, valid := s.lookupIngestUID(strings.TrimSpace(c.PostForm("token")))
	if !valid {
		// 错误令牌按 IP 限流：令牌 64 位 hex 本身枚举不动，限流防的是
		// 有人拿泄露的旧令牌反复试探（吊销后重试也算失败）。
		s.ingestLimiter.Fail(ip)
		fail(c, 401, "令牌无效或已吊销")
		return
	}
	s.ingestLimiter.Reset(ip)

	u, err := normalizeURL(c.PostForm("url"))
	if err != nil || u == "" {
		badRequest(c, "网址无效，只支持 http/https")
		return
	}
	title := strings.TrimSpace(c.PostForm("title"))

	saved := s.ingestIcon(c, uid)

	site, created, err := s.upsertIngestSite(uid, u, title, saved)
	if err != nil {
		serverError(c, err)
		return
	}

	next, remaining := s.queues.popCanonical(uid, canonicalURL(u))
	ok(c, gin.H{
		"ok":        true,
		"id":        site.ID,
		"created":   created,
		"icon":      saved != nil,
		"next":      next,
		"remaining": remaining,
	})
}

// ingestIcon 尽力拿到图标并落盘：优先用书签同源抓到的字节，
// 没有再试服务端直连 iconUrl。拿不到不算失败 —— 收藏本身比图标重要。
func (s *Server) ingestIcon(c *gin.Context, uid uint) *service.SavedIcon {
	if file, err := c.FormFile("icon"); err == nil && file != nil {
		f, err := file.Open()
		if err != nil {
			return nil
		}
		defer f.Close()
		// +1 留给 SaveIconData 判超限，和 SaveIcon 的读法是同一个套路。
		data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
		if err != nil {
			return nil
		}
		if saved, err := s.fetcher.SaveIconData(s.cfg.UploadDir(), uid, data, ""); err == nil {
			s.recordIconUpload(uid, saved)
			return saved
		}
		return nil
	}
	if iconURL := strings.TrimSpace(c.PostForm("iconUrl")); iconURL != "" {
		if saved, err := s.fetcher.SaveIcon(s.cfg.UploadDir(), uid, iconURL); err == nil {
			s.recordIconUpload(uid, saved)
			return saved
		}
	}
	return nil
}

// upsertIngestSite 按 canonical URL 查已有卡片：有则补空标题/换图标，
// 没有则新建。返回的 created 让书签能告诉用户这条是新增还是更新。
func (s *Server) upsertIngestSite(uid uint, u, title string, saved *service.SavedIcon) (*model.Site, bool, error) {
	canon := canonicalURL(u)

	// canonicalURL 是应用层规则（忽略协议/www/末尾斜杠），SQL 表达不了，
	// 拉到内存里比。单用户卡片量级是几百，这个扫法比维护冗余列简单。
	var sites []model.Site
	if err := s.db.Where("user_id = ?", uid).Find(&sites).Error; err != nil {
		return nil, false, err
	}
	for i := range sites {
		if canonicalURL(sites[i].URL) != canon {
			continue
		}
		updates := map[string]any{}
		// 标题为空、或还是域名兜底的，才算「没有标题」，和 backfillSite 同口径。
		if title != "" && (sites[i].Title == "" || sites[i].Title == hostOf(sites[i].URL)) {
			updates["title"] = title
		}
		// 浏览器专程提交回来的图标，url 类型的旧图标直接换掉；
		// 手动选的图标库/文字图标不动（shouldReplaceIcon 的语义就在这里）。
		if saved != nil && shouldReplaceIcon(&sites[i], true) {
			updates["icon_value"] = saved.Path
			updates["icon_type"] = model.IconTypeURL
		}
		if err := s.applyBackfill(uid, sites[i].ID, updates, nil); err != nil {
			return nil, false, err
		}
		return &sites[i], false, nil
	}

	// 新卡片落到第一个分组（没有分组就是未分组），排在该组末尾。
	var group model.Group
	gid := uint(0)
	if res := s.db.Where("user_id = ?", uid).Order("sort asc, id asc").Limit(1).Find(&group); res.Error != nil {
		return nil, false, res.Error
	} else if res.RowsAffected > 0 {
		gid = group.ID
	}
	var maxSort struct{ M int }
	s.db.Model(&model.Site{}).Where("user_id = ? AND group_id = ?", uid, gid).
		Select("COALESCE(MAX(sort), -1) as m").Scan(&maxSort)

	if title == "" {
		title = hostOf(u)
	}
	site := model.Site{
		UserID: uid, GroupID: gid, Title: title, URL: u,
		IconType: model.IconTypeURL, Sort: maxSort.M + 1,
	}
	if saved != nil {
		site.IconValue = saved.Path
	}
	if err := s.db.Create(&site).Error; err != nil {
		return nil, false, err
	}
	return &site, true, nil
}
