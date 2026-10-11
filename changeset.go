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
	if waitErr != nil && !clipped {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = waitErr.Error()
		}
		return string(data), false, fmt.Errorf("git %s failed: %s", strings.Join(args, " "), msg)
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
	for _, ch := range changes {
		applyChunk(ch, byPath, config.MaxDiffLines)
		ch.Staged = stagedSet[ch.Path]
		ch.Unstaged = unstagedSet[ch.Path]
	}
	// 兜底：清单里没有但 patch 里出现的文件（理论上不该发生）
	for _, ck := range chunks {
		if _, ok := lookupChange(changes, ck.Path); !ok && ck.Path != "" {
			ch := &Change{Path: ck.Path, Status: "M"}
			applyChunk(ch, byPath, config.MaxDiffLines)
			changes = append(changes, ch)
		}
	}
	for _, p := range untracked {
		changes = append(changes, &Change{Path: p, Status: "??", Untracked: true})
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
		}
		chunks := splitPatch(patch)
		byPath := indexChunks(chunks)
		for _, ch := range changes {
			applyChunk(ch, byPath, config.MaxDiffLines)
		}
		for _, ck := range chunks {
			if _, ok := lookupChange(changes, ck.Path); !ok && ck.Path != "" {
				ch := &Change{Path: ck.Path, Status: "M"}
				applyChunk(ch, byPath, config.MaxDiffLines)
				changes = append(changes, ch)
			}
		}
		sort.SliceStable(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
		grp.Changes = changes
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
func markPaths(nsOut string, set map[string]bool, withOld bool) {
	changes, err := parseNameStatusZ(nsOut)
	if err != nil {
		return
	}
	for _, ch := range changes {
		set[ch.Path] = true
		if withOld && ch.OldPath != "" {
			set[ch.OldPath] = true
		}
	}
}

func lookupChange(changes []*Change, path string) (*Change, bool) {
	if path == "" {
		return nil, false
	}
	for _, ch := range changes {
		if ch.Path == path {
			return ch, true
		}
	}
	return nil, false
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
			if i+2 >= len(fields) {
				break
			}
			out = append(out, &Change{Status: code, OldPath: fields[i+1], Path: fields[i+2]})
			i += 3
			continue
		}
		if i+1 >= len(fields) {
			break
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
			// 保留给删除类 diff 兜底，正常由 `+++`/name-status 决定
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

// applyChunk 把 patch 分块的结果贴到一个 Change 上，并按需截断。
func applyChunk(ch *Change, byPath map[string]*patchChunk, maxLines int) {
	ck := byPath[ch.Path]
	if ck == nil && ch.OldPath != "" {
		ck = byPath[ch.OldPath]
	}
	if ck == nil {
		return
	}
	ch.Added, ch.Deleted, ch.Binary = ck.Added, ck.Deleted, ck.Binary
	ch.Diff = ck.Body
	ch.DiffFull = strings.Count(ck.Body, "\n")
	if maxLines > 0 {
		if trimmed, omitted := capDiffLines(ck.Body, maxLines); omitted > 0 {
			ch.Diff = trimmed
			ch.DiffOmitted = omitted
		}
	}
}

// capDiffLines 保留前 maxLines 行，返回截断后的正文与被省略的行数。
func capDiffLines(body string, maxLines int) (string, int) {
	if maxLines <= 0 {
		return body, 0
	}
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	if len(lines) <= maxLines {
		return body, 0
	}
	omitted := len(lines) - maxLines
	kept := lines[:maxLines]
	var b strings.Builder
	for _, ln := range kept {
		b.WriteString(ln)
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "... [sourcepack] %d of %d diff lines omitted (--max-diff-lines %d)\n", omitted, len(lines), maxLines)
	return b.String(), omitted
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
func syntheticNewFileDiff(relPath string, content []byte, maxLines int) (string, int, int) {
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
	if maxLines > 0 && len(shown) > maxLines {
		omitted = len(shown) - maxLines
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
		fmt.Fprintf(&b, "... [sourcepack] %d of %d diff lines omitted (--max-diff-lines %d)\n", omitted, total, maxLines)
	}
	return b.String(), total, omitted
}
