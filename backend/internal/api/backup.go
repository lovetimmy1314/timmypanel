// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"timmypanel/internal/middleware"
	"timmypanel/internal/model"
	"timmypanel/internal/service"
)

// BackupVersion 是导出格式版本号，以后改结构时用来做兼容。
const BackupVersion = 1

// BackupFile 是导出的 JSON 结构。不含密码等敏感信息。
type BackupFile struct {
	Version    int            `json:"version"`
	ExportedAt time.Time      `json:"exportedAt"`
	Username   string         `json:"username"`
	Groups     []backupGroup  `json:"groups"`
	Sites      []backupSite   `json:"sites"`
	Settings   model.Settings `json:"settings"`
}

type backupGroup struct {
	Name      string `json:"name"`
	Sort      int    `json:"sort"`
	Collapsed bool   `json:"collapsed"`
}

type backupSite struct {
	GroupName   string `json:"groupName"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	LanURL      string `json:"lanUrl"`
	Description string `json:"description"`
	IconType    string `json:"iconType"`
	IconValue   string `json:"iconValue"`
	IconBg      string `json:"iconBg"`
	OpenMode    string `json:"openMode"`
	Sort        int    `json:"sort"`
	Hidden      bool   `json:"hidden"`
}

// buildBackup 把某个用户的全部数据组装成可导出的结构。
// 分组用名字而不是 ID 引用，这样导入到别的账号里也能正确还原归属。
func (s *Server) buildBackup(uid uint) (*BackupFile, error) {
	var user model.User
	if err := s.db.First(&user, uid).Error; err != nil {
		return nil, err
	}
	var groups []model.Group
	if err := s.db.Where("user_id = ?", uid).Order("sort asc, id asc").Find(&groups).Error; err != nil {
		return nil, err
	}
	var sites []model.Site
	if err := s.db.Where("user_id = ?", uid).Order("sort asc, id asc").Find(&sites).Error; err != nil {
		return nil, err
	}
	nameByID := map[uint]string{}
	for _, g := range groups {
		nameByID[g.ID] = g.Name
	}

	bf := &BackupFile{
		Version:    BackupVersion,
		ExportedAt: time.Now(),
		Username:   user.Username,
		Settings:   s.loadSettings(uid),
	}
	for _, g := range groups {
		bf.Groups = append(bf.Groups, backupGroup{Name: g.Name, Sort: g.Sort, Collapsed: g.Collapsed})
	}
	for _, it := range sites {
		bf.Sites = append(bf.Sites, backupSite{
			GroupName: nameByID[it.GroupID], Title: it.Title, URL: it.URL, LanURL: it.LanURL,
			Description: it.Description, IconType: it.IconType, IconValue: it.IconValue,
			IconBg: it.IconBg, OpenMode: it.OpenMode, Sort: it.Sort, Hidden: it.Hidden,
		})
	}
	return bf, nil
}

// handleExportBackup 导出备份。withFiles=1 时连同上传的图片打成 zip。
func (s *Server) handleExportBackup(c *gin.Context) {
	uid := middleware.UserID(c)
	bf, err := s.buildBackup(uid)
	if err != nil {
		serverError(c, err)
		return
	}
	stamp := time.Now().Format("20060102-150405")

	if c.Query("withFiles") != "1" {
		data, err := json.MarshalIndent(bf, "", "  ")
		if err != nil {
			serverError(c, err)
			return
		}
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="timmypanel-%s.json"`, stamp))
		c.Data(http.StatusOK, "application/json; charset=utf-8", data)
		return
	}

	// 先打完整包再回 200。边写边回的话打包中途失败，浏览器已经收下
	// 半截 zip，导入时才发现坏了。
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("data.json")
	if err != nil {
		serverError(c, err)
		return
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(bf); err != nil {
		serverError(c, err)
		return
	}
	userDir := filepath.Join(s.cfg.UploadDir(), strconv.FormatUint(uint64(uid), 10))
	if err := addDirToZip(zw, userDir, "uploads"); err != nil {
		serverError(c, err)
		return
	}
	if err := zw.Close(); err != nil {
		serverError(c, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="timmypanel-%s.zip"`, stamp))
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}

// addDirToZip 把目录递归塞进 zip，目录不存在时静默跳过。
func addDirToZip(zw *zip.Writer, dir, prefix string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	return filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		w, err := zw.Create(prefix + "/" + filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
}

// handleImportBackup 导入备份。mode=merge 追加去重，mode=overwrite 先清空当前账号数据。
// 无论哪种模式，导入前都会先在 data/backups 落一份当前数据的快照。
func (s *Server) handleImportBackup(c *gin.Context) {
	mode := c.DefaultQuery("mode", "merge")
	if mode != "merge" && mode != "overwrite" {
		badRequest(c, "导入模式无效")
		return
	}
	uid := middleware.UserID(c)

	var bf BackupFile
	assets, err := s.readBackupPayload(c, &bf)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	if bf.Version > BackupVersion {
		badRequest(c, "备份文件版本较新，请先升级 Timmypanel")
		return
	}

	snap, err := s.snapshot(uid, "before-import")
	if err != nil {
		// 覆盖导入会先删库。快照没落下还继续，等于拆了安全网再动手。
		slog.Error("导入前快照失败，已中止", "err", err)
		fail(c, http.StatusInternalServerError, "导入前备份失败，已中止以免丢数据")
		return
	}
	slog.Info("导入前已自动快照", "path", snap)

	if n, err := s.restoreBackupAssets(uid, assets); err != nil {
		serverError(c, err)
		return
	} else if n > 0 {
		slog.Info("已从备份还原上传文件", "count", n)
	}

	var imported, skipped int
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if mode == "overwrite" {
			if err := tx.Where("user_id = ?", uid).Delete(&model.Site{}).Error; err != nil {
				return err
			}
			if err := tx.Where("user_id = ?", uid).Delete(&model.Group{}).Error; err != nil {
				return err
			}
		}

		existing := map[string]bool{}
		if mode == "merge" {
			var urls []string
			tx.Model(&model.Site{}).Where("user_id = ?", uid).Pluck("url", &urls)
			for _, u := range urls {
				existing[canonicalURL(u)] = true
			}
		}

		groupIDs := map[string]uint{}
		for _, g := range bf.Groups {
			name := strings.TrimSpace(g.Name)
			if name == "" {
				continue
			}
			id, err := resolveGroup(tx, uid, name)
			if err != nil {
				return err
			}
			// merge 不能改已有分组的 sort/collapsed：用户本地的折叠和排序是自己排的，
			// 合入一份备份不该把分组条打乱。overwrite 刚删光重建，必须按备份还原。
			if mode == "overwrite" {
				if err := tx.Model(&model.Group{}).Where("id = ?", id).
					Updates(map[string]any{"sort": g.Sort, "collapsed": g.Collapsed}).Error; err != nil {
					return err
				}
			}
			groupIDs[name] = id
		}

		for _, it := range bf.Sites {
			u, err := normalizeURL(it.URL)
			if err != nil || u == "" {
				continue
			}
			if existing[canonicalURL(u)] {
				skipped++
				continue
			}
			existing[canonicalURL(u)] = true

			gid := uint(0)
			if name := strings.TrimSpace(it.GroupName); name != "" {
				if cached, hit := groupIDs[name]; hit {
					gid = cached
				} else {
					resolved, err := resolveGroup(tx, uid, name)
					if err != nil {
						return err
					}
					groupIDs[name] = resolved
					gid = resolved
				}
			}
			iconType := it.IconType
			switch iconType {
			case model.IconTypeURL, model.IconTypeIconify, model.IconTypeText:
			default:
				iconType = model.IconTypeURL
			}
			title := strings.TrimSpace(it.Title)
			if title == "" {
				title = hostOf(u)
			}
			openMode := it.OpenMode
			if openMode != "self" {
				openMode = "blank"
			}
			lan := ""
			if strings.TrimSpace(it.LanURL) != "" {
				if v, err := normalizeURL(it.LanURL); err == nil {
					lan = v
				}
			}
			site := model.Site{
				UserID: uid, GroupID: gid, Title: title, URL: u, LanURL: lan,
				Description: strings.TrimSpace(it.Description), IconType: iconType,
				IconValue: sanitizeIconValue(iconType, remapOwnUploadPath(it.IconValue, uid)),
				IconBg:    sanitizeIconBg(it.IconBg),
				OpenMode:  openMode, Sort: it.Sort, Hidden: it.Hidden,
			}
			if err := tx.Create(&site).Error; err != nil {
				return err
			}
			imported++
		}

		if c.Query("withSettings") != "0" {
			st := bf.Settings
			st.Background.Value = remapOwnUploadPath(st.Background.Value, uid)
			normalizeSettings(&st)
			data, err := model.Encode(st)
			if err != nil {
				return err
			}
			if err := tx.Save(&model.Setting{UserID: uid, Data: data}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		serverError(c, err)
		return
	}
	ok(c, gin.H{"imported": imported, "skipped": skipped, "mode": mode})
}

// backupAsset 是 zip 里 uploads/ 树的一条文件。Rel 是剥掉 uploads/ 前缀后的相对路径
// （形如 bg/foo.jpg），内容稍后按嗅探结果落盘。
type backupAsset struct {
	Rel  string
	Data []byte
}

const (
	maxBackupJSONBytes  = 32 << 20
	maxBackupAssetBytes = 8 << 20
	maxBackupAssets     = 500
)

// readBackupPayload 支持三种上传方式：JSON body、上传 .json 文件、上传 .zip 备份包。
// zip 里的 uploads/ 树一并读出，调用方再按当前用户 ID 还原。
func (s *Server) readBackupPayload(c *gin.Context, bf *BackupFile) ([]backupAsset, error) {
	ct := c.ContentType()
	if strings.Contains(ct, "application/json") {
		return nil, json.NewDecoder(io.LimitReader(c.Request.Body, maxBackupJSONBytes)).Decode(bf)
	}

	file, err := c.FormFile("file")
	if err != nil {
		return nil, fmt.Errorf("请上传备份文件")
	}
	if file.Size > 64<<20 {
		return nil, fmt.Errorf("备份文件不能超过 64MB")
	}
	f, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if strings.HasSuffix(strings.ToLower(file.Filename), ".zip") {
		zr, err := zip.NewReader(f, file.Size)
		if err != nil {
			return nil, fmt.Errorf("zip 解析失败: %w", err)
		}
		return readBackupZip(zr, bf)
	}
	return nil, json.NewDecoder(io.LimitReader(f, maxBackupJSONBytes)).Decode(bf)
}

func readBackupZip(zr *zip.Reader, bf *BackupFile) ([]backupAsset, error) {
	var assets []backupAsset
	var foundJSON bool
	for _, entry := range zr.File {
		name := path.Clean("/" + strings.ReplaceAll(entry.Name, `\`, "/"))
		name = strings.TrimPrefix(name, "/")
		if entry.FileInfo().IsDir() {
			continue
		}
		if name == "data.json" {
			rc, err := entry.Open()
			if err != nil {
				return nil, err
			}
			err = json.NewDecoder(io.LimitReader(rc, maxBackupJSONBytes)).Decode(bf)
			rc.Close()
			if err != nil {
				return nil, err
			}
			foundJSON = true
			continue
		}
		rel, ok := zipUploadRel(name)
		if !ok {
			continue
		}
		if entry.UncompressedSize64 > maxBackupAssetBytes {
			slog.Warn("备份里的图片过大，已跳过", "name", name, "size", entry.UncompressedSize64)
			continue
		}
		if len(assets) >= maxBackupAssets {
			return nil, fmt.Errorf("zip 里图片超过 %d 张", maxBackupAssets)
		}
		rc, err := entry.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(rc, maxBackupAssetBytes+1))
		rc.Close()
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > maxBackupAssetBytes {
			slog.Warn("备份里的图片过大，已跳过", "name", name)
			continue
		}
		assets = append(assets, backupAsset{Rel: rel, Data: data})
	}
	if !foundJSON {
		return nil, fmt.Errorf("zip 包里没有找到 data.json")
	}
	return assets, nil
}

// zipUploadRel 只收 uploads/{bg|icons}/{文件名}。导出时用户目录已经摊平到
// uploads/ 下，所以这里不再认中间那一段 uid。带 ..、多层目录、其它 kind 一律丢掉。
func zipUploadRel(name string) (string, bool) {
	if !strings.HasPrefix(name, "uploads/") {
		return "", false
	}
	rel := strings.TrimPrefix(name, "uploads/")
	if rel == "" || strings.Contains(rel, "..") {
		return "", false
	}
	segs := strings.Split(rel, "/")
	if len(segs) != 2 {
		return "", false
	}
	if segs[0] != "bg" && segs[0] != "icons" {
		return "", false
	}
	if segs[1] == "" || strings.ContainsAny(segs[1], `"'<>?#`) {
		return "", false
	}
	return rel, true
}

// restoreBackupAssets 把 zip 里的图片落到当前用户目录，并补 Upload 记录。
// 按内容嗅探类型，不信任 zip 里的文件名；文件名只用来决定落到 bg 还是 icons。
func (s *Server) restoreBackupAssets(uid uint, assets []backupAsset) (int, error) {
	if len(assets) == 0 {
		return 0, nil
	}
	n := 0
	for _, a := range assets {
		if err := s.restoreOneAsset(uid, a); err != nil {
			slog.Warn("还原备份图片失败", "rel", a.Rel, "err", err)
			continue
		}
		n++
	}
	return n, nil
}

func (s *Server) restoreOneAsset(uid uint, a backupAsset) error {
	// 只按内容验是不是图，文件名沿用 zip 里的 —— 卡片 JSON 引用的就是这个名字。
	_, mime, err := service.SniffStoredImage(a.Data)
	if err != nil {
		return err
	}
	rel := "/uploads/" + strconv.FormatUint(uint64(uid), 10) + "/" + a.Rel
	abs, err := uploadDiskPath(s.cfg.UploadDir(), rel, uid)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(abs, a.Data, 0o644); err != nil {
		return err
	}
	up := model.Upload{UserID: uid, Path: rel, Mime: mime, Size: int64(len(a.Data))}
	return s.db.Where("user_id = ? AND path = ?", uid, rel).FirstOrCreate(&up).Error
}

// remapOwnUploadPath 把备份里指向旧账号上传目录的路径改到当前用户名下。
// 跨账号导入时 uid 会变；不改的话卡片图标指着别人的目录，鉴权后全是 404。
func remapOwnUploadPath(v string, uid uint) string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "/uploads/") {
		return v
	}
	clean := path.Clean("/" + strings.TrimPrefix(v, "/"))
	segs := strings.Split(strings.TrimPrefix(clean, "/"), "/")
	if len(segs) != 4 || segs[0] != "uploads" {
		return v
	}
	if _, err := strconv.ParseUint(segs[1], 10, 64); err != nil {
		return v
	}
	if segs[2] != "bg" && segs[2] != "icons" {
		return v
	}
	if segs[3] == "" {
		return v
	}
	return "/uploads/" + strconv.FormatUint(uint64(uid), 10) + "/" + segs[2] + "/" + segs[3]
}

