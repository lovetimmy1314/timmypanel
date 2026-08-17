// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package api

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"timmypanel/internal/middleware"
	"timmypanel/internal/model"
)

// scoped 返回一个已经限定在当前用户下的查询。所有涉及用户数据的读写都必须
// 从这里开始，避免某个 handler 忘记加 user_id 造成越权。
func (s *Server) scoped(c *gin.Context) *gorm.DB {
	return s.db.Where("user_id = ?", middleware.UserID(c))
}

// ownsGroup 确认分组属于该用户。任何接受外部传入 groupId 的写操作都必须先过这里，
// 否则能把卡片挂到别人的分组 ID 上（数据不会泄露，但会变成前端找不到的孤儿卡片）。
// groupID 为 0 表示「未分组」，是合法值。
func (s *Server) ownsGroup(uid, groupID uint) bool {
	if groupID == 0 {
		return true
	}
	var n int64
	s.db.Model(&model.Group{}).Where("id = ? AND user_id = ?", groupID, uid).Count(&n)
	return n > 0
}

func (s *Server) handleListGroups(c *gin.Context) {
	var groups []model.Group
	if err := s.scoped(c).Order("sort asc, id asc").Find(&groups).Error; err != nil {
		serverError(c, err)
		return
	}
	ok(c, gin.H{"items": groups})
}

// Collapsed 用指针：前端重命名时只发 {name}，用 bool 的话零值会把折叠状态
// 悄悄清掉。nil 表示「这次不改这个字段」。
type groupReq struct {
	Name      string `json:"name"`
	Collapsed *bool  `json:"collapsed"`
}

func (s *Server) handleCreateGroup(c *gin.Context) {
	var req groupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求格式错误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		badRequest(c, "分组名不能为空")
		return
	}
	uid := middleware.UserID(c)

	var maxSort struct{ M int }
	s.db.Model(&model.Group{}).Where("user_id = ?", uid).
		Select("COALESCE(MAX(sort), -1) as m").Scan(&maxSort)

	g := model.Group{UserID: uid, Name: req.Name, Sort: maxSort.M + 1}
	if req.Collapsed != nil {
		g.Collapsed = *req.Collapsed
	}
	if err := s.db.Create(&g).Error; err != nil {
		serverError(c, err)
		return
	}
	ok(c, g)
}

func (s *Server) handleUpdateGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "分组 ID 无效")
		return
	}
	var req groupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求格式错误")
		return
	}
	var g model.Group
	if err := s.scoped(c).First(&g, id).Error; err != nil {
		notFound(c)
		return
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		g.Name = name
	}
	if req.Collapsed != nil {
		g.Collapsed = *req.Collapsed
	}
	if err := s.db.Save(&g).Error; err != nil {
		serverError(c, err)
		return
	}
	ok(c, g)
}

// handleDeleteGroup 删除分组；组内卡片按 mode 决定是一起删还是挪到未分组。
func (s *Server) handleDeleteGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "分组 ID 无效")
		return
	}
	uid := middleware.UserID(c)
	var g model.Group
	if err := s.scoped(c).First(&g, id).Error; err != nil {
		notFound(c)
		return
	}
	withSites := c.Query("mode") == "cascade"

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if withSites {
			if err := tx.Where("user_id = ? AND group_id = ?", uid, g.ID).Delete(&model.Site{}).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Model(&model.Site{}).Where("user_id = ? AND group_id = ?", uid, g.ID).
				Update("group_id", 0).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&g).Error
	})
	if err != nil {
		serverError(c, err)
		return
	}
	ok(c, gin.H{"ok": true})
}

type sortItem struct {
	ID      uint `json:"id"`
	GroupID uint `json:"groupId"`
	Sort    int  `json:"sort"`
}

type sortReq struct {
	Items []sortItem `json:"items"`
}

func (s *Server) handleSortGroups(c *gin.Context) {
	var req sortReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求格式错误")
		return
	}
	uid := middleware.UserID(c)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, it := range req.Items {
			if err := tx.Model(&model.Group{}).Where("id = ? AND user_id = ?", it.ID, uid).
				Update("sort", it.Sort).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		serverError(c, err)
		return
	}
	ok(c, gin.H{"ok": true})
}

// resolveGroup 按名字查分组，没有就建一个。批量导入时用来还原书签目录结构。
func resolveGroup(tx *gorm.DB, uid uint, name string) (uint, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, nil
	}
	var g model.Group
	err := tx.Where("user_id = ? AND name = ?", uid, name).First(&g).Error
	if err == nil {
		return g.ID, nil
	}
	if err != gorm.ErrRecordNotFound {
		return 0, err
	}
	var maxSort struct{ M int }
	tx.Model(&model.Group{}).Where("user_id = ?", uid).
		Select("COALESCE(MAX(sort), -1) as m").Scan(&maxSort)
	g = model.Group{UserID: uid, Name: name, Sort: maxSort.M + 1}
	if err := tx.Create(&g).Error; err != nil {
		return 0, err
	}
	return g.ID, nil
}
