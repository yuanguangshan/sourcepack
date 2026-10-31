package main

// 符号链接的安全边界测试。
//
// 守的背景 bug（v2.1.0 实测）：
//
//	ln -s /etc/passwd link.txt && sourcepack -d .
//	→ 产物里出现 /etc/passwd 的完整内容，无任何提示
//	ln -s /dev/zero zero.txt && sourcepack -d .
//	→ 卡死（体积上限按 lstat 尺寸判定为 0，读取却跟随链接无限读）
//
// 产物默认会被 -p 推到远端或贴进 LLM，所以"静默跟随链接"既是内容外泄，
// 也是绕过 --max-size 的后门。git 从不跟随符号链接（只存链接目标文本），
// 这里对齐同样的语义：链接一律跳过，并在 -v / --dry-run 的 skipped 里有据可查。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// makeSymlink 在 Windows 上可能因缺少权限而失败，此时跳过用例。
func makeSymlink(t *testing.T, oldname, newname string) {
	t.Helper()
	if err := os.Symlink(oldname, newname); err != nil {
		t.Skipf("本平台无法创建符号链接（%v），跳过", err)
	}
}

// 目录外的文件不能因为一个符号链接就被打进产物。
func TestScanDirectoryDoesNotFollowSymlinks(t *testing.T) {
	outside := t.TempDir()
	mustWrite(t, filepath.Join(outside, "secret.txt"), "OUTSIDE-SECRET-CONTENT\n")

	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "real.go"), "package real\n")
	makeSymlink(t, filepath.Join(outside, "secret.txt"), filepath.Join(root, "leak.txt"))
	// 目录链接：WalkDir 本来就不会下钻，但要保证它不会当成文件被读取
	mustMkdir(t, filepath.Join(outside, "tree"))
	mustWrite(t, filepath.Join(outside, "tree", "deep.txt"), "DEEP-INSIDE-OUTSIDE\n")
	makeSymlink(t, filepath.Join(outside, "tree"), filepath.Join(root, "dirlink"))

	cfg := Config{RootDir: root, OutputFile: "out.md", MaxFileSize: 500 * 1024}
	files, stats, skipped := scanDirectory(cfg)

	var got []string
	for _, f := range files {
		got = append(got, f.RelPath)
	}
	if len(got) != 1 || got[0] != "real.go" {
		t.Fatalf("只应纳入真实文件 real.go，实际: %v", got)
	}
	if stats.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", stats.FileCount)
	}

	// 跳过的链接必须显式记录，而不是静默消失
	reasons := map[string]string{}
	for _, s := range skipped {
		reasons[s.RelPath] = s.Reason
	}
	for _, rel := range []string{"leak.txt", "dirlink"} {
		if reasons[rel] != "Symlink (not followed)" {
			t.Errorf("%s 应记为 Symlink (not followed)，实际 %q", rel, reasons[rel])
		}
	}

	// 端到端：产物里不能出现目录外的内容
	outPath := filepath.Join(t.TempDir(), "out.md")
	out, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeContent(cfg, files, stats, out); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"OUTSIDE-SECRET-CONTENT", "DEEP-INSIDE-OUTSIDE"} {
		if strings.Contains(string(data), needle) {
			t.Errorf("产物里出现了目录外的内容 %q——符号链接被跟随了", needle)
		}
	}
}

// git 模式（-g/-G）没有目录遍历，符号链接必须单独拦住：
// git 跟踪的链接在 diff 里是一条普通变更，正文读取若跟随链接同样会外泄。
func TestGitModeDoesNotFollowSymlinks(t *testing.T) {
	outside := t.TempDir()
	mustWrite(t, filepath.Join(outside, "secret.txt"), "OUTSIDE-SECRET-CONTENT\n")

	dir := newGitFixture(t)
	writeFile(t, dir, "real.go", "package real\n")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "init")

	// 提交后新建一个指向仓库外的符号链接（未跟踪），再动一个普通文件，
	// 让 "-g" 的清单里同时有"链接"和"普通变更"。
	makeSymlink(t, filepath.Join(outside, "secret.txt"), filepath.Join(dir, "leak.txt"))
	writeFile(t, dir, "real.go", "package real // touched\n")

	cs, files, _, skipped := collectForTest(t, gitConfig(Config{GitDiff: true}, dir))

	for _, f := range files {
		if f.RelPath == "leak.txt" {
			t.Fatalf("符号链接 leak.txt 不应作为正文文件纳入")
		}
	}
	if ch := changeByPath(cs, "leak.txt"); ch == nil {
		t.Fatalf("leak.txt 仍应出现在变更清单里（只是没有正文）")
	} else if ch.NoContent != "Symlink (not followed)" {
		t.Errorf("leak.txt 的 NoContent = %q, want %q", ch.NoContent, "Symlink (not followed)")
	}
	found := false
	for _, s := range skipped {
		if s.RelPath == "leak.txt" && s.Reason == "Symlink (not followed)" {
			found = true
		}
	}
	if !found {
		t.Errorf("leak.txt 应出现在 skipped 清单里，实际: %v", skipped)
	}
}

// 扫描器不能因为链接指向特殊文件而挂死。
// /dev/zero 在有 /dev 的类 Unix 系统上存在；不存在时跳过。
func TestScanDirectorySymlinkToCharDeviceDoesNotHang(t *testing.T) {
	if _, err := os.Lstat("/dev/zero"); err != nil {
		t.Skip("/dev/zero 不存在，跳过")
	}
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "real.go"), "package real\n")
	makeSymlink(t, "/dev/zero", filepath.Join(root, "zero.txt"))

	done := make(chan struct{})
	var files []FileMetadata
	go func() {
		files, _, _ = scanDirectory(Config{RootDir: root, OutputFile: "out.md", MaxFileSize: 500 * 1024})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("扫描被 /dev/zero 的符号链接卡住了——体积上限又没拦住")
	}
	for _, f := range files {
		if f.RelPath == "zero.txt" {
			t.Fatal("zero.txt 是符号链接，不应被纳入")
		}
	}
}
