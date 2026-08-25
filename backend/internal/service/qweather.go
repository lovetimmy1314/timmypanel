// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package service

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// qweatherToWMO 把和风的现象码收成前端已经认识的 WMO 粗分类。
// 前端 weatherGroup 只认 8 档，精确到「小雨 / 中雨」没有对应图标，
// 所以这里按大类映射，未知码回落到阴（和 Open-Meteo 未知码同一条路）。
func qweatherToWMO(code int) int {
	switch {
	case code == 100, code == 900, code == 901:
		return 0 // 晴 / 热 / 冷
	case code == 101, code == 102, code == 103:
		return 1 // 多云 / 少云 / 晴间多云
	case code == 104:
		return 3 // 阴
	case code >= 500 && code <= 515:
		return 45 // 雾 / 霾 / 沙尘
	case code == 309:
		return 51 // 毛毛雨
	case code == 302, code == 303, code == 304:
		return 95 // 雷阵雨
	case code >= 400 && code <= 499:
		return 71 // 雪
	case code >= 300 && code <= 399:
		return 61 // 雨
	default:
		return 3
	}
}

func qweatherHeaders(key string) map[string]string {
	return map[string]string{"X-QW-Api-Key": key}
}

func qweatherURL(host, path string) string {
	return "https://" + host + path
}

func (w *WeatherService) currentQWeather(qlat, qlon float64, creds QWeatherCreds, lang string) (*Weather, error) {
	headers := qweatherHeaders(creds.Key)
	coord := fmt.Sprintf("%.2f/%.2f", qlat, qlon)
	qLang := url.Values{}
	qLang.Set("lang", lang)

	var raw qwCurrentResponse
	if err := w.fetcher.GetJSONHeader(qweatherURL(creds.Host, "/weather/v1/current/"+coord+"?"+qLang.Encode()), weatherBodyLimit, headers, &raw); err != nil {
		return nil, err
	}
	out, err := raw.toWeather(w.now().Unix())
	if err != nil {
		return nil, err
	}

	// 下面三次都失败不影响实况：卡片照样出温度。空气质量和预警是额外接口，
	// 欠费/没权限时经常 403，不能把整张卡拖死。
	dailyQ := url.Values{}
	dailyQ.Set("days", "1")
	dailyQ.Set("lang", lang)
	// localTime=true：日出日落按目标地点的钟点切，东八区 05:22 不会变成 UTC 21:22。
	dailyQ.Set("localTime", "true")
	var daily qwDailyResponse
	if err := w.fetcher.GetJSONHeader(qweatherURL(creds.Host, "/weather/v1/daily/"+coord+"?"+dailyQ.Encode()), weatherBodyLimit, headers, &daily); err == nil {
		daily.apply(out, w.now())
	}

	var air qwAirResponse
	if err := w.fetcher.GetJSONHeader(qweatherURL(creds.Host, "/airquality/v1/current/"+coord+"?"+qLang.Encode()), weatherBodyLimit, headers, &air); err == nil {
		air.apply(out)
	}

	var alerts qwAlertResponse
	if err := w.fetcher.GetJSONHeader(qweatherURL(creds.Host, "/weatheralert/v1/current/"+coord+"?"+qLang.Encode()), weatherBodyLimit, headers, &alerts); err == nil {
		alerts.apply(out)
	}
	return out, nil
}

func (w *WeatherService) geocodeQWeather(query, lang string, creds QWeatherCreds) ([]GeoPlace, error) {
	q := url.Values{}
	q.Set("location", query)
	q.Set("number", "8")
	q.Set("lang", lang)

	var raw qwGeoResponse
	if err := w.fetcher.GetJSONHeader(qweatherURL(creds.Host, "/geo/v2/city/lookup?"+q.Encode()), weatherBodyLimit, qweatherHeaders(creds.Key), &raw); err != nil {
		return nil, err
	}
	return raw.toPlaces()
}

// ---- 上游报文 ----

type qwMeasure struct {
	Value *float64 `json:"value"`
}

