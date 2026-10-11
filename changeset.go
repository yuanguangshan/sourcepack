package main

// ============================================================
//  Git Change Packaging  (-g / --git-diff, -G / --commits)
// ============================================================
//
// 设计原则：这里只负责"把 Git 的变更变成一份 changeset"，
// 过滤（-i/-x/-m/-X/-I）、忽略、敏感文件 gate、体积上限、渲染、
// 剪贴板/推送/-s 全部复用 godoc.go 里既有的 pipeline。
//
// 之所以自己调 git 而不是引 go-git：
//   - go-git 会给二进制增加数 MB，且不完整支持 filters / submodule /
//     core.autocrlf 这些真实行为；
//   - 调 git 天然尊重用户已有配置，且出问题时的报错就是 git 自己的报错。
//
// 关键实现细节（都是踩过的坑，别改）：
//  1. --no-pager + GIT_PAGER=cat + cmd.Stdin=nil：否则 git log 可能等
//     stdin，把自动化工具挂死。
//  2. -c core.quotepath=false：否则中文文件名在 patch 里会变成
//     \344\275\240 这种八进制转义。
//  3. 一切"文件清单"都用 -z（NUL 分隔）解析：路径含空格/换行时，
//     非 -z 的 --name-status 输出根本无法可靠切分。
//  4. --no-ext-diff --no-textconv：否则用户配置的外部 diff / textconv
//     会被触发。
//  5. 从 diff 正文里解析路径时要注意 git 给含空格路径加的尾部 TAB。

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// emptyTreeSHA 是 Git 里那个众所周知的空树对象，用于给"没有父提交"
	// 的根提交（以及 unborn HEAD）做 diff 基线。
	emptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

	// maxPatchBytes 是单次 diff 调用的输出上限。截图一个 500MB 的 diff
	// 进内存没有意义，超限时会在最后一个完整文件边界处截断并标注。
	maxPatchBytes = 64 << 20

	// warnDiffLines 超过这个总行数会在终端和文档里提示输出偏大。
	warnDiffLines = 20000

	logFieldSep = "\x1f"
)

// Change 是一个文件在一次 changeset 中的变更记录。
type Change struct {
	Path        string // 新路径（重命名/复制后的路径）
	OldPath     string // 仅重命名/复制时非空
	Status      string // git 原始状态码：A M D R100 C75 T U ??
	Added       int
	Deleted     int
	Binary      bool
	Diff        string // unified diff 正文（已按 --max-diff-lines 截断）
	DiffFull    int    // 截断前的总行数
	DiffOmitted int    // 被 --max-diff-lines 省略的行数
	Staged      bool   // 在索引里（相对 base 已 git add）
	Unstaged    bool   // 工作区相对索引还有改动（部分暂存）
	Untracked   bool
	Filtered    string // 非空表示被过滤规则排除，值为原因
	NoContent   string // 非空表示不输出全文，值为原因
}

// StatusLabel 把 git 状态码翻译成人话。
func (c *Change) StatusLabel() string {
	if c.Status == "" {
		return "changed"
	}
	switch c.Status[0] {
	case 'A':
		return "added"
	case 'M':
		return "modified"
	case 'D':
		return "deleted"
	case 'R':
		return "renamed"
	case 'C':
		return "copied"
	case 'T':
		return "type changed"
	case 'U':
		return "unmerged"
	case '?':
		return "untracked"
	}
	return "changed"
}

// Flags 返回一行简短的附加标记，例如 "staged, partially staged"。
func (c *Change) Flags() []string {
	var f []string
	if c.Untracked {
		f = append(f, "untracked")
	}
	if c.Staged {
		f = append(f, "staged")
	}
	if c.Unstaged {
		f = append(f, "unstaged")
	}
	if c.Binary {
		f = append(f, "binary")
	}
	return f
}

// CommitGroup 是一次提交（-G 模式）或一组未提交改动（-g 模式）的变更集合。
type CommitGroup struct {
	SHA     string
	Short   string
	Date    string
	Author  string
	Subject string
	Parents []string
	IsMerge bool
	Changes []*Change
}

// ChangeSet 是一次 Git 变更打包的完整结果。
type ChangeSet struct {
	RepoRoot       string
	Branch         string
	Head           string
	HeadSubject    string
	Unborn         bool
	Mode           string // "working tree" | "staged" | "history"
	Spec           string // 人类可读的来源描述
	Pathspec       string // 非空表示被 -d 限制在仓库子目录下
	Commits        []*CommitGroup
	Excluded       []SkippedFile // 被过滤规则排除的变更
	UniquePaths    int
	TotalAdded     int
	TotalDeleted   int
	TotalDiffLines int
	StagedCount    int
	UnstagedCount  int
	UntrackedCount int
	PatchClipped   bool
	Warnings       []string
}

