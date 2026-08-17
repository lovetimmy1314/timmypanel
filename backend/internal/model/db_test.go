package model

import (
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestResetPassword(t *testing.T) {
	db := openTestDB(t)

	hash, err := bcrypt.GenerateFromPassword([]byte("old-password"), BcryptCost)
	if err != nil {
		t.Fatal(err)
	}
	u := User{Username: "admin", PasswordHash: string(hash), Nickname: "admin", Role: RoleAdmin}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Session{
		UserID:     u.ID,
		TokenHash:  "deadbeef",
		ExpiresAt:  time.Now().Add(time.Hour),
		LastUsedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}

	plain, err := ResetPassword(db, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if !ValidPassword(plain) {
		t.Fatalf("生成的密码不合规: %q", plain)
	}
	if plain == "old-password" {
		t.Fatal("新密码不应等于旧密码")
	}

	var got User
	if err := db.First(&got, u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte(plain)) != nil {
		t.Fatal("新密码对不上库里的 hash")
	}
	if bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte("old-password")) == nil {
		t.Fatal("旧密码仍能通过")
	}

	var n int64
	if err := db.Model(&Session{}).Where("user_id = ?", u.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("应踢掉全部会话，还剩 %d", n)
	}
}

func TestResetPasswordMissingUser(t *testing.T) {
	db := openTestDB(t)
	if _, err := ResetPassword(db, "nobody"); err == nil {
		t.Fatal("不存在的用户应报错")
	}
	if _, err := ResetPassword(db, "  "); err == nil {
		t.Fatal("空用户名应报错")
	}
}
