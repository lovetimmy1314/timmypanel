// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"timmypanel/internal/middleware"
	"timmypanel/internal/model"
)

func (s *Server) handleAdminListUsers(c *gin.Context) {
	var users []model.User
	if err := s.db.Order("id asc").Find(&users).Error; err != nil {
		serverError(c, err)
		return
	}
	out := make([]gin.H, 0, len(users))
	for i := range users {
		var sites, groups int64
		s.db.Model(&model.Site{}).Where("user_id = ?", users[i].ID).Count(&sites)
		s.db.Model(&model.Group{}).Where("user_id = ?", users[i].ID).Count(&groups)
		out = append(out, gin.H{
			"id": users[i].ID, "username": users[i].Username, "nickname": users[i].Nickname,
			"role": users[i].Role, "disabled": users[i].Disabled, "createdAt": users[i].CreatedAt,
			"siteCount": sites, "groupCount": groups,
		})
	}
	ok(c, gin.H{"items": out})
}

type adminUserReq struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Password2 string `json:"password2"`
	Nickname  string `json:"nickname"`
	Role      string `json:"role"`
	Disabled  bool   `json:"disabled"`
}

func (s *Server) handleAdminCreateUser(c *gin.Context) {
	var req adminUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求格式错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !model.ValidUsername(req.Username) {
		badRequest(c, "用户名需 2~32 位，以字母或数字开头，只能包含字母、数字和 _ . -")
		return
	}
	if req.Password != req.Password2 {
		badRequest(c, "两次输入的密码不一致")
		return
	}
	if !model.ValidPassword(req.Password) {
		badRequest(c, "密码至少 8 位，且不能超过 72 字节")
		return
	}
	role := req.Role
	if role != model.RoleAdmin {
		role = model.RoleUser
	}
	var n int64
	s.db.Model(&model.User{}).Where("username = ?", req.Username).Count(&n)
	if n > 0 {
		badRequest(c, "用户名已存在")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), model.BcryptCost)
	if err != nil {
		serverError(c, err)
		return
	}
	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		nickname = req.Username
	}
	u := model.User{Username: req.Username, PasswordHash: string(hash), Nickname: nickname, Role: role}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&u).Error; err != nil {
			return err
		}
		return model.SeedUserData(tx, u.ID)
	})
	if err != nil {
		serverError(c, err)
		return
	}
	ok(c, toUserDTO(&u))
}

func (s *Server) handleAdminUpdateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "用户 ID 无效")
		return
	}
	var req adminUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求格式错误")
		return
	}
	var u model.User
	if err := s.db.First(&u, id).Error; err != nil {
		notFound(c)
		return
	}
	me := middleware.CurrentUser(c)

	// 管理员不能把自己降级或停用，否则可能把最后一个管理员锁在门外。
	if u.ID == me.ID && (req.Role == model.RoleUser || req.Disabled) {
		badRequest(c, "不能降级或停用当前登录的管理员账号")
		return
	}
	if nickname := strings.TrimSpace(req.Nickname); nickname != "" {
		u.Nickname = nickname
	}
	if req.Role == model.RoleAdmin || req.Role == model.RoleUser {
		u.Role = req.Role
	}
	u.Disabled = req.Disabled
	if req.Password != "" {
		if req.Password != req.Password2 {
			badRequest(c, "两次输入的密码不一致")
			return
		}
		if !model.ValidPassword(req.Password) {
			badRequest(c, "密码至少 8 位，且不能超过 72 字节")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), model.BcryptCost)
		if err != nil {
			serverError(c, err)
			return
		}
		u.PasswordHash = string(hash)
		// 改了别人的密码就把他的会话全清掉。
		if err := s.session.DestroyAllForUser(u.ID); err != nil {
			serverError(c, err)
			return
		}
	}
	if u.Disabled {
		_ = s.session.DestroyAllForUser(u.ID)
	}
	if err := s.db.Save(&u).Error; err != nil {
		serverError(c, err)
		return
	}
	ok(c, toUserDTO(&u))
}

// handleAdminDeleteUser 删除账号，连带清掉他的全部数据和上传文件。
func (s *Server) handleAdminDeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "用户 ID 无效")
		return
	}
	uid := uint(id)
	if uid == middleware.UserID(c) {
		fail(c, http.StatusBadRequest, "不能删除当前登录的账号")
		return
	}
	var count int64
	s.db.Model(&model.User{}).Where("role = ? AND id <> ?", model.RoleAdmin, uid).Count(&count)
	var target model.User
	if err := s.db.First(&target, uid).Error; err != nil {
		notFound(c)
		return
	}
	if target.Role == model.RoleAdmin && count == 0 {
		badRequest(c, "至少要保留一个管理员")
		return
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		for _, m := range []any{&model.Site{}, &model.Group{}, &model.Setting{}, &model.Upload{}, &model.Session{}, &model.IngestToken{}} {
			if err := tx.Where("user_id = ?", uid).Delete(m).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&model.User{}, uid).Error
	})
	if err != nil {
		serverError(c, err)
		return
	}
	_ = os.RemoveAll(filepath.Join(s.cfg.UploadDir(), strconv.FormatUint(uint64(uid), 10)))
	ok(c, gin.H{"ok": true})
}
