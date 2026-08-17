// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"timmypanel/internal/middleware"
	"timmypanel/internal/model"
)

const maxUploadBytes = 8 << 20 // 8MB，背景图够用了

// 只接受这几种位图。SVG 能内嵌脚本，同源访问时是实打实的 XSS 面，直接不收。
var allowedUploadTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// handleUpload 接收图片上传（背景图 / 自定义图标）。
func (s *Server) handleUpload(c *gin.Context) {
	kind := c.DefaultPostForm("kind", "bg")
	if kind != "bg" && kind != "icons" {
		badRequest(c, "上传类型无效")
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		badRequest(c, "请选择要上传的图片")
		return
	}
	if file.Size > maxUploadBytes {
		badRequest(c, "图片不能超过 8MB")
		return
	}
	f, err := file.Open()
	if err != nil {
		serverError(c, err)
		return
	}
	defer f.Close()

	// 以文件实际内容判定类型，不信任客户端给的 Content-Type 和扩展名。
	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	mime := http.DetectContentType(head[:n])
	mime = strings.Split(mime, ";")[0]
	ext, allowed := allowedUploadTypes[mime]
	if !allowed {
		badRequest(c, "只支持 PNG / JPG / WebP / GIF 图片")
		return
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		serverError(c, err)
		return
	}

	uid := middleware.UserID(c)
	dir := filepath.Join(s.cfg.UploadDir(), strconv.FormatUint(uint64(uid), 10), kind)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		serverError(c, err)
		return
	}
	name := randomName() + ext
	dst, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		serverError(c, err)
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, io.LimitReader(f, maxUploadBytes))
	if err != nil {
		serverError(c, err)
		return
	}

	rel := "/uploads/" + strconv.FormatUint(uint64(uid), 10) + "/" + kind + "/" + name
	if err := s.db.Create(&model.Upload{UserID: uid, Path: rel, Mime: mime, Size: written}).Error; err != nil {
		serverError(c, err)
		return
	}
	ok(c, gin.H{"path": rel, "size": written, "mime": mime})
}

// handleServeUpload 提供上传文件的读取。走鉴权中间件，所以外人拿到 URL 也取不到；
// 同源 <img> 会自动带上会话 Cookie，前端无需特殊处理。
func (s *Server) handleServeUpload(c *gin.Context) {
	rel := serveUploadPath(c.Param("path"))
	// path.Clean 已经消化了 ..，这里再确认一次，避免任何形式的目录穿越。
	if strings.Contains(rel, "..") {
		fail(c, http.StatusBadRequest, "路径非法")
		return
	}
	// 上传目录按 /{userID}/... 划分，只允许读自己的那一份。
	uid := middleware.UserID(c)
	segs := strings.SplitN(strings.TrimPrefix(rel, "/uploads/"), "/", 2)
	if len(segs) < 2 || segs[0] != strconv.FormatUint(uint64(uid), 10) {
		notFound(c)
		return
	}
	// 必须走 uploadDiskPath：它先剥掉 /uploads/ 再 Join。
	// 这里若把 gin 通配段（带前导 /）直接丢给 filepath.Join，Linux 上后一段是
	// 绝对路径，Join 会丢掉 upload 根目录，前缀检查失败，所有图片 400。
	// Windows 开发机看不出来。
	abs, err := uploadDiskPath(s.cfg.UploadDir(), rel, uid)
	if err != nil {
		fail(c, http.StatusBadRequest, "路径非法")
		return
	}
	if st, err := os.Stat(abs); err != nil || st.IsDir() {
		notFound(c)
		return
	}
	// 抓来的图标里可能有 SVG，而 SVG 能内嵌脚本。放在 <img> 里本来就不会执行，
	// 危险的是有人直接导航到这个 URL —— CSP sandbox 把这种情况也堵死了。
	c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, max-age=86400")
	c.File(abs)
}

// uploadDTO 是图库列表对外的形状。kind 从 path 解析而不是存字段，
// 免得为了一个能算出来的值动表结构。
type uploadDTO struct {
	ID        uint      `json:"id"`
	Path      string    `json:"path"`
	Kind      string    `json:"kind"`
	Mime      string    `json:"mime"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
}

// serveUploadPath 把 /uploads/* 的通配段收成 uploadDiskPath 能认的库路径。
// gin 给的 *path 带前导 /（/1/bg/foo.png），不能直接 Join 到 upload 根目录。
func serveUploadPath(param string) string {
	rel := path.Clean("/" + strings.TrimPrefix(param, "/"))
	return "/uploads" + rel
}

// uploadKind 从 /uploads/{uid}/{kind}/{name} 里取出 kind。
// 认不出来的返回空串（历史数据或手工塞进库的记录）。
func uploadKind(rel string) string {
	segs := strings.Split(strings.TrimPrefix(rel, "/"), "/")
	if len(segs) < 4 || segs[0] != "uploads" {
		return ""
	}
	switch segs[2] {
	case "bg", "icons":
		return segs[2]
	}
	return ""
}

// uploadDiskPath 把库里的 /uploads/{uid}/... 还原成磁盘绝对路径，并确认它确实
// 落在该用户自己的上传目录里。库里的 path 是历史写进去的数据，不能直接 Join 到
// os.Remove 上 —— 一条脏记录（比如 path 是 /uploads/1/../../config.yaml）
// 就能删到上传目录外面去。
func uploadDiskPath(root, rel string, uid uint) (string, error) {
	clean := path.Clean("/" + strings.TrimPrefix(rel, "/"))
	prefix := "/uploads/" + strconv.FormatUint(uint64(uid), 10) + "/"
	if !strings.HasPrefix(clean, prefix) {
		return "", errors.New("上传路径不属于该用户")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(filepath.Join(absRoot, filepath.FromSlash(strings.TrimPrefix(clean, "/uploads/"))))
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(abs, absRoot+string(os.PathSeparator)) {
		return "", errors.New("上传路径越界")
	}
	return abs, nil
}

// handleListUploads 列出当前用户的图库。?kind=bg|icons 可筛选。
func (s *Server) handleListUploads(c *gin.Context) {
	var rows []model.Upload
	if err := s.scoped(c).Order("id desc").Find(&rows).Error; err != nil {
		serverError(c, err)
		return
	}
	want := c.Query("kind")
	items := make([]uploadDTO, 0, len(rows))
	for _, r := range rows {
		kind := uploadKind(r.Path)
		if want != "" && kind != want {
			continue
		}
		items = append(items, uploadDTO{
			ID: r.ID, Path: r.Path, Kind: kind, Mime: r.Mime, Size: r.Size, CreatedAt: r.CreatedAt,
		})
	}
	ok(c, gin.H{"items": items})
}

// handleDeleteUpload 删除一张图库图片：先按归属查记录，再删磁盘文件和库记录。
// 不去扫卡片和设置里有没有引用这张图 —— 断掉的图标前端已有 @error 回落。
func (s *Server) handleDeleteUpload(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "文件 ID 无效")
		return
	}
	var row model.Upload
	if err := s.scoped(c).First(&row, id).Error; err != nil {
		notFound(c)
		return
	}
	if abs, err := uploadDiskPath(s.cfg.UploadDir(), row.Path, row.UserID); err != nil {
		// 路径不可信就只删库记录，别拿它去动磁盘。
		slog.Warn("图库记录的路径不可用，只删记录", "id", row.ID, "path", row.Path, "err", err)
	} else if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
		serverError(c, err)
		return
	}
	if err := s.db.Delete(&model.Upload{}, row.ID).Error; err != nil {
		serverError(c, err)
		return
	}
	ok(c, gin.H{"ok": true})
}

func randomName() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