// snapshot 把某账号当前数据导出到 data/backups，用于导入前兜底和每日自动备份。
func (s *Server) snapshot(uid uint, tag string) (string, error) {
	bf, err := s.buildBackup(uid)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(bf, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.cfg.BackupDir(), 0o755); err != nil {
		return "", err
	}
	// 真正的防线在这里，而不是建号时的 model.ValidUsername —— 那条只管新账号，
	// 老库里可能已经存着带 / 或 .. 的用户名，照搬进 filepath.Join 就写出 data 目录了。
	name := fmt.Sprintf("%s-%s-%s.json", safeFileSegment(bf.Username), tag, time.Now().Format("20060102-150405"))
	full := filepath.Join(s.cfg.BackupDir(), name)
	if err := os.WriteFile(full, data, 0o600); err != nil {
		return "", err
	}
	return full, nil
}

// safeFileSegment 把任意字符串压成能安全用作文件名的一段：字母数字和 _ . - 原样保留，
// 其余一律换成 _，再削掉开头的点（挡住 ".."、".ssh" 这类）。空串回落到 "user"。
func safeFileSegment(s string) string {
	mapped := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '_', r == '-', r == '.':
			return r
		default:
			return '_'
		}
	}, s)
	mapped = strings.TrimLeft(mapped, ".")
	if mapped == "" {
		return "user"
	}
	return mapped
}

