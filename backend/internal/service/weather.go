// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package service

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// 默认上游是 Open-Meteo：免注册、免 key。地址写死在这里，不接受前端传入 ——
	// 前端只给坐标。配了和风之后改走 qweather.go 里那一套，域名来自配置白名单。
	weatherAPIURL = "https://api.open-meteo.com/v1/forecast"
	geocodeAPIURL = "https://geocoding-api.open-meteo.com/v1/search"

	weatherBodyLimit = 256 << 10 // 上游回的 JSON 只有几百字节，256KB 是宽出天际的上限
	weatherTTL       = 10 * time.Minute
	geocodeTTL       = time.Hour
	// 缓存条目上限。一个自用导航站不会有几百个不同坐标，这个数字是防「有人拿
	// 随机坐标刷接口，把缓存撑成内存泄漏」的。
	weatherCacheMax = 256
	geocodeCacheMax = 256

	// 每个用户每小时最多让多少次地点搜索真的出站。命中缓存的不算。
	// 地点搜索的关键词是用户随手敲的，不设限等于给了一个「登录后可用的
	// 出站请求放大器」——限流器只拦出站，界面上的重复搜索照样秒回。
	geocodeQuotaPerHour = 60
)

// Weather 是给前端的当前天气。温度一律摄氏度，换算成华氏由前端做
// ——单位是显示偏好，不该让缓存按单位再分一份。
type Weather struct {
	TempC      float64  `json:"tempC"`
	FeelsLikeC float64  `json:"feelsLikeC"`
	Code       int      `json:"code"`  // WMO weather code，前端据此选图标
	IsDay      bool     `json:"isDay"` // 上游按当地日出日落判定，不是按服务器时间
	Humidity   int      `json:"humidity"`
	WindKph    float64  `json:"windKph"`
	MaxC       *float64 `json:"maxC"` // 今日最高/最低。上游偶尔不给 daily，这时是 null
	MinC       *float64 `json:"minC"`
	UpdatedAt  int64    `json:"updatedAt"` // 这份数据是什么时候抓的，unix 秒
}

