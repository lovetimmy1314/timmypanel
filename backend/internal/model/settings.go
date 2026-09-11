// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package model

import (
	"encoding/json"
	"strings"
)

// Background 是首页背景配置。
type Background struct {
	Type  string  `json:"type"`  // image | color | gradient
	Value string  `json:"value"` // 图片地址 / 颜色值 / 渐变 CSS
	Blur  int     `json:"blur"`  // 高斯模糊像素，0~30
	Mask  float64 `json:"mask"`  // 暗色遮罩透明度，0~0.85
}

// Layout 控制卡片栅格外观。
type Layout struct {
	CardSize   string `json:"cardSize"`   // sm | md | lg
	ShowTitle  bool   `json:"showTitle"`  // 旧字段：与 showLogo 同步，留给老备份
	ShowLogo   bool   `json:"showLogo"`   // 顶部是否显示品牌文案
	ShowDesc   bool   `json:"showDesc"`   // 卡片是否显示描述
	SiteName   string `json:"siteName"`   // 旧字段：与 logoText 同步
	LogoText   string `json:"logoText"`   // 顶栏品牌文案
	ShowClock  bool   `json:"showClock"`  // 是否显示时钟
	GroupStyle string `json:"groupStyle"` // section | tabs
	FooterHTML string `json:"footerHtml"` // 自定义页脚，入库前必须消毒
}

// SearchEngine 是一个可切换的外部搜索引擎，url 中用 %s 占位关键词。
type SearchEngine struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Icon string `json:"icon"`
}

// SearchStyle 是搜索栏外观。空字符串表示沿用默认毛玻璃。
type SearchStyle struct {
	Bg     string `json:"bg"`
	Color  string `json:"color"`
	Border string `json:"border"`
}

// SearchBar 是一个附加搜索框。主搜索框由 SearchConf 的 Enabled/Default 描述，
// Bars 里的每一项都会在首页主搜索框下面纵向多排一行，共用同一份引擎清单和配色。
type SearchBar struct {
	Enabled bool   `json:"enabled"`
	Default string `json:"default"` // local 或某个引擎名
}

// SearchConf 是搜索框配置。
type SearchConf struct {
	Enabled bool           `json:"enabled"`
	Default string         `json:"default"` // local 或某个引擎名
	Engines []SearchEngine `json:"engines"`
	Bars    []SearchBar    `json:"bars"` // 附加搜索框，排在主搜索框下面
	Style   SearchStyle    `json:"style"`
}

// WeatherConf 是首页左上角那个悬浮天气组件的配置。坐标是给后端代理上游用的，
// 地点名只用来显示——两者都由用户在设置里搜地点选定，或由浏览器定位写入。
type WeatherConf struct {
	Enabled bool `json:"enabled"`
	// LocationMode 为 auto 时坐标由浏览器定位给出（只存在 localStorage，不入库），
	// 拿不到就回落到这里存的城市。
	LocationMode string  `json:"locationMode"` // manual | auto
	City         string  `json:"city"`
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	Unit         string  `json:"unit"` // c | f，只影响显示，接口一律回摄氏度
	// Provider 空 = 跟实例默认（yaml 配了和风就用，否则 Open-Meteo）。
	// open-meteo 强制免 key；qweather 用下面的 Host/Key，缺一项回落。
	Provider     string `json:"provider"`
	QWeatherHost string `json:"qweatherHost"`
	QWeatherKey  string `json:"qweatherKey"`
}

// CalendarConf 是首页右上角那个悬浮万年历的配置。农历、节气、节日全在后端算
// （决策 034），这里只有显示相关的两项。
type CalendarConf struct {
	Enabled   bool   `json:"enabled"`
	WeekStart string `json:"weekStart"` // mon | sun，月面板从周一还是周日起排
}

