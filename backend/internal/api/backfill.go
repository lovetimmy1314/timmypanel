// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package api

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"

	"timmypanel/internal/middleware"
	"timmypanel/internal/model"
	"timmypanel/internal/service"
)

// maxBackfillPerRequest 是一次补全请求能处理的卡片数上限。
// 这个端点是**同步**的（前端要逐条显示成功还是失败），而每条最坏要发两次出站
// 请求（页面 + 图标），各自受 fetch.timeout_sec 约束。上限放大只会让请求挂到
// 反代超时，宁可让前端分批发 —— 前端也正好靠分批画进度。
const maxBackfillPerRequest = 20

// backfillConcurrency 与 handleParseSites 保持一致：抓的是别人的站点，
// 并发开大了既容易被限流，也算不上礼貌。
const backfillConcurrency = 5

// backfillFields 指定这次要补哪几项。
type backfillFields struct {
	Icon        bool `json:"icon"`
	Title       bool `json:"title"`
	Description bool `json:"description"`
}

func (f backfillFields) empty() bool { return !f.Icon && !f.Title && !f.Description }

type backfillReq struct {
	IDs    []uint         `json:"ids"`
	Fields backfillFields `json:"fields"`
	// Overwrite 为 true 时连已经有值的字段也覆盖。默认只补空的。
	Overwrite bool `json:"overwrite"`
}

