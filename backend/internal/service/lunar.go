// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package service

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// 农历放在后端算，不在前端：这类转换正是 conventions.md 点名「必须有 Go 单测」的
// 纯函数，放后端才有测试兜着；节假日表也能跟着二进制发版更新，不用重编前端。

// LunarMinYear / LunarMaxYear 是压缩表覆盖的公历年份区间。超出这个区间的日期
// 只回公历部分，农历字段留空——宁可少显示，也不外推一个假的农历。
const (
	LunarMinYear = 1900
	LunarMaxYear = 2100
)

// lunarInfo 是 1900–2100 每年一个的压缩农历数据，通行的那张表。
// 位含义（从低到高）：
//
//	bit 0-3   闰月月份，0 表示当年无闰月
//	bit 4-15  正月到十二月的大小月，1 = 30 天，0 = 29 天（bit 15 是正月）
//	bit 16    闰月的大小，1 = 30 天，0 = 29 天
var lunarInfo = [...]uint32{
	0x04bd8, 0x04ae0, 0x0a570, 0x054d5, 0x0d260, 0x0d950, 0x16554, 0x056a0, 0x09ad0, 0x055d2, // 1900-1909
	0x04ae0, 0x0a5b6, 0x0a4d0, 0x0d250, 0x1d255, 0x0b540, 0x0d6a0, 0x0ada2, 0x095b0, 0x14977, // 1910-1919
	0x04970, 0x0a4b0, 0x0b4b5, 0x06a50, 0x06d40, 0x1ab54, 0x02b60, 0x09570, 0x052f2, 0x04970, // 1920-1929
	0x06566, 0x0d4a0, 0x0ea50, 0x06e95, 0x05ad0, 0x02b60, 0x186e3, 0x092e0, 0x1c8d7, 0x0c950, // 1930-1939
	0x0d4a0, 0x1d8a6, 0x0b550, 0x056a0, 0x1a5b4, 0x025d0, 0x092d0, 0x0d2b2, 0x0a950, 0x0b557, // 1940-1949
	0x06ca0, 0x0b550, 0x15355, 0x04da0, 0x0a5b0, 0x14573, 0x052b0, 0x0a9a8, 0x0e950, 0x06aa0, // 1950-1959
	0x0aea6, 0x0ab50, 0x04b60, 0x0aae4, 0x0a570, 0x05260, 0x0f263, 0x0d950, 0x05b57, 0x056a0, // 1960-1969
	0x096d0, 0x04dd5, 0x04ad0, 0x0a4d0, 0x0d4d4, 0x0d250, 0x0d558, 0x0b540, 0x0b6a0, 0x195a6, // 1970-1979
	0x095b0, 0x049b0, 0x0a974, 0x0a4b0, 0x0b27a, 0x06a50, 0x06d40, 0x0af46, 0x0ab60, 0x09570, // 1980-1989
	0x04af5, 0x04970, 0x064b0, 0x074a3, 0x0ea50, 0x06b58, 0x05ac0, 0x0ab60, 0x096d5, 0x092e0, // 1990-1999
	0x0c960, 0x0d954, 0x0d4a0, 0x0da50, 0x07552, 0x056a0, 0x0abb7, 0x025d0, 0x092d0, 0x0cab5, // 2000-2009
	0x0a950, 0x0b4a0, 0x0baa4, 0x0ad50, 0x055d9, 0x04ba0, 0x0a5b0, 0x15176, 0x052b0, 0x0a930, // 2010-2019
	0x07954, 0x06aa0, 0x0ad50, 0x05b52, 0x04b60, 0x0a6e6, 0x0a4e0, 0x0d260, 0x0ea65, 0x0d530, // 2020-2029
	0x05aa0, 0x076a3, 0x096d0, 0x04afb, 0x04ad0, 0x0a4d0, 0x1d0b6, 0x0d250, 0x0d520, 0x0dd45, // 2030-2039
	0x0b5a0, 0x056d0, 0x055b2, 0x049b0, 0x0a577, 0x0a4b0, 0x0aa50, 0x1b255, 0x06d20, 0x0ada0, // 2040-2049
	0x14b63, 0x09370, 0x049f8, 0x04970, 0x064b0, 0x168a6, 0x0ea50, 0x06b20, 0x1a6c4, 0x0aae0, // 2050-2059
	0x0a2e0, 0x0d2e3, 0x0c960, 0x0d557, 0x0d4a0, 0x0da50, 0x05d55, 0x056a0, 0x0a6d0, 0x055d4, // 2060-2069
	0x052d0, 0x0a9b8, 0x0a950, 0x0b4a0, 0x0b6a6, 0x0ad50, 0x055a0, 0x0aba4, 0x0a5b0, 0x052b0, // 2070-2079
	0x0b273, 0x06930, 0x07337, 0x06aa0, 0x0ad50, 0x14b55, 0x04b60, 0x0a570, 0x054e4, 0x0d160, // 2080-2089
	0x0e968, 0x0d520, 0x0daa0, 0x16aa6, 0x056d0, 0x04ae0, 0x0a9d4, 0x0a2d0, 0x0d150, 0x0f252, // 2090-2099
	0x0d520, // 2100
}