// ============================================================
//  git 调用封装
// ============================================================

type gitRunner struct {
	dir      string
	pathspec string // 仓库根相对路径，"" 表示整个仓库
}

func gitAvailable() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git executable not found in PATH (required by -g/--git-diff and -G/--commits)")
	}
	return nil
}

func newGitRunner(dir string) *gitRunner { return &gitRunner{dir: dir} }

// scoped 返回 -- <pathspec> 后缀，未限定时为空。
func (g *gitRunner) scoped() []string {
	if g.pathspec == "" {
		return nil
	}
	return []string{"--", g.pathspec}
}

func (g *gitRunner) command(args ...string) *exec.Cmd {
	full := make([]string, 0, len(args)+5)
	full = append(full, "--no-pager", "-c", "core.quotepath=false", "-C", g.dir)
	full = append(full, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(),
		"GIT_PAGER=cat",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
	)
	cmd.Stdin = nil // 防止 git 去读 stdin 而挂住
	return cmd
}

func (g *gitRunner) run(args ...string) (string, error) {
	var out, errBuf bytes.Buffer
	cmd := g.command(args...)
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return out.String(), fmt.Errorf("git %s failed: %s", strings.Join(args, " "), msg)
	}
	return out.String(), nil
}

// runLimited 读取 git 输出，但最多读 limit 字节；clipped 表示被截断。
func (g *gitRunner) runLimited(limit int, args ...string) (string, bool, error) {
	var errBuf bytes.Buffer
	cmd := g.command(args...)
	cmd.Stderr = &errBuf
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", false, err
	}
	if err := cmd.Start(); err != nil {
		return "", false, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	data, readErr := io.ReadAll(io.LimitReader(stdout, int64(limit)+1))
	clipped := len(data) > limit
	if clipped {
		data = data[:limit]
	}
	// 必须把剩余输出排干，否则子进程可能阻塞在写管道上导致 Wait 挂死。
	_, _ = io.Copy(io.Discard, stdout)
	waitErr := cmd.Wait()
	// clipped 时同样要报错：上面已经把管道排干，子进程不会因为我们停止读取
	// 而被 SIGPIPE 杀掉，所以此刻的 waitErr 一定是 git 自己失败了——不能吞。
	if waitErr != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = waitErr.Error()
		}
		return string(data), clipped, fmt.Errorf("git %s failed: %s", strings.Join(args, " "), msg)
	}
	if readErr != nil && readErr != io.EOF {
		return string(data), clipped, readErr
	}
	return string(data), clipped, nil
}

// ============================================================
//  收集 changeset
// ============================================================

func collectChangeSet(config *Config) (*ChangeSet, error) {
	if err := gitAvailable(); err != nil {
		return nil, err
	}
	if config.GitRev == "" {
		config.GitRev = "HEAD"
	}

	userDir := config.RootDir
	top, err := newGitRunner(userDir).run("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("git mode needs a git working tree (run inside a repository or pass -d): %w", err)
	}
	repoRoot := filepath.FromSlash(strings.TrimSpace(top))
	if repoRoot == "" {
		return nil, errors.New("git mode: could not determine repository root")
	}

	// 计算 -d 相对仓库根的路径：非空时把打包范围限制在该子目录。
	//
	// 用 git rev-parse --show-prefix 而不是 filepath.Rel：后者在
	// /tmp → /private/tmp 这类符号链接下会算出 "../../tmp/..." 而失效
	// （--show-toplevel 返回的是真实路径，-d 可能是软链路径）。
	pathspec := ""
	if pfx, perr := newGitRunner(userDir).run("rev-parse", "--show-prefix"); perr == nil {
		pathspec = strings.Trim(filepath.ToSlash(strings.TrimSpace(pfx)), "/")
	}

	// 后续所有路径都以仓库根为基准，输出标题也用仓库名。
	config.RootDir = repoRoot
	g := newGitRunner(repoRoot)
	g.pathspec = pathspec

	cs := &ChangeSet{RepoRoot: repoRoot, Pathspec: pathspec}

	if br, berr := g.run("symbolic-ref", "--short", "HEAD"); berr == nil && strings.TrimSpace(br) != "" {
		cs.Branch = strings.TrimSpace(br)
	} else if sha, serr := g.run("rev-parse", "--short", "HEAD"); serr == nil {
		cs.Branch = "detached at " + strings.TrimSpace(sha)
	} else {
		cs.Branch = "(no commits yet)"
	}

	if head, herr := g.run("rev-parse", "HEAD"); herr == nil {
		cs.Head = strings.TrimSpace(head)
		if s, serr := g.run("log", "-1", "--format=%s"); serr == nil {
			cs.HeadSubject = strings.TrimSpace(s)
		}
	} else {
		cs.Unborn = true
		cs.HeadSubject = "(no commits yet)"
	}

	if config.GitLog > 0 {
		if err := collectHistory(g, config, cs); err != nil {
			return nil, err
		}
	} else {
		if err := collectWorkingTree(g, config, cs); err != nil {
			return nil, err
		}
	}

	cs.UniquePaths = countUniquePaths(cs)
	summarizeChangeSet(cs)
	return cs, nil
}

