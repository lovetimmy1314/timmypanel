// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package service

import (
	"testing"
	"time"
)

func TestSolarToLunar(t *testing.T) {
	cases := []struct {
		y, m, d int
		want    string // 农历月+日，闰月带「闰」
	}{
		// 春节：这几年的正月初一是公开可查的定点
		{2024, 2, 10, "正月初一"},
		{2025, 1, 29, "正月初一"},
		{2026, 2, 17, "正月初一"},
		// 除夕当天仍属腊月
		{2026, 2, 16, "腊月廿九"},
		// 中秋
		{2024, 9, 17, "八月十五"},
		{2025, 10, 6, "八月十五"},
		// 闰月年：2020 闰四月、2023 闰二月，闰月首日和同名平月首日都要对
		{2020, 4, 23, "四月初一"},
		{2020, 5, 23, "闰四月初一"},
		{2020, 6, 21, "五月初一"},
		{2023, 2, 20, "二月初一"},
		{2023, 3, 22, "闰二月初一"},
		// 农历月末跨月：腊月最后一天的下一天必须是正月初一（见下面的连续性用例）
		{2025, 12, 31, "冬月十二"},
		{2026, 1, 1, "冬月十三"},
		// 表的起点
		{1900, 1, 31, "正月初一"},
	}
	for _, c := range cases {
		lu, ok := SolarToLunar(c.y, c.m, c.d)
		if !ok {
			t.Fatalf("%d-%02d-%02d 转换失败", c.y, c.m, c.d)
		}
		if got := lu.MonthName + lu.DayName; got != c.want {
			t.Errorf("%d-%02d-%02d = %s，期望 %s", c.y, c.m, c.d, got, c.want)
		}
	}
}

// 农历日必须逐天连续：要么 +1，要么跨月回到初一。整段扫一遍能抓出压缩表
// 解码里所有的「多一天/少一天」，比逐个挑对照日靠谱。
func TestLunarContinuity(t *testing.T) {
	prev, ok := SolarToLunar(1900, 1, 31)
	if !ok {
		t.Fatal("起点转换失败")
	}
	d := time.Date(1900, 2, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2100, 12, 31, 0, 0, 0, 0, time.UTC)
	for ; !d.After(end); d = d.AddDate(0, 0, 1) {
		cur, ok := SolarToLunar(d.Year(), int(d.Month()), d.Day())
		if !ok {
			// 表覆盖到农历 2100 年末为止，之后落空是预期的，到此为止
			break
		}
		switch {
		case cur.Day == prev.Day+1:
			if cur.Month != prev.Month || cur.Leap != prev.Leap {
				t.Fatalf("%s 月份跳变：%v → %v", d.Format("2006-01-02"), prev, cur)
			}
		case cur.Day == 1:
			if prev.Day != 29 && prev.Day != 30 {
				t.Fatalf("%s 上一个月只有 %d 天", d.Format("2006-01-02"), prev.Day)
			}
		default:
			t.Fatalf("%s 农历日不连续：%d → %d", d.Format("2006-01-02"), prev.Day, cur.Day)
		}
		prev = cur
	}
	if prev.Year < 2100 {
		t.Fatalf("表提前断在农历 %d 年", prev.Year)
	}
}

func TestSolarToLunarOutOfRange(t *testing.T) {
	for _, c := range [][3]int{{1899, 12, 31}, {1900, 1, 30}, {2101, 6, 1}, {3000, 1, 1}} {
		if _, ok := SolarToLunar(c[0], c[1], c[2]); ok {
			t.Errorf("%v 不该转换成功", c)
		}
	}
}

func TestGanZhiZodiac(t *testing.T) {
	cases := []struct {
		year           int
		ganzhi, zodiac string
	}{
		{1900, "庚子", "鼠"},
		{1984, "甲子", "鼠"},
		{2024, "甲辰", "龙"},
		{2025, "乙巳", "蛇"},
		{2026, "丙午", "马"},
	}
	for _, c := range cases {
		if got := GanZhi(c.year); got != c.ganzhi {
			t.Errorf("GanZhi(%d) = %s，期望 %s", c.year, got, c.ganzhi)
		}
		if got := Zodiac(c.year); got != c.zodiac {
			t.Errorf("Zodiac(%d) = %s，期望 %s", c.year, got, c.zodiac)
		}
	}
}

// 节气日期与公开数据比对。取的都是天文年历上的定点，日期错一天就说明
// 太阳黄经那套算法或者时区换算出了问题。
func TestSolarTermOf(t *testing.T) {
	cases := []struct {
		y, m, d int
		want    string
	}{
		{2024, 2, 4, "立春"},
		{2024, 3, 20, "春分"},
		{2024, 6, 21, "夏至"},
		{2024, 9, 22, "秋分"},
		{2024, 12, 21, "冬至"},
		{2025, 2, 3, "立春"},
		{2025, 4, 4, "清明"},
		{2025, 12, 21, "冬至"},
		{2026, 2, 4, "立春"},
		{2026, 3, 20, "春分"},
		{2000, 12, 21, "冬至"},
	}
	for _, c := range cases {
		if got := SolarTermOf(c.y, c.m, c.d); got != c.want {
			t.Errorf("%d-%02d-%02d = %q，期望 %s", c.y, c.m, c.d, got, c.want)
		}
	}
	if got := SolarTermOf(2026, 8, 25); got != "" {
		t.Errorf("非节气日返回了 %q", got)
	}
}