// lunarEpoch：1900-01-31 是农历 1900 年正月初一，整张表从这天起算。
var lunarEpoch = time.Date(1900, 1, 31, 0, 0, 0, 0, time.UTC)

var (
	lunarMonthNames = [...]string{"正月", "二月", "三月", "四月", "五月", "六月", "七月", "八月", "九月", "十月", "冬月", "腊月"}
	lunarDayNames   = [...]string{
		"初一", "初二", "初三", "初四", "初五", "初六", "初七", "初八", "初九", "初十",
		"十一", "十二", "十三", "十四", "十五", "十六", "十七", "十八", "十九", "二十",
		"廿一", "廿二", "廿三", "廿四", "廿五", "廿六", "廿七", "廿八", "廿九", "三十",
	}
	heavenlyStems   = []rune("甲乙丙丁戊己庚辛壬癸")
	earthlyBranches = []rune("子丑寅卯辰巳午未申酉戌亥")
	zodiacAnimals   = []rune("鼠牛虎兔龙蛇马羊猴鸡狗猪")
)

// Lunar 是一个公历日期对应的农历。Name 系字段直接就是可显示的中文，
// 前端不做任何拼装（农历专名不进 i18n，决策 034）。
type Lunar struct {
	Year      int
	Month     int
	Day       int
	Leap      bool
	MonthName string // 正月 / 闰四月
	DayName   string // 初一 … 三十
}

// leapMonth 返回该农历年的闰月月份，0 表示不闰。
func leapMonth(y int) int {
	return int(lunarInfo[y-LunarMinYear] & 0xf)
}

// leapMonthDays 返回闰月的天数，无闰月时是 0。
func leapMonthDays(y int) int {
	if leapMonth(y) == 0 {
		return 0
	}
	if lunarInfo[y-LunarMinYear]&0x10000 != 0 {
		return 30
	}
	return 29
}

// lunarMonthDays 返回该农历年第 m 个平月（非闰月）的天数。
func lunarMonthDays(y, m int) int {
	if lunarInfo[y-LunarMinYear]&(0x10000>>uint(m)) != 0 {
		return 30
	}
	return 29
}

// lunarYearDays 返回该农历年的总天数（含闰月）。
func lunarYearDays(y int) int {
	sum := 348 // 12 个月按 29 天打底，下面把大月多出的那天补回来
	for i := uint32(0x8000); i > 0x8; i >>= 1 {
		if lunarInfo[y-LunarMinYear]&i != 0 {
			sum++
		}
	}
	return sum + leapMonthDays(y)
}