// collectWorkingTree 处理 -g：工作区 + 索引 相对 --rev 的变更。
func collectWorkingTree(g *gitRunner, config *Config, cs *ChangeSet) error {
	base := config.GitRev
	if cs.Unborn {
		base = emptyTreeSHA
		cs.Warnings = append(cs.Warnings, "repository has no commits yet — diffing against the empty tree")
	} else if _, err := g.run("rev-parse", "--verify", base+"^{commit}"); err != nil {
		return fmt.Errorf("invalid --rev %q: %w", config.GitRev, err)
	}

	scope := ""
	if cs.Pathspec != "" {
		scope = " (scoped to " + cs.Pathspec + ")"
	}
	if config.GitStaged {
		cs.Mode = "staged"
		cs.Spec = "git diff --cached " + base + scope
	} else {
		cs.Mode = "working tree"
		cs.Spec = "git diff " + base + scope + "  (working tree + index vs " + base + ")"
	}

	diffCommon := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "-M"}
	if config.GitStaged {
		diffCommon = append(diffCommon, "--cached")
	}
	diffCommon = append(diffCommon, base)

	// 1. 文件清单（权威来源，-z 解析）
	nsArgs := []string{"diff", "--no-ext-diff", "--name-status", "-z", "-M"}
	if config.GitStaged {
		nsArgs = append(nsArgs, "--cached")
	}
	nsArgs = append(nsArgs, base)
	nsArgs = append(nsArgs, g.scoped()...)

	nsOut, err := g.run(nsArgs...)
	if err != nil {
		return err
	}
	changes, err := parseNameStatusZ(nsOut)
	if err != nil {
		return err
	}

	// 2. 暂存/未暂存标记（仅在 -g 非 --staged 时分别求）
	stagedSet := map[string]bool{}
	unstagedSet := map[string]bool{}
	if !config.GitStaged && !cs.Unborn {
		stagedArgs := append([]string{"diff", "--no-ext-diff", "--name-status", "-z", "-M", "--cached", base}, g.scoped()...)
		if out, cerr := g.run(stagedArgs...); cerr == nil {
			markPaths(out, stagedSet, true)
		}
		unstagedArgs := append([]string{"diff", "--no-ext-diff", "--name-status", "-z", "-M"}, g.scoped()...)
		if out, uerr := g.run(unstagedArgs...); uerr == nil {
			markPaths(out, unstagedSet, false)
		}
	}

	// 3. 未跟踪文件（--staged 时不适用）
	var untracked []string
	if !config.GitStaged {
		lsArgs := append([]string{"ls-files", "--others", "--exclude-standard", "--full-name", "-z"}, g.scoped()...)
		if out, lerr := g.run(lsArgs...); lerr == nil {
			for _, p := range strings.Split(out, "\x00") {
				if p != "" {
					untracked = append(untracked, p)
				}
			}
		}
	}

	// 4. diff 正文（带字节上限）
	patchArgs := append(append([]string{}, diffCommon...), g.scoped()...)
	patch, clipped, perr := g.runLimited(maxPatchBytes, patchArgs...)
	if perr != nil {
		return perr
	}
	if clipped {
		patch = trimToLastCompleteChunk(patch)
		cs.PatchClipped = true
		cs.Warnings = append(cs.Warnings, "diff output exceeded 64 MB and was truncated at a file boundary")
	}

	chunks := splitPatch(patch)
	byPath := indexChunks(chunks)
	changeIndex := indexChanges(changes)
	for _, ch := range changes {
		applyChunk(ch, byPath, config.MaxDiffLines, config.MaxFileSize)
		ch.Staged = stagedSet[ch.Path]
		ch.Unstaged = unstagedSet[ch.Path]
	}
	changes = appendUnmatchedChunks(changes, changeIndex, chunks, byPath, config.MaxDiffLines, config.MaxFileSize)
	for _, p := range untracked {
		changes = append(changes, &Change{Path: p, Status: "??", Untracked: true})
	}

	if config.GitStaged {
		// --staged 的清单本身就来自 `git diff --cached`，每一条都是暂存的。
		// 上面的 stagedSet 只在非 --staged 时才填充，所以这里必须显式标，
		// 否则 summarizeChangeSet 的 switch 全落空、StagedCount 恒为 0。
		for _, ch := range changes {
			ch.Staged = true
		}
	}

	sort.SliceStable(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })

	subject := "uncommitted changes"
	if config.GitStaged {
		subject = "staged changes"
	}
	cs.Commits = []*CommitGroup{{
		SHA:     cs.Head,
		Short:   shortSHA(cs.Head),
		Date:    time.Now().Format("2006-01-02"),
		Subject: subject,
		Changes: changes,
	}}
	return nil
}