// QuickAccessConf 是首页右侧悬浮快捷访问卡片的配置。
// 仅在 PC 桌面端显示，卡片只展示网站图标和名称，悬停展示描述。
type QuickAccessConf struct {
	Enabled bool   `json:"enabled"`
	SiteIDs []uint `json:"siteIds"`
}

// EngineSeedVersion 是内置搜索引擎清单的版本号。往 DefaultSettings 里加内置引擎时
// 必须 +1，否则老账号看不到新引擎（它们的 engines 数组非空，不会回落到默认值）。
const EngineSeedVersion = 1

// Settings 是一个用户的全部界面设置。
type Settings struct {
	Background  Background      `json:"background"`
	Layout      Layout          `json:"layout"`
	Search      SearchConf      `json:"search"`
	Weather     WeatherConf     `json:"weather"`
	Calendar    CalendarConf    `json:"calendar"`
	QuickAccess QuickAccessConf `json:"quickAccess"`
	Theme       string          `json:"theme"`    // auto | light | dark
	Language    string          `json:"language"` // zh | en
	Network     string          `json:"network"`  // wan | lan
	// EngineSeed 是服务端记账字段，记录这份设置已经补过哪一版的内置引擎。
	// 前端不认识也不该发它（types.ts 里故意没有），保存时由 handlePutSettings 盖成当前值。
	EngineSeed int `json:"engineSeed"`
}

// DefaultSettings 返回新账号的初始设置。
func DefaultSettings() Settings {
	return Settings{
		Background: Background{Type: "gradient", Value: "linear-gradient(135deg,#1e3a8a 0%,#0f172a 60%,#312e81 100%)", Blur: 0, Mask: 0.15},
		Layout: Layout{
			CardSize:   "md",
			ShowTitle:  true,
			ShowLogo:   true,
			ShowDesc:   true,
			SiteName:   "我的导航",
			LogoText:   "我的导航",
			ShowClock:  true,
			GroupStyle: "section",
		},
		Search: SearchConf{
			Enabled: true,
			Default: "local",
			Engines: []SearchEngine{
				{Name: "Google", URL: "https://www.google.com/search?q=%s", Icon: "mdi:google"},
				{Name: "Bing", URL: "https://www.bing.com/search?q=%s", Icon: "mdi:microsoft-bing"},
				{Name: "百度", URL: "https://www.baidu.com/s?wd=%s", Icon: "mdi:magnify"},
				{Name: "GitHub", URL: "https://github.com/search?q=%s", Icon: "mdi:github"},
				// 关键词在路径里而不是 query 上，%s 的位置随之不同——占位符是纯字符串替换，两种都支持。
				{Name: "磁力搜索", URL: "https://yhg007.com/search-%s-0-0-1.html", Icon: "mdi:magnet"},
			},
			Bars: []SearchBar{},
		},
		// 默认开着但没有城市：卡片这时显示一个「选择城市」的入口，不发任何请求，
		// 也不弹浏览器定位授权。默认关掉的话这个功能等于藏起来了，没人会知道它在。
		Weather: WeatherConf{Enabled: true, LocationMode: "manual", Unit: "c", Provider: "open-meteo"},
		// 万年历默认开着：它不发任何出站请求，也不需要用户先配点什么才有内容。
		Calendar:    CalendarConf{Enabled: true, WeekStart: "mon"},
		QuickAccess: QuickAccessConf{Enabled: true, SiteIDs: []uint{}},
		Theme:       "auto",
		Language:    "zh",
		Network:     "wan",
		EngineSeed:  EngineSeedVersion,
	}
}

// seedBuiltinEngines 把这份设置还没补过的内置引擎按名字追加进去。
// 只在 engineSeed 落后于当前版本时做，所以用户手动删掉的内置引擎不会自己长回来
// ——删完保存那一下就会把 engineSeed 写成当前值。
func seedBuiltinEngines(s *Settings) {
	if s.EngineSeed >= EngineSeedVersion {
		return
	}
	have := make(map[string]bool, len(s.Search.Engines))
	for _, e := range s.Search.Engines {
		have[e.Name] = true
	}
	for _, e := range DefaultSettings().Search.Engines {
		if !have[e.Name] {
			s.Search.Engines = append(s.Search.Engines, e)
		}
	}
}