// StartAutoBackup 每天给所有账号导出一份快照，只保留最近 keep 份。
func (s *Server) StartAutoBackup() {
	if !s.cfg.Backup.AutoDaily {
		return
	}
	go func() {
		for {
			var users []model.User
			if err := s.db.Find(&users).Error; err != nil {
				slog.Warn("自动备份读取用户失败", "err", err)
			}
			for _, u := range users {
				if _, err := s.snapshot(u.ID, "auto"); err != nil {
					slog.Warn("自动备份失败", "user", u.Username, "err", err)
				}
			}
			s.pruneBackups()
			time.Sleep(24 * time.Hour)
		}
	}()
}

// pruneBackups 按文件名时间倒序保留最近 N 份自动备份，手动快照不动。
func (s *Server) pruneBackups() {
	entries, err := os.ReadDir(s.cfg.BackupDir())
	if err != nil {
		return
	}
	// 按账号分别保留，否则用户多的时候会互相挤掉对方的备份。
	byUser := map[string][]string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		// 时间戳里不含 "-auto-"，所以最后一个才是用户名和 tag 的真正分界；
		// 用 Index 的话，名字里带 "-auto-" 的账号会被截断并和别人的备份混到一组。
		idx := strings.LastIndex(name, "-auto-")
		if idx < 0 {
			continue
		}
		user := name[:idx]
		byUser[user] = append(byUser[user], name)
	}
	for _, names := range byUser {
		sort.Sort(sort.Reverse(sort.StringSlice(names)))
		for i := s.cfg.Backup.Keep; i < len(names); i++ {
			_ = os.Remove(filepath.Join(s.cfg.BackupDir(), names[i]))
		}
	}
}