// collectHistory 处理 -G：最近 N 个提交。
func collectHistory(g *gitRunner, config *Config, cs *ChangeSet) error {
	if cs.Unborn {
		return errors.New("repository has no commits yet: -G/--commits needs at least one commit")
	}
	cs.Mode = "history"
	mergeNote := ""
	if config.GitNoMerges {
		mergeNote = " --no-merges"
	}
	scope := ""
	if cs.Pathspec != "" {
		scope = " -- " + cs.Pathspec
	}
	cs.Spec = fmt.Sprintf("git log -%d%s%s", config.GitLog, mergeNote, scope)

	logArgs := []string{"log", fmt.Sprintf("-%d", config.GitLog), "--date=short",
		"--format=%H" + logFieldSep + "%h" + logFieldSep + "%an" + logFieldSep + "%ad" + logFieldSep + "%P" + logFieldSep + "%s"}
	if config.GitNoMerges {
		logArgs = append(logArgs, "--no-merges")
	}
	logArgs = append(logArgs, g.scoped()...)

	out, err := g.run(logArgs...)
	if err != nil {
		return err
	}
	var groups []*CommitGroup
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, logFieldSep)
		if len(parts) < 6 {
			continue
		}
		grp := &CommitGroup{
			SHA:     parts[0],
			Short:   parts[1],
			Author:  parts[2],
			Date:    parts[3],
			Subject: parts[5],
		}
		if p := strings.TrimSpace(parts[4]); p != "" {
			grp.Parents = strings.Fields(p)
		}
		grp.IsMerge = len(grp.Parents) > 1
		groups = append(groups, grp)
	}
	if len(groups) == 0 {
		cs.Warnings = append(cs.Warnings, "no commits matched the requested range")
		cs.Commits = nil
		return nil
	}

	for _, grp := range groups {
		base := emptyTreeSHA
		if len(grp.Parents) > 0 {
			base = grp.Parents[0]
		}
		nsArgs := append([]string{"diff", "--no-ext-diff", "--name-status", "-z", "-M", base, grp.SHA}, g.scoped()...)
		nsOut, nerr := g.run(nsArgs...)
		if nerr != nil {
			return nerr
		}
		changes, perr := parseNameStatusZ(nsOut)
		if perr != nil {
			return perr
		}

		patchArgs := append([]string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "-M", base, grp.SHA}, g.scoped()...)
		patch, clipped, derr := g.runLimited(maxPatchBytes, patchArgs...)
		if derr != nil {
			return derr
		}
		if clipped {
			patch = trimToLastCompleteChunk(patch)
			cs.PatchClipped = true
			// 警告在循环外统一记一次：-G N 下多个 commit 都可能超限，
			// 在循环里 append 会让同一条警告重复出现
		}
		chunks := splitPatch(patch)
		byPath := indexChunks(chunks)
		changeIndex := indexChanges(changes)
		for _, ch := range changes {
			applyChunk(ch, byPath, config.MaxDiffLines, config.MaxFileSize)
		}
		changes = appendUnmatchedChunks(changes, changeIndex, chunks, byPath, config.MaxDiffLines, config.MaxFileSize)
		sort.SliceStable(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
		grp.Changes = changes
	}

	if cs.PatchClipped {
		cs.Warnings = append(cs.Warnings, "diff output exceeded 64 MB and was truncated at a file boundary")
	}

	cs.Commits = groups
	return nil
}

