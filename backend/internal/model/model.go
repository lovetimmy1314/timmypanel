// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package model

import (
	"regexp"
	"time"
)

// usernamePattern 要求首字符是字母或数字，其余只允许字母、数字和 _ . -
// 这不是格式洁癖：用户名会被拼进自动备份的文件名（api 包的 snapshot），
// 一旦放开路径分隔符、或允许以 . 开头，每日备份就能写到 data/backups 之外。
var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// ValidUsername 判断用户名是否可用：2~32 字节，且符合上面的字符集。
func ValidUsername(s string) bool {
	if len(s) < 2 || len(s) > 32 {
		return false
	}
	return usernamePattern.MatchString(s)
}

// bcrypt 只吃前 72 字节，再长 GenerateFromPassword 直接报错，handler 会变成 500。
const (
	MinPasswordBytes = 8
	MaxPasswordBytes = 72
)

// ValidPassword 判断密码长度是否可哈希。按字节计，和 bcrypt 一致。
func ValidPassword(s string) bool {
	n := len(s)
	return n >= MinPasswordBytes && n <= MaxPasswordBytes
}

// 角色常量。
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// 卡片图标类型。
const (
	IconTypeURL     = "url"     // 图片地址（本地 /uploads/... 或外链）
	IconTypeIconify = "iconify" // iconify 图标名，如 mdi:github
	IconTypeText    = "text"    // 文字（一般取标题首字）
)

// User 是一个账号。每个账号的数据相互隔离。
type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"size:120;not null" json:"-"`
	Nickname     string    `gorm:"size:64" json:"nickname"`
	Role         string    `gorm:"size:16;not null;default:user" json:"role"`
	Disabled     bool      `gorm:"not null;default:false" json:"disabled"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Session 是服务端会话。Cookie 里只放原始 token，库里只存它的 SHA-256，
// 这样即使库泄露也无法直接冒用，同时支持“踢下线”。
type Session struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index;not null" json:"userId"`
	TokenHash  string    `gorm:"size:64;uniqueIndex;not null" json:"-"`
	UserAgent  string    `gorm:"size:255" json:"userAgent"`
	IP         string    `gorm:"size:64" json:"ip"`
	Remember   bool      `gorm:"not null;default:false" json:"remember"`
	ExpiresAt  time.Time `gorm:"index;not null" json:"expiresAt"`
	LastUsedAt time.Time `json:"lastUsedAt"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Group 是卡片分组。
type Group struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"-"`
	Name      string    `gorm:"size:64;not null" json:"name"`
	Sort      int       `gorm:"not null;default:0" json:"sort"`
	Collapsed bool      `gorm:"not null;default:false" json:"collapsed"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Site 是一张导航卡片。
type Site struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"index;not null" json:"-"`
	GroupID     uint      `gorm:"index;not null;default:0" json:"groupId"`
	Title       string    `gorm:"size:128;not null" json:"title"`
	URL         string    `gorm:"size:1024;not null" json:"url"`
	LanURL      string    `gorm:"size:1024" json:"lanUrl"` // 内网地址，可空
	Description string    `gorm:"size:512" json:"description"`
	IconType    string    `gorm:"size:16;not null;default:url" json:"iconType"`
	IconValue   string    `gorm:"size:1024" json:"iconValue"`
	IconBg      string    `gorm:"size:32" json:"iconBg"`
	OpenMode    string    `gorm:"size:16;not null;default:blank" json:"openMode"` // blank | self
	Sort        int       `gorm:"not null;default:0" json:"sort"`
	Hidden      bool      `gorm:"not null;default:false" json:"hidden"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Setting 是每个用户的界面设置，整块以 JSON 存，加字段不用改表。
type Setting struct {
	UserID    uint      `gorm:"primaryKey" json:"-"`
	Data      string    `gorm:"type:text" json:"-"`
	UpdatedAt time.Time `json:"-"`
}

// SiteConfig 是实例级配置，整个站点只有一行（ID 固定为 1）。
// 和每个用户各一份的 Setting 不是一回事：登录页未登录时也要读它。
// 存 JSON 的理由同 Setting（决策 007）：加字段不用改表。
type SiteConfig struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	Data      string    `gorm:"type:text" json:"-"`
	UpdatedAt time.Time `json:"-"`
}

// IngestToken 是「浏览器书签反向上传」的凭证，每个用户最多一条（重新生成即替换）。
// 和 Session 同理：书签里放原文，库里只存 SHA-256，库泄露也冒用不了。
// 它只能调 ingest 一个端点，所以泄露的爆炸半径远小于会话 token。
type IngestToken struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex;not null" json:"userId"`
	TokenHash string    `gorm:"size:64;uniqueIndex;not null" json:"-"`
	CreatedAt time.Time `json:"createdAt"`
}

// Upload 记录上传的图片，便于备份打包和清理。
type Upload struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"-"`
	Path      string    `gorm:"size:512;not null" json:"path"` // 形如 /uploads/1/bg/xxx.jpg
	Mime      string    `gorm:"size:64" json:"mime"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
}
