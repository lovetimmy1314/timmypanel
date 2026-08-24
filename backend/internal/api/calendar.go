// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"timmypanel/internal/service"
)

// handleCalendarMonth 返回某年某月的万年历数据（农历、节气、节日、班休）。
//
// **年月由前端按本地时区算好再传**，这里不读 time.Now()：服务器时区通常是 UTC，
// 靠服务端判断「今天是几号」在东八区必错一天。整个端点是纯计算，没有出站请求，
// 也不碰用户数据，但仍挂在 authed 组下——这是登录后才有的界面功能。
func (s *Server) handleCalendarMonth(c *gin.Context) {
	y, errY := strconv.Atoi(c.Query("y"))
	m, errM := strconv.Atoi(c.Query("m"))
	if errY != nil || errM != nil || m < 1 || m > 12 {
		badRequest(c, "缺少年月参数")
		return
	}
	// 农历压缩表只覆盖 1900–2100，越界的年份连公历骨架也不给
	// ——回一个没有农历的空壳日历只会让人以为是数据丢了。
	if y < service.LunarMinYear || y > service.LunarMaxYear {
		badRequest(c, "年份超出可用范围（1900–2100）")
		return
	}
	ok(c, service.MonthDays(y, m))
}
