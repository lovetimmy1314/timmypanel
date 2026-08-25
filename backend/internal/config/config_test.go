// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// envBool 的错误分支是安全相关的：原来 `c.Secure, _ = strconv.ParseBool(v)`
// 把错误丢掉，TP_SECURE=yes 会静默变成 false，会话 Cookie 就没了 Secure 标记。
func TestEnvBool(t *testing.T) {
	const key = "TP_TEST_BOOL"

	// 没设时沿用当前值，两个方向都要成立
	if got := envBool(key, true); got != true {
		t.Errorf("未设置时应沿用 true，实际 %v", got)
	}
	if got := envBool(key, false); got != false {
		t.Errorf("未设置时应沿用 false，实际 %v", got)
	}

	// ParseBool 认得的写法照常生效，带空白也要认
	for _, v := range []string{"true", "TRUE", "True", "1", "t", " true "} {
		t.Setenv(key, v)
		if got := envBool(key, false); got != true {
			t.Errorf("envBool(%q) = %v，期望 true", v, got)
		}
	}
	for _, v := range []string{"false", "FALSE", "0", "f"} {
		t.Setenv(key, v)
		if got := envBool(key, true); got != false {
			t.Errorf("envBool(%q) = %v，期望 false", v, got)
		}
	}

	// 认不出来的一律沿用原值，**不能**退成 false
	for _, v := range []string{"yes", "on", "是", "no", "off", "truthy"} {
		t.Setenv(key, v)
		if got := envBool(key, true); got != true {
			t.Errorf("envBool(%q) 把 true 改成了 %v，安全开关被静默关掉", v, got)
		}
		if got := envBool(key, false); got != false {
			t.Errorf("envBool(%q) 把 false 改成了 %v", v, got)
		}
	}
}

func TestNormalizeQWeatherHost(t *testing.T) {
	cases := map[string]string{
		"h2a9cf3mhs.xy.qweatherapi.com":            "h2a9cf3mhs.xy.qweatherapi.com",
		"https://h2a9cf3mhs.xy.qweatherapi.com":    "h2a9cf3mhs.xy.qweatherapi.com",
		"https://h2a9cf3mhs.xy.qweatherapi.com/":   "h2a9cf3mhs.xy.qweatherapi.com",
		"  HTTP://H2A9CF3MHS.XY.QWEATHERAPI.COM  ": "h2a9cf3mhs.xy.qweatherapi.com",
		"https://evil.example/steal":               "",
		"h2a9cf3mhs.xy.qweatherapi.com:443":        "",
		"user:pass@h2a9cf3mhs.xy.qweatherapi.com":  "",
		"": "",
	}
	for in, want := range cases {
		if got := normalizeQWeatherHost(in); got != want {
			t.Errorf("normalizeQWeatherHost(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestValidQWeatherHost(t *testing.T) {
	ok := []string{
		"h2a9cf3mhs.xy.qweatherapi.com",
		"abc.qweatherapi.com",
		"api.qweather.com",
		"devapi.qweather.com",
		"geoapi.qweather.com",
	}
	for _, h := range ok {
		if !validQWeatherHost(h) {
			t.Errorf("%q 应放行", h)
		}
	}
	bad := []string{
		"",
		"qweatherapi.com",
		".qweatherapi.com",
		"evil.com",
		"api.qweather.com.evil.com",
		"example.com",
		strings.Repeat("a", 254) + ".qweatherapi.com",
	}
	for _, h := range bad {
		if validQWeatherHost(h) {
			t.Errorf("%q 应拒绝", h)
		}
	}
}

func TestUseQWeather(t *testing.T) {
	cfg := defaults()
	if cfg.UseQWeather() {
		t.Fatal("默认配置不该走和风")
	}
	cfg.Weather.QWeatherHost = "h2a9cf3mhs.xy.qweatherapi.com"
	cfg.Weather.QWeatherKey = "k"
	if !cfg.UseQWeather() {
		t.Fatal("host+key 齐了就该走和风")
	}
	cfg.Weather.Provider = "open-meteo"
	if cfg.UseQWeather() {
		t.Fatal("强制 open-meteo 应压过 host+key")
	}
	cfg.Weather.Provider = "qweather"
	cfg.Weather.QWeatherKey = ""
	if cfg.UseQWeather() {
		t.Fatal("缺 key 不该半开")
	}
}

func TestWeatherOmittedWhenEmpty(t *testing.T) {
	b, err := yaml.Marshal(defaults())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "weather") {
		t.Fatalf("空 weather 不应写入配置: %s", b)
	}
}

func TestNormalizeWeather(t *testing.T) {
	c := defaults()
	c.Weather.Provider = "QWeather"
	c.Weather.QWeatherHost = "https://h2a9cf3mhs.xy.qweatherapi.com/"
	c.Weather.QWeatherKey = "  abc  "
	c.normalize()
	if c.Weather.Provider != "qweather" {
		t.Fatalf("provider 应小写，得到 %q", c.Weather.Provider)
	}
	if c.Weather.QWeatherHost != "h2a9cf3mhs.xy.qweatherapi.com" {
		t.Fatalf("host 没剥 scheme: %q", c.Weather.QWeatherHost)
	}
	if c.Weather.QWeatherKey != "abc" {
		t.Fatalf("key 没去空白: %q", c.Weather.QWeatherKey)
	}

	c.Weather.Provider = "foo"
	c.Weather.QWeatherHost = "evil.example"
	c.normalize()
	if c.Weather.Provider != "" {
		t.Fatalf("非法 provider 应清空，得到 %q", c.Weather.Provider)
	}
	if c.Weather.QWeatherHost != "" {
		t.Fatalf("非白名单 host 应清空，得到 %q", c.Weather.QWeatherHost)
	}
}
