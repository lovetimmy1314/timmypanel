// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

// Package session 管理服务端会话：Cookie 里只放随机 token 原文，
// 数据库里只存它的 SHA-256，因此库泄露也无法直接冒用，且支持踢下线。
package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"timmypanel/internal/model"
)

// CookieName 是会话 Cookie 的名字。
const CookieName = "tp_session"

// 不勾选“记住我”时，会话 Cookie 无 Max-Age（关浏览器即失效），
// 但服务端仍给一个上限，防止会话表里留下永不过期的记录。
const shortSessionHours = 12

// 剩余有效期低于该比例时滑动续期，避免每次请求都写库。
const renewThreshold = 0.25

// lastUsedFlush 是 last_used_at / IP 的最小写库间隔。设备列表的「最近活跃」
// 最多慢这一档；首页每点一次卡片都会打 API，不能每次鉴权都 UPDATE。
const lastUsedFlush = 5 * time.Minute

// ErrInvalid 表示 Cookie 缺失、无效或已过期。
var ErrInvalid = errors.New("会话无效或已过期")

// Manager 持有会话所需的依赖。
type Manager struct {
	db           *gorm.DB
	rememberDays int
	secure       bool
}

// New 构造 Manager。secure 为 true 时 Cookie 打 Secure 标记（HTTPS 部署必须）。
func New(db *gorm.DB, rememberDays int, secure bool) *Manager {
	return &Manager{db: db, rememberDays: rememberDays, secure: secure}
}

// Create 新建会话并把 Cookie 写进响应。
func (m *Manager) Create(c *gin.Context, userID uint, remember bool) (*model.Session, error) {
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	maxAge := shortSessionHours * 3600
	if remember {
		maxAge = m.rememberDays * 24 * 3600
	}
	s := &model.Session{
		UserID:     userID,
		TokenHash:  hashToken(token),
		UserAgent:  truncate(c.Request.UserAgent(), 255),
		IP:         c.ClientIP(),
		Remember:   remember,
		ExpiresAt:  time.Now().Add(time.Duration(maxAge) * time.Second),
		LastUsedAt: time.Now(),
	}
	if err := m.db.Create(s).Error; err != nil {
		return nil, err
	}

	// 不勾“记住我”时 MaxAge=0 → 浏览器按会话 Cookie 处理，关掉浏览器就没了。
	cookieAge := 0
	if remember {
		cookieAge = maxAge
	}
	m.setCookie(c, token, cookieAge)
	return s, nil
}

// Validate 校验请求携带的会话，必要时滑动续期。
func (m *Manager) Validate(c *gin.Context) (*model.User, *model.Session, error) {
	token, err := c.Cookie(CookieName)
	if err != nil || token == "" {
		return nil, nil, ErrInvalid
	}
	var s model.Session
	if err := m.db.Where("token_hash = ?", hashToken(token)).First(&s).Error; err != nil {
		return nil, nil, ErrInvalid
	}
	if time.Now().After(s.ExpiresAt) {
		m.db.Delete(&s)
		return nil, nil, ErrInvalid
	}
	var u model.User
	if err := m.db.First(&u, s.UserID).Error; err != nil {
		return nil, nil, ErrInvalid
	}
	if u.Disabled {
		return nil, nil, ErrInvalid
	}

	now := time.Now()
	total := m.sessionLifetime(s.Remember)
	renew := time.Until(s.ExpiresAt) < time.Duration(float64(total)*renewThreshold)
	if renew {
		s.ExpiresAt = now.Add(total)
		if s.Remember {
			m.setCookie(c, token, int(total.Seconds()))
		}
	}
	// 续期本来就要写库，顺手刷新活跃时间；否则按 lastUsedFlush 节流。
	if renew || now.Sub(s.LastUsedAt) >= lastUsedFlush {
		s.LastUsedAt = now
		s.IP = c.ClientIP()
		m.db.Model(&s).Select("expires_at", "last_used_at", "ip").Updates(&s)
	}

	return &u, &s, nil
}

// Destroy 删除当前会话并清掉 Cookie。
func (m *Manager) Destroy(c *gin.Context) {
	if token, err := c.Cookie(CookieName); err == nil && token != "" {
		m.db.Where("token_hash = ?", hashToken(token)).Delete(&model.Session{})
	}
	m.setCookie(c, "", -1)
}

// DestroyAllForUser 踢掉某账号的所有会话（改密码后调用）。
func (m *Manager) DestroyAllForUser(userID uint) error {
	return m.db.Where("user_id = ?", userID).Delete(&model.Session{}).Error
}

// DestroyOne 删除指定会话，用于“踢掉其它设备”。
func (m *Manager) DestroyOne(userID, sessionID uint) error {
	return m.db.Where("user_id = ? AND id = ?", userID, sessionID).Delete(&model.Session{}).Error
}

// List 返回某账号当前有效的会话列表。
func (m *Manager) List(userID uint) ([]model.Session, error) {
	var out []model.Session
	err := m.db.Where("user_id = ? AND expires_at > ?", userID, time.Now()).
		Order("last_used_at desc").Find(&out).Error
	return out, err
}

// CurrentID 返回当前请求对应的会话 ID，用于在列表中标出“本机”。
func (m *Manager) CurrentID(c *gin.Context) uint {
	token, err := c.Cookie(CookieName)
	if err != nil || token == "" {
		return 0
	}
	var s model.Session
	if err := m.db.Select("id").Where("token_hash = ?", hashToken(token)).First(&s).Error; err != nil {
		return 0
	}
	return s.ID
}

func (m *Manager) sessionLifetime(remember bool) time.Duration {
	if remember {
		return time.Duration(m.rememberDays) * 24 * time.Hour
	}
	return shortSessionHours * time.Hour
}

func (m *Manager) setCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