type qwCurrentResponse struct {
	Condition struct {
		Text string `json:"text"`
		Code string `json:"code"`
	} `json:"condition"`
	Temperature qwMeasure `json:"temperature"`
	FeelsLike   qwMeasure `json:"feelsLike"`
	Humidity    *float64  `json:"humidity"`
	Wind        struct {
		Direction struct {
			Compass string `json:"compass"`
		} `json:"direction"`
		Speed qwMeasure `json:"speed"`
	} `json:"wind"`
	Precipitation struct {
		Amount qwMeasure `json:"amount"`
	} `json:"precipitation"`
	Pressure   qwMeasure `json:"pressure"`
	Visibility qwMeasure `json:"visibility"`
	UVIndex    *float64  `json:"uvIndex"`
}

func (r qwCurrentResponse) toWeather(at int64) (*Weather, error) {
	if r.Temperature.Value == nil {
		return nil, errors.New("上游返回的天气数据不完整")
	}
	code, _ := strconv.Atoi(strings.TrimSpace(r.Condition.Code))
	out := &Weather{
		TempC:         *r.Temperature.Value,
		Code:          qweatherToWMO(code),
		IsDay:         true, // 实况接口不给昼夜，有日预报时再按日出日落改
		UpdatedAt:     at,
		ConditionText: strings.TrimSpace(r.Condition.Text),
		WindDir:       strings.ToLower(strings.TrimSpace(r.Wind.Direction.Compass)),
	}
	if !validWindCompass(out.WindDir) {
		out.WindDir = ""
	}
	out.FeelsLikeC = out.TempC
	if r.FeelsLike.Value != nil {
		out.FeelsLikeC = *r.FeelsLike.Value
	}
	if r.Humidity != nil {
		h := *r.Humidity
		// 文档是 [0, 1]；>1 当成已经是百分数，避免 69 被再乘成 6900。
		if h <= 1 {
			h *= 100
		}
		out.Humidity = int(math.Round(h))
	}
	if r.Wind.Speed.Value != nil {
		// 和风风速是 m/s，前端展示 km/h，×3.6 对齐 Open-Meteo。
		out.WindKph = *r.Wind.Speed.Value * 3.6
	}
	if r.UVIndex != nil {
		v := int(math.Round(*r.UVIndex))
		out.UVIndex = &v
	}
	if r.Visibility.Value != nil && *r.Visibility.Value > 0 {
		km := *r.Visibility.Value / 1000
		out.VisibilityKm = &km
	}
	if r.Pressure.Value != nil {
		v := *r.Pressure.Value
		out.PressureHpa = &v
	}
	if r.Precipitation.Amount.Value != nil && *r.Precipitation.Amount.Value > 0 {
		v := *r.Precipitation.Amount.Value
		out.PrecipMm = &v
	}
	return out, nil
}

type qwDailyResponse struct {
	Days []struct {
		TemperatureMax qwMeasure `json:"temperatureMax"`
		TemperatureMin qwMeasure `json:"temperatureMin"`
		Astro          struct {
			Sunrise string `json:"sunrise"`
			Sunset  string `json:"sunset"`
		} `json:"astro"`
	} `json:"days"`
}

func (r qwDailyResponse) apply(out *Weather, now time.Time) {
	if len(r.Days) == 0 {
		return
	}
	d := r.Days[0]
	if d.TemperatureMax.Value != nil {
		v := *d.TemperatureMax.Value
		out.MaxC = &v
	}
	if d.TemperatureMin.Value != nil {
		v := *d.TemperatureMin.Value
		out.MinC = &v
	}
	if day, ok := isDaytime(now, d.Astro.Sunrise, d.Astro.Sunset); ok {
		out.IsDay = day
	}
	out.Sunrise = formatClock(d.Astro.Sunrise)
	out.Sunset = formatClock(d.Astro.Sunset)
}

func formatClock(s string) string {
	t, ok := parseQWTime(s)
	if !ok {
		return ""
	}
	return t.Format("15:04")
}

var windCompass = map[string]struct{}{
	"n": {}, "nne": {}, "ne": {}, "ene": {},
	"e": {}, "ese": {}, "se": {}, "sse": {},
	"s": {}, "ssw": {}, "sw": {}, "wsw": {},
	"w": {}, "wnw": {}, "nw": {}, "nnw": {},
}