func shortSHA(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

func countUniquePaths(cs *ChangeSet) int {
	seen := map[string]bool{}
	for _, grp := range cs.Commits {
		for _, ch := range grp.Changes {
			seen[ch.Path] = true
		}
	}
	return len(seen)
}

// summarizeChangeSet 汇总计数。可重复调用（会先清零），
// 因为 buildChangeFiles 会把被过滤的变更标记出来，标记后需要重算一次。
func summarizeChangeSet(cs *ChangeSet) {
	cs.TotalAdded, cs.TotalDeleted, cs.TotalDiffLines = 0, 0, 0
	cs.StagedCount, cs.UnstagedCount, cs.UntrackedCount = 0, 0, 0

	perPath := map[string][2]int{}
	for _, grp := range cs.Commits {
		for _, ch := range grp.Changes {
			switch {
			case ch.Untracked:
				cs.UntrackedCount++
			case ch.Staged && ch.Unstaged:
				cs.StagedCount++
				cs.UnstagedCount++
			case ch.Staged:
				cs.StagedCount++
			case ch.Unstaged:
				cs.UnstagedCount++
			}
			if ch.Filtered == "" {
				cur := perPath[ch.Path]
				perPath[ch.Path] = [2]int{cur[0] + ch.Added, cur[1] + ch.Deleted}
				cs.TotalDiffLines += ch.DiffFull
			}
		}
	}
	for _, v := range perPath {
		cs.TotalAdded += v[0]
		cs.TotalDeleted += v[1]
	}
}

// markPaths 把 --name-status -z 输出里的所有路径塞进集合。
//
// includeOldPath 为真时连重命名的旧路径一起标记：暂存集合来自
// `git diff --cached`，它报的是 "R100 old new"；而后续 `git diff`
// （工作区 vs 索引）不会再提 old。若不给旧路径打标记，"已暂存的重命名"
// 这一侧就会漏掉 staged 标记。
func markPaths(nsOut string, set map[string]bool, includeOldPath bool) {
	changes, err := parseNameStatusZ(nsOut)
	if err != nil {
		// 这里只负责 staged/unstaged 标记，解析失败不影响主清单
		// （主清单解析失败会直接让整个命令报错）
		return
	}
	for _, ch := range changes {
		set[ch.Path] = true
		if includeOldPath && ch.OldPath != "" {
			set[ch.OldPath] = true
		}
	}
}

// indexChanges 建立 path → Change 索引。兜底逻辑要按路径查重，
// 线性扫在 -G 50 那种量级下是 O(块 × 变更)，没必要。
func indexChanges(changes []*Change) map[string]*Change {
	m := make(map[string]*Change, len(changes))
	for _, ch := range changes {
		m[ch.Path] = ch
	}
	return m
}

// appendUnmatchedChunks 是防御性兜底：patch 里出现、但 --name-status
// 清单里没有的文件也要输出，否则会静默丢内容（正常情况下不该发生，
// 例如 git 版本差异导致两侧排序不一致时）。
func appendUnmatchedChunks(changes []*Change, index map[string]*Change, chunks []patchChunk, byPath map[string]*patchChunk, maxDiffLines int, maxDiffBytes int64) []*Change {
	for _, ck := range chunks {
		if ck.Path == "" {
			continue
		}
		if _, ok := index[ck.Path]; ok {
			continue
		}
		ch := &Change{Path: ck.Path, Status: "M"}
		applyChunk(ch, byPath, maxDiffLines, maxDiffBytes)
		changes = append(changes, ch)
		index[ck.Path] = ch
	}
	return changes
}

// ============================================================
//  --name-status -z 解析
// ============================================================

// parseNameStatusZ 解析 `git diff --name-status -z` 的输出。
//
// -z 下的实际字节形如：
//
//	A\0added.txt\0                  → 单路径
//	R100\0sub/deep.go\0sub/renamed.go\0  → 状态码 + 旧路径 + 新路径
//
// 非 -z 形式对含空格/换行的路径无法可靠切分，所以绝不解析非 -z 输出。
func parseNameStatusZ(s string) ([]*Change, error) {
	fields := strings.Split(s, "\x00")
	var out []*Change
	for i := 0; i < len(fields); {
		code := fields[i]
		if code == "" {
			i++
			continue
		}
		if len(code) > 1 && (code[0] == 'R' || code[0] == 'C') {
			if i+2 >= len(fields) || fields[i+1] == "" || fields[i+2] == "" {
				return out, fmt.Errorf("malformed --name-status -z output: %q is missing the old/new path pair", code)
			}
			out = append(out, &Change{Status: code, OldPath: fields[i+1], Path: fields[i+2]})
			i += 3
			continue
		}
		if i+1 >= len(fields) || fields[i+1] == "" {
			return out, fmt.Errorf("malformed --name-status -z output: status %q has no path", code)
		}
		out = append(out, &Change{Status: code, Path: fields[i+1]})
		i += 2
	}
	return out, nil
}

// ============================================================
//  diff 正文解析
// ============================================================

type patchChunk struct {
	Path       string // 新路径（可能为空，需回退到 HeaderPath）
	HeaderPath string // 从 `diff --git a/X b/Y` 行得到的兜底路径
	Body       string
	Added      int
	Deleted    int
	Binary     bool
}

// splitPatch 按 `diff --git ` 边界把整份 patch 切成每文件一块，
// 并顺带统计每块的增删行数。
//
// 统计必须在"进入 hunk"之后才按 +/- 计数：因为 hunk 正文里一行
// "---- foo" 同样以 "---" 开头，纯前缀判断会把正文误当成文件头。
func splitPatch(patch string) []patchChunk {
	var chunks []patchChunk
	var cur *patchChunk
	inHunk := false
	var body strings.Builder
	flush := func() {
		if cur != nil {
			cur.Body = body.String()
			chunks = append(chunks, *cur)
		}
	}
	for _, ln := range strings.Split(patch, "\n") {
		if strings.HasPrefix(ln, "diff --git ") {
			flush()
			cur = &patchChunk{HeaderPath: parseDiffGitHeader(ln)}
			body.Reset()
			inHunk = false
			body.WriteString(ln)
			body.WriteByte('\n')
			continue
		}
		if cur == nil {
			continue // 跳过 patch 之前的杂项输出
		}
		body.WriteString(ln)
		body.WriteByte('\n')
		if inHunk {
			switch {
			case strings.HasPrefix(ln, "+"):
				cur.Added++
			case strings.HasPrefix(ln, "-"):
				cur.Deleted++
			}
			continue
		}
		switch {
		case strings.HasPrefix(ln, "@@"):
			inHunk = true
		case strings.HasPrefix(ln, "+++ "):
			if p := parseDiffPath(ln[4:]); p != "" {
				cur.Path = p
			}
		case strings.HasPrefix(ln, "--- "):
			// 删除类 diff 没有可用的 `+++` 行（它是 `+++ /dev/null`，解析成空），
			// 所以这里兜一次底；其余情况由 `+++` / name-status 决定。
			if cur.Path == "" {
				if p := parseDiffPath(ln[4:]); p != "" {
					cur.Path = p
				}
			}
		case strings.HasPrefix(ln, "rename to "):
			cur.Path = unquoteGitPath(strings.TrimSpace(strings.TrimPrefix(ln, "rename to ")))
		case strings.HasPrefix(ln, "copy to "):
			cur.Path = unquoteGitPath(strings.TrimSpace(strings.TrimPrefix(ln, "copy to ")))
		case strings.HasPrefix(ln, "Binary files "), strings.HasPrefix(ln, "GIT binary patch"):
			cur.Binary = true
		}
	}
	flush()
	for i := range chunks {
		if chunks[i].Path == "" {
			chunks[i].Path = chunks[i].HeaderPath
		}
	}
	return chunks
}

func indexChunks(chunks []patchChunk) map[string]*patchChunk {
	m := make(map[string]*patchChunk, len(chunks))
	for i := range chunks {
		if chunks[i].Path != "" {
			m[chunks[i].Path] = &chunks[i]
		}
	}
	return m
}

// parseDiffGitHeader 从 `diff --git a/X b/Y` 中取新路径作为兜底。
func parseDiffGitHeader(line string) string {
	rest := strings.TrimPrefix(line, "diff --git ")
	if i := strings.LastIndex(rest, " b/"); i > 0 {
		if p := parseDiffPath(rest[i+1:]); p != "" {
			return p
		}
	}
	return parseDiffPath(rest)
}

// parseDiffPath 解析 `+++`/`---`/`b/xxx` 里的路径。
//
// 注意：git 对含空格的路径会写成 `--- a/hello world.txt\t`（尾部 TAB
// 是"后面紧跟时间戳"的分隔符），所以必须先剥掉 TAB；而被引号包起来的
// 路径（含双引号/反斜杠/控制字符）要按 C 风格反转义。
func parseDiffPath(s string) string {
	var p string
	if strings.HasPrefix(s, "\"") {
		end := -1
		for i := 1; i < len(s); i++ {
			if s[i] == '\\' {
				i++
				continue
			}
			if s[i] == '"' {
				end = i
				break
			}
		}
		if end > 0 {
			p = unquoteGitPath(s[:end+1])
		} else {
			p = s
		}
	} else if i := strings.IndexByte(s, '\t'); i >= 0 {
		p = s[:i]
	} else {
		p = s
	}
	if p == "/dev/null" {
		return ""
	}
	p = unquoteGitPath(p)
	p = strings.TrimPrefix(p, "a/")
	p = strings.TrimPrefix(p, "b/")
	return p
}

// unquoteGitPath 处理 git 的 C 风格引号（仅在路径含双引号/反斜杠/控制
// 字符时出现；中文在 core.quotepath=false 下是原样 UTF-8，不会加引号）。
func unquoteGitPath(s string) string {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return s
	}
	inner := s[1 : len(s)-1]
	var b strings.Builder
	for i := 0; i < len(inner); i++ {
		if inner[i] != '\\' || i+1 >= len(inner) {
			b.WriteByte(inner[i])
			continue
		}
		i++
		switch inner[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '"':
			b.WriteByte('"')
		case '\\':
			b.WriteByte('\\')
		case 'a':
			b.WriteByte(7)
		case 'b':
			b.WriteByte(8)
		case 'f':
			b.WriteByte(12)
		case 'v':
			b.WriteByte(11)
		default:
			// 八进制转义 \NNN
			if inner[i] >= '0' && inner[i] <= '7' {
				j := i
				for j < len(inner) && j < i+3 && inner[j] >= '0' && inner[j] <= '7' {
					j++
				}
				var v int
				for _, c := range inner[i:j] {
					v = v*8 + int(c-'0')
				}
				b.WriteByte(byte(v))
				i = j - 1
			} else {
				b.WriteByte(inner[i])
			}
		}
	}
	return b.String()
}

