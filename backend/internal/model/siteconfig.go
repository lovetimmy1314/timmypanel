// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package model

import (
	"encoding/json"
	"strings"
)

// SiteConfigID 是 site_config 表里唯一那一行的主键。整站一份配置，不按用户分。
const SiteConfigID = 1

// SiteConfigData 是实例级配置的内容。字段少，但都要在**未登录**的登录页上用，
// 所以这里只能放公开也无所谓的东西，别把实例机密加进来。
type SiteConfigData struct {
	SiteTitle       string `json:"siteTitle"`       // 浏览器标题 / 登录页大标题
	SiteIcon        string `json:"siteIcon"`        // favicon，/uploads/ 路径
	LoginBackground string `json:"loginBackground"` // 登录页背景，/uploads/ 路径或 http(s)
}

// DefaultSiteConfig 返回未配置过时的实例配置。
func DefaultSiteConfig() SiteConfigData {
	return SiteConfigData{SiteTitle: "Timmypanel"}
}

// Decode 把库里的 JSON 解成配置，损坏或为空时回落到默认值。
func (s *SiteConfig) Decode() SiteConfigData {
	out := DefaultSiteConfig()
	if s == nil || s.Data == "" {
		return out
	}
	if err := json.Unmarshal([]byte(s.Data), &out); err != nil {
		return DefaultSiteConfig()
	}
	if strings.TrimSpace(out.SiteTitle) == "" {
		out.SiteTitle = DefaultSiteConfig().SiteTitle
	}
	return out
}

// EncodeSiteConfig 把配置序列化回存储字段。
func EncodeSiteConfig(d SiteConfigData) (string, error) {
	b, err := json.Marshal(d)
	return string(b), err
}