// SolarToLunar 把公历日期转成农历。超出表覆盖范围时第二个返回值为 false。
// 入参只取年月日，不收 time.Time——农历是「哪一天」的属性，掺进时区就会在
// UTC 服务器上错一天（这也是端点要求前端把本地年月算好再传的原因）。
func SolarToLunar(y, m, d int) (Lunar, bool) {
	if y < LunarMinYear || y > LunarMaxYear {
		return Lunar{}, false
	}
	date := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	offset := int(date.Sub(lunarEpoch) / (24 * time.Hour))
	if offset < 0 {
		return Lunar{}, false
	}

	year := LunarMinYear
	for ; year <= LunarMaxYear; year++ {
		days := lunarYearDays(year)
		if offset < days {
			break
		}
		offset -= days
	}
	// 表最后一年只覆盖到农历 2100 年末，再往后没有数据（2101 年元旦前后就在这儿落空）。
	if year > LunarMaxYear {
		return Lunar{}, false
	}

	leap := leapMonth(year)
	month, isLeap, found := 0, false, false
	for mo := 1; mo <= 12; mo++ {
		days := lunarMonthDays(year, mo)
		if offset < days {
			month, isLeap, found = mo, false, true
			break
		}
		offset -= days
		// 闰月排在同名平月之后
		if mo == leap {
			days = leapMonthDays(year)
			if offset < days {
				month, isLeap, found = mo, true, true
				break
			}
			offset -= days
		}
	}
	if !found {
		return Lunar{}, false
	}

	name := lunarMonthNames[month-1]
	if isLeap {
		name = "闰" + name
	}
	return Lunar{
		Year:      year,
		Month:     month,
		Day:       offset + 1,
		Leap:      isLeap,
		MonthName: name,
		DayName:   lunarDayNames[offset],
	}, true
}

// GanZhi 返回农历年的干支，如 1900 → 庚子。
func GanZhi(lunarYear int) string {
	i := ((lunarYear-1900+36)%60 + 60) % 60
	return string(heavenlyStems[i%10]) + string(earthlyBranches[i%12])
}

// Zodiac 返回农历年的生肖。
func Zodiac(lunarYear int) string {
	i := ((lunarYear-1900+36)%12 + 12) % 12
	return string(zodiacAnimals[i])
}

// ---- 24 节气 ----

// solarTermNames 按黄经从 285°（小寒）起每 15° 一个，与下标一一对应。
var solarTermNames = [24]string{
	"小寒", "大寒", "立春", "雨水", "惊蛰", "春分",
	"清明", "谷雨", "立夏", "小满", "芒种", "夏至",
	"小暑", "大暑", "立秋", "处暑", "白露", "秋分",
	"寒露", "霜降", "立冬", "小雪", "大雪", "冬至",
}

var termCache = struct {
	sync.RWMutex
	m map[int][24]string
}{m: make(map[int][24]string)}

func rad(deg float64) float64 { return deg * math.Pi / 180 }

// norm360 把角度归一到 [0,360)。
func norm360(v float64) float64 {
	v = math.Mod(v, 360)
	if v < 0 {
		v += 360
	}
	return v
}

// angleDiff 返回 target-cur 折算到 (-180,180] 的差值，跨 0°/360° 也不会算成反方向
// ——春分那个节气的目标黄经正好是 0°，不折算的话迭代会朝着 360° 一路跑飞。
func angleDiff(target, cur float64) float64 {
	return math.Mod(math.Mod(target-cur, 360)+540, 360) - 180
}

// julianDay 把 UTC 时刻换成儒略日。
func julianDay(t time.Time) float64 {
	return float64(t.UTC().Unix())/86400 + 2440587.5
}

