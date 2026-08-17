// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

// Package middleware 提供鉴权、CSRF 防护和登录限流。
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"timmypanel/internal/model"
	"timmypanel/internal/session"
)

// gin context 中保存当前用户 / 会话的 key。
const (
	ctxUserKey    = "tp_user"
	ctxSessionKey = "tp_session"
)

// Auth 校验会话，未登录直接 401。
func Auth(mgr *session.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, sess, err := mgr.Validate(c)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录或登录已过期"})
			return
		}
		c.Set(ctxUserKey, u)
		c.Set(ctxSessionKey, sess)
		c.Next()
	}
}

// RequireAdmin 只放行管理员。
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil || u.Role != model.RoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
			return
		}
		c.Next()
	}
}

// CurrentUser 取出当前登录用户，未登录返回 nil。
func CurrentUser(c *gin.Context) *model.User {
	if v, ok := c.Get(ctxUserKey); ok {
		if u, ok := v.(*model.User); ok {
			return u
		}
	}
	return nil
}

// UserID 取当前用户 ID，未登录返回 0。
func UserID(c *gin.Context) uint {
	if u := CurrentUser(c); u != nil {
		return u.ID
	}
	return 0
}

// CurrentSession 取出当前请求对应的会话，未登录返回 nil。
func CurrentSession(c *gin.Context) *model.Session {
	if v, ok := c.Get(ctxSessionKey); ok {
		if s, ok := v.(*model.Session); ok {
			return s
		}
	}
	return nil
}

// CSRF 拦截跨站伪造请求。会话 Cookie 是 SameSite=Lax，跨站表单提交本就带不上，
// 这里再要求所有写操作带一个自定义头 —— 简单表单无法设置自定义头，
// 而带自定义头的跨域 XHR 会先发预检并被 CORS 挡住。
func CSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if strings.TrimSpace(c.GetHeader("X-Requested-With")) == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "缺少 X-Requested-With 请求头"})
			return
		}
		c.Next()
	}
}

// contentSecurityPolicy 是主站文档的 CSP。页面会渲染用户导入的标题和图标地址，
// XSS 面客观存在（见决策 002），这一层用来兜住「万一真被注入」。
//
// 各项的理由：
//   - script-src 'self'：不给内联脚本任何机会，这是最主要的一道。
//   - style-src 加 'unsafe-inline'：Naive UI 是 CSS-in-JS，运行时往
//     <style> 里注入样式，去掉这项整个界面会失去样式。
//   - img-src 放开 http/https/data：卡片图标允许填任意外链，也允许小体积
//     的 data URI 位图（sanitizeIconValue 已经挡掉了 svg+xml）。
//   - connect-src 只留 'self'：图标集已经打进前端产物，不再需要 iconify 的
//     api.iconify.design / api.simplesvg.com / api.unisvg.com 三个域（决策 019）。
//     **别因为图标不显示就把它们加回来** —— 那说明图标集没生成或没打进去，
//     去看 frontend/scripts/gen-icons.mjs 和 src/icons/index.ts。
//   - frame-ancestors 'self'：和 X-Frame-Options 同义，给支持 CSP 的浏览器用。
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: http: https:; " +
	"font-src 'self' data:; " +
	"connect-src 'self'; " +
	"object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'"

// SecurityHeaders 补一层基础安全响应头（反代层没配时的兜底）。
// 注意 /uploads/* 会用自己那条更严的 CSP 覆盖这里（见 handleServeUpload）。
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		c.Next()
	}
}
