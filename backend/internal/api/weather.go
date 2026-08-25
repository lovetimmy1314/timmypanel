// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"timmypanel/internal/middleware"
	"timmypanel/internal/service"
)

// maxGeocodeQueryRunes 是地点搜索关键词的长度上限。地名再长也到不了这个数，
// 这里是防「拿一个超长串去构造上游 URL」的。
const maxGeocodeQueryRunes = 32

// handleWeather 替前端去问上游天气。浏览器不直连上游：CSP 的 connect-src 只有
// 'self'，而且直连等于把访客 IP 送给第三方（决策 033）。
func (s *Server) handleWeather(c *gin.Context) {
	lat, errLat := strconv.ParseFloat(c.Query("lat"), 64)
	lon, errLon := strconv.ParseFloat(c.Query("lon"), 64)
	if errLat != nil || errLon != nil {
		badRequest(c, "缺少坐标参数")
		return
	}
	// 坐标越界是调用方的错，要回 400。放到下面统一当上游失败处理的话，
	// 客户端只会看到一句「天气服务暂时不可用」，排查时会往上游那边找。
	if !service.ValidCoord(lat, lon) {
		badRequest(c, "坐标不合法")
		return
	}
	data, err := s.weather.Current(lat, lon)
	if err != nil {
		// 上游不通是常态（网络、限流、被墙），不值得记成 error，也别把上游的
		// 原始错误吐给前端 —— 里面带着我们拼的完整 URL。
		slog.Warn("获取天气失败", "err", err)
		fail(c, http.StatusBadGateway, "天气服务暂时不可用")
		return
	}
	ok(c, data)
}

// handleWeatherGeocode 按名字搜地点（城市或区），给设置里的选择器用。
func (s *Server) handleWeatherGeocode(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		badRequest(c, "请输入地点名")
		return
	}
	if len([]rune(q)) > maxGeocodeQueryRunes {
		badRequest(c, "地点名过长")
		return
	}
	items, err := s.weather.Geocode(middleware.UserID(c), q, c.Query("lang"))
	if err != nil {
		slog.Warn("地点搜索失败", "err", err)
		fail(c, http.StatusBadGateway, "地点搜索暂时不可用")
		return
	}
	ok(c, gin.H{"items": items})
}
