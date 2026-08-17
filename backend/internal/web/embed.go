// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

// Package web 把构建好的前端静态资源嵌进二进制，实现单文件部署。
package web

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// dist 由 frontend 构建产出（vite build 的 outDir 指向这里）。
// 用 all: 前缀是为了把 Vite 生成的以下划线开头的目录也带上。
//
//go:embed all:dist
var dist embed.FS

// Register 挂载前端静态资源，并对未知路径回落到 index.html 交给前端路由。
func Register(r *gin.Engine) error {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return err
	}
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		return errors.New("前端资源缺失，请先在 frontend 目录执行 npm run build")
	}
	fileServer := http.FileServer(http.FS(sub))

	// serveIndex 直接把 index.html 写出去。这里不能用 http.FileServer 或
	// gin 的 FileFromFS —— 它们会把 /index.html 301 到目录根，
	// 于是 /admin 这类深链接会被重定向到 /，前端路由就丢了。
	serveIndex := func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	}

	r.NoRoute(func(c *gin.Context) {
		reqPath := c.Request.URL.Path
		// API 和上传路径不该走到这里，命中就是真的 404。
		if strings.HasPrefix(reqPath, "/api/") || strings.HasPrefix(reqPath, "/uploads/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
			return
		}
		clean := strings.TrimPrefix(path.Clean("/"+reqPath), "/")
		if clean == "" || clean == "index.html" {
			serveIndex(c)
			return
		}
		if f, err := sub.Open(clean); err == nil {
			f.Close()
			// 带内容哈希的资源可以长缓存，index.html 必须每次校验。
			if strings.HasPrefix(clean, "assets/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		// 带哈希的资源找不到就是真的没有（多半是 index.html 缓存过期了）。
		// 这里回落成 HTML 只会让浏览器报一个莫名其妙的 MIME 错误，不如直接 404。
		if strings.HasPrefix(clean, "assets/") {
			c.Status(http.StatusNotFound)
			return
		}
		// 前端是 SPA，其余深链接一律交给 index.html。
		serveIndex(c)
	})
	return nil
}