// deltaTSeconds 是力学时与世界时之差（秒），Espenak & Meeus 的分段多项式。
// 1900–2100 内是几十秒到两百多秒的量级，只有节气恰好压在午夜前后几分钟时才影响
// 到日期，但公式是现成的，没必要省这一项。
func deltaTSeconds(year float64) float64 {
	switch {
	case year < 1920:
		t := year - 1900
		return -2.79 + 1.494119*t - 0.0598939*t*t + 0.0061966*t*t*t - 0.000197*t*t*t*t
	case year < 1941:
		t := year - 1920
		return 21.20 + 0.84493*t - 0.076100*t*t + 0.0020936*t*t*t
	case year < 1961:
		t := year - 1950
		return 29.07 + 0.407*t - t*t/233 + t*t*t/2547
	case year < 1986:
		t := year - 1975
		return 45.45 + 1.067*t - t*t/260 - t*t*t/718
	case year < 2005:
		t := year - 2000
		return 63.86 + 0.3345*t - 0.060374*t*t + 0.0017275*t*t*t + 0.000651814*t*t*t*t + 0.00002373599*t*t*t*t*t
	case year < 2050:
		t := year - 2000
		return 62.92 + 0.32217*t + 0.005589*t*t
	default:
		u := (year - 1820) / 100
		return -20 + 32*u*u - 0.5628*(2150-year)
	}
}

// sunApparentLongitude 返回力学时 jde 时刻太阳的视黄经（度）。
// Meeus《Astronomical Algorithms》第 25 章的低精度公式，误差约 0.01°，
// 折算成时间约 15 分钟——只有节气正好落在午夜前后一刻钟内才可能判错日期。
// 这比「每年一个固定偏移量」那套线性近似准得多，也不需要逐年的修正表。
func sunApparentLongitude(jde float64) float64 {
	t := (jde - 2451545.0) / 36525.0
	l0 := 280.46646 + 36000.76983*t + 0.0003032*t*t
	m := rad(357.52911 + 35999.05029*t - 0.0001537*t*t)
	c := (1.914602-0.004817*t-0.000014*t*t)*math.Sin(m) +
		(0.019993-0.000101*t)*math.Sin(2*m) +
		0.000289*math.Sin(3*m)
	omega := rad(125.04 - 1934.136*t)
	return norm360(l0 + c - 0.00569 - 0.00478*math.Sin(omega))
}

// solarTermDate 返回某年第 index 个节气所在的时刻（已换算到北京时间 UTC+8）。
// 节气归属哪一天必须按东八区算：用服务器本地时区或 UTC 都会在跨零点时错一天。
func solarTermDate(year, index int) time.Time {
	target := float64(285 + 15*index)
	// 起点估个大概：第 index 个节气落在 (index/2+1) 月的 6 号或 21 号附近，
	// 离真值不超过两三天，下面几轮就收敛。
	day := 6
	if index%2 == 1 {
		day = 21
	}
	jd := julianDay(time.Date(year, time.Month(index/2+1), day, 12, 0, 0, 0, time.UTC))
	dt := deltaTSeconds(float64(year)) / 86400
	for i := 0; i < 10; i++ {
		diff := angleDiff(target, sunApparentLongitude(jd+dt))
		if math.Abs(diff) < 1e-7 {
			break
		}
		jd += diff / 0.9856 // 太阳黄经每天走约 0.9856°
	}
	sec := math.Round((jd - 2440587.5) * 86400)
	return time.Unix(int64(sec), 0).UTC().Add(8 * time.Hour)
}

// solarTerms 返回某年 24 个节气各自落在哪一天（"01-06" 这种月日串）。
// 按年缓存：一次月视图要查 31 天，逐天现算等于把同一年的 24 个节气算 31 遍。
func solarTerms(year int) [24]string {
	termCache.RLock()
	v, ok := termCache.m[year]
	termCache.RUnlock()
	if ok {
		return v
	}
	var out [24]string
	for i := 0; i < 24; i++ {
		out[i] = solarTermDate(year, i).Format("01-02")
	}
	termCache.Lock()
	termCache.m[year] = out
	termCache.Unlock()
	return out
}

// SolarTermOf 返回该公历日期是哪个节气，不是节气则返回空串。
func SolarTermOf(y, m, d int) string {
	key := fmt.Sprintf("%02d-%02d", m, d)
	for i, t := range solarTerms(y) {
		if t == key {
			return solarTermNames[i]
		}
	}
	return ""
}
