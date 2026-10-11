package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/spf13/pflag"
)

// ============================================================
//  纯解析层单测（不需要 git）
// ============================================================

func TestParseNameStatusZ(t *testing.T) {
	// -z 的真实字节：A\0p\0 / R100\0old\0new\0
	in := "A\x00added.txt\x00M\x00hello world.txt\x00R100\x00src/old.go\x00src/new.go\x00" +
		"D\x00gone.md\x00C75\x00base.go\x00copy.go\x00M\x00中文 文档.md\x00"
	changes, err := parseNameStatusZ(in)
	if err != nil {
		t.Fatalf("parseNameStatusZ: %v", err)
	}
	want := []struct{ status, path, old string }{
		{"A", "added.txt", ""},
		{"M", "hello world.txt", ""},
		{"R100", "src/new.go", "src/old.go"},
		{"D", "gone.md", ""},
		{"C75", "copy.go", "base.go"},
		{"M", "中文 文档.md", ""},
	}
	if len(changes) != len(want) {
		t.Fatalf("解析出 %d 条，期望 %d 条: %+v", len(changes), len(want), changes)
	}
	for i, w := range want {
		got := changes[i]
		if got.Status != w.status || got.Path != w.path || got.OldPath != w.old {
			t.Errorf("第 %d 条 = {%q %q %q}，期望 {%q %q %q}", i, got.Status, got.Path, got.OldPath, w.status, w.path, w.old)
		}
	}
}

// 这个 error 返回以前永远是 nil，三个调用方都在认真检查它——死通道。
// 现在做成真校验：畸形输出必须报错，而不是静默丢掉变更。
func TestParseNameStatusZRejectsMalformed(t *testing.T) {
	for _, bad := range []string{"R100\x00only-old-path\x00", "M\x00", "M", "A\x00"} {
		if _, err := parseNameStatusZ(bad); err == nil {
			t.Errorf("畸形输入 %q 应当报错而不是静默截断", bad)
		}
	}
	if out, err := parseNameStatusZ(""); err != nil || len(out) != 0 {
		t.Errorf("空输入应返回空结果且无错误: %v %v", out, err)
	}
}

func TestStatusLabel(t *testing.T) {
	cases := map[string]string{
		"A": "added", "M": "modified", "D": "deleted", "R100": "renamed",
		"C75": "copied", "T": "type changed", "U": "unmerged", "??": "untracked", "": "changed",
	}
	for code, want := range cases {
		got := (&Change{Status: code}).StatusLabel()
		if got != want {
			t.Errorf("StatusLabel(%q) = %q, 期望 %q", code, got, want)
		}
	}
}

