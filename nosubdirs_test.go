package main

import (
	"os"
	"path/filepath"
	"testing"
)

// -n / --no-subdirs 的回归测试。
//
// 守的背景：该参数长期失效——原实现用 strings.Contains(relPath, "/")
// 判断"是否子目录"，但 WalkDir 走到一级子目录时 relPath 就是 "src"
// （不含 "/"），条件永不成立。此前没有任何测试把 NoSubdirs 设为 true，
// 所以问题一直没暴露。
func TestNoSubdirsExcludesSubdirectoryFiles(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "src"))
	mustMkdir(t, filepath.Join(root, "src", "deep"))
	mustWrite(t, filepath.Join(root, "top.md"), "# top")
	mustWrite(t, filepath.Join(root, "README.md"), "# readme")
	mustWrite(t, filepath.Join(root, "src", "a.go"), "package main")
	mustWrite(t, filepath.Join(root, "src", "deep", "b.go"), "package deep")

	cfg := Config{
		RootDir:         root,
		NoSubdirs:       true,
		NoDefaultIgnore: true,
		NoGitignore:     true,
		MaxFileSize:     500 * 1024,
	}
	files, _, err := scanDirectory(cfg)
	if err != nil {
		t.Fatalf("scanDirectory 失败: %v", err)
	}

	for _, f := range files {
		rel := filepath.ToSlash(f.RelPath)
		if rel != filepath.Base(rel) { // 含 "/" 说明来自子目录
			t.Errorf("❌ NoSubdirs 未生效：扫到了子目录文件 %s", rel)
		}
	}
	// 根目录文件必须仍在（避免"一个都没扫到"也算通过）
	if len(files) != 2 {
		t.Errorf("应只扫到根目录 2 个文件，实际 %d 个: %v", len(files), files)
	}
}

// 对照：不设 NoSubdirs 时应扫到全部 4 个文件（证明上面的过滤确实在起作用）
func TestWithoutNoSubdirsScansAll(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "src"))
	mustWrite(t, filepath.Join(root, "top.md"), "# top")
	mustWrite(t, filepath.Join(root, "README.md"), "# readme")
	mustWrite(t, filepath.Join(root, "src", "a.go"), "package main")

	cfg := Config{
		RootDir:         root,
		NoSubdirs:       false,
		NoDefaultIgnore: true,
		NoGitignore:     true,
		MaxFileSize:     500 * 1024,
	}
	files, _, err := scanDirectory(cfg)
	if err != nil {
		t.Fatalf("scanDirectory 失败: %v", err)
	}
	if len(files) != 3 {
		t.Errorf("不设 NoSubdirs 应扫到 3 个文件，实际 %d 个: %v", len(files), files)
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
