// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package api

import (
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"timmypanel/internal/middleware"
	"timmypanel/internal/model"
)

// loadSettings 读当前用户的设置，没有记录时返回默认值。
func (s *Server) loadSettings(uid uint) model.Settings {
	var row model.Setting
	if err := s.db.Where("user_id = ?", uid).First(&row).Error; err != nil {
		return model.DefaultSettings()
	}
	return row.Decode()
}

func (s *Server) handleGetSettings(c *gin.Context) {
	ok(c, s.loadSettings(middleware.UserID(c)))
}

func (s *Server) handlePutSettings(c *gin.Context) {
	var in model.Settings
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "请求格式错误")
		return
	}
	normalizeSettings(&in)
	// 前端不发 engineSeed，这里由服务端盖章：用户这次保存看到的就是当前这版内置引擎，
	// 所以他若在界面上删掉某个内置引擎，下次读取不会再被补回来。
	in.EngineSeed = model.EngineSeedVersion

	data, err := model.Encode(in)
	if err != nil {
		serverError(c, err)
		return
	}
	uid := middleware.UserID(c)
	if err := s.db.Save(&model.Setting{UserID: uid, Data: data}).Error; err != nil {
		serverError(c, err)
		return
	}
	ok(c, in)
}

// 搜索引擎清单的上限。设置整块以 JSON 存一行，前端每次进首页都全量拉回来，
// 所以这些数字是防「一次 PUT 把这一行撑到兆级」的，不是功能约束——
// 真有人要配 32 个搜索引擎，那是另一个产品了。
const (
	maxSearchEngines   = 32
	maxEngineNameRunes = 32
	maxEngineURLBytes  = 512
)

// engineIconPattern 收紧引擎图标名。前端只会把它当 iconify 图标名用，而图标集
// 只打包了 mdi（决策 019），别的前缀画不出来。形状对不上就清空——这个字段
// 目前前端连编辑入口都没有（SearchPanel 建引擎时写死 icon: ”），
// 非空值只可能来自内置默认引擎，收紧不会误伤用户数据。
var engineIconPattern = regexp.MustCompile(`^mdi:[a-z0-9]+(-[a-z0-9]+)*$`)

// safeCSSColor 判断一个值能否安全地当作 CSS 颜色/渐变直接写进 style。
// 只放行颜色和渐变函数真正需要的字符集：字母数字、# % . , ( ) 空格和连字符。
// 于是 url(...) 里的 / : 会被挡掉（那会让背景去外站发请求，泄露访问者 IP），
// 分号和花括号这类想越出属性值的字符也进不来。
func safeCSSColor(v string) bool {
	if v == "" || len(v) > 512 {
		return false
	}
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '#', r == '%', r == '.', r == ',', r == '(', r == ')', r == ' ', r == '-':
		default:
			return false
		}
	}
	// 兜一层：即便字符都合法，也不该出现这几个关键字。
	lower := strings.ToLower(v)
	for _, bad := range []string{"url", "expression", "image", "element", "attr"} {
		if strings.Contains(lower, bad) {
			return false
		}
	}
	return true
}

