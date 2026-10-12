package main

// 以 `git check-ignore` 为真值的对照测试。
//
// 这是匹配器最关键的防线：README 承诺"支持 ! 否定、** 通配等完整 gitignore
// 语法"，而旧实现里 ** 系列几乎全废（实测 **/*.log、src/**/*.go 一个都不生效），
// 却没有任何测试——因为测试只覆盖了"我想到的写法"。
// 这里换一种做法：把模式表交给**git 自己**判定，我们只负责对齐。

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// oraclePatterns 是交给 git 与我们的匹配器同时判定的模式表。
var oraclePatterns = []string{
	"*.log",
	"**/*.log",
	"**/*.tmp",
	"src/**/*.go",
	"**/node_modules",
	"a/**/b",
	"abc/**",
	"build/",
	"/rooted.txt",
	"docs/**/*.md",
	"*.min.js",
	"temp?",
	"[ab]racket.txt",
	"keep/**/deep.txt",
}

// oraclePaths 是被判定的路径（相对仓库根，"/" 分隔）。
var oraclePaths = []string{
	"app.log",
	"src/app.log",
	"a/b/server.log",
	"x.tmp",
	"src/x.tmp",
	"handler.go",
	"src/handler.go",
	"src/pkg/handler.go",
	"other/handler.go",
	"node_modules",
	"deep/node_modules",
	"a/b",
	"a/x/b",
	"a/x/y/b",
	"a/b/c",
	"abc",
	"abc/f.txt",
	"abc/d/deep.txt",
	"abcd",
	"build",
	"build/out.o",
	"src/build/out.o",
	"builder",
	"rooted.txt",
	"src/rooted.txt",
	"docs/guide.md",
	"docs/a/b/guide.md",
	"readme.md",
	"bundle.min.js",
	"src/bundle.min.js",
	"temp1",
	"tempx",
	"temp",
	"aracket.txt",
	"bracket.txt",
	"cracket.txt",
	"keep/deep.txt",
	"keep/a/deep.txt",
	"aaaa",
	"notdeep.txt",
}

// TestIgnoreMatcherMatchesGitCheckIgnore 逐条与 git 对照。
//
// 注意祖先目录被忽略的情况：git check-ignore 对"父目录被忽略"的文件同样返回
// ignore，而我们的遍历会在父目录上 SkipDir、git 模式则由 git 给清单，
// 所以这里对文件单独判定时用"该文件自身的规则链"，与 git 的答案一致。
func TestIgnoreMatcherMatchesGitCheckIgnore(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("未安装 git，跳过与 git 的对照测试")
	}
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")

	// 把所有模式写进根 .gitignore，并让路径真实存在（git check-ignore 对
	// 不存在的路径也能判定，但真实存在更贴近使用场景）。
	mustWrite(t, filepath.Join(dir, ".gitignore"), strings.Join(oraclePatterns, "\n")+"\n")
	for _, p := range oraclePaths {
		full := filepath.Join(dir, filepath.FromSlash(p))
		// 名字里没有扩展名的一律建目录（build、node_modules、abc 这些是目录）
		if strings.Contains(filepath.Base(p), ".") {
			writeFile(t, dir, p, "x\n") // 会顺便建好父目录
		} else {
			mustMkdir(t, full)
		}
	}

	matcher := newIgnoreMatcher(dir, ".gitignore")

	mismatches := 0
	for _, p := range oraclePaths {
		cmd := exec.Command("git", "check-ignore", "-q", "--no-index", "--", p)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		gitIgnored := cmd.Run() == nil

		isDir := !strings.Contains(filepath.Base(p), ".")
		applies, ignored, _ := matcher.ShouldIgnore(p, isDir)
		ours := applies && ignored

		if ours != gitIgnored {
			mismatches++
			t.Errorf("%-22s git=%v 我们=%v", p, gitIgnored, ours)
		}
	}
	if mismatches == 0 {
		t.Logf("与 git check-ignore 完全一致（%d 个模式 × %d 条路径）", len(oraclePatterns), len(oraclePaths))
	}
}