// Decode 把库里的 JSON 解成 Settings，损坏或为空时回落到默认值，
// 并把默认值中新增的字段补上（老数据向前兼容）。
func (s *Setting) Decode() Settings {
	out := DefaultSettings()
	if s == nil || s.Data == "" {
		return out
	}
	// out 是默认值打底，Unmarshal 不会把 JSON 里缺席的字段清零——engineSeed 必须先归零，
	// 否则老数据（没这个键）会被当成「已补过最新版内置引擎」，白白跳过 seedBuiltinEngines。
	out.EngineSeed = 0
	if err := json.Unmarshal([]byte(s.Data), &out); err != nil {
		return DefaultSettings()
	}
	if out.Theme == "" {
		out.Theme = "auto"
	}
	// 老数据没有 language：默认中文，和这个项目原本的唯一语言保持一致。
	if out.Language == "" {
		out.Language = "zh"
	}
	if out.Network == "" {
		out.Network = "wan"
	}
	// 老数据没有 weather 这一块：enabled 会保持 DefaultSettings 里的 true（out 是
	// 默认值打底），另外两个字符串字段得在这儿补，否则前端拿到空串会当成非法值。
	if out.Weather.LocationMode == "" {
		out.Weather.LocationMode = "manual"
	}
	if out.Weather.Unit == "" {
		out.Weather.Unit = "c"
	}
	if out.Weather.Provider == "" {
		out.Weather.Provider = "open-meteo"
	}
	// 老数据没有 calendar 这一块：enabled 保持 DefaultSettings 里的 true（out 是
	// 默认值打底），weekStart 得在这儿补，否则前端拿到空串排不出这个月的格子。
	if out.Calendar.WeekStart == "" {
		out.Calendar.WeekStart = "mon"
	}
	// 老数据没有 quickAccess 这一块：enabled 保持 DefaultSettings 里的 true，
	// siteIds 补空切片，避免序列化成 null。
	if out.QuickAccess.SiteIDs == nil {
		out.QuickAccess.SiteIDs = []uint{}
	}
	if out.Layout.CardSize == "" {
		out.Layout.CardSize = "md"
	}
	if out.Layout.GroupStyle == "" {
		out.Layout.GroupStyle = "section"
	}
	if len(out.Search.Engines) == 0 {
		out.Search.Engines = DefaultSettings().Search.Engines
	}
	// nil 切片会序列化成 null，前端拿到 null 就得逐处判空。统一给成空数组。
	if out.Search.Bars == nil {
		out.Search.Bars = []SearchBar{}
	}
	seedBuiltinEngines(&out)
	// 老数据没有 showLogo / logoText：从旧字段回填。bool 零值无法区分
	// 「没写」和 false，所以看原始 JSON 里有没有这个键。
	var raw map[string]json.RawMessage
	if json.Unmarshal([]byte(s.Data), &raw) == nil {
		if layoutRaw, ok := raw["layout"]; ok {
			var layout map[string]json.RawMessage
			if json.Unmarshal(layoutRaw, &layout) == nil {
				if _, ok := layout["showLogo"]; !ok {
					out.Layout.ShowLogo = out.Layout.ShowTitle
				}
				if _, ok := layout["logoText"]; !ok || strings.TrimSpace(out.Layout.LogoText) == "" {
					out.Layout.LogoText = out.Layout.SiteName
				}
			}
		}
	}
	if strings.TrimSpace(out.Layout.LogoText) == "" {
		out.Layout.LogoText = out.Layout.SiteName
	}
	return out
}

// Encode 把 Settings 序列化回存储字段。
func Encode(s Settings) (string, error) {
	b, err := json.Marshal(s)
	return string(b), err
}