// diffBodyLooksBinary 检查 git 给出的 diff 正文里有没有裸字节。
//
// 为什么需要这层：git 的二进制判定只看前 8000 字节里有没有 NUL，
// 所以"前 8000 字节干净、后面才有 NUL"的文件、以及无 NUL 但非法
// UTF-8 的文件（Latin-1/GBK 文本、部分裸二进制），git 都会输出**逐行
// 文本 diff**，正文里就是原始字节。直接落盘会让整个产物变成非法 UTF-8。
func diffBodyLooksBinary(body string) bool {
	if body == "" {
		return false
	}
	if strings.IndexByte(body, 0) >= 0 {
		return true
	}
	return !utf8.ValidString(body)
}

// applyChunk 把 patch 分块的结果贴到一个 Change 上，并按需截断。
func applyChunk(ch *Change, byPath map[string]*patchChunk, maxLines int, maxBytes int64) {
	ck := byPath[ch.Path]
	if ck == nil && ch.OldPath != "" {
		ck = byPath[ch.OldPath]
	}
	if ck == nil {
		return
	}
	ch.Added, ch.Deleted, ch.Binary = ck.Added, ck.Deleted, ck.Binary

	if diffBodyLooksBinary(ck.Body) {
		// 不出正文，只保留清单与状态（渲染层会写 "Binary file"）。
		// DiffFull 保持 0：这段正文不会进产物，"总量过大"告警不该算它。
		ch.Binary = true
		return
	}
	ch.DiffFull = strings.Count(ck.Body, "\n")
	if trimmed, omitted := capDiff(ck.Body, maxLines, maxBytes); omitted > 0 {
		ch.Diff = trimmed
		ch.DiffOmitted = omitted
	} else {
		ch.Diff = ck.Body
	}
}

