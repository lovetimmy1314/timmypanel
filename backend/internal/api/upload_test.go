package api

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestUploadKind(t *testing.T) {
	cases := map[string]string{
		"/uploads/1/bg/abc.jpg":    "bg",
		"/uploads/12/icons/x.png":  "icons",
		"/uploads/1/other/x.png":   "",
		"/uploads/1/bg":            "",
		"/static/1/bg/x.png":       "",
		"":                         "",
		"uploads/3/icons/deep.png": "icons",
	}
	for in, want := range cases {
		if got := uploadKind(in); got != want {
			t.Errorf("uploadKind(%q) = %q，期望 %q", in, got, want)
		}
	}
}

// uploadDiskPath 是删文件前的安全判定：库里的 path 是历史数据，
// 不能直接 Join 到 os.Remove 上。
func TestUploadDiskPath(t *testing.T) {
	root := filepath.Join(string(filepath.Separator)+"data", "uploads")

	got, err := uploadDiskPath(root, "/uploads/7/bg/a.jpg", 7)
	if err != nil {
		t.Fatalf("合法路径被拒: %v", err)
	}
	if want, _ := filepath.Abs(filepath.Join(root, "7", "bg", "a.jpg")); got != want {
		t.Fatalf("uploadDiskPath = %q，期望 %q", got, want)
	}

	bad := []struct {
		rel string
		uid uint
	}{
		{"/uploads/8/bg/a.jpg", 7},          // 别人的目录
		{"/uploads/7/../8/bg/a.jpg", 7},     // 穿越到别人的目录
		{"/uploads/7/../../config.yaml", 7}, // 穿越出上传根目录
		{"/etc/passwd", 7},                  // 根本不在 uploads 下
		{"/uploads/7", 7},                   // 只到用户目录，没有文件
		{"", 7},
	}
	for _, tc := range bad {
		if p, err := uploadDiskPath(root, tc.rel, tc.uid); err == nil {
			t.Errorf("uploadDiskPath(%q, uid=%d) = %q，期望被拒", tc.rel, tc.uid, p)
		}
	}
}

// serveUploadPath 必须把 gin 通配段收成 /uploads/{uid}/...，再交给 uploadDiskPath。
// 老写法把 "/1/bg/a.jpg" 直接 Join 到 upload 根目录：Linux 上后一段是绝对路径，
// Join 丢掉根目录，前缀检查失败，所有图片 400。
func TestServeUploadPath(t *testing.T) {
	cases := map[string]string{
		"/1/bg/a.jpg":    "/uploads/1/bg/a.jpg",
		"1/bg/a.jpg":     "/uploads/1/bg/a.jpg",
		"/1/icons/x.png": "/uploads/1/icons/x.png",
		"/1/bg/../x.png": "/uploads/1/x.png",
	}
	for in, want := range cases {
		if got := serveUploadPath(in); got != want {
			t.Errorf("serveUploadPath(%q) = %q，期望 %q", in, got, want)
		}
	}

	root := filepath.Join(t.TempDir(), "uploads")
	got, err := uploadDiskPath(root, serveUploadPath("/7/bg/a.jpg"), 7)
	if err != nil {
		t.Fatalf("serve 路径应能还原成磁盘路径: %v", err)
	}
	if want, _ := filepath.Abs(filepath.Join(root, "7", "bg", "a.jpg")); got != want {
		t.Fatalf("还原结果 = %q，期望 %q", got, want)
	}

	// 钉死那个 Join 坑：带前导 / 的相对段在 Unix 上是绝对路径。
	if runtime.GOOS != "windows" {
		joined := filepath.Join(root, filepath.FromSlash("/7/bg/a.jpg"))
		if joined == got {
			t.Error("filepath.Join 吃掉绝对段这个坑如果消失了，serveUploadPath 的理由要重写")
		}
	}
}
