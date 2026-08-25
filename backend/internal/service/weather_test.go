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
	w := NewWeatherService(nil, "", "")
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
	w := NewWeatherService(nil, "", "")
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
	w := NewWeatherService(nil, "", "")
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

func TestQWeatherToWMO(t *testing.T) {
	cases := map[int]int{
		100: 0,
		102: 1,
		104: 3,
		309: 51,
		305: 61,
		302: 95,
		400: 71,
		501: 45,
		900: 0,
		999: 3,
		42:  3,
	}
	for in, want := range cases {
		if got := qweatherToWMO(in); got != want {
			t.Errorf("qweatherToWMO(%d) = %d，期望 %d", in, got, want)
		}
	}
}

func TestQWeatherCurrentToWeather(t *testing.T) {
	raw := `{
	  "condition": {"text": "少云", "code": "102"},
	  "temperature": {"value": 31.71, "unit": "°C"},
	  "feelsLike": {"value": 33.64, "unit": "°C"},
	  "humidity": 0.69,
	  "wind": {"direction": {"compass": "SW"}, "speed": {"value": 4.74, "unit": "m/s"}},
	  "precipitation": {"amount": {"value": 0.8, "unit": "mm"}},
	  "pressure": {"value": 1001.5, "unit": "hPa"},
	  "visibility": {"value": 29020, "unit": "m"},
	  "uvIndex": 3
	}`
	var resp qwCurrentResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	out, err := resp.toWeather(1000)
	if err != nil {
		t.Fatalf("toWeather 失败: %v", err)
	}
	if out.TempC != 31.71 || out.FeelsLikeC != 33.64 {
		t.Fatalf("温度不对: %+v", out)
	}
	if out.Code != 1 {
		t.Fatalf("102 应映射成 WMO 1，得到 %d", out.Code)
	}
	if out.Humidity != 69 {
		t.Fatalf("湿度 0.69 应变成 69%%，得到 %d", out.Humidity)
	}
	if math.Abs(out.WindKph-4.74*3.6) > 1e-9 {
		t.Fatalf("风速应按 m/s×3.6 转 km/h，得到 %v", out.WindKph)
	}
	if out.ConditionText != "少云" || out.WindDir != "sw" {
		t.Fatalf("现象/风向不对: %+v", out)
	}
	if out.UVIndex == nil || *out.UVIndex != 3 {
		t.Fatalf("紫外线不对: %+v", out)
	}
	if out.VisibilityKm == nil || math.Abs(*out.VisibilityKm-29.02) > 1e-9 {
		t.Fatalf("能见度应对 m÷1000: %+v", out)
	}
	if out.PressureHpa == nil || *out.PressureHpa != 1001.5 {
		t.Fatalf("气压不对: %+v", out)
	}
	if out.PrecipMm == nil || *out.PrecipMm != 0.8 {
		t.Fatalf("降水不对: %+v", out)
	}
	if !out.IsDay {
		t.Fatal("没有日预报时昼夜默认白天")
	}

	// 湿度已经是百分数时不要再乘 100。
	raw = `{"condition":{"code":"100"},"temperature":{"value":20},"humidity":62}`
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	out, err = resp.toWeather(1)
	if err != nil {
		t.Fatal(err)
	}
	if out.Humidity != 62 {
		t.Fatalf("湿度 62 应原样保留，得到 %d", out.Humidity)
	}

	if _, err := (qwCurrentResponse{}).toWeather(1); err == nil {
		t.Fatal("缺温度应报错")
	}
}

func TestQWeatherDailyApply(t *testing.T) {
	raw := `{
	  "days": [{
	    "temperatureMax": {"value": 29.94},
	    "temperatureMin": {"value": 20.93},
	    "astro": {
	      "sunrise": "2024-08-11T04:22:00Z",
	      "sunset": "2024-08-11T19:34:00Z"
	    }
	  }]
	}`
	var daily qwDailyResponse
	if err := json.Unmarshal([]byte(raw), &daily); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	out := &Weather{IsDay: true}
	now := time.Date(2024, 8, 11, 12, 0, 0, 0, time.UTC)
	daily.apply(out, now)
	if out.MaxC == nil || *out.MaxC != 29.94 || out.MinC == nil || *out.MinC != 20.93 {
		t.Fatalf("最高最低不对: %+v", out)
	}
	if !out.IsDay {
		t.Fatal("正午应判为白天")
	}
	if out.Sunrise != "04:22" || out.Sunset != "19:34" {
		t.Fatalf("日出日落应对当地钟点: sunrise=%q sunset=%q", out.Sunrise, out.Sunset)
	}
	now = time.Date(2024, 8, 11, 21, 0, 0, 0, time.UTC)
	daily.apply(out, now)
	if out.IsDay {
		t.Fatal("日落后应判为夜间")
	}
}

func TestQWeatherCurrentZeroPrecipAndBadWind(t *testing.T) {
	raw := `{
	  "condition": {"code": "100"},
	  "temperature": {"value": 20},
	  "wind": {"direction": {"compass": "vrb"}},
	  "precipitation": {"amount": {"value": 0}}
	}`
	var resp qwCurrentResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	out, err := resp.toWeather(1)
	if err != nil {
		t.Fatal(err)
	}
	if out.PrecipMm != nil {
		t.Fatalf("降水 0 不该传: %+v", out)
	}
	if out.WindDir != "" {
		t.Fatalf("vrb 不是固定风向: %q", out.WindDir)
	}
}

