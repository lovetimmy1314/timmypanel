// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

// Package api 提供 HTTP 接口层。
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"timmypanel/internal/config"
	"timmypanel/internal/middleware"
	"timmypanel/internal/service"
	"timmypanel/internal/session"
)

// Server 持有各 handler 共用的依赖。
type Server struct {
	db      *gorm.DB
	cfg     *config.Config
	session *session.Manager
	limiter *middleware.LoginLimiter
	fetcher *service.Fetcher
	// ingestLimiter 限的是错误令牌的尝试，不是正常提交。
	ingestLimiter *middleware.LoginLimiter
	queues        ingestQueues
}

// NewServer 构造接口层。
func NewServer(db *gorm.DB, cfg *config.Config) *Server {
	return &Server{
		db:      db,
		cfg:     cfg,
		session: session.New(db, cfg.Auth.RememberDays, cfg.Server.Secure),
		limiter: middleware.NewLoginLimiter(5, 15*time.Minute, 15*time.Minute),
		// 令牌本身 64 位 hex 枚举不动，这个限流防的是拿泄露旧令牌反复试探。
		ingestLimiter: middleware.NewLoginLimiter(10, 15*time.Minute, 15*time.Minute),
		fetcher:       service.NewFetcher(cfg.Fetch.AllowPrivate, time.Duration(cfg.Fetch.TimeoutSec)*time.Second),
	}
}

// Register 挂载所有路由。
func (s *Server) Register(r *gin.Engine) {
	r.Use(middleware.SecurityHeaders())

	v1 := r.Group("/api/v1", middleware.CSRF())
	{
		v1.POST("/auth/login", s.handleLogin)
		v1.GET("/auth/config", s.handleAuthConfig)
		// 免登录：登录页的图标和背景。只吐实例配置里指向的那一个文件，
		// 不接受路径参数，所以不等于把 /uploads/* 放开。
		v1.GET("/auth/site-asset/:kind", s.handleSiteAsset)

		authed := v1.Group("", middleware.Auth(s.session))
		{
			authed.GET("/auth/me", s.handleMe)
			authed.POST("/auth/logout", s.handleLogout)
			authed.POST("/auth/password", s.handleChangePassword)
			authed.GET("/auth/sessions", s.handleListSessions)
			authed.DELETE("/auth/sessions/:id", s.handleDeleteSession)

			authed.GET("/groups", s.handleListGroups)
			authed.POST("/groups", s.handleCreateGroup)
			authed.PUT("/groups/:id", s.handleUpdateGroup)
			authed.DELETE("/groups/:id", s.handleDeleteGroup)
			authed.PATCH("/groups/sort", s.handleSortGroups)

			authed.GET("/sites", s.handleListSites)
			authed.POST("/sites", s.handleCreateSite)
			authed.PUT("/sites/:id", s.handleUpdateSite)
			authed.DELETE("/sites/:id", s.handleDeleteSite)
			authed.PATCH("/sites/sort", s.handleSortSites)
			authed.POST("/sites/batch", s.handleBatchCreateSites)
			authed.POST("/sites/parse", s.handleParseSites)
			authed.POST("/sites/bookmarks", s.handleParseBookmarks)
			authed.POST("/sites/backfill", s.handleBackfillSites)

			authed.GET("/settings", s.handleGetSettings)
			authed.PUT("/settings", s.handlePutSettings)

			authed.POST("/upload", s.handleUpload)
			authed.GET("/uploads", s.handleListUploads)
			authed.DELETE("/uploads/:id", s.handleDeleteUpload)

			authed.GET("/backup/export", s.handleExportBackup)
			authed.POST("/backup/import", s.handleImportBackup)

			// 收藏书签（浏览器反向上传）的令牌和待补队列，走会话。
			authed.GET("/ingest/token", s.handleGetIngestToken)
			authed.POST("/ingest/token", s.handleCreateIngestToken)
			authed.DELETE("/ingest/token", s.handleDeleteIngestToken)
			authed.PUT("/ingest/queue", s.handlePutIngestQueue)

			admin := authed.Group("/admin", middleware.RequireAdmin())
			{
				admin.GET("/users", s.handleAdminListUsers)
				admin.POST("/users", s.handleAdminCreateUser)
				admin.PUT("/users/:id", s.handleAdminUpdateUser)
				admin.DELETE("/users/:id", s.handleAdminDeleteUser)

				admin.GET("/site", s.handleGetSiteConfig)
				admin.PUT("/site", s.handlePutSiteConfig)
			}
		}
	}

	// 上传的图标和背景图同样要登录才能取 —— 同源 <img> 会自动带上会话 Cookie，
	// 所以前端无需特殊处理，但外人拿到 URL 也看不到。
	r.GET("/uploads/*path", middleware.Auth(s.session), s.handleServeUpload)

	// 浏览器书签的反向上传。故意挂在 CSRF 组之外：书签是在别人站点的上下文里
	// 执行的跨站 simple request，设不了 X-Requested-With；鉴权靠表单里的专属
	// 令牌而不是会话 Cookie，CSRF 防护对它没有意义（决策 026）。
	r.OPTIONS("/api/v1/ingest", s.handleIngestOptions)
	r.POST("/api/v1/ingest", s.handleIngest)
}

// ---- 通用响应助手 ----

func fail(c *gin.Context, code int, msg string) {
	c.AbortWithStatusJSON(code, gin.H{"error": msg})
}

func badRequest(c *gin.Context, msg string) { fail(c, http.StatusBadRequest, msg) }

func notFound(c *gin.Context) { fail(c, http.StatusNotFound, "资源不存在") }

// serverError 只给客户端一句通用文案，细节进日志 —— 原始错误里常带表名、
// 文件路径这类信息，公网部署时没必要吐给对面。
func serverError(c *gin.Context, err error) {
	slog.Error("请求处理失败", "method", c.Request.Method, "path", c.Request.URL.Path, "err", err)
	fail(c, http.StatusInternalServerError, "服务器错误，请稍后重试")
}

func ok(c *gin.Context, data any) { c.JSON(http.StatusOK, data) }
