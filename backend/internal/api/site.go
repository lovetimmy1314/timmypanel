// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package api

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"timmypanel/internal/middleware"
	"timmypanel/internal/model"
	"timmypanel/internal/service"
)

func (s *Server) handleListSites(c *gin.Context) {
	var sites []model.Site
	if err := s.scoped(c).Order("sort asc, id asc").Find(&sites).Error; err != nil {
		serverError(c, err)
		return
	}
	ok(c, gin.H{"items": sites})
}

type siteReq struct {
	GroupID     uint   `json:"groupId"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	LanURL      string `json:"lanUrl"`
	Description string `json:"description"`
	IconType    string `json:"iconType"`
	IconValue   string `json:"iconValue"`
	IconBg      string `json:"iconBg"`
	OpenMode    string `json:"openMode"`
	Hidden      bool   `json:"hidden"`
}

// normalizeURL 补全协议并做基本校验。只接受 http/https，挡掉 javascript: 之类的伪协议。
func normalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if !strings.Contains(raw, "://") {
		// 内网地址（192.168.x.x:8080 这类）绝大多数只有 http，
		// 一律补 https 会让内外网切换里的内网链接打不开。
		if isPrivateHostPort(raw) {
			raw = "http://" + raw
		} else {
			raw = "https://" + raw
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return "", errUnsupportedScheme
	}
	if u.Host == "" {
		return "", errUnsupportedScheme
	}
	return u.String(), nil
}

// isPrivateHostPort 判断一个没带协议的地址是否指向内网/本机。
func isPrivateHostPort(raw string) bool {
	host := raw
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".local") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast())
}

var errUnsupportedScheme = &schemeError{}

type schemeError struct{}

func (e *schemeError) Error() string { return "只支持 http/https 地址" }

func (s *Server) applySiteReq(c *gin.Context, site *model.Site, req *siteReq) bool {
	u, err := normalizeURL(req.URL)
	if err != nil || u == "" {
		badRequest(c, "网址无效，只支持 http/https")
		return false
	}
	lan := ""
	if strings.TrimSpace(req.LanURL) != "" {
		lan, err = normalizeURL(req.LanURL)
		if err != nil {
			badRequest(c, "内网地址无效，只支持 http/https")
			return false
		}
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = hostOf(u)
	}
	if !s.ownsGroup(middleware.UserID(c), req.GroupID) {
		badRequest(c, "分组不存在")
		return false
	}
	iconType := req.IconType
	switch iconType {
	case model.IconTypeURL, model.IconTypeIconify, model.IconTypeText:
	default:
		iconType = model.IconTypeURL
	}
	openMode := req.OpenMode
	if openMode != "self" {
		openMode = "blank"
	}

	site.GroupID = req.GroupID
	site.Title = title
	site.URL = u
	site.LanURL = lan
	site.Description = strings.TrimSpace(req.Description)
	site.IconType = iconType
	site.IconValue = sanitizeIconValue(iconType, req.IconValue)
	site.IconBg = sanitizeIconBg(req.IconBg)
	site.OpenMode = openMode
	site.Hidden = req.Hidden
	return true
}

// sanitizeIconValue 限制图标字段的取值：本地上传路径、http(s) 外链、
// 小体积的位图 data URI，或者 iconify 图标名。其它一律丢弃 —— 尤其要挡掉
// data:image/svg+xml 和 javascript:，它们进了 <img>/<a> 就是 XSS 面。
func sanitizeIconValue(iconType, v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if iconType == model.IconTypeIconify || iconType == model.IconTypeText {
		if len(v) > 64 {
			return v[:64]
		}
		return v
	}
	lower := strings.ToLower(v)
	switch {
	case strings.HasPrefix(v, "/uploads/"):
		return v
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"):
		return v
	case strings.HasPrefix(lower, "data:image/png;base64,"),
		strings.HasPrefix(lower, "data:image/jpeg;base64,"),
		strings.HasPrefix(lower, "data:image/webp;base64,"),
		strings.HasPrefix(lower, "data:image/gif;base64,"):
		if len(v) > 32<<10 {
			return ""
		}
		return v
	default:
		return ""
	}
}

// sanitizeIconBg 收紧卡片色块背景。这个值会直通 SiteCard 的 style.background，
// 和 settings.background.value 同一类注入点。空串表示「没指定，用标题哈希色」。
// 列宽是 32（颜色选择器只会写出 #RRGGBB），更长的一律丢掉。
func sanitizeIconBg(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || len(v) > 32 || !safeCSSColor(v) {
		return ""
	}
	return v
}

func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

func (s *Server) handleCreateSite(c *gin.Context) {
	var req siteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求格式错误")
		return
	}
	uid := middleware.UserID(c)
	site := model.Site{UserID: uid}
	if !s.applySiteReq(c, &site, &req) {
		return
	}
	var maxSort struct{ M int }
	s.db.Model(&model.Site{}).Where("user_id = ? AND group_id = ?", uid, site.GroupID).
		Select("COALESCE(MAX(sort), -1) as m").Scan(&maxSort)
	site.Sort = maxSort.M + 1

	if err := s.db.Create(&site).Error; err != nil {
		serverError(c, err)
		return
	}
	ok(c, site)
}

func (s *Server) handleUpdateSite(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "卡片 ID 无效")
		return
	}
	var req siteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求格式错误")
		return
	}
	var site model.Site
	if err := s.scoped(c).First(&site, id).Error; err != nil {
		notFound(c)
		return
	}
	if !s.applySiteReq(c, &site, &req) {
		return
	}
	if err := s.db.Save(&site).Error; err != nil {
		serverError(c, err)
		return
	}
	ok(c, site)
}

func (s *Server) handleDeleteSite(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "卡片 ID 无效")
		return
	}
	res := s.scoped(c).Delete(&model.Site{}, id)
	if res.Error != nil {
		serverError(c, res.Error)
		return
	}
	if res.RowsAffected == 0 {
		notFound(c)
		return
	}
	ok(c, gin.H{"ok": true})
}

// handleSortSites 一次事务写入整个拖拽结果，避免中途失败留下半套顺序。
func (s *Server) handleSortSites(c *gin.Context) {
	var req sortReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求格式错误")
		return
	}
	uid := middleware.UserID(c)

	// 先把请求里出现的目标分组都校验一遍，防止把卡片挪到别人的分组下。
	groupIDs := map[uint]bool{}
	for _, it := range req.Items {
		if it.GroupID != 0 {
			groupIDs[it.GroupID] = true
		}
	}
	for gid := range groupIDs {
		if !s.ownsGroup(uid, gid) {
			badRequest(c, "分组不存在")
			return
		}
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, it := range req.Items {
			if err := tx.Model(&model.Site{}).Where("id = ? AND user_id = ?", it.ID, uid).
				Updates(map[string]any{"group_id": it.GroupID, "sort": it.Sort}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		serverError(c, err)
		return
	}
	ok(c, gin.H{"ok": true})
}

// ---- 批量导入 ----

type batchItem struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	LanURL      string `json:"lanUrl"`
	Description string `json:"description"`
	IconType    string `json:"iconType"`
	IconValue   string `json:"iconValue"`
	OpenMode    string `json:"openMode"`
	Hidden      bool   `json:"hidden"`
	GroupName   string `json:"groupName"` // 优先于 groupId，用于还原书签目录
}

type batchReq struct {
	GroupID  uint        `json:"groupId"`
	Dedupe   bool        `json:"dedupe"`
	AutoIcon bool        `json:"autoIcon"`
	Items    []batchItem `json:"items"`
}

func (s *Server) handleBatchCreateSites(c *gin.Context) {
	var req batchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求格式错误")
		return
	}
	if len(req.Items) == 0 {
		badRequest(c, "没有可导入的条目")
		return
	}
	if len(req.Items) > 2000 {
		badRequest(c, "单次最多导入 2000 条")
		return
	}
	uid := middleware.UserID(c)
	// 兜底分组同样要校验归属；按 groupName 走的那条路径由 resolveGroup 保证。
	if !s.ownsGroup(uid, req.GroupID) {
		badRequest(c, "分组不存在")
		return
	}

	// 已有的 URL 集合，用于去重。
	existing := map[string]bool{}
	if req.Dedupe {
		var urls []string
		s.db.Model(&model.Site{}).Where("user_id = ?", uid).Pluck("url", &urls)
		for _, u := range urls {
			existing[canonicalURL(u)] = true
		}
	}

	var created []model.Site
	var skipped, invalid int

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 每个分组各自维护一个递增的 sort 起点。
		nextSort := map[uint]int{}
		sortFor := func(gid uint) int {
			if v, okv := nextSort[gid]; okv {
				nextSort[gid] = v + 1
				return v
			}
			var m struct{ M int }
			tx.Model(&model.Site{}).Where("user_id = ? AND group_id = ?", uid, gid).
				Select("COALESCE(MAX(sort), -1) as m").Scan(&m)
			nextSort[gid] = m.M + 2
			return m.M + 1
		}
		groupCache := map[string]uint{}

		for _, it := range req.Items {
			u, err := normalizeURL(it.URL)
			if err != nil || u == "" {
				invalid++
				continue
			}
			if req.Dedupe && existing[canonicalURL(u)] {
				skipped++
				continue
			}
			existing[canonicalURL(u)] = true

			gid := req.GroupID
			if name := strings.TrimSpace(it.GroupName); name != "" {
				if cached, hit := groupCache[name]; hit {
					gid = cached
				} else {
					resolved, err := resolveGroup(tx, uid, name)
					if err != nil {
						return err
					}
					groupCache[name] = resolved
					gid = resolved
				}
			}

			title := strings.TrimSpace(it.Title)
			if title == "" {
				title = hostOf(u)
			}
			lan := ""
			if strings.TrimSpace(it.LanURL) != "" {
				if v, err := normalizeURL(it.LanURL); err == nil {
					lan = v
				}
			}
			iconType := it.IconType
			switch iconType {
			case model.IconTypeURL, model.IconTypeIconify, model.IconTypeText:
			default:
				iconType = model.IconTypeURL
			}
			openMode := it.OpenMode
			if openMode != "self" {
				openMode = "blank"
			}
			site := model.Site{
				UserID: uid, GroupID: gid, Title: title, URL: u, LanURL: lan,
				Description: strings.TrimSpace(it.Description),
				IconType:    iconType, IconValue: sanitizeIconValue(iconType, it.IconValue),
				OpenMode: openMode, Sort: sortFor(gid), Hidden: it.Hidden,
			}
			if err := tx.Create(&site).Error; err != nil {
				return err
			}
			created = append(created, site)
		}
		return nil
	})
	if err != nil {
		serverError(c, err)
		return
	}

	if req.AutoIcon && len(created) > 0 {
		go s.backfillIcons(uid, created)
	}
	ok(c, gin.H{"created": len(created), "skipped": skipped, "invalid": invalid, "items": created})
}

// canonicalURL 归一化 URL 用于去重：忽略协议、www 前缀和末尾斜杠。
func canonicalURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(raw))
	}
	host := strings.ToLower(strings.TrimPrefix(u.Host, "www."))
	path := strings.TrimSuffix(u.Path, "/")
	q := u.RawQuery
	if q != "" {
		q = "?" + q
	}
	return host + path + q
}

// backfillIcons / recordIconUpload 在 backfill.go —— 和手动批量补全共用一套抓取逻辑。

// ---- 抓取元信息 ----

type parseReq struct {
	URLs []string `json:"urls"`
}

type parseResult struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	IconURL     string `json:"iconUrl"`
	Error       string `json:"error,omitempty"`
}

// handleParseSites 抓取一批 URL 的标题和图标，图标下载到本地后返回本地路径。
func (s *Server) handleParseSites(c *gin.Context) {
	var req parseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求格式错误")
		return
	}
	if len(req.URLs) == 0 {
		badRequest(c, "没有待解析的地址")
		return
	}
	if len(req.URLs) > maxBackfillPerRequest {
		badRequest(c, fmt.Sprintf("单次最多解析 %d 条", maxBackfillPerRequest))
		return
	}
	uid := middleware.UserID(c)
	results := make([]parseResult, len(req.URLs))

	service.ForEachLimited(len(req.URLs), 5, func(i int) {
		raw, err := normalizeURL(req.URLs[i])
		if err != nil || raw == "" {
			results[i] = parseResult{URL: req.URLs[i], Error: "地址无效"}
			return
		}
		res := parseResult{URL: raw}
		meta, err := s.fetcher.Fetch(raw)
		if err != nil {
			res.Error = err.Error()
			res.Title = hostOf(raw)
			results[i] = res
			return
		}
		res.Title = meta.Title
		res.Description = meta.Description
		if saved, err := s.fetcher.SaveFirstIcon(s.cfg.UploadDir(), uid, meta.IconURLs); err == nil {
			res.IconURL = saved.Path
			s.recordIconUpload(uid, saved)
		}
		if res.Title == "" {
			res.Title = hostOf(raw)
		}
		results[i] = res
	})

	ok(c, gin.H{"items": results})
}

// handleParseBookmarks 解析浏览器导出的 Netscape 书签 HTML，只返回解析结果，
// 让前端预览确认后再调 /sites/batch 真正写库。
func (s *Server) handleParseBookmarks(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		badRequest(c, "请上传书签文件")
		return
	}
	if file.Size > 20<<20 {
		badRequest(c, "书签文件不能超过 20MB")
		return
	}
	f, err := file.Open()
	if err != nil {
		serverError(c, err)
		return
	}
	defer f.Close()

	items, err := service.ParseBookmarks(f)
	if err != nil {
		badRequest(c, "解析书签失败: "+err.Error())
		return
	}
	out := make([]batchItem, 0, len(items))
	for _, it := range items {
		out = append(out, batchItem{
			Title: it.Title, URL: it.URL, IconValue: it.Icon, GroupName: it.Folder,
		})
	}
	ok(c, gin.H{"items": out, "count": len(out)})
}