func validWindCompass(s string) bool {
	_, ok := windCompass[s]
	return ok
}

type qwAirIndex struct {
	Code       string   `json:"code"`
	AQI        *float64 `json:"aqi"`
	AQIDisplay string   `json:"aqiDisplay"`
	Category   string   `json:"category"`
}

type qwAirResponse struct {
	Indexes []qwAirIndex `json:"indexes"`
}

func (r qwAirResponse) apply(out *Weather) {
	pick := pickAQI(r.Indexes)
	if pick == nil || pick.AQI == nil {
		return
	}
	v := int(math.Round(*pick.AQI))
	out.AQI = &v
	out.AQICategory = strings.TrimSpace(pick.Category)
}

// pickAQI 优先中国国标（cn-mee），没有就拿第一条非 QAQI 的本地指数。
// QAQI 在中国不适用，排最后。
func pickAQI(indexes []qwAirIndex) *qwAirIndex {
	var fallback, qaqi *qwAirIndex
	for i := range indexes {
		it := &indexes[i]
		switch it.Code {
		case "cn-mee", "cn-mee-1h":
			return it
		case "qaqi":
			qaqi = it
		default:
			if fallback == nil {
				fallback = it
			}
		}
	}
	if fallback != nil {
		return fallback
	}
	return qaqi
}

type qwAlertResponse struct {
	Alerts []struct {
		Headline  string `json:"headline"`
		EventType struct {
			Name string `json:"name"`
		} `json:"eventType"`
		Color struct {
			Code string `json:"code"`
		} `json:"color"`
		MessageType struct {
			Code string `json:"code"`
		} `json:"messageType"`
	} `json:"alerts"`
}

func (r qwAlertResponse) apply(out *Weather) {
	for _, a := range r.Alerts {
		if a.MessageType.Code == "cancel" {
			continue
		}
		title := strings.TrimSpace(a.Headline)
		if title == "" {
			name := strings.TrimSpace(a.EventType.Name)
			color := strings.TrimSpace(a.Color.Code)
			switch {
			case name != "" && color != "":
				title = name + " " + color
			case name != "":
				title = name
			}
		}
		if title == "" {
			continue
		}
		if r := []rune(title); len(r) > 48 {
			title = string(r[:48])
		}
		out.Alert = title
		return
	}
}

func isDaytime(now time.Time, sunrise, sunset string) (bool, bool) {
	rise, ok1 := parseQWTime(sunrise)
	set, ok2 := parseQWTime(sunset)
	if !ok1 || !ok2 {
		return true, false
	}
	return !now.Before(rise) && now.Before(set), true
}

// parseQWTime 认 RFC3339，也认文档示例那种没有秒的写法（2024-08-11T04:22Z）。
func parseQWTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

type qwGeoResponse struct {
	Code     string `json:"code"`
	Location []struct {
		Name    string `json:"name"`
		Adm1    string `json:"adm1"`
		Adm2    string `json:"adm2"`
		Country string `json:"country"`
		Lat     string `json:"lat"`
		Lon     string `json:"lon"`
	} `json:"location"`
}

func (r qwGeoResponse) toPlaces() ([]GeoPlace, error) {
	// 204/404 是「没这个地方」，对搜索框来说就是空列表，不是上游故障。
	if r.Code != "" && r.Code != "200" {
		if r.Code == "204" || r.Code == "404" {
			return nil, nil
		}
		return nil, fmt.Errorf("上游返回 %s", r.Code)
	}
	out := make([]GeoPlace, 0, len(r.Location))
	for _, it := range r.Location {
		if strings.TrimSpace(it.Name) == "" {
			continue
		}
		lat, errLat := strconv.ParseFloat(it.Lat, 64)
		lon, errLon := strconv.ParseFloat(it.Lon, 64)
		if errLat != nil || errLon != nil || !ValidCoord(lat, lon) {
			continue
		}
		out = append(out, GeoPlace{
			Name:    it.Name,
			Admin1:  it.Adm1,
			Admin2:  it.Adm2,
			Country: it.Country,
			Lat:     lat,
			Lon:     lon,
		})
	}
	return out, nil
}