func TestPickAQI(t *testing.T) {
	cn := 46.0
	us := 80.0
	qa := 0.9
	got := pickAQI([]qwAirIndex{
		{Code: "us-epa", AQI: &us, Category: "Good"},
		{Code: "cn-mee", AQI: &cn, Category: "优"},
		{Code: "qaqi", AQI: &qa, Category: "Excellent"},
	})
	if got == nil || got.Code != "cn-mee" {
		t.Fatalf("应优先 cn-mee: %+v", got)
	}
	got = pickAQI([]qwAirIndex{
		{Code: "qaqi", AQI: &qa},
		{Code: "us-epa", AQI: &us, Category: "Good"},
	})
	if got == nil || got.Code != "us-epa" {
		t.Fatalf("没有国标时应拿本地指数: %+v", got)
	}
	var air qwAirResponse
	air.Indexes = []qwAirIndex{{Code: "cn-mee", AQI: &cn, Category: "优"}}
	out := &Weather{}
	air.apply(out)
	if out.AQI == nil || *out.AQI != 46 || out.AQICategory != "优" {
		t.Fatalf("AQI 没写上: %+v", out)
	}
}

func TestQWeatherAlertApply(t *testing.T) {
	raw := `{
	  "alerts": [
	    {"headline":"过期","messageType":{"code":"cancel"}},
	    {"headline":"临桂区气象台更新大风蓝色预警信号","messageType":{"code":"update"},
	     "eventType":{"name":"大风"},"color":{"code":"blue"}}
	  ]
	}`
	var resp qwAlertResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	out := &Weather{}
	resp.apply(out)
	if out.Alert != "临桂区气象台更新大风蓝色预警信号" {
		t.Fatalf("应跳过 cancel 取第一条生效预警: %q", out.Alert)
	}

	// 另起一份：json.Unmarshal 复用切片底层数组时，缺席字段会留下一次的值。
	var fallback qwAlertResponse
	if err := json.Unmarshal([]byte(`{"alerts":[{"eventType":{"name":"暴雨"},"color":{"code":"orange"},"messageType":{"code":"alert"}}]}`), &fallback); err != nil {
		t.Fatal(err)
	}
	out = &Weather{}
	fallback.apply(out)
	if out.Alert != "暴雨 orange" {
		t.Fatalf("没标题时应拼事件+颜色: %q", out.Alert)
	}
}

func TestQWeatherGeoToPlaces(t *testing.T) {
	raw := `{
	  "code": "200",
	  "location": [
	    {"name":"天河","adm1":"广东省","adm2":"广州市","country":"中国","lat":"23.12518","lon":"113.34251"},
	    {"name":"","lat":"1","lon":"1"},
	    {"name":"坏坐标","lat":"999","lon":"0"}
	  ]
	}`
	var resp qwGeoResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	places, err := resp.toPlaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(places) != 1 || places[0].Name != "天河" || places[0].Admin2 != "广州市" {
		t.Fatalf("结果不对: %+v", places)
	}

	empty := qwGeoResponse{Code: "204"}
	got, err := empty.toPlaces()
	if err != nil || got != nil {
		t.Fatalf("204 应是空列表不是错误: %v %v", got, err)
	}
	if _, err := (qwGeoResponse{Code: "401"}).toPlaces(); err == nil {
		t.Fatal("401 应报错")
	}
}

func TestIsDaytime(t *testing.T) {
	now := time.Date(2024, 8, 11, 12, 0, 0, 0, time.UTC)
	if day, ok := isDaytime(now, "2024-08-11T04:22:00Z", "2024-08-11T19:34:00Z"); !ok || !day {
		t.Fatal("正午应是白天")
	}
	// 文档示例没有秒。
	if day, ok := isDaytime(now, "2024-08-11T04:22Z", "2024-08-11T19:34Z"); !ok || !day {
		t.Fatal("无秒的 RFC3339 也应认")
	}
	if day, ok := isDaytime(now, "坏", "2024-08-11T19:34:00Z"); ok || !day {
		t.Fatal("解析失败应回落 true,false")
	}
}

func TestResolveCreds(t *testing.T) {
	om := NewWeatherService(nil, "", "")
	if om.resolveCreds(QWeatherCreds{}).ok() {
		t.Fatal("没配和风应走 Open-Meteo")
	}
	inst := NewWeatherService(nil, "h.xy.qweatherapi.com", "k")
	got := inst.resolveCreds(QWeatherCreds{})
	if !got.ok() || got.Host != "h.xy.qweatherapi.com" {
		t.Fatalf("用户没配时应回落到 yaml: %+v", got)
	}
	user := inst.resolveCreds(QWeatherCreds{Host: "u.xy.qweatherapi.com", Key: "u"})
	if user.Host != "u.xy.qweatherapi.com" {
		t.Fatalf("用户凭据应压过 yaml: %+v", user)
	}
	half := NewWeatherService(nil, "h.xy.qweatherapi.com", "")
	if half.resolveCreds(QWeatherCreds{Host: "u.xy.qweatherapi.com", Key: ""}).ok() {
		t.Fatal("缺 key 不该半开")
	}
	if providerPrefix(QWeatherCreds{}) != "om" || providerPrefix(QWeatherCreds{Host: "h", Key: "k"}) != "qw" {
		t.Fatal("缓存键前缀不对")
	}
}
