package api

import (
	"archive/zip"
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// safeFileSegment 是自动备份写盘前的最后一道防线：老库里可能存着建号校验
// 上线前留下的不合法用户名，净化不到位就会写出 data/backups。
func TestSafeFileSegment(t *testing.T) {
	cases := map[string]string{
		"admin":       "admin",
		"a.b-c_1":     "a.b-c_1",
		"..":          "user",
		"../../tmp/x": "_.._tmp_x", // 开头的点被削掉，剩下的分隔符已换成 _
		`a\b`:         "a_b",
		"中文名":         "___",
		"":            "user",
		".hidden":     "hidden",
	}
	for in, want := range cases {
		if got := safeFileSegment(in); got != want {
			t.Errorf("safeFileSegment(%q) = %q，期望 %q", in, got, want)
		}
	}

	// 净化后拼成的路径必须仍落在备份目录里。
	root := filepath.Join("data", "backups")
	for _, in := range []string{"../../tmp/x", "..", `..\..\x`, "/etc/passwd"} {
		full := filepath.Join(root, safeFileSegment(in)+"-auto-20260815-120000.json")
		if !strings.HasPrefix(full, root+string(filepath.Separator)) {
			t.Errorf("用户名 %q 净化后逃出了备份目录: %s", in, full)
		}
	}
}

func TestZipUploadRel(t *testing.T) {
	ok := map[string]string{
		"uploads/bg/a.jpg":    "bg/a.jpg",
		"uploads/icons/x.png": "icons/x.png",
		"uploads/icons/a-b_c": "icons/a-b_c",
	}
	for in, want := range ok {
		got, yes := zipUploadRel(in)
		if !yes || got != want {
			t.Errorf("zipUploadRel(%q) = %q, %v，期望 %q, true", in, got, yes, want)
		}
	}
	bad := []string{
		"data.json",
		"uploads/1/bg/a.jpg", // 导出不会带 uid 这一段
		"uploads/other/a.png",
		"uploads/bg/../x.png",
		"uploads/bg/a/b.png",
		"uploads/bg/",
		`uploads/bg/a".png`,
		"",
	}
	for _, in := range bad {
		if got, yes := zipUploadRel(in); yes {
			t.Errorf("zipUploadRel(%q) = %q，期望拒绝", in, got)
		}
	}
}

func TestRemapOwnUploadPath(t *testing.T) {
	if got := remapOwnUploadPath("/uploads/1/icons/a.png", 7); got != "/uploads/7/icons/a.png" {
		t.Errorf("跨账号应改写 uid，得到 %q", got)
	}
	if got := remapOwnUploadPath("/uploads/7/bg/x.jpg", 7); got != "/uploads/7/bg/x.jpg" {
		t.Errorf("同账号应保持原样，得到 %q", got)
	}
	unchanged := []string{
		"",
		"https://cdn.example.com/i.png",
		"/uploads/1/other/a.png",
		"mdi:github",
	}
	for _, in := range unchanged {
		if got := remapOwnUploadPath(in, 7); got != in {
			t.Errorf("remapOwnUploadPath(%q) = %q，期望不动", in, got)
		}
	}
}

func TestReadBackupZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("data.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(`{"version":1,"sites":[]}`)); err != nil {
		t.Fatal(err)
	}
	w, err = zw.Create("uploads/icons/a.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("fakepng")); err != nil {
		t.Fatal(err)
	}
	w, err = zw.Create("uploads/1/bg/skip.jpg") // 带 uid 的多余一层，丢掉
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("nope")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var bf BackupFile
	assets, err := readBackupZip(zr, &bf)
	if err != nil {
		t.Fatalf("readBackupZip: %v", err)
	}
	if bf.Version != 1 {
		t.Fatalf("version = %d", bf.Version)
	}
	if len(assets) != 1 || assets[0].Rel != "icons/a.png" {
		t.Fatalf("assets = %+v，期望只收下 icons/a.png", assets)
	}
	if string(assets[0].Data) != "fakepng" {
		t.Fatalf("asset data = %q", assets[0].Data)
	}
}
