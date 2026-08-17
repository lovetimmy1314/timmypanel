// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"timmypanel/internal/middleware"
	"timmypanel/internal/model"
)

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

type userDTO struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Role     string `json:"role"`
	Disabled bool   `json:"disabled"`
}

func toUserDTO(u *model.User) userDTO {
	return userDTO{ID: u.ID, Username: u.Username, Nickname: u.Nickname, Role: u.Role, Disabled: u.Disabled}
}

// handleAuthConfig 在 siteconfig.go —— 登录页要读的都是实例级配置。

func (s *Server) handleLogin(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求格式错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		badRequest(c, "用户名和密码不能为空")
		return
	}

	// IP 和用户名各限一路：既防单 IP 暴力破解，也防分布式撞同一个账号。
	ipKey := "ip:" + c.ClientIP()
	userKey := "user:" + strings.ToLower(req.Username)
	for _, k := range []string{ipKey, userKey} {
		if locked, left := s.limiter.Locked(k); locked {
			fail(c, http.StatusTooManyRequests,
				"失败次数过多，请 "+strconv.Itoa(int(left.Minutes())+1)+" 分钟后再试")
			return
		}
	}

	var u model.User
	err := s.db.Where("username = ?", req.Username).First(&u).Error
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)) != nil {
		s.limiter.Fail(ipKey)
		s.limiter.Fail(userKey)
		// 不区分“用户不存在”和“密码错误”，避免枚举账号。
		fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if u.Disabled {
		fail(c, http.StatusForbidden, "账号已被禁用")
		return
	}

	s.limiter.Reset(ipKey)
	s.limiter.Reset(userKey)

	if _, err := s.session.Create(c, u.ID, req.Remember); err != nil {
		serverError(c, err)
		return
	}
	ok(c, gin.H{"user": toUserDTO(&u)})
}

func (s *Server) handleMe(c *gin.Context) {
	ok(c, gin.H{"user": toUserDTO(middleware.CurrentUser(c))})
}

func (s *Server) handleLogout(c *gin.Context) {
	s.session.Destroy(c)
	ok(c, gin.H{"ok": true})
}

type changePasswordReq struct {
	Old  string `json:"old"`
	New  string `json:"new"`
	New2 string `json:"new2"`
}

func (s *Server) handleChangePassword(c *gin.Context) {
	var req changePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求格式错误")
		return
	}
	if req.New != req.New2 {
		badRequest(c, "两次输入的新密码不一致")
		return
	}
	if !model.ValidPassword(req.New) {
		badRequest(c, "新密码至少 8 位，且不能超过 72 字节")
		return
	}
	u := middleware.CurrentUser(c)
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Old)) != nil {
		fail(c, http.StatusUnauthorized, "原密码不正确")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.New), model.BcryptCost)
	if err != nil {
		serverError(c, err)
		return
	}
	if err := s.db.Model(&model.User{}).Where("id = ?", u.ID).
		Update("password_hash", string(hash)).Error; err != nil {
		serverError(c, err)
		return
	}
	// 改密后踢掉所有设备，再给当前浏览器重签。remember 沿用旧会话，
	// 别写死 true——否则一次改密就把会话 Cookie 升级成 30 天。
	remember := false
	if sess := middleware.CurrentSession(c); sess != nil {
		remember = sess.Remember
	}
	if err := s.session.DestroyAllForUser(u.ID); err != nil {
		serverError(c, err)
		return
	}
	if _, err := s.session.Create(c, u.ID, remember); err != nil {
		serverError(c, err)
		return
	}
	ok(c, gin.H{"ok": true})
}

type sessionDTO struct {
	ID         uint      `json:"id"`
	UserAgent  string    `json:"userAgent"`
	IP         string    `json:"ip"`
	Remember   bool      `json:"remember"`
	Current    bool      `json:"current"`
	ExpiresAt  time.Time `json:"expiresAt"`
	LastUsedAt time.Time `json:"lastUsedAt"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (s *Server) handleListSessions(c *gin.Context) {
	uid := middleware.UserID(c)
	list, err := s.session.List(uid)
	if err != nil {
		serverError(c, err)
		return
	}
	cur := s.session.CurrentID(c)
	out := make([]sessionDTO, 0, len(list))
	for _, it := range list {
		out = append(out, sessionDTO{
			ID: it.ID, UserAgent: it.UserAgent, IP: it.IP, Remember: it.Remember,
			Current: it.ID == cur, ExpiresAt: it.ExpiresAt,
			LastUsedAt: it.LastUsedAt, CreatedAt: it.CreatedAt,
		})
	}
	ok(c, gin.H{"items": out})
}

func (s *Server) handleDeleteSession(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "会话 ID 无效")
		return
	}
	if err := s.session.DestroyOne(middleware.UserID(c), uint(id)); err != nil {
		serverError(c, err)
		return
	}
	ok(c, gin.H{"ok": true})
}