// 嵌套 .gitignore：子目录里的 .gitignore 也必须生效（旧实现只读根目录）。
func TestNestedGitignoreFilesAreHonored(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".gitignore"), "*.log\n")
	mustMkdir(t, filepath.Join(root, "sub"))
	mustWrite(t, filepath.Join(root, "sub", ".gitignore"), "*.tmp\nignored.go\n")
	mustMkdir(t, filepath.Join(root, "sub", "deep"))
	mustWrite(t, filepath.Join(root, "sub", "deep", ".gitignore"), "deep_only.go\n")
	mustWrite(t, filepath.Join(root, "sub", "deep", "deep_only.go"), "package deep\n")

	matcher := newIgnoreMatcher(root, ".gitignore")
	cases := []struct {
		path string
		want bool
	}{
		{"a.log", true},                 // 根 .gitignore
		{"sub/b.log", true},             // 根的规则对子目录同样生效
		{"sub/c.tmp", true},             // 子目录 .gitignore
		{"c.tmp", false},                // 子目录规则不外溢到根
		{"sub/ignored.go", true},        //
		{"sub/deep/deep_only.go", true}, // 更深一层
		{"deep_only.go", false},
		{"sub/keep.go", false},
	}
	for _, c := range cases {
		_, ignored, _ := matcher.ShouldIgnore(c.path, false)
		if ignored != c.want {
			t.Errorf("%s: ignored=%v，期望 %v", c.path, ignored, c.want)
		}
	}
}

// 负向模式（!）按"最后一条命中的规则生效"，且 .gdignore 压过 .gitignore。
func TestIgnoreNegationAndGdignorePrecedence(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".gitignore"), "*.log\n!keep.log\n")
	mustWrite(t, filepath.Join(root, ".gdignore"), "*.md\n!keep.md\n")
	// .gdignore 压过 .gitignore：这里把 .gitignore 忽略的 keep.log 重新纳入
	mustWrite(t, filepath.Join(root, ".gdignore"), "*.md\n!keep.md\n!keep.log\n")

	matcher := newIgnoreMatcher(root, ".gitignore", ".gdignore")
	cases := []struct {
		path string
		want bool
	}{
		{"x.log", true},
		{"keep.log", false}, // 被 .gitignore 的 ! 重新纳入，且 .gdignore 也放行
		{"doc.md", true},
		{"keep.md", false},
		{"main.go", false},
	}
	for _, c := range cases {
		_, ignored, source := matcher.ShouldIgnore(c.path, false)
		if ignored != c.want {
			t.Errorf("%s: ignored=%v（来源 %s），期望 %v", c.path, ignored, source, c.want)
		}
	}

	// 来源要能说清"被谁排除的"（git 模式的排除清单会用到）
	if _, _, source := matcher.ShouldIgnore("doc.md", false); source != ".gdignore" {
		t.Errorf("doc.md 应报告来源 .gdignore，实际 %q", source)
	}
	if _, _, source := matcher.ShouldIgnore("x.log", false); source != ".gitignore" {
		t.Errorf("x.log 应报告来源 .gitignore，实际 %q", source)
	}
}

// 命令行 -I 的优先级最高，且支持 ** 与 ! （旧实现是 filepath.Match 的简化 glob，
// **/*.log 这类写法完全无效，Windows 上 * 还会跨过 /）。
func TestIgnoreCLIPatterns(t *testing.T) {
	root := t.TempDir()
	matcher := newIgnoreMatcher(root) // 没有 ignore 文件，只有 CLI 模式
	matcher.addCLIPatterns([]string{"**/*.log", "src/*.go", "!src/keep.go"})

	cases := []struct {
		path string
		want bool
	}{
		{"app.log", true},
		{"a/b/server.log", true},
		{"src/handler.go", true},
		{"src/sub/handler.go", false}, // "*" 不跨过 "/"
		{"src/keep.go", false},        // ! 重新纳入
		{"other.go", false},
	}
	for _, c := range cases {
		_, ignored := matcher.MatchCLI(c.path, false)
		if ignored != c.want {
			t.Errorf("-I 判定 %s = %v，期望 %v", c.path, ignored, c.want)
		}
	}
}

// 目录规则在 git 变更模式里也必须兜住"目录下的文件"（那里没有遍历，不会 SkipDir）。
func TestDirOnlyRuleCoversFilesInGitMode(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".gdignore"), "build/\n")
	matcher := newIgnoreMatcher(root, ".gdignore")

	for _, p := range []string{"build", "build/out.o", "build/deep/out.o", "builder", "src/build"} {
		_, ignored, _ := matcher.ShouldIgnore(p, strings.HasSuffix(p, "build") || p == "build")
		switch p {
		case "builder", "src/build":
			// src/build 命中（任意层级的 build 目录），builder 不命中
			want := p == "src/build"
			if ignored != want {
				t.Errorf("%s: ignored=%v，期望 %v", p, ignored, want)
			}
		default:
			if !ignored {
				t.Errorf("%s 应被 build/ 忽略", p)
			}
		}
	}
}
