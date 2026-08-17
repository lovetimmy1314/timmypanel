// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// BcryptCost 是密码哈希强度。
const BcryptCost = 12

// Open 打开（并在需要时创建）sqlite 数据库，跑完自动迁移。
func Open(path string) (*gorm.DB, error) {
	// _pragma 参数由 glebarez/sqlite 透传给 modernc 驱动：
	// WAL 提升并发读写，busy_timeout 避免偶发 database is locked。
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("打开数据库: %w", err)
	}
	if err := db.AutoMigrate(&User{}, &Session{}, &Group{}, &Site{}, &Setting{}, &Upload{}, &SiteConfig{}, &IngestToken{}); err != nil {
		return nil, fmt.Errorf("数据库迁移: %w", err)
	}
	return db, nil
}

// EnsureInitialAdmin 在库中一个用户都没有时创建初始管理员。
// 返回 true 表示本次确实创建了账号（调用方据此打印初始密码）。
func EnsureInitialAdmin(db *gorm.DB, username, password string) (bool, error) {
	var count int64
	if err := db.Model(&User{}).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	if username == "" || password == "" {
		return false, errors.New("缺少初始管理员用户名或密码")
	}
	if !ValidPassword(password) {
		return false, errors.New("初始管理员密码需 8~72 字节")
	}
	if !ValidUsername(username) {
		return false, errors.New("初始管理员用户名需 2~32 位，以字母或数字开头，只能包含字母、数字和 _ . -")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return false, err
	}
	u := &User{Username: username, PasswordHash: string(hash), Nickname: username, Role: RoleAdmin}
	if err := db.Create(u).Error; err != nil {
		return false, err
	}
	return true, SeedUserData(db, u.ID)
}

// ResetPassword 给已有账号换一串随机密码，并踢掉该账号全部会话。
// 给 CLI -reset-password 用：库开着 WAL，容器不用停。返回明文新密码。
func ResetPassword(db *gorm.DB, username string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return "", errors.New("请指定要重置的用户名")
	}
	var u User
	res := db.Where("username = ?", username).Limit(1).Find(&u)
	if res.Error != nil {
		return "", res.Error
	}
	if res.RowsAffected == 0 {
		return "", fmt.Errorf("用户 %s 不存在", username)
	}
	plain, err := randomPassword()
	if err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), BcryptCost)
	if err != nil {
		return "", err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&User{}).Where("id = ?", u.ID).
			Update("password_hash", string(hash)).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", u.ID).Delete(&Session{}).Error
	})
	if err != nil {
		return "", err
	}
	return plain, nil
}

func randomPassword() (string, error) {
	b := make([]byte, 9)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SeedUserData 给新账号写入默认设置和一个默认分组。
func SeedUserData(db *gorm.DB, userID uint) error {
	data, err := Encode(DefaultSettings())
	if err != nil {
		return err
	}
	if err := db.Save(&Setting{UserID: userID, Data: data}).Error; err != nil {
		return err
	}
	var n int64
	if err := db.Model(&Group{}).Where("user_id = ?", userID).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return db.Create(&Group{UserID: userID, Name: "常用", Sort: 0}).Error
	}
	return nil
}

// StartSessionGC 定期清理过期会话，避免 sessions 表无限增长。
func StartSessionGC(db *gorm.DB) {
	go func() {
		for {
			res := db.Where("expires_at < ?", time.Now()).Delete(&Session{})
			if res.Error != nil {
				slog.Warn("清理过期会话失败", "err", res.Error)
			} else if res.RowsAffected > 0 {
				slog.Info("清理过期会话", "count", res.RowsAffected)
			}
			time.Sleep(time.Hour)
		}
	}()
}