// GeoPlace 是地点搜索的一条结果。
type GeoPlace struct {
	Name    string  `json:"name"`
	Admin1  string  `json:"admin1"` // 省/州
	Admin2  string  `json:"admin2"` // 市/区。中国的区往往在 name 里，admin2 是所属市
	Country string  `json:"country"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

type cacheEntry[T any] struct {
	value T
	at    time.Time
}

// WeatherService 替前端去问上游，并把结果缓存住。
//
// 为什么要有服务端这一层：CSP 的 connect-src 只有 'self'（决策 019），浏览器
// 直连上游会被自己挡掉；直连还等于把每个访客的 IP 送给第三方；而且只有在这里
// 才能缓存 —— 否则每开一个标签页、每次刷新都是一次出站。
type WeatherService struct {
	fetcher *Fetcher
	now     func() time.Time // 测试里替换掉，才能不睡觉就走到过期分支
	// 实例级回落：用户设置没配齐和风时用 yaml 那一份。半开（只配了其中一个）
	// 在 config.UseQWeather 就被挡掉了，这里同样要求成对。
	fallbackHost string
	fallbackKey  string

	mu      sync.Mutex
	weather map[string]cacheEntry[Weather]
	geo     map[string]cacheEntry[[]GeoPlace]
	quota   map[uint]*quotaWindow
}

// QWeatherCreds 是一次请求选用的和风凭据。Host 和 Key 都非空才走和风。
type QWeatherCreds struct {
	Host string
	Key  string
}

// NewWeatherService 构造天气服务。fetcher 必须是全局那一个：SSRF 防护、超时和
// 连接复用都挂在它身上。fallbackHost/Key 是 yaml 里的实例级回落，用户设置优先。
func NewWeatherService(f *Fetcher, fallbackHost, fallbackKey string) *WeatherService {
	return &WeatherService{
		fetcher:      f,
		now:          time.Now,
		fallbackHost: fallbackHost,
		fallbackKey:  fallbackKey,
		weather:      map[string]cacheEntry[Weather]{},
		geo:          map[string]cacheEntry[[]GeoPlace]{},
		quota:        map[uint]*quotaWindow{},
	}
}

func (c QWeatherCreds) ok() bool {
	return c.Host != "" && c.Key != ""
}

func (w *WeatherService) resolveCreds(user QWeatherCreds) QWeatherCreds {
	if user.ok() {
		return user
	}
	if w.fallbackHost != "" && w.fallbackKey != "" {
		return QWeatherCreds{Host: w.fallbackHost, Key: w.fallbackKey}
	}
	return QWeatherCreds{}
}

func providerPrefix(creds QWeatherCreds) string {
	if creds.ok() {
		return "qw"
	}
	return "om"
}

// quantizeCoord 把坐标收到小数点后两位（约 1 公里）。缓存键和真正发给上游的
// 坐标都用这个值，和入库的 roundCoord 对齐——否则库里 39.91、出站却变成 39.9，
// 海淀和朝阳会被并成同一格。上游国内格点仍约 9–15km，再细没有新信息。
func quantizeCoord(v float64) float64 {
	return math.Round(v*100) / 100
}

// ValidCoord 判断坐标是否落在合法区间。NaN 比较永远为假，所以 NaN 会被这里挡掉。
func ValidCoord(lat, lon float64) bool {
	return lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}

// Current 返回某个坐标的当前天气，10 分钟内的重复请求直接吃缓存。
// user 是这次请求的用户凭据；没配齐就回落到实例级 yaml。
func (w *WeatherService) Current(lat, lon float64, user QWeatherCreds) (*Weather, error) {
	if !ValidCoord(lat, lon) {
		return nil, errors.New("坐标不合法")
	}
	qlat, qlon := quantizeCoord(lat), quantizeCoord(lon)
	creds := w.resolveCreds(user)
	// 缓存键带上游前缀：切到和风之后不能把 Open-Meteo 那格的旧数据当新的用。
	key := fmt.Sprintf("%s:%.2f,%.2f", providerPrefix(creds), qlat, qlon)

	if v, hit := w.cachedWeather(key); hit {
		return &v, nil
	}

	var (
		out *Weather
		err error
	)
	if creds.ok() {
		out, err = w.currentQWeather(qlat, qlon, creds)
	} else {
		out, err = w.currentOpenMeteo(qlat, qlon)
	}
	if err != nil {
		return nil, err
	}
	w.storeWeather(key, *out)
	return out, nil
}

func (w *WeatherService) currentOpenMeteo(qlat, qlon float64) (*Weather, error) {
	q := url.Values{}
	q.Set("latitude", fmt.Sprintf("%.2f", qlat))
	q.Set("longitude", fmt.Sprintf("%.2f", qlon))
	q.Set("current", "temperature_2m,relative_humidity_2m,apparent_temperature,is_day,weather_code,wind_speed_10m")
	q.Set("daily", "temperature_2m_max,temperature_2m_min")
	// timezone=auto 让上游按目标坐标所在时区切「今天」，daily 那两个值才是当地的今天。
	q.Set("timezone", "auto")
	q.Set("forecast_days", "1")

	var raw weatherResponse
	if err := w.fetcher.GetJSON(weatherAPIURL+"?"+q.Encode(), weatherBodyLimit, &raw); err != nil {
		return nil, err
	}
	return raw.toWeather(w.now().Unix())
}

// Geocode 按名字搜地点（城市或区）。uid 用来限出站次数，命中缓存的搜索不消耗配额。
func (w *WeatherService) Geocode(uid uint, query, lang string, user QWeatherCreds) ([]GeoPlace, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("请输入地点名")
	}
	if lang != "en" {
		lang = "zh"
	}
	creds := w.resolveCreds(user)
	key := providerPrefix(creds) + "|" + lang + "|" + strings.ToLower(query)
	if v, hit := w.cachedGeo(key); hit {
		return v, nil
	}
	if !w.allowGeocode(uid) {
		return nil, errors.New("地点搜索太频繁，请稍后再试")
	}

	var places []GeoPlace
	var err error
	if creds.ok() {
		places, err = w.geocodeQWeather(query, lang, creds)
	} else {
		places, err = w.geocodeOpenMeteo(query, lang)
	}
	if err != nil {
		return nil, err
	}
	w.storeGeo(key, places)
	return places, nil
}

func (w *WeatherService) geocodeOpenMeteo(query, lang string) ([]GeoPlace, error) {
	q := url.Values{}
	q.Set("name", query)
	q.Set("count", "8")
	q.Set("language", lang)
	q.Set("format", "json")

	var raw geocodeResponse
	if err := w.fetcher.GetJSON(geocodeAPIURL+"?"+q.Encode(), weatherBodyLimit, &raw); err != nil {
		return nil, err
	}
	return raw.toPlaces(), nil
}

// ---- 上游报文 ----

// weatherResponse 是 Open-Meteo 的当前天气响应。字段用指针接：上游对不认识的
// 参数是静默忽略而不是报错，缺字段时用零值当真实温度是最坏的一种失败。
type weatherResponse struct {
	Current struct {
		Temperature *float64 `json:"temperature_2m"`
		Humidity    *float64 `json:"relative_humidity_2m"`
		Apparent    *float64 `json:"apparent_temperature"`
		IsDay       *int     `json:"is_day"`
		WeatherCode *int     `json:"weather_code"`
		WindSpeed   *float64 `json:"wind_speed_10m"`
	} `json:"current"`
	Daily struct {
		Max []float64 `json:"temperature_2m_max"`
		Min []float64 `json:"temperature_2m_min"`
	} `json:"daily"`
}

func (r weatherResponse) toWeather(at int64) (*Weather, error) {
	if r.Current.Temperature == nil || r.Current.WeatherCode == nil {
		return nil, errors.New("上游返回的天气数据不完整")
	}
	out := &Weather{
		TempC:     *r.Current.Temperature,
		Code:      *r.Current.WeatherCode,
		IsDay:     r.Current.IsDay == nil || *r.Current.IsDay == 1,
		UpdatedAt: at,
	}
	// 体感温度缺席时退回实测温度，而不是留 0 —— 0℃ 会被当成真值显示出来。
	out.FeelsLikeC = out.TempC
	if r.Current.Apparent != nil {
		out.FeelsLikeC = *r.Current.Apparent
	}
	if r.Current.Humidity != nil {
		out.Humidity = int(math.Round(*r.Current.Humidity))
	}
	if r.Current.WindSpeed != nil {
		out.WindKph = *r.Current.WindSpeed
	}
	if len(r.Daily.Max) > 0 {
		v := r.Daily.Max[0]
		out.MaxC = &v
	}
	if len(r.Daily.Min) > 0 {
		v := r.Daily.Min[0]
		out.MinC = &v
	}
	return out, nil
}

type geocodeResponse struct {
	Results []struct {
		Name      string  `json:"name"`
		Admin1    string  `json:"admin1"`
		Admin2    string  `json:"admin2"`
		Country   string  `json:"country"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"results"`
}

func (r geocodeResponse) toPlaces() []GeoPlace {
	out := make([]GeoPlace, 0, len(r.Results))
	for _, it := range r.Results {
		if strings.TrimSpace(it.Name) == "" || !ValidCoord(it.Latitude, it.Longitude) {
			continue
		}
		out = append(out, GeoPlace{
			Name:    it.Name,
			Admin1:  it.Admin1,
			Admin2:  it.Admin2,
			Country: it.Country,
			Lat:     it.Latitude,
			Lon:     it.Longitude,
		})
	}
	return out
}

// ---- 缓存与配额 ----

func (w *WeatherService) cachedWeather(key string) (Weather, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	e, ok := w.weather[key]
	if !ok || w.now().Sub(e.at) > weatherTTL {
		return Weather{}, false
	}
	return e.value, true
}

func (w *WeatherService) storeWeather(key string, v Weather) {
	w.mu.Lock()
	defer w.mu.Unlock()
	evict(w.weather, weatherCacheMax, w.now(), weatherTTL)
	w.weather[key] = cacheEntry[Weather]{value: v, at: w.now()}
}

func (w *WeatherService) cachedGeo(key string) ([]GeoPlace, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	e, ok := w.geo[key]
	if !ok || w.now().Sub(e.at) > geocodeTTL {
		return nil, false
	}
	return e.value, true
}

func (w *WeatherService) storeGeo(key string, v []GeoPlace) {
	w.mu.Lock()
	defer w.mu.Unlock()
	evict(w.geo, geocodeCacheMax, w.now(), geocodeTTL)
	w.geo[key] = cacheEntry[[]GeoPlace]{value: v, at: w.now()}
}

// evict 在写入前给缓存腾地方：先清过期的，还满就丢最旧的那条。
// 调用方必须已经持锁。
func evict[T any](m map[string]cacheEntry[T], max int, now time.Time, ttl time.Duration) {
	if len(m) < max {
		return
	}
	for k, e := range m {
		if now.Sub(e.at) > ttl {
			delete(m, k)
		}
	}
	for len(m) >= max {
		oldestKey := ""
		var oldestAt time.Time
		for k, e := range m {
			if oldestKey == "" || e.at.Before(oldestAt) {
				oldestKey, oldestAt = k, e.at
			}
		}
		if oldestKey == "" {
			return
		}
		delete(m, oldestKey)
	}
}

// quotaWindow 是一个用户当前这一小时窗口内的出站计数。
type quotaWindow struct {
	start time.Time
	n     int
}

// allowGeocode 判断这次地点搜索能不能出站，并计数。
func (w *WeatherService) allowGeocode(uid uint) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.now()
	// 顺手清掉窗口已经翻篇的用户，免得 map 随用户数只增不减。
	for id, q := range w.quota {
		if now.Sub(q.start) >= time.Hour {
			delete(w.quota, id)
		}
	}
	q, ok := w.quota[uid]
	if !ok {
		w.quota[uid] = &quotaWindow{start: now, n: 1}
		return true
	}
	if q.n >= geocodeQuotaPerHour {
		return false
	}
	q.n++
	return true
}
