// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package api

import (
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"timmypanel/internal/model"
)

// 登录页要在未登录状态下显示站点图标和背景，而 /uploads/* 是强制登录的
// （隔离模型：路径首段必须等于当前用户 ID，不能为了登录页把整个目录放开）。
// 折中办法是这个只读端点：它只认这两个 kind，且只吐当前实例配置里指向的
// 那一个文件。完整理由见决策 011。
const (
	siteAssetIcon    = "icon"
	siteAssetLoginBg = "login-bg"
)

// siteAssetURL 是登录页拿图用的公开地址。
func siteAssetURL(kind string) string { return "/api/v1/auth/site-asset/" + kind }

// loadSiteConfig 读实例配置，没有记录时返回默认值。
func (s *Server) loadSiteConfig() model.SiteConfigData {
	var row model.SiteConfig
	if err := s.db.First(&row, model.SiteConfigID).Error; err != nil {
		return model.DefaultSiteConfig()
	}
	return row.Decode()
}

// isOwnUploadPath 判断值是不是本站上传目录里的图片路径：/uploads/{uid}/{bg|icons}/{name}。
// 站点图标和登录背景都要在未登录时对外可读，所以形状必须卡死，
// 不能放行任意 /uploads/ 前缀的字符串。
func isOwnUploadPath(v string) bool {
	if v == "" || len(v) > 512 {
		return false
	}
	clean := path.Clean("/" + strings.TrimPrefix(v, "/"))
	if clean != v {
		return false
	}
	segs := strings.Split(strings.TrimPrefix(clean, "/"), "/")
	if len(segs) != 4 || segs[0] != "uploads" {
		return false
	}
	if _, err := strconv.ParseUint(segs[1], 10, 64); err != nil {
		return false
	}
	if segs[2] != "bg" && segs[2] != "icons" {
		return false
	}
	return segs[3] != "" && !strings.ContainsAny(segs[3], `"'<>?#`)
}

// normalizeSiteConfig 收紧管理员提交的实例配置。这三项都会被前端直接塞进
// <title> / <link rel=icon> / CSS background，一样不能少。
func normalizeSiteConfig(in *model.SiteConfigData) {
	in.SiteTitle = strings.TrimSpace(in.SiteTitle)
	if in.SiteTitle == "" {
		in.SiteTitle = model.DefaultSiteConfig().SiteTitle
	}
	// 按字符截，别把中文站名切碎成坏 UTF-8。
	if r := []rune(in.SiteTitle); len(r) > 32 {
		in.SiteTitle = string(r[:32])
	}

	// 图标只收自家上传的图：外链 favicon 会让每个访问者去第三方发请求，
	// data: URI 则要额外验 base64 和 SVG 内容，都不划算。
	in.SiteIcon = strings.TrimSpace(in.SiteIcon)
	if !isOwnUploadPath(in.SiteIcon) {
		in.SiteIcon = ""
	}

	in.LoginBackground = strings.TrimSpace(in.LoginBackground)
	lower := strings.ToLower(in.LoginBackground)
	if !isOwnUploadPath(in.LoginBackground) &&
		!strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		in.LoginBackground = ""
	}
	if strings.ContainsAny(in.LoginBackground, `"'<>`) {
		in.LoginBackground = ""
	}
}

// handleAuthConfig 供登录页读取无需鉴权的信息。指向本地上传的图统一换成
// 公开只读地址，前端不用关心它到底存在哪。
func (s *Server) handleAuthConfig(c *gin.Context) {
	cfg := s.loadSiteConfig()
	icon := ""
	if cfg.SiteIcon != "" {
		icon = siteAssetURL(siteAssetIcon)
	}
	loginBg := cfg.LoginBackground
	if isOwnUploadPath(loginBg) {
		loginBg = siteAssetURL(siteAssetLoginBg)
	}
	ok(c, gin.H{
		"allowRegister":   s.cfg.Auth.AllowRegister,
		"siteTitle":       cfg.SiteTitle,
		"siteIcon":        icon,
		"loginBackground": loginBg,
	})
}

// handleSiteAsset 公开吐出实例配置里指向的那一个文件，别的一概不给 ——
// 它不接受路径参数，所以没法拿来遍历别人的上传目录。
func (s *Server) handleSiteAsset(c *gin.Context) {
	cfg := s.loadSiteConfig()
	var rel string
	switch c.Param("kind") {
	case siteAssetIcon:
		rel = cfg.SiteIcon
	case siteAssetLoginBg:
		rel = cfg.LoginBackground
	default:
		notFound(c)
		return
	}
	if !isOwnUploadPath(rel) {
		notFound(c)
		return
	}
	segs := strings.Split(strings.TrimPrefix(rel, "/"), "/")
	uid, err := strconv.ParseUint(segs[1], 10, 64)
	if err != nil {
		notFound(c)
		return
	}
	abs, err := uploadDiskPath(s.cfg.UploadDir(), rel, uint(uid))
	if err != nil {
		notFound(c)
		return
	}
	if st, err := os.Stat(abs); err != nil || st.IsDir() {
		notFound(c)
		return
	}
	// 抓来的图标里可能有 SVG，SVG 能内嵌脚本。这里是**免登录**端点，
	// 直接导航过来的风险比 /uploads/* 更实在，sandbox 这层不能省（决策 004）。
	c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "public, max-age=300")
	c.File(abs)
}

func (s *Server) handleGetSiteConfig(c *gin.Context) {
	ok(c, s.loadSiteConfig())
}

func (s *Server) handlePutSiteConfig(c *gin.Context) {
	var in model.SiteConfigData
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "请求格式错误")
		return
	}
	normalizeSiteConfig(&in)
	data, err := model.EncodeSiteConfig(in)
	if err != nil {
		serverError(c, err)
		return
	}
	if err := s.db.Save(&model.SiteConfig{ID: model.SiteConfigID, Data: data}).Error; err != nil {
		serverError(c, err)
		return
	}
	ok(c, in)
}
