// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package service

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

// 一段真实形状的上游响应，字段顺序和名字都按 Open-Meteo 实际返回的抄。
const sampleWeatherJSON = `{
  "latitude": 39.9, "longitude": 116.4, "timezone": "Asia/Shanghai",
  "current": {
    "time": "2026-08-24T14:00",
    "temperature_2m": 26.3,
    "relative_humidity_2m": 62,
    "apparent_temperature": 27.9,
    "is_day": 1,
    "weather_code": 3,
    "wind_speed_10m": 9.2
  },
  "daily": {
    "time": ["2026-08-24"],
    "temperature_2m_max": [31.1],
    "temperature_2m_min": [22.4]
  }
}`

func parseSample(t *testing.T, raw string) *Weather {
	t.Helper()
	var resp weatherResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("解析上游报文失败: %v", err)
	}
	out, err := resp.toWeather(1000)
	if err != nil {
		t.Fatalf("toWeather 失败: %v", err)
	}
	return out
}

func TestWeatherResponseToWeather(t *testing.T) {
	w := parseSample(t, sampleWeatherJSON)
	if w.TempC != 26.3 || w.FeelsLikeC != 27.9 || w.Code != 3 {
		t.Fatalf("温度/体感/天气码不对: %+v", w)
	}
	if !w.IsDay || w.Humidity != 62 || w.WindKph != 9.2 {
		t.Fatalf("昼夜/湿度/风速不对: %+v", w)
	}
	if w.MaxC == nil || *w.MaxC != 31.1 || w.MinC == nil || *w.MinC != 22.4 {
		t.Fatalf("今日最高/最低不对: %+v", w)
	}
	if w.UpdatedAt != 1000 {
		t.Fatalf("抓取时间应由调用方给定，得到 %d", w.UpdatedAt)
	}
}

// 上游对不认识的参数是静默忽略的，所以「少了 daily」是真会发生的情况：
// 这时最高最低必须是 null，不能变成 0℃ 显示出去。
func TestWeatherResponseWithoutDaily(t *testing.T) {
	w := parseSample(t, `{"current":{"temperature_2m":-3.5,"weather_code":71,"is_day":0}}`)
	if w.MaxC != nil || w.MinC != nil {
		t.Fatalf("没有 daily 时最高最低应为 null: %+v", w)
	}
	// 体感缺席时退回实测温度，留 0 会被当成真值显示。
	if w.FeelsLikeC != -3.5 {
		t.Fatalf("体感缺席应退回实测温度，得到 %v", w.FeelsLikeC)
	}
	if w.IsDay {
		t.Fatal("is_day=0 应判为夜间")
	}
	if w.Humidity != 0 || w.WindKph != 0 {
		t.Fatalf("缺席字段应为零值: %+v", w)
	}
}

// 温度或天气码缺席说明这份报文没法用（限流页、错误页都长这样），必须报错，
// 不能拿一堆零值当天气。
func TestWeatherResponseIncomplete(t *testing.T) {
	for _, raw := range []string{`{}`, `{"current":{"weather_code":3}}`, `{"current":{"temperature_2m":20}}`} {
		var resp weatherResponse
		if err := json.Unmarshal([]byte(raw), &resp); err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if _, err := resp.toWeather(1); err == nil {
			t.Fatalf("残缺报文应报错: %s", raw)
		}
	}
}

func TestGeocodeResponseToPlaces(t *testing.T) {
	raw := `{"results":[
	  {"name":"海淀","admin1":"北京市","admin2":"北京市","country":"中国","latitude":39.99064,"longitude":116.28868},
	  {"name":"Berlin","admin1":"Berlin","country":"Deutschland","latitude":52.52,"longitude":13.41},
	  {"name":"","latitude":1,"longitude":1},
	  {"name":"坏坐标","latitude":999,"longitude":0}
	]}`
	var resp geocodeResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	places := resp.toPlaces()
	if len(places) != 2 {
		t.Fatalf("空名字和越界坐标都该被丢掉，得到 %d 条: %+v", len(places), places)
	}
	if places[0].Name != "海淀" || places[0].Admin1 != "北京市" || places[0].Admin2 != "北京市" {
		t.Fatalf("带 admin2 的结果不对: %+v", places[0])
	}
	if places[1].Name != "Berlin" || places[1].Admin2 != "" {
		t.Fatalf("缺 admin2 时应为空串: %+v", places[1])
	}
}

func TestQuantizeCoord(t *testing.T) {
	cases := map[float64]float64{
		39.9075:  39.91,
		116.3972: 116.40,
		-0.044:   -0.04,
		-33.86:   -33.86,
	}
	for in, want := range cases {
		if got := quantizeCoord(in); math.Abs(got-want) > 1e-9 {
			t.Fatalf("quantizeCoord(%v) = %v，期望 %v", in, got, want)
		}
	}
}

func TestValidCoord(t *testing.T) {
	if !ValidCoord(39.9, 116.4) || !ValidCoord(-90, 180) {
		t.Fatal("合法坐标被拒")
	}
	if ValidCoord(91, 0) || ValidCoord(0, 181) || ValidCoord(math.NaN(), 0) || ValidCoord(0, math.Inf(1)) {
		t.Fatal("非法坐标被放行")
	}
}

// 缓存的两条要求：TTL 内命中、过期不命中。用可控时钟走，不睡觉。
func TestWeatherCacheTTL(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	w := NewWeatherService(nil)
	w.now = func() time.Time { return now }

	w.storeWeather("k", Weather{TempC: 20})
	if v, hit := w.cachedWeather("k"); !hit || v.TempC != 20 {
		t.Fatal("刚写进去就该命中")
	}
	now = now.Add(weatherTTL + time.Second)
	if _, hit := w.cachedWeather("k"); hit {
		t.Fatal("过期后不该命中")
	}
}

// 缓存条目数必须有上限，否则拿随机坐标刷接口就是一条内存泄漏。
func TestWeatherCacheEviction(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	w := NewWeatherService(nil)
	w.now = func() time.Time { return now }

	for i := 0; i < weatherCacheMax+20; i++ {
		w.storeWeather(string(rune('a'+i%26))+time.Duration(i).String(), Weather{TempC: float64(i)})
		now = now.Add(time.Second) // 让每条的时间戳不同，才能确定「最旧的那条」
	}
	if len(w.weather) > weatherCacheMax {
		t.Fatalf("缓存条目数 %d 超过上限 %d", len(w.weather), weatherCacheMax)
	}
}

func TestGeocodeQuota(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	w := NewWeatherService(nil)
	w.now = func() time.Time { return now }

	for i := 0; i < geocodeQuotaPerHour; i++ {
		if !w.allowGeocode(7) {
			t.Fatalf("第 %d 次就被拦，配额是 %d", i+1, geocodeQuotaPerHour)
		}
	}
	if w.allowGeocode(7) {
		t.Fatal("超过配额后应被拦下")
	}
	// 换个用户不受影响：配额是按用户记的。
	if !w.allowGeocode(8) {
		t.Fatal("另一个用户不该受牵连")
	}
	// 窗口翻篇后恢复。
	now = now.Add(time.Hour + time.Second)
	if !w.allowGeocode(7) {
		t.Fatal("新窗口应重新放行")
	}
}
