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

func (w *WeatherService) qweatherHeaders() map[string]string {
	return map[string]string{"X-QW-Api-Key": w.qwKey}
}

func (w *WeatherService) qweatherURL(path string) string {
	return "https://" + w.qwHost + path
}

func (w *WeatherService) currentQWeather(qlat, qlon float64) (*Weather, error) {
	headers := w.qweatherHeaders()
	currentURL := w.qweatherURL(fmt.Sprintf("/weather/v1/current/%.2f/%.2f", qlat, qlon))

	var raw qwCurrentResponse
	if err := w.fetcher.GetJSONHeader(currentURL, weatherBodyLimit, headers, &raw); err != nil {
		return nil, err
	}
	out, err := raw.toWeather(w.now().Unix())
	if err != nil {
		return nil, err
	}

	// 日预报失败不影响实况：最高最低和昼夜会缺，卡片照样能显示温度。
	dailyURL := w.qweatherURL(fmt.Sprintf("/weather/v1/daily/%.2f/%.2f?days=1", qlat, qlon))
	var daily qwDailyResponse
	if err := w.fetcher.GetJSONHeader(dailyURL, weatherBodyLimit, headers, &daily); err == nil {
		daily.apply(out, w.now())
	}
	return out, nil
}

func (w *WeatherService) geocodeQWeather(query, lang string) ([]GeoPlace, error) {
	q := url.Values{}
	q.Set("location", query)
	q.Set("number", "8")
	q.Set("lang", lang)

	var raw qwGeoResponse
	if err := w.fetcher.GetJSONHeader(w.qweatherURL("/geo/v2/city/lookup?"+q.Encode()), weatherBodyLimit, w.qweatherHeaders(), &raw); err != nil {
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
		Code string `json:"code"`
	} `json:"condition"`
	Temperature qwMeasure `json:"temperature"`
	FeelsLike   qwMeasure `json:"feelsLike"`
	Humidity    *float64  `json:"humidity"`
	Wind        struct {
		Speed qwMeasure `json:"speed"`
	} `json:"wind"`
}

func (r qwCurrentResponse) toWeather(at int64) (*Weather, error) {
	if r.Temperature.Value == nil {
		return nil, errors.New("上游返回的天气数据不完整")
	}
	code, _ := strconv.Atoi(strings.TrimSpace(r.Condition.Code))
	out := &Weather{
		TempC:     *r.Temperature.Value,
		Code:      qweatherToWMO(code),
		IsDay:     true, // 实况接口不给昼夜，有日预报时再按日出日落改
		UpdatedAt: at,
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
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04Z07:00"} {
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