func TestParseDiffPath(t *testing.T) {
	cases := map[string]string{
		"a/main.go":                "main.go",
		"b/main.go":                "main.go",
		"b/dir/main.go":            "dir/main.go",
		"/dev/null":                "",
		"a/hello world.txt\t":      "hello world.txt", // git 给含空格路径加的尾 TAB
		"b/中文 文档.md\t":             "中文 文档.md",
		`"a/wei\"rd.txt"`:          `wei"rd.txt`,
		`"b/tab\there.txt"`:        "tab\there.txt",
		`"b/octal\344\270\255.md"`: "octal中.md",
	}
	for in, want := range cases {
		if got := parseDiffPath(in); got != want {
			t.Errorf("parseDiffPath(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

func TestParseDiffGitHeader(t *testing.T) {
	cases := map[string]string{
		"diff --git a/main.go b/main.go":                 "main.go",
		"diff --git a/src/old.go b/src/new.go":           "src/new.go",
		`diff --git a/a b/x.txt b/x.txt`:                 "x.txt",
		`diff --git "a/wei rd" "b/wei rd"`:               "wei rd",
		"diff --git a/hello world.txt b/hello world.txt": "hello world.txt",
	}
	for in, want := range cases {
		if got := parseDiffGitHeader(in); got != want {
			t.Errorf("parseDiffGitHeader(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

// 关键回归：hunk 正文里以 "---" / "+++" 开头的行不能被当成文件头，
// 也不能被漏计（前缀判断的经典坑）。
func TestSplitPatchCountsAndPaths(t *testing.T) {
	patch := "diff --git a/added.txt b/added.txt\n" +
		"new file mode 100644\n" +
		"index 0000000..6ad4673\n" +
		"--- /dev/null\n" +
		"+++ b/added.txt\n" +
		"@@ -0,0 +1,2 @@\n" +
		"+aa\n+bb\n" +
		"diff --git a/sep.md b/sep.md\n" +
		"index 1111111..2222222 100644\n" +
		"--- a/sep.md\t\n" +
		"+++ b/sep.md\t\n" +
		"@@ -1,3 +1,4 @@\n" +
		" keep\n" +
		"---- separator ----\n" + // 正文里以 --- 开头
		"++++ plus ----\n" + // 正文里以 +++ 开头
		"-old\n+new\n" +
		"diff --git a/src/old.go b/src/new.go\n" +
		"similarity index 100%\n" +
		"rename from src/old.go\n" +
		"rename to src/new.go\n" +
		"diff --git a/logo.bin b/logo.bin\n" +
		"index 44d77d2..6db238c 100644\n" +
		"Binary files a/logo.bin and b/logo.bin differ\n"

	chunks := splitPatch(patch)
	if len(chunks) != 4 {
		t.Fatalf("应有 4 块，实际 %d: %+v", len(chunks), chunks)
	}
	if chunks[0].Path != "added.txt" || chunks[0].Added != 2 || chunks[0].Deleted != 0 || chunks[0].Binary {
		t.Errorf("added.txt 块解析错误: %+v", chunks[0])
	}
	if chunks[1].Path != "sep.md" {
		t.Errorf("含空格/带 TAB 的路径解析错误: %q", chunks[1].Path)
	}
	// "+new" 与 "++++ plus ----" 各算 1 增；"-old" 与 "---- separator ----" 各算 1 删
	// （关键是正文里以 --- / +++ 开头的行既不能被当成文件头，也不能漏计）
	if chunks[1].Added != 2 || chunks[1].Deleted != 2 {
		t.Errorf("sep.md 增删计数错误: +%d -%d，期望 +2 -2", chunks[1].Added, chunks[1].Deleted)
	}
	if chunks[2].Path != "src/new.go" || chunks[2].HeaderPath != "src/new.go" {
		t.Errorf("rename to 解析错误: %+v", chunks[2])
	}
	if !chunks[3].Binary || chunks[3].Added != 0 {
		t.Errorf("二进制块解析错误: %+v", chunks[3])
	}
}

func TestCapDiff(t *testing.T) {
	body := "l1\nl2\nl3\nl4\nl5\n"
	if got, omitted := capDiff(body, 10, 0); omitted != 0 || got != body {
		t.Errorf("未超限时不该改动: omitted=%d got=%q", omitted, got)
	}
	got, omitted := capDiff(body, 2, 0)
	if omitted != 3 {
		t.Errorf("omitted = %d，期望 3", omitted)
	}
	if !strings.HasPrefix(got, "l1\nl2\n") || !strings.Contains(got, "3 of 5 diff lines omitted") {
		t.Errorf("截断结果不符合预期:\n%s", got)
	}
	if got, _ := capDiff(body, 0, 0); got != body {
		t.Errorf("maxLines=0/maxBytes=0 应视为不限制")
	}

	// 字节上限：单行超长文件（例如一行 20MB）用行数截不住，必须按字节兜住。
	var big strings.Builder
	for i := 0; i < 200; i++ {
		big.WriteString(strings.Repeat("x", 20))
		big.WriteByte('\n')
	}
	capped, omitted := capDiff(big.String(), 2000, 1024)
	if omitted == 0 {
		t.Fatal("超过字节上限时应截断")
	}
	if len(capped) > 1024+120 {
		t.Errorf("字节截断没兜住：正文 %d 字节（上限 1024）", len(capped))
	}
	if !strings.Contains(capped, "--max-size 1 KB") {
		t.Errorf("字节截断缺少标注:\n%s", capped[len(capped)-120:])
	}

	// 单个超长行、没有任何换行：行数上限完全无效，字节上限必须生效
	oneLine := strings.Repeat("y", 2<<20)
	capped, omitted = capDiff(oneLine, 2000, 4096)
	if omitted == 0 || len(capped) > 4096+120 {
		t.Errorf("单行超长正文没被字节上限兜住: omitted=%d len=%d", omitted, len(capped))
	}
}

func TestTrimToLastCompleteChunk(t *testing.T) {
	patch := "diff --git a/a b/a\n+1\n" + "diff --git a/b b/b\n+2\n" + "diff --git a/c b/c\n+3(trunc"
	got := trimToLastCompleteChunk(patch)
	if strings.Contains(got, "b/c") || !strings.Contains(got, "b/b") {
		t.Errorf("应在 b/b 块末尾截断，实际:\n%s", got)
	}
}

func TestSyntheticNewFileDiff(t *testing.T) {
	diff, total, omitted := syntheticNewFileDiff("新 文件.md", []byte("a\nb\nc\n"), 0, 0)
	if total != 3 || omitted != 0 {
		t.Errorf("total/omitted = %d/%d，期望 3/0", total, omitted)
	}
	for _, want := range []string{
		"diff --git a/新 文件.md b/新 文件.md",
		"new file mode 100644",
		"--- /dev/null",
		"+++ b/新 文件.md",
		"@@ -0,0 +1,3 @@",
		"+a\n+b\n+c\n",
	} {
		if !strings.Contains(diff, want) {
			t.Errorf("生成的补丁缺少 %q:\n%s", want, diff)
		}
	}
	if _, _, omitted := syntheticNewFileDiff("x.txt", []byte("1\n2\n3\n4\n"), 2, 0); omitted != 2 {
		t.Errorf("按 --max-diff-lines 截断失败: omitted=%d", omitted)
	}
	if d, total, _ := syntheticNewFileDiff("empty.txt", []byte{}, 0, 0); total != 0 || !strings.Contains(d, "@@ -0,0 +1,0 @@") {
		t.Errorf("空文件补丁异常: total=%d\n%s", total, d)
	}
}

func TestUnquoteGitPath(t *testing.T) {
	cases := map[string]string{
		"plain":          "plain",
		`"a\tb"`:         "a\tb",
		`"a\nb"`:         "a\nb",
		`"a\\b"`:         `a\b`,
		`"a\"b"`:         `a"b`,
		`"\344\270\255"`: "中",
	}
	for in, want := range cases {
		if got := unquoteGitPath(in); got != want {
			t.Errorf("unquoteGitPath(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

func TestFormatHelpers(t *testing.T) {
	if got := mdCode("a b.go"); got != "`a b.go`" {
		t.Errorf("mdCode 普通路径 = %q", got)
	}
	if got := mdCode("we`ird"); got != "we`ird" {
		t.Errorf("mdCode 含反引号应原样返回，实际 %q", got)
	}
	if got := mdCell("a|b\nc"); got != `a\|b c` {
		t.Errorf("mdCell = %q", got)
	}
}

// ============================================================
//  参数解析与组合校验
// ============================================================

func newTestFlagSet() *pflag.FlagSet {
	return pflag.NewFlagSet("sourcepack-test", pflag.ContinueOnError)
}

func TestParseFlagsGitModeDefaults(t *testing.T) {
	c := parseFlagsInto(newTestFlagSet(), []string{"-g"})
	if !c.GitDiff || !c.gitMode() {
		t.Fatalf("-g 未开启 git 模式: %+v", c)
	}
	if c.OutputFile != "git_changes.md" {
		t.Errorf("git 模式默认输出名应为 git_changes.md，实际 %q", c.OutputFile)
	}
	if c.GitRev != "HEAD" || c.MaxDiffLines != 2000 {
		t.Errorf("默认值不对: rev=%q maxDiffLines=%d", c.GitRev, c.MaxDiffLines)
	}
	if err := validateConfig(c, newTestFlagSet()); err != nil {
		t.Errorf("默认组合不该报错: %v", err)
	}
}

func TestParseFlagsSnapshotModeKeepsDefaultOutput(t *testing.T) {
	c := parseFlagsInto(newTestFlagSet(), []string{"-i", "go"})
	if c.OutputFile != "project_snapshot.md" {
		t.Errorf("非 git 模式默认输出名被改动: %q", c.OutputFile)
	}
	if c.IncludeExts[0] != ".go" {
		t.Errorf("-i go 未解析为 .go: %v", c.IncludeExts)
	}
}

func TestParseFlagsExplicitOutWins(t *testing.T) {
	c := parseFlagsInto(newTestFlagSet(), []string{"-G", "3", "-o", "my.md"})
	if c.OutputFile != "my.md" || c.GitLog != 3 {
		t.Errorf("-G 3 -o my.md 解析错误: %+v", c)
	}
}

func TestValidateConfigCombinations(t *testing.T) {
	cases := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"-g", "-G", "2"}, "mutually exclusive"},
		{[]string{"-G", "0"}, ">= 1"},
		{[]string{"-G", "-3"}, ">= 1"},
		{[]string{"--staged"}, "--staged only applies"},
		{[]string{"--no-merges"}, "--no-merges only applies"},
		{[]string{"--rev", "HEAD~1"}, "--rev only applies"},
		{[]string{"--diff-only"}, "--diff-only only applies"},
		{[]string{"--max-diff-lines", "10"}, "--max-diff-lines only applies"},
		{[]string{"-g", "--max-diff-lines", "-1"}, "must be >= 0"},
		{[]string{"-g", "-n"}, "--no-subdirs has no effect"},
	}
	for _, tc := range cases {
		fs := newTestFlagSet()
		c := parseFlagsInto(fs, tc.args)
		err := validateConfig(c, fs)
		if err == nil {
			t.Errorf("%v 应当报错（期望包含 %q）", tc.args, tc.wantErr)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%v 的报错为 %q，期望包含 %q", tc.args, err.Error(), tc.wantErr)
		}
	}

	// 合法组合不应报错
	for _, args := range [][]string{
		{"-g"}, {"-g", "--staged"}, {"-g", "--rev", "HEAD~1"}, {"-G", "5", "--no-merges"},
		{"-G", "5", "--diff-only"}, {"-g", "--max-diff-lines", "0"},
	} {
		fs := newTestFlagSet()
		if err := validateConfig(parseFlagsInto(fs, args), fs); err != nil {
			t.Errorf("%v 不该报错: %v", args, err)
		}
	}
}

// ============================================================
//  git 集成测试
// ============================================================

func newGitFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("未安装 git，跳过 git 集成测试")
	}
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	gitIn(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")
	gitIn(t, dir, "config", "user.email", "t@t.c")
	gitIn(t, dir, "config", "user.name", "T")
	gitIn(t, dir, "config", "commit.gpgsign", "false")
	return dir
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@t.c",
		"GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@t.c",
		"GIT_CONFIG_NOSYSTEM=1",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v 失败: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	mustMkdir(t, filepath.Dir(p))
	mustWrite(t, p, content)
}

func gitConfig(cfg Config, root string) Config {
	cfg.RootDir = root
	if cfg.MaxFileSize == 0 {
		cfg.MaxFileSize = 500 * 1024
	}
	if cfg.GitRev == "" {
		cfg.GitRev = "HEAD"
	}
	return cfg
}

// 造一个内容齐全的仓库：历史 3 个提交（含一次 merge）+ 混合状态的脏工作区。
func buildMixedFixture(t *testing.T) string {
	t.Helper()
	dir := newGitFixture(t)

	writeFile(t, dir, "src/main.go", "package main\n\nfunc main() {}\n")
	writeFile(t, dir, "src/deep/old.go", "package deep\n\nfunc Deep() {}\n")
	writeFile(t, dir, "docs/中文 文档.md", "# 标题\n\n正文\n")
	writeFile(t, dir, "space name.txt", "l1\nl2\nl3\n")
	writeFile(t, dir, "ignored-but-tracked.txt", "f1\n")
	writeFile(t, dir, ".env", "SECRET=abc\n")
	writeFile(t, dir, "logo.bin", "binary\x00\x01zz\n")
	gitIn(t, dir, "add", "-A", "-f", ".")
	gitIn(t, dir, "commit", "-q", "-m", "init")

	writeFile(t, dir, ".gitignore", "ignored-but-tracked.txt\nnode_modules/\n*.log\n")
	gitIn(t, dir, "add", ".gitignore")
	gitIn(t, dir, "commit", "-q", "-m", "chore: gitignore")

	writeFile(t, dir, "docs/tmp.md", "tmp\n")
	gitIn(t, dir, "add", "docs/tmp.md")
	gitIn(t, dir, "commit", "-q", "-m", "docs: tmp")
	gitIn(t, dir, "rm", "-q", "docs/tmp.md")
	gitIn(t, dir, "commit", "-q", "-m", "chore: rm tmp")

	gitIn(t, dir, "checkout", "-q", "-b", "feature/y")
	writeFile(t, dir, "src/feat.go", "package f\n")
	gitIn(t, dir, "add", "src/feat.go")
	gitIn(t, dir, "commit", "-q", "-m", "feat: feat.go")
	gitIn(t, dir, "checkout", "-q", "main")
	writeFile(t, dir, "docs/main_side.md", "main side\n")
	gitIn(t, dir, "add", "docs/main_side.md")
	gitIn(t, dir, "commit", "-q", "-m", "docs: main side")
	gitIn(t, dir, "merge", "-q", "--no-ff", "feature/y", "-m", "Merge branch 'feature/y'")
	gitIn(t, dir, "branch", "-q", "-d", "feature/y")

	// 脏工作区：已暂存 / 部分暂存 / 未暂存 / 重命名 / 未跟踪
	writeFile(t, dir, "src/main.go", "package main\n\nfunc main() { A() }\n")
	gitIn(t, dir, "add", "src/main.go")
	writeFile(t, dir, "src/staged_new.go", "package staged\n")
	gitIn(t, dir, "add", "src/staged_new.go")
	gitIn(t, dir, "mv", "src/deep/old.go", "src/deep/new.go")
	writeFile(t, dir, "src/main.go", "package main\n\nfunc main() { A(); B() }\n") // 部分暂存
	writeFile(t, dir, "docs/中文 文档.md", "# 标题\n\n正文（已改）\n")
	writeFile(t, dir, "space name.txt", "l1\nX\nl3\nl4\n")
	writeFile(t, dir, "ignored-but-tracked.txt", "f1\nf2\n")
	writeFile(t, dir, ".env", "SECRET=xyz\n")
	writeFile(t, dir, "logo.bin", "binary\x00\x01zzz\n")
	writeFile(t, dir, "docs/未跟踪 新文件.md", "新文件\n第二行\n")
	writeFile(t, dir, "untracked.go", "package u\n")
	writeFile(t, dir, "debug.log", "log line\n")      // 被 .gitignore 忽略
	writeFile(t, dir, "node_modules/new.js", "k\n")   // 被 .gitignore 忽略
	writeFile(t, dir, "vendor_gd.go", "package gd\n") // 被 .gdignore 忽略
	writeFile(t, dir, ".gdignore", "vendor_gd.go\n")
	return dir
}

func collectForTest(t *testing.T, cfg Config) (*ChangeSet, []FileMetadata, Stats, []SkippedFile) {
	t.Helper()
	cs, err := collectChangeSet(&cfg)
	if err != nil {
		t.Fatalf("collectChangeSet: %v", err)
	}
	files, stats, skipped := buildChangeFiles(&cfg, cs)
	return cs, files, stats, skipped
}

func changeByPath(cs *ChangeSet, path string) *Change {
	for _, grp := range cs.Commits {
		for _, ch := range grp.Changes {
			if ch.Path == path {
				return ch
			}
		}
	}
	return nil
}

func TestGitWorkingTreeClassification(t *testing.T) {
	dir := buildMixedFixture(t)
	cfg := gitConfig(Config{GitDiff: true, MaxDiffLines: 2000}, dir)
	cs, files, _, skipped := collectForTest(t, cfg)

	if cs.Mode != "working tree" {
		t.Errorf("Mode = %q", cs.Mode)
	}
	if cs.Branch != "main" {
		t.Errorf("Branch = %q", cs.Branch)
	}
	if len(cs.Commits) != 1 {
		t.Fatalf("工作区模式应只有 1 组，实际 %d", len(cs.Commits))
	}

	// 暂存/未暂存/未跟踪 与 git 的真实判定一致
	if cs.StagedCount != 3 {
		t.Errorf("staged = %d，期望 3（src/main.go、源码重命名、src/staged_new.go）", cs.StagedCount)
	}
	if cs.UntrackedCount != 4 {
		t.Errorf("untracked = %d，期望 4", cs.UntrackedCount)
	}

	// 部分暂存：同一文件既在索引里、工作区又有新改动
	main := changeByPath(cs, "src/main.go")
	if main == nil || !main.Staged || !main.Unstaged {
		t.Errorf("src/main.go 应同时标记 staged 与 unstaged: %+v", main)
	}

	// 重命名（含中文/空格路径的解析正确性）
	ren := changeByPath(cs, "src/deep/new.go")
	if ren == nil || ren.OldPath != "src/deep/old.go" || ren.StatusLabel() != "renamed" {
		t.Errorf("重命名解析错误: %+v", ren)
	}

	// 未跟踪文件
	un := changeByPath(cs, "docs/未跟踪 新文件.md")
	if un == nil || !un.Untracked || un.Status != "??" {
		t.Errorf("未跟踪文件解析错误: %+v", un)
	}
	if un.Added == 0 {
		t.Errorf("未跟踪文件应补出新增行数: %+v", un)
	}

	// 含空格路径的 diff 必须正确归属（曾经会被认成 "to src/..." 造出重复条目）
	sp := changeByPath(cs, "space name.txt")
	if sp == nil || sp.Diff == "" || sp.Added == 0 {
		t.Errorf("含空格路径的 diff 未归属: %+v", sp)
	}
	if len(cs.Commits[0].Changes) != len(uniquePaths(cs.Commits[0].Changes)) {
		t.Errorf("变更列表出现重复路径: %+v", cs.Commits[0].Changes)
	}

	// 二进制：git 判定优先，正文不输出
	bin := changeByPath(cs, "logo.bin")
	if bin == nil || !bin.Binary || bin.NoContent != "Binary file" {
		t.Errorf("二进制文件处理错误: %+v", bin)
	}

	// 被 .gitignore 忽略但已跟踪的文件是真实变更，必须保留
	if changeByPath(cs, "ignored-but-tracked.txt") == nil {
		t.Error("已跟踪且命中 .gitignore 的文件被错误排除")
	}
	// 未被跟踪且被忽略的文件由 git 过滤，不进入变更集
	for _, p := range []string{"debug.log", "node_modules/new.js"} {
		if changeByPath(cs, p) != nil {
			t.Errorf("%s 不该进入变更集", p)
		}
	}

	// 敏感文件走既有 gate
	if ch := changeByPath(cs, ".env"); ch == nil || ch.Filtered != "Sensitive file" {
		t.Errorf(".env 应被敏感文件规则排除: %+v", ch)
	}
	// .gdignore 同样生效
	if ch := changeByPath(cs, "vendor_gd.go"); ch == nil || ch.Filtered != "Ignored by ignore file" {
		t.Errorf("vendor_gd.go 应被 .gdignore 排除: %+v", ch)
	}
	// 内容候选里不应出现被过滤的路径
	for _, f := range files {
		if f.RelPath == ".env" || f.RelPath == "vendor_gd.go" {
			t.Errorf("被过滤的文件仍进入内容候选: %s", f.RelPath)
		}
	}
	foundSkip := false
	for _, s := range skipped {
		if s.RelPath == ".env" {
			foundSkip = true
		}
	}
	if !foundSkip {
		t.Error("被过滤的变更应出现在 skipped 里（可在输出中如实列出）")
	}
}

func uniquePaths(changes []*Change) map[string]bool {
	m := map[string]bool{}
	for _, ch := range changes {
		m[ch.Path] = true
	}
	return m
}

func TestGitFiltersReuseSnapshotSemantics(t *testing.T) {
	dir := buildMixedFixture(t)

	// -i go：只剩 .go
	cfg := gitConfig(Config{GitDiff: true, IncludeExts: []string{".go"}, MaxDiffLines: 2000}, dir)
	_, files, _, _ := collectForTest(t, cfg)
	for _, f := range files {
		if !strings.HasSuffix(f.RelPath, ".go") {
			t.Errorf("-i go 未生效，仍包含 %s", f.RelPath)
		}
	}
	if len(files) == 0 {
		t.Fatal("-i go 过滤过度，一个文件都不剩")
	}

	// -a：关闭所有忽略规则 → 敏感文件放行
	cfg = gitConfig(Config{GitDiff: true, All: true, NoDefaultIgnore: true, NoGitignore: true, MaxDiffLines: 2000}, dir)
	cs, files, _, _ := collectForTest(t, cfg)
	if ch := changeByPath(cs, ".env"); ch == nil || ch.Filtered != "" {
		t.Errorf("-a 应放行敏感文件: %+v", ch)
	}
	foundEnv := false
	for _, f := range files {
		if f.RelPath == ".env" {
			foundEnv = true
		}
	}
	if !foundEnv {
		t.Error("-a 下 .env 应进入内容候选")
	}
}

// 未跟踪文件在 git diff 里一行都没有，摘要的 +N 与"总量过大"告警
// 都得靠数正文行数补上（否则 -g 下一个 3 万行的新文件完全不触发告警）。
func TestGitUntrackedLinesCountTowardTotals(t *testing.T) {
	dir := newGitFixture(t)
	writeFile(t, dir, "tracked.go", "package a\n")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "init")
	writeFile(t, dir, "新文件.md", "l1\nl2\nl3\n")

	cfg := gitConfig(Config{GitDiff: true, MaxDiffLines: 2000}, dir)
	cs, files, _, _ := collectForTest(t, cfg)

	if cs.UniquePaths != 1 || len(files) != 1 {
		t.Fatalf("应只有 1 个未跟踪变更，实际 paths=%d files=%d", cs.UniquePaths, len(files))
	}
	ch := changeByPath(cs, "新文件.md")
	if ch == nil || !ch.Untracked || ch.Diff != "" {
		t.Fatalf("该文件应为无 diff 的未跟踪文件: %+v", ch)
	}
	if cs.TotalAdded != 3 || cs.TotalDeleted != 0 {
		t.Errorf("+%d −%d，期望 +3 −0", cs.TotalAdded, cs.TotalDeleted)
	}
	if cs.TotalDiffLines != 3 {
		t.Errorf("TotalDiffLines = %d，期望 3（未跟踪文件按正文行数计入总量告警）", cs.TotalDiffLines)
	}
}

// 回归：--diff-only 的合成补丁必须和默认模式走同一道 gate。
//
// 背景：24f54cb 里合成补丁生成在主循环的成功路径内（天然在所有
// noContent gate 之后）；9b20203 把它挪成独立循环后只检查 filteredReason，
// 于是未跟踪的二进制 / 超限文件也被合成成 diff——二进制裸字节进产物，
// 60MB 的未跟踪文件实测产出 62MB 的 Markdown（--max-size 完全失效）。
func TestGitDiffOnlyRespectsBinaryAndSizeGates(t *testing.T) {
	dir := newGitFixture(t)
	writeFile(t, dir, "seed.txt", "package a\n")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "init")
	writeFile(t, dir, "new.bin", "PNG\x00\x00\x01\x02BIN\x00\xff\xfe")
	writeFile(t, dir, "huge.txt", strings.Repeat("line\n", 2000)) // 10KB > 1KB 上限

	cfg := gitConfig(Config{GitDiff: true, DiffOnly: true, MaxDiffLines: 2000, MaxFileSize: 1024}, dir)
	cs, files, stats, _ := collectForTest(t, cfg)

	bin := changeByPath(cs, "new.bin")
	if bin == nil || bin.Diff != "" {
		t.Errorf("二进制的未跟踪文件在 --diff-only 下不该有合成 diff: %+v", bin)
	}
	if bin == nil || !bin.Binary {
		t.Errorf("二进制判定应传递到 Change 上（否则标记与文案都说成文本）: %+v", bin)
	}
	huge := changeByPath(cs, "huge.txt")
	if huge == nil || huge.Diff != "" || huge.NoContent != "Size limit" {
		t.Errorf("超过 --max-size 的未跟踪文件不该被合成 diff: %+v", huge)
	}
	for _, f := range files {
		if f.RelPath == "new.bin" || f.RelPath == "huge.txt" {
			t.Errorf("被 gate 拦下的文件不该进内容候选: %s", f.RelPath)
		}
	}

	var buf bytes.Buffer
	if err := writeChangeContent(cfg, files, stats, cs, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if len(out) > 4096 {
		t.Errorf("--diff-only 产物 %d 字节：超限/二进制文件仍被写进来了", len(out))
	}
	if strings.ContainsRune(out, 0) {
		t.Error("产物里出现了 NUL 字节")
	}
	if !utf8.ValidString(out) {
		t.Error("产物不是合法 UTF-8")
	}
	if strings.Contains(out, "full content included below") {
		t.Error("二进制文件不该声称\"full content included below\"（自相矛盾）")
	}
}

// -G 下同一路径可能出现在多个提交里：正文读的是"当前工作区"，
// 判定就必须用最新一次出现的元数据（旧实现取最旧那次）。
func TestGitHistoryContentUsesNewestOccurrence(t *testing.T) {
	// 旧二进制 → 新文本：正文应当输出
	dir := newGitFixture(t)
	writeFile(t, dir, "f.txt", "BIN\x00\x01payload\n")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "c1 binary")
	writeFile(t, dir, "f.txt", "plain text\n")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "c2 text")

	cs, files, _, _ := collectForTest(t, gitConfig(Config{GitLog: 2, MaxDiffLines: 2000}, dir))
	found := false
	for _, f := range files {
		if f.RelPath == "f.txt" {
			found = true
		}
	}
	if !found {
		t.Errorf("最新状态已是文本，正文不该被省略: %+v", changeByPath(cs, "f.txt"))
	}

	// 旧文本 → 新二进制：正文与 diff 都不该输出
	dir2 := newGitFixture(t)
	writeFile(t, dir2, "g.txt", "plain\n")
	gitIn(t, dir2, "add", "-A")
	gitIn(t, dir2, "commit", "-q", "-m", "c1 text")
	writeFile(t, dir2, "g.txt", "BIN\x00\x01payload\n")
	gitIn(t, dir2, "add", "-A")
	gitIn(t, dir2, "commit", "-q", "-m", "c2 binary")

	cs2, files2, _, _ := collectForTest(t, gitConfig(Config{GitLog: 2, MaxDiffLines: 2000}, dir2))
	for _, f := range files2 {
		if f.RelPath == "g.txt" {
			t.Errorf("最新状态是二进制，正文不该输出: %s", f.RelPath)
		}
	}
	// 二进制变更保留 git 自己的安全提示（"Binary files ... differ"），
	// 但绝不能带原始字节
	if g := changeByPath(cs2, "g.txt"); g == nil || !g.Binary || g.NoContent != "Binary file" {
		t.Errorf("二进制变更应标记 Binary 且不输出正文: %+v", g)
	} else if strings.ContainsRune(g.Diff, 0) {
		t.Errorf("二进制 diff 里出现裸字节: %q", g.Diff)
	}
}

// 产出必须是合法 UTF-8 且不含 NUL —— 这是"喂给 AI 的 Markdown"的硬约束。
// 覆盖三类会漏字节的文件：真二进制、无 NUL 非法 UTF-8、合法 UTF-8 但含 NUL。
func TestChangeOutputIsValidUTF8AndNULFree(t *testing.T) {
	dir := newGitFixture(t)
	writeFile(t, dir, "tracked_bin.dat", "PNG\x00\x00\x01init\n")
	writeFile(t, dir, "tracked_nonul.dat", "\x80\x81\x82\xff-init\n")
	writeFile(t, dir, "tracked_utf8nul.txt", "abc\x00init\n")
	writeFile(t, dir, "stray_nul.md", "text\nstray\x00init\n")
	gitIn(t, dir, "add", "-A", "-f", ".")
	gitIn(t, dir, "commit", "-q", "-m", "init")
	writeFile(t, dir, "tracked_bin.dat", "PNG\x00\x00\x01changed\n")
	writeFile(t, dir, "tracked_nonul.dat", "\x80\x81\x82\xff-changed\n")
	writeFile(t, dir, "tracked_utf8nul.txt", "abc\x00changed\n")
	writeFile(t, dir, "stray_nul.md", "text\nstray\x00changed\n")
	writeFile(t, dir, "untracked.bin", "PNG\x00\x00\x01new\n")
	writeFile(t, dir, "untracked_nonul.dat", "\xfe\xff\x80new\n")

	for _, diffOnly := range []bool{false, true} {
		cfg := gitConfig(Config{GitDiff: true, DiffOnly: diffOnly, MaxDiffLines: 2000}, dir)
		cs, files, stats, _ := collectForTest(t, cfg)
		var buf bytes.Buffer
		if err := writeChangeContent(cfg, files, stats, cs, &buf); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if strings.ContainsRune(out, 0) {
			t.Errorf("diff-only=%v: 产物含 NUL 字节", diffOnly)
		}
		if !utf8.ValidString(out) {
			t.Errorf("diff-only=%v: 产物不是合法 UTF-8", diffOnly)
		}
	}
}

func TestGitDiffOnlyOmitsContentButKeepsUntrackedDiff(t *testing.T) {
	dir := buildMixedFixture(t)
	cfg := gitConfig(Config{GitDiff: true, DiffOnly: true, MaxDiffLines: 2000}, dir)
	cs, _, _, skipped := collectForTest(t, cfg)

	// --diff-only 不是"跳过"，正文只是不输出：跳过列表里不该塞满无正文条目
	for _, s := range skipped {
		if s.Reason == "Not in working tree" || s.Reason == "Binary file" {
			t.Errorf("--diff-only 下不该把无正文当成跳过: %+v", s)
		}
	}

	var buf bytes.Buffer
	if err := writeChangeContent(cfg, nil, Stats{DirMap: map[string]*DirStats{}, ExtMap: map[string]*ExtStats{}}, cs, &buf); err != nil {
		t.Fatalf("writeChangeContent: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "Full content") {
		t.Error("--diff-only 输出里不应有全文小节")
	}
	// 未跟踪文件在 --diff-only 下必须靠合成补丁现身
	un := changeByPath(cs, "docs/未跟踪 新文件.md")
	if un == nil || !strings.Contains(un.Diff, "new file mode") {
		t.Errorf("未跟踪文件缺少合成 diff: %+v", un)
	}
	if !strings.Contains(out, "new file mode") {
		t.Error("输出里看不到未跟踪文件的合成补丁")
	}
}

func TestGitHistoryCommitsAndNoMerges(t *testing.T) {
	dir := buildMixedFixture(t)

	cfg := gitConfig(Config{GitLog: 3, MaxDiffLines: 2000}, dir)
	cs, _, _, _ := collectForTest(t, cfg)
	if cs.Mode != "history" || len(cs.Commits) != 3 {
		t.Fatalf("-G 3 应产出 3 组，实际 %d（mode=%s）", len(cs.Commits), cs.Mode)
	}
	// 最新提交是 merge
	if !cs.Commits[0].IsMerge {
		t.Errorf("最新提交应是 merge: %+v", cs.Commits[0])
	}
	// merge 提交相对第一父也有 diff（feature 分支带来的 feat.go）
	if changeByPath(cs, "src/feat.go") == nil {
		t.Error("merge 提交（相对第一父）的变更缺失")
	}
	// 已删除文件：正文不输出，但 diff 要在（删除发生在更早的提交，需要覆盖到）
	csAll, _, _, _ := collectForTest(t, gitConfig(Config{GitLog: 7, MaxDiffLines: 2000}, dir))
	if ch := changeByPath(csAll, "docs/tmp.md"); ch == nil || ch.Diff == "" || ch.StatusLabel() != "deleted" {
		t.Errorf("删除文件的处理错误: %+v", ch)
	} else if ch.NoContent != "Not in working tree" {
		t.Errorf("已不在工作区的文件应标注无正文: %+v", ch)
	}

	cfg = gitConfig(Config{GitLog: 5, GitNoMerges: true, MaxDiffLines: 2000}, dir)
	cs, _, _, _ = collectForTest(t, cfg)
	if len(cs.Commits) != 5 {
		t.Fatalf("--no-merges 下 -G 5 应有 5 组，实际 %d", len(cs.Commits))
	}
	for _, grp := range cs.Commits {
		if grp.IsMerge {
			t.Errorf("--no-merges 未排除 merge: %s %s", grp.Short, grp.Subject)
		}
	}
}

func TestGitMaxDiffLinesTruncates(t *testing.T) {
	dir := buildMixedFixture(t)
	cfg := gitConfig(Config{GitDiff: true, MaxDiffLines: 3}, dir)
	cs, _, _, _ := collectForTest(t, cfg)

	truncated := 0
	for _, grp := range cs.Commits {
		for _, ch := range grp.Changes {
			if ch.DiffOmitted > 0 {
				truncated++
				if !strings.Contains(ch.Diff, "diff lines omitted") {
					t.Errorf("截断标记缺失: %s", ch.Path)
				}
				if ch.DiffFull <= ch.DiffOmitted {
					t.Errorf("截断计数不合理: full=%d omitted=%d", ch.DiffFull, ch.DiffOmitted)
				}
			}
		}
	}
	if truncated == 0 {
		t.Error("--max-diff-lines 3 应该截断至少一个文件")
	}
}

func TestGitSubdirScope(t *testing.T) {
	dir := buildMixedFixture(t)
	cfg := gitConfig(Config{GitDiff: true, MaxDiffLines: 2000}, filepath.Join(dir, "docs"))
	cs, _, _, _ := collectForTest(t, cfg)

	if cs.Pathspec != "docs" {
		t.Fatalf("Pathspec = %q，期望 docs（临时目录在 macOS 上还涉及 /var → /private/var 软链）", cs.Pathspec)
	}
	if cs.UniquePaths == 0 {
		t.Fatal("子目录范围内应有变更")
	}
	for _, grp := range cs.Commits {
		for _, ch := range grp.Changes {
			if !strings.HasPrefix(ch.Path, "docs/") {
				t.Errorf("子目录范围外的变更混进来了: %s", ch.Path)
			}
		}
	}
	if !strings.Contains(cs.Spec, "scoped to docs") {
		t.Errorf("Spec 未标注范围: %q", cs.Spec)
	}
}

func TestGitUnbornHead(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("未安装 git")
	}
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	gitIn(t, dir, "config", "user.email", "t@t.c")
	gitIn(t, dir, "config", "user.name", "T")
	writeFile(t, dir, "fresh.txt", "a\nb\n")
	gitIn(t, dir, "add", "fresh.txt")

	cfg := gitConfig(Config{GitDiff: true, MaxDiffLines: 2000}, dir)
	cs, _, _, _ := collectForTest(t, cfg)
	if !cs.Unborn {
		t.Error("空仓库应标记 Unborn")
	}
	if ch := changeByPath(cs, "fresh.txt"); ch == nil || ch.StatusLabel() != "added" {
		t.Errorf("空仓库下暂存的新文件应显示为 added: %+v", ch)
	}
	if len(cs.Warnings) == 0 {
		t.Error("空仓库应给出警告")
	}

	// -G 在空仓库必须明确报错，而不是静默输出空快照
	if _, err := collectChangeSet(&Config{RootDir: dir, GitLog: 3, MaxDiffLines: 2000}); err == nil {
		t.Error("-G 在无提交仓库里应当报错")
	}
}

func TestGitNotARepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("未安装 git")
	}
	// t.TempDir() 不在任何 git 仓库里（若真在，就不该断言）
	dir := t.TempDir()
	if _, err := collectChangeSet(&Config{RootDir: dir, GitDiff: true, MaxDiffLines: 2000}); err == nil {
		t.Skip("临时目录意外处于某个 git 仓库内，跳过")
	}
}

func TestWriteChangeContentSmoke(t *testing.T) {
	dir := buildMixedFixture(t)
	cfg := gitConfig(Config{GitDiff: true, MaxDiffLines: 2000}, dir)
	cs, files, stats, _ := collectForTest(t, cfg)

	var buf bytes.Buffer
	if err := writeChangeContent(cfg, files, stats, cs, &buf); err != nil {
		t.Fatalf("writeChangeContent: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"# Git Change Snapshot:",
		"## Repository",
		"## Change Summary",
		"## Excluded by rules",
		"## Renamed / Copied Files",
		"## Binary Files (no textual diff)",
		"## Changes",
		"```diff",
		"Full content",
		"Sensitive file",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("变更快照缺少 %q", want)
		}
	}
	// 代码围栏必须成对（含反引号的正文/差异不能截断 Markdown）
	if n := countFenceLines(out); n%2 != 0 {
		t.Errorf("代码围栏行数为奇数(%d)，Markdown 结构被截断", n)
	}
}

func countFenceLines(s string) int {
	n := 0
	for _, ln := range strings.Split(s, "\n") {
		if strings.HasPrefix(ln, "```") {
			n++
		}
	}
	return n
}

// 围栏长度必须超过正文里最长的反引号串，否则 diff 里的 ``` 会提前闭合代码块。
func TestChangeContentFenceEscalation(t *testing.T) {
	dir := newGitFixture(t)
	writeFile(t, dir, "doc.md", "before\n")
	writeFile(t, dir, "four.md", "x\n")
	gitIn(t, dir, "add", "-A", "-f", ".")
	gitIn(t, dir, "commit", "-q", "-m", "init")
	// 改动留在工作区：-g 才有 diff；其中 diff 正文含 ```、全文含 ````
	writeFile(t, dir, "doc.md", "before\n```go\nfmt.Println(1)\n```\n")
	writeFile(t, dir, "four.md", "x\n````\ny\n")

	cfg := gitConfig(Config{GitDiff: true, MaxDiffLines: 2000}, dir)
	cs, files, stats, _ := collectForTest(t, cfg)
	var buf bytes.Buffer
	if err := writeChangeContent(cfg, files, stats, cs, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "````diff") {
		t.Error("diff 里含 ``` 时围栏未加长（应为 ````diff）")
	}
	if !strings.Contains(out, "`````markdown") {
		t.Error("正文里含 ```` 时围栏未加长（应为 `````markdown）")
	}
}

// 全量快照的渲染不能被 git 模式的改动带偏（原来内联在 writeContent 里的
// 内容块已抽成 writeFileBody，这里守住输出形状）。
func TestSnapshotRenderingUnchangedByRefactor(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.go"), "package a\n")
	cfg := Config{RootDir: root, OutputFile: "out.md", MaxFileSize: 500 * 1024}
	files, stats, _ := scanDirectory(cfg)

	var buf bytes.Buffer
	if err := writeContent(cfg, files, stats, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"# Project Snapshot:", "## Project Structure", "## Table of Contents", "## a.go", "```go", "package a"} {
		if !strings.Contains(out, want) {
			t.Errorf("全量快照输出缺少 %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "# Git Change Snapshot") {
		t.Error("非 git 模式不应走变更渲染")
	}
}