// normalizeSettings 收紧用户提交的设置：数值夹到合理区间，背景地址只允许
// 本地上传路径或 http(s) 外链（挡掉 javascript:/data: 之类会被塞进 CSS 的东西）。
func normalizeSettings(in *model.Settings) {
	switch in.Background.Type {
	case "image", "color", "gradient":
	default:
		in.Background.Type = "gradient"
	}
	// background.value 会被前端直接塞进 CSS，所以三种类型都要收紧，不能只管 image。
	in.Background.Value = strings.TrimSpace(in.Background.Value)
	switch in.Background.Type {
	case "image":
		lower := strings.ToLower(in.Background.Value)
		valid := strings.HasPrefix(in.Background.Value, "/uploads/") ||
			strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
		if !valid {
			in.Background = model.DefaultSettings().Background
		}
	default:
		// 颜色和渐变只允许 CSS 颜色/渐变函数用得到的那些字符。挡掉 url(...)
		// 这种会向外发请求的写法，以及分号之类想越出属性值的尝试。
		if !safeCSSColor(in.Background.Value) {
			in.Background.Value = model.DefaultSettings().Background.Value
			in.Background.Type = model.DefaultSettings().Background.Type
		}
	}
	if in.Background.Blur < 0 {
		in.Background.Blur = 0
	}
	if in.Background.Blur > 30 {
		in.Background.Blur = 30
	}
	if in.Background.Mask < 0 {
		in.Background.Mask = 0
	}
	if in.Background.Mask > 0.85 {
		in.Background.Mask = 0.85
	}

	switch in.Layout.CardSize {
	case "sm", "md", "lg":
	default:
		in.Layout.CardSize = "md"
	}
	if in.Layout.GroupStyle != "tabs" {
		in.Layout.GroupStyle = "section"
	}
	in.Layout.LogoText = strings.TrimSpace(in.Layout.LogoText)
	in.Layout.SiteName = strings.TrimSpace(in.Layout.SiteName)
	if in.Layout.LogoText == "" {
		in.Layout.LogoText = in.Layout.SiteName
	}
	if in.Layout.LogoText == "" {
		in.Layout.LogoText = model.DefaultSettings().Layout.LogoText
	}
	// 按字符截而不是按字节：中文品牌名切在多字节序列中间会写出坏 UTF-8，
	// 序列化时被换成 U+FFFD，用户看到的就是一串问号。
	if r := []rune(in.Layout.LogoText); len(r) > 32 {
		in.Layout.LogoText = string(r[:32])
	}
	// 新旧字段保持同值，老备份导回来也不会丢品牌文案。
	in.Layout.SiteName = in.Layout.LogoText
	in.Layout.ShowTitle = in.Layout.ShowLogo
	in.Layout.FooterHTML = sanitizeFooterHTML(in.Layout.FooterHTML)

	switch in.Theme {
	case "auto", "light", "dark":
	default:
		in.Theme = "auto"
	}
	// 语言只收前端真正有词典的这两种。放开了会让前端拿到一个查不到词条的
	// locale，界面上是一片 key 而不是回落到中文。
	switch in.Language {
	case "zh", "en":
	default:
		in.Language = "zh"
	}
	if in.Network != "lan" {
		in.Network = "wan"
	}

	// 就地过滤：engines 和 in.Search.Engines 共用底层数组，写下标永远不超过读下标。
	engines := in.Search.Engines[:0]
	for _, e := range in.Search.Engines {
		if len(engines) >= maxSearchEngines {
			break
		}
		e.Name = strings.TrimSpace(e.Name)
		e.URL = strings.TrimSpace(e.URL)
		e.Icon = strings.TrimSpace(e.Icon)
		lower := strings.ToLower(e.URL)
		if e.Name == "" || !strings.Contains(e.URL, "%s") {
			continue
		}
		if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
			continue
		}
		if len(e.URL) > maxEngineURLBytes {
			continue
		}
		// 引擎名按字符截（「磁力搜索」这种中文名是常态），不按字节。
		if r := []rune(e.Name); len(r) > maxEngineNameRunes {
			e.Name = string(r[:maxEngineNameRunes])
		}
		if !engineIconPattern.MatchString(e.Icon) {
			e.Icon = ""
		}
		engines = append(engines, e)
	}
	in.Search.Engines = engines
	if len(in.Search.Engines) == 0 {
		in.Search.Engines = model.DefaultSettings().Search.Engines
	}
	if in.Search.Default == "" {
		in.Search.Default = "local"
	}
	// 删掉当前默认引擎后 default 会悬空，搜索栏选中一个不存在的项。
	if in.Search.Default != "local" {
		found := false
		for _, e := range in.Search.Engines {
			if e.Name == in.Search.Default {
				found = true
				break
			}
		}
		if !found {
			in.Search.Default = "local"
		}
	}
	if in.Search.Style.Bg != "" && !safeCSSColor(in.Search.Style.Bg) {
		in.Search.Style.Bg = ""
	}
	if in.Search.Style.Color != "" && !safeCSSColor(in.Search.Style.Color) {
		in.Search.Style.Color = ""
	}
	if in.Search.Style.Border != "" && !safeCSSColor(in.Search.Style.Border) {
		in.Search.Style.Border = ""
	}
}