// 每年 24 个节气必须各占一天、按序排列，且都落在预期的月份里。
func TestSolarTermsWellFormed(t *testing.T) {
	for _, y := range []int{1900, 1950, 2026, 2099, 2100} {
		terms := solarTerms(y)
		seen := map[string]bool{}
		prev := ""
		for i, v := range terms {
			if seen[v] {
				t.Errorf("%d 年第 %d 个节气与前面撞在同一天 %s", y, i, v)
			}
			seen[v] = true
			if v <= prev {
				t.Errorf("%d 年节气顺序乱了：%s 排在 %s 之后", y, v, prev)
			}
			prev = v
			if wantMonth := i/2 + 1; int(v[0]-'0')*10+int(v[1]-'0') != wantMonth {
				t.Errorf("%d 年 %s 落在 %s，期望 %d 月", y, solarTermNames[i], v, wantMonth)
			}
		}
	}
}

func TestFestivalOf(t *testing.T) {
	cases := []struct {
		y, m, d int
		want    string
	}{
		{2026, 1, 1, "元旦"},
		{2026, 2, 16, "除夕"},
		{2026, 2, 17, "春节"},
		{2026, 3, 3, "元宵节"},
		{2026, 9, 25, "中秋节"},
		{2026, 10, 1, "国庆节"},
		{2025, 1, 28, "除夕"},
		{2024, 2, 9, "除夕"},
		{2026, 8, 24, ""},
	}
	for _, c := range cases {
		if got := festivalOf(c.y, c.m, c.d); got != c.want {
			t.Errorf("%d-%02d-%02d = %q，期望 %q", c.y, c.m, c.d, got, c.want)
		}
	}
}

func TestDayTypeOf(t *testing.T) {
	cases := []struct{ date, want string }{
		{"2025-10-01", "off"},
		{"2025-10-05", "off"}, // 假期盖住的周末也是「休」
		{"2025-09-28", "work"},
		{"2025-10-11", "work"},
		{"2025-08-24", ""}, // 普通周末不带角标
		{"2030-10-01", ""}, // 没有这一年的数据
		{"2024-02-10", "off"},
		{"2024-02-18", "work"},
		{"坏日期", ""},
	}
	for _, c := range cases {
		if got := DayTypeOf(c.date); got != c.want {
			t.Errorf("DayTypeOf(%s) = %q，期望 %q", c.date, got, c.want)
		}
	}
	if !HasHolidayData(2025) {
		t.Error("2025 应当有调休数据")
	}
	if HasHolidayData(2030) {
		t.Error("2030 不该有调休数据")
	}
}

func TestMonthDays(t *testing.T) {
	m := MonthDays(2026, 2)
	if len(m.Items) != 28 {
		t.Fatalf("2026 年 2 月有 %d 天", len(m.Items))
	}
	// 跨春节的月份前后两截分属两个干支年，逐日给才不会有一半是错的
	if got := m.Items[15].GanZhi; got != "乙巳" { // 2026-02-16 除夕，还在乙巳年
		t.Errorf("2026-02-16 干支 = %s，期望 乙巳", got)
	}
	if got := m.Items[16].GanZhi + m.Items[16].Zodiac; got != "丙午马" { // 2026-02-17 正月初一
		t.Errorf("2026-02-17 干支生肖 = %s，期望 丙午马", got)
	}
	if m.HasHolidayData {
		t.Error("2026 年目前没有调休数据，不该报有")
	}
	first := m.Items[0]
	if first.Date != "2026-02-01" || first.Day != 1 || first.Weekday != 0 {
		t.Errorf("2026-02-01 应是周日：%+v", first)
	}
	if m.Items[16].Festival != "春节" || m.Items[16].LunarDay != "初一" {
		t.Errorf("2026-02-17 应是春节正月初一：%+v", m.Items[16])
	}
	// 整个一月都在正月初一之前，仍属乙巳蛇年
	for _, day := range MonthDays(2026, 1).Items {
		if day.Zodiac != "蛇" {
			t.Fatalf("%s 生肖 = %s，期望 蛇", day.Date, day.Zodiac)
		}
	}
	// 边界年不 panic，且农历字段该空就空
	for _, y := range []int{1900, 2100} {
		if got := len(MonthDays(y, 12).Items); got != 31 {
			t.Errorf("%d 年 12 月有 %d 天", y, got)
		}
	}
}
