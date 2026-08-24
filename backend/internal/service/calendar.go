// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package service

import (
	"fmt"
	"time"
)

// CalendarDay 是万年历里的一天。农历、节气、节日名都是拼好的中文，前端直接显示
// ——这些是中文历法专名，没有通行英译，所以不进 i18n（决策 034）。
type CalendarDay struct {
	Date       string `json:"date"`       // 2026-08-24
	Day        int    `json:"day"`        // 公历日，前端不用再从 date 里切
	Weekday    int    `json:"weekday"`    // 0=周日 … 6=周六
	LunarMonth string `json:"lunarMonth"` // 七月 / 闰六月，超出农历表覆盖时为空
	LunarDay   string `json:"lunarDay"`   // 初二
	// GanZhi/Zodiac 是**这一天**所在农历年的干支和生肖，逐日给而不是整月一份：
	// 农历年在正月初一换，跨春节的那个月里前后两截分属两个干支年，
	// 月级字段无论取月初还是月中都会有一半是错的。
	GanZhi    string `json:"ganZhi"`    // 丙午
	Zodiac    string `json:"zodiac"`    // 马
	Festival  string `json:"festival"`  // 节日名，没有则空
	SolarTerm string `json:"solarTerm"` // 节气名，没有则空
	DayType   string `json:"dayType"`   // "" | off（放假）| work（补班）
}

// CalendarMonth 是一个月的全部数据。
type CalendarMonth struct {
	Year  int `json:"y"`
	Month int `json:"m"`
	// HasHolidayData 表示这一年有没有法定调休数据。没有时前端不画班/休角标
	// ——宁可少显示，也不拿去年的表猜今年。
	HasHolidayData bool          `json:"hasHolidayData"`
	Items          []CalendarDay `json:"items"`
}

// solarFestivals 是公历节日，键为「月-日」。
var solarFestivals = map[string]string{
	"1-1":   "元旦",
	"2-14":  "情人节",
	"3-8":   "妇女节",
	"3-12":  "植树节",
	"4-1":   "愚人节",
	"5-1":   "劳动节",
	"5-4":   "青年节",
	"6-1":   "儿童节",
	"7-1":   "建党节",
	"8-1":   "建军节",
	"9-10":  "教师节",
	"10-1":  "国庆节",
	"12-24": "平安夜",
	"12-25": "圣诞节",
}

// lunarFestivals 是农历节日，键为「农历月-农历日」。闰月不算节日
// （闰四月初八不是浴佛节），所以查表前要先排除闰月。
var lunarFestivals = map[string]string{
	"1-1":   "春节",
	"1-15":  "元宵节",
	"2-2":   "龙抬头",
	"5-5":   "端午节",
	"7-7":   "七夕节",
	"7-15":  "中元节",
	"8-15":  "中秋节",
	"9-9":   "重阳节",
	"12-8":  "腊八节",
	"12-23": "小年",
}

// festivalOf 返回该日的节日名。农历节日优先于公历节日：两者撞在一起时
// （比如春节遇上情人节），中国人过的是前者。
func festivalOf(y, m, d int) string {
	if lu, ok := SolarToLunar(y, m, d); ok && !lu.Leap {
		// 除夕没有固定的农历日（腊月可能是 29 或 30 天），靠「明天是不是正月初一」判定。
		next := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
		if nl, ok := SolarToLunar(next.Year(), int(next.Month()), next.Day()); ok &&
			nl.Month == 1 && nl.Day == 1 && !nl.Leap {
			return "除夕"
		}
		if name, ok := lunarFestivals[fmt.Sprintf("%d-%d", lu.Month, lu.Day)]; ok {
			return name
		}
	}
	return solarFestivals[fmt.Sprintf("%d-%d", m, d)]
}

// MonthDays 组装某年某月的整月数据。纯计算，不读 time.Now()——年月由前端按
// 本地时区算好再传上来，服务器时区通常是 UTC，靠服务端判断「今天」必错一天。
func MonthDays(y, m int) CalendarMonth {
	first := time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
	total := first.AddDate(0, 1, -1).Day()

	out := CalendarMonth{
		Year:           y,
		Month:          m,
		HasHolidayData: HasHolidayData(y),
		Items:          make([]CalendarDay, 0, total),
	}
	for d := 1; d <= total; d++ {
		date := fmt.Sprintf("%04d-%02d-%02d", y, m, d)
		day := CalendarDay{
			Date:      date,
			Day:       d,
			Weekday:   int(first.AddDate(0, 0, d-1).Weekday()),
			Festival:  festivalOf(y, m, d),
			SolarTerm: SolarTermOf(y, m, d),
			DayType:   DayTypeOf(date),
		}
		if lu, ok := SolarToLunar(y, m, d); ok {
			day.LunarMonth = lu.MonthName
			day.LunarDay = lu.DayName
			day.GanZhi = GanZhi(lu.Year)
			day.Zodiac = Zodiac(lu.Year)
		}
		out.Items = append(out.Items, day)
	}
	return out
}