// backfillResult 是单张卡片的补全结果。只回填真正写进去的字段，
// 前端据此就地更新，不必整表重拉。
type backfillResult struct {
	ID          uint   `json:"id"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	IconValue   string `json:"iconValue,omitempty"`
	Changed     bool   `json:"changed"`
	Error       string `json:"error,omitempty"`
}

// shouldReplaceIcon 判断抓来的 favicon 该不该顶掉卡片现有的图标。
// 用户特意选了图标库图标或文字图标的，**即便勾了「覆盖已有」也不动** ——
// 那是明确的手动选择，补全是来填空的，不是来推翻它的。
func shouldReplaceIcon(site *model.Site, overwrite bool) bool {
	if strings.TrimSpace(site.IconValue) == "" {
		return true
	}
	if site.IconType != model.IconTypeURL {
		return false
	}
	return overwrite
}

// backfillSite 抓一张卡片的元信息，算出该写回哪些列；updates 为空表示没什么可改的。
// 出站请求一律走 s.fetcher（决策 003：SSRF 防护挂在它的 DialContext 上，
// 另起 http.Client 就绕过了防线）。
//
// 返回的 SavedIcon 交给调用方去补 model.Upload 记录 —— 抓来的图标要和手动上传的
// 一样能在图库里看到、能被账号清理删掉。
func (s *Server) backfillSite(uid uint, site *model.Site, f backfillFields, overwrite bool) (map[string]any, *service.SavedIcon, error) {
	meta, err := s.fetcher.Fetch(site.URL)
	if err != nil {
		return nil, nil, err
	}
	updates := map[string]any{}
	var saved *service.SavedIcon

	// 标题为空、或还是 normalizeURL 拿域名兜底填的，都算「没有标题」。
	if f.Title && meta.Title != "" &&
		(overwrite || site.Title == "" || site.Title == hostOf(site.URL)) {
		updates["title"] = meta.Title
	}
	if f.Description && meta.Description != "" && (overwrite || site.Description == "") {
		updates["description"] = meta.Description
	}
	if f.Icon && len(meta.IconURLs) > 0 && shouldReplaceIcon(site, overwrite) {
		// 图标抓失败不算整条失败：标题和描述已经拿到了，没必要一起丢掉。
		if got, err := s.fetcher.SaveFirstIcon(s.cfg.UploadDir(), uid, meta.IconURLs); err == nil {
			updates["icon_value"] = got.Path
			updates["icon_type"] = model.IconTypeURL
			saved = got
		} else {
			slog.Debug("补全图标失败", "url", site.URL, "err", err)
		}
	}
	return updates, saved, nil
}

// applyBackfill 把 backfillSite 算出的改动写库，并给抓来的图标补上传记录。
func (s *Server) applyBackfill(uid uint, siteID uint, updates map[string]any, saved *service.SavedIcon) error {
	if len(updates) == 0 {
		return nil
	}
	// 带上 user_id：这里的 ID 来自请求，多一个条件比信任调用方便宜。
	if err := s.db.Model(&model.Site{}).Where("id = ? AND user_id = ?", siteID, uid).
		Updates(updates).Error; err != nil {
		return err
	}
	if saved != nil {
		s.recordIconUpload(uid, saved)
	}
	return nil
}

// handleBackfillSites 给**已经存在**的卡片批量补图标和描述。
// 导入时勾「自动抓取」走的是 backfillIcons（后台 goroutine，无反馈），
// 这个端点是给「导入时没勾」「手动加的卡片没图标」这些情况用的，
// 所以做成同步的：前端要能看见每一条的结果。
func (s *Server) handleBackfillSites(c *gin.Context) {
	var req backfillReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求格式错误")
		return
	}
	if req.Fields.empty() {
		badRequest(c, "至少要选择补全一项内容")
		return
	}
	// 同一个 ID 传两遍会并发抓同一个站点，去重顺手做掉。
	ids := make([]uint, 0, len(req.IDs))
	seen := map[uint]bool{}
	for _, id := range req.IDs {
		if id != 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		badRequest(c, "没有待补全的卡片")
		return
	}
	if len(ids) > maxBackfillPerRequest {
		badRequest(c, fmt.Sprintf("单次最多补全 %d 条，请分批提交", maxBackfillPerRequest))
		return
	}
	uid := middleware.UserID(c)

	// 请求里给的是 ID，必须按归属查回来才能用 —— 直接拿 ID 去 Update
	// 就是拿别人的卡片当自己的。查不到的在结果里单独标出，不让整批失败。
	var sites []model.Site
	if err := s.scoped(c).Where("id IN ?", ids).Find(&sites).Error; err != nil {
		serverError(c, err)
		return
	}
	byID := make(map[uint]*model.Site, len(sites))
	for i := range sites {
		byID[sites[i].ID] = &sites[i]
	}

	results := make([]backfillResult, len(ids))
	service.ForEachLimited(len(ids), backfillConcurrency, func(i int) {
		id := ids[i]
		res := backfillResult{ID: id}
		site := byID[id]
		if site == nil {
			res.Error = "卡片不存在"
			results[i] = res
			return
		}
		updates, saved, err := s.backfillSite(uid, site, req.Fields, req.Overwrite)
		if err != nil {
			res.Error = err.Error()
			results[i] = res
			return
		}
		if err := s.applyBackfill(uid, id, updates, saved); err != nil {
			slog.Warn("写回补全结果失败", "id", id, "err", err)
			res.Error = "保存失败"
			results[i] = res
			return
		}
		if title, hit := updates["title"].(string); hit {
			res.Title = title
		}
		if desc, hit := updates["description"].(string); hit {
			res.Description = desc
		}
		if icon, hit := updates["icon_value"].(string); hit {
			res.IconValue = icon
		}
		res.Changed = len(updates) > 0
		results[i] = res
	})

	var changed, failed, unchanged int
	for _, r := range results {
		switch {
		case r.Error != "":
			failed++
		case r.Changed:
			changed++
		default:
			unchanged++
		}
	}
	ok(c, gin.H{"items": results, "changed": changed, "unchanged": unchanged, "failed": failed})
}

// backfillIcons 在批量导入后于后台补抓标题和图标，失败不影响已导入的数据。
// 并发上限和 handleParseSites 一致 —— 串行跑的话，一次导入 2000 条最坏要几个小时。
// 这里三项全补、不覆盖已有值，和手动补全共用 backfillSite，免得两条路径的
// 「什么算空、什么该覆盖」慢慢长歪。
func (s *Server) backfillIcons(uid uint, sites []model.Site) {
	fields := backfillFields{Icon: true, Title: true, Description: true}
	service.ForEachLimited(len(sites), backfillConcurrency, func(i int) {
		site := sites[i]
		updates, saved, err := s.backfillSite(uid, &site, fields, false)
		if err != nil {
			slog.Debug("补抓元信息失败", "url", site.URL, "err", err)
			return
		}
		if err := s.applyBackfill(uid, site.ID, updates, saved); err != nil {
			slog.Warn("写回元信息失败", "id", site.ID, "err", err)
		}
	})
}

// recordIconUpload 给抓来的图标补一条 Upload 记录，和手动上传保持一致，
// 否则备份打包和账号清理都看不到这些文件。图标按内容哈希命名，重复抓到
// 同一个图标时 FirstOrCreate 不会插重复行。
func (s *Server) recordIconUpload(uid uint, saved *service.SavedIcon) {
	up := model.Upload{UserID: uid, Path: saved.Path, Mime: saved.Mime, Size: saved.Size}
	if err := s.db.Where("user_id = ? AND path = ?", uid, saved.Path).
		FirstOrCreate(&up).Error; err != nil {
		slog.Warn("记录图标文件失败", "path", saved.Path, "err", err)
	}
}