// splitDiffLines 切分 diff 正文，且不把结尾换行算成一行。
func splitDiffLines(body string) []string {
	lines := strings.Split(body, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}

// capDiff 截断 diff 正文：**先按字节、再按行**，返回正文与省略行数。
//
// 为什么必须先管字节：--max-diff-lines 只数行，单行超长文件
// （压缩过的 JS/JSON、单行大文件）一行 20MB 也照样进产物。实测
// 一个已跟踪的单行 20MB 文本会产出 40MB 的 Markdown。
func capDiff(body string, maxLines int, maxBytes int64) (string, int) {
	if maxBytes > 0 && int64(len(body)) > maxBytes {
		return capDiffBytes(body, maxBytes)
	}
	if maxLines <= 0 {
		return body, 0
	}
	lines := splitDiffLines(body)
	if len(lines) <= maxLines {
		return body, 0
	}
	omitted := len(lines) - maxLines
	var b strings.Builder
	for _, ln := range lines[:maxLines] {
		b.WriteString(ln)
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "... [sourcepack] %d of %d diff lines omitted (--max-diff-lines %d)\n", omitted, len(lines), maxLines)
	return b.String(), omitted
}

// capDiffBytes 按字节上限保留完整的行，超出部分丢弃并标注。
// 单行本身就超上限时（压缩过的 JS/JSON、单行大文件）只能在该行内硬切，
// 且必须切在 UTF-8 字符边界上，否则产物会变成非法 UTF-8。
func capDiffBytes(body string, maxBytes int64) (string, int) {
	lines := splitDiffLines(body)
	var b strings.Builder
	var used int64
	kept := 0
	cutMidLine := false
	for _, ln := range lines {
		add := int64(len(ln)) + 1
		if used+add > maxBytes {
			if kept == 0 {
				if piece := truncateUTF8(ln, maxBytes-1); piece != "" {
					b.WriteString(piece)
					b.WriteByte('\n')
					kept++
					cutMidLine = true
				}
			}
			break
		}
		b.WriteString(ln)
		b.WriteByte('\n')
		used += add
		kept++
	}
	// 返回的 omitted 兼作"正文被改过吗"的标记（调用方只在 omitted > 0 时
	// 采用截断结果）。单行硬切时该行只保留一部分，按"整行丢弃数"是 0，
	// 但必须让调用方知道正文变了，所以记 1；标注文案里仍用真实行数。
	omitted := len(lines) - kept
	if cutMidLine {
		omitted = 1
	}
	switch {
	case cutMidLine:
		fmt.Fprintf(&b, "... [sourcepack] diff truncated at %d KB (first line exceeds the limit); %d further lines omitted\n", maxBytes/1024, len(lines)-kept)
	case omitted > 0:
		fmt.Fprintf(&b, "... [sourcepack] %d of %d diff lines omitted (--max-size %d KB)\n", omitted, len(lines), maxBytes/1024)
	}
	return b.String(), omitted
}

// truncateUTF8 把字符串截到不超过 n 字节，并在必要时回退到完整的字符边界。
func truncateUTF8(s string, n int64) string {
	if n <= 0 {
		return ""
	}
	if int64(len(s)) <= n {
		return s
	}
	cut := s[:n]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}

// trimToLastCompleteChunk 把超出字节上限的 patch 截到最后一个完整文件块。
func trimToLastCompleteChunk(patch string) string {
	idx := strings.LastIndex(patch, "\ndiff --git ")
	if idx < 0 {
		if strings.HasPrefix(patch, "diff --git ") {
			return ""
		}
		return patch
	}
	return patch[:idx+1]
}

// syntheticNewFileDiff 为未跟踪文件造一个 git 风格的"新增文件"补丁。
// 只在 --diff-only 下才需要：默认模式会直接输出全文，没必要重复一遍。
func syntheticNewFileDiff(relPath string, content []byte, maxLines int, maxBytes int64) (string, int, int) {
	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	total := len(lines)
	if total == 1 && lines[0] == "" {
		total = 0
		lines = nil
	}
	body := make([]string, 0, total)
	for _, ln := range lines {
		body = append(body, "+"+ln)
	}
	shown := body
	omitted := 0
	var used int64
	// 先按字节：单行超长的新文件（例如 60MB 单行）用行数截不住。
	//
	// 注意这里与 capDiffBytes 有意不同：那边会对超限的单行做 truncateUTF8
	// 硬切（因为那行本来就在 git 的 diff 里，切一半也比丢掉强）；新文件是
	// 我们自己合成的，宁可不造这一行，也不要产出半行假装是文件内容。
	if maxBytes > 0 {
		for i, ln := range shown {
			used += int64(len(ln)) + 1
			if used > maxBytes {
				omitted = len(shown) - i
				shown = shown[:i]
				break
			}
		}
	}
	if maxLines > 0 && len(shown) > maxLines {
		omitted += len(shown) - maxLines
		shown = shown[:maxLines]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\n", relPath, relPath)
	b.WriteString("new file mode 100644\n")
	b.WriteString("--- /dev/null\n")
	fmt.Fprintf(&b, "+++ b/%s\n", relPath)
	fmt.Fprintf(&b, "@@ -0,0 +1,%d @@\n", total)
	for _, ln := range shown {
		b.WriteString(ln)
		b.WriteByte('\n')
	}
	if omitted > 0 {
		fmt.Fprintf(&b, "... [sourcepack] %d of %d diff lines omitted (new file)\n", omitted, total)
	}
	return b.String(), total, omitted
}
