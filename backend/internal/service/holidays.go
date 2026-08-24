// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package service

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// 法定节假日的放假和补班安排每年由国务院办公厅单独发通知，没有任何规律可推
// （同一个节日哪年调休、调哪个周末，全看当年通知），所以只能按年写死一张表。
//
// **没有某一年的数据时，那一年一个班/休角标都不显示**，只显示节日名——
// 拿去年的表猜今年会给出确凿的错误信息，比不显示更糟。加新一年的数据时，
// 照抄当年通知的原文，别从「一般规律」推。
//
// 每条 off/work 里写「MM-DD」或「MM-DD~MM-DD」（闭区间）。
// off 要含被假期盖住的周末（日历上那几天也是「休」），
// 假期之外的普通周末不写——那是常规休息日，不该带角标。

type holidayYear struct {
	off  []string
	work []string
}

var holidayArrangements = map[int]holidayYear{
	// 国办发明电〔2023〕7 号
	2024: {
		off: []string{
			"01-01",       // 元旦
			"02-10~02-17", // 春节
			"04-04~04-06", // 清明节
			"05-01~05-05", // 劳动节
			"06-10",       // 端午节
			"09-15~09-17", // 中秋节
			"10-01~10-07", // 国庆节
		},
		work: []string{"02-04", "02-18", "04-07", "04-28", "05-11", "09-14", "09-29", "10-12"},
	},
	// 国办发明电〔2024〕8 号
	2025: {
		off: []string{
			"01-01",       // 元旦
			"01-28~02-04", // 春节（除夕起放）
			"04-04~04-06", // 清明节
			"05-01~05-05", // 劳动节
			"05-31~06-02", // 端午节
			"10-01~10-08", // 国庆节、中秋节
		},
		work: []string{"01-26", "02-08", "04-27", "09-28", "10-11"},
	},
	// 国办发明电〔2025〕7 号（2025-11-04 发布）
	2026: {
		off: []string{
			"01-01~01-03", // 元旦
			"02-15~02-23", // 春节（腊月廿八起放，到正月初七）
			"04-04~04-06", // 清明节
			"05-01~05-05", // 劳动节
			"06-19~06-21", // 端午节
			"09-25~09-27", // 中秋节
			"10-01~10-07", // 国庆节
		},
		work: []string{"01-04", "02-14", "02-28", "05-09", "09-20", "10-10"},
	},
}

var holidayTable = struct {
	sync.Once
	m map[int]map[string]string
}{}

// buildHolidayTable 把上面的区间展开成「年 → MM-DD → off|work」。
// 展开一次就够，之后每次查表都是 map 命中。
func buildHolidayTable() {
	holidayTable.m = make(map[int]map[string]string, len(holidayArrangements))
	for year, conf := range holidayArrangements {
		days := make(map[string]string, 32)
		for _, span := range conf.off {
			for _, d := range expandSpan(year, span) {
				days[d] = "off"
			}
		}
		for _, span := range conf.work {
			for _, d := range expandSpan(year, span) {
				days[d] = "work"
			}
		}
		holidayTable.m[year] = days
	}
}

// expandSpan 把「MM-DD」或「MM-DD~MM-DD」展开成逐天的 MM-DD 列表。
// 写错格式的条目直接跳过：这张表是编译期写死的常量，回退比 panic 强
// ——一条录错的假期不该让整个服务起不来。
func expandSpan(year int, span string) []string {
	from, to, found := strings.Cut(span, "~")
	if !found {
		to = from
	}
	start, ok := parseMonthDay(year, from)
	if !ok {
		return nil
	}
	end, ok := parseMonthDay(year, to)
	if !ok || end.Before(start) {
		return nil
	}
	var out []string
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		out = append(out, d.Format("01-02"))
	}
	return out
}

func parseMonthDay(year int, v string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", fmt.Sprintf("%04d-%s", year, v))
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// HasHolidayData 表示这一年有没有法定调休数据。
func HasHolidayData(year int) bool {
	holidayTable.Do(buildHolidayTable)
	_, ok := holidayTable.m[year]
	return ok
}

// DayTypeOf 返回某天的班/休类型，date 形如 2025-10-01。
// 没有该年数据、或那天既不放假也不补班时返回空串。
func DayTypeOf(date string) string {
	holidayTable.Do(buildHolidayTable)
	if len(date) != 10 {
		return ""
	}
	year := 0
	if _, err := fmt.Sscanf(date[:4], "%d", &year); err != nil {
		return ""
	}
	return holidayTable.m[year][date[5:]]
}
