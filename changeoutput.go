package main

// ============================================================
//  Git 变更模式的过滤与渲染
// ============================================================
//
//  这一层刻意复用全量快照的所有筛选/忽略/敏感文件判定，区别只有两点：
//   1. 文件清单来自 git（-g/-G），而不是 WalkDir；
//   2. 输出排版是"变更快照"（摘要表 + per-file diff + 变更文件全文）。
//
//  在 git 模式下**不再套用 .gitignore**：git 自己已经按 .gitignore 过滤
//  过未跟踪文件（--exclude-standard），而"已跟踪但恰好命中 .gitignore"
//  的文件才是真实变更，不能因为一条 ignore 规则就把它藏起来。
//  .gdignore（Sourcepack 自己的忽略文件）仍然生效。

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// buildChangeFiles 把 changeset 转成与 scanDirectory 相同形状的结果，
// 于是后续的 -s / -c / -p / --icloud / --dry-run 全都不用改。
func buildChangeFiles(config *Config, cs *ChangeSet) ([]FileMetadata, Stats, []SkippedFile) {
	stats := Stats{
		DirMap: make(map[string]*DirStats),
		ExtMap: make(map[string]*ExtStats),
	}
	var files []FileMetadata

	gd := loadGdignore(config.RootDir)
	matcher := &gitignoreMatcher{patterns: gd.patterns}

	// 输出文件自己不该出现在变更里（与全量模式一致）。
	// 走真实路径再算相对路径，避免 -o /tmp/... 与仓库真实路径
	// /private/tmp/... 在符号链接下对不上而漏排除。
	outRel := filepath.ToSlash(config.OutputFile)
	if filepath.IsAbs(config.OutputFile) {
		outAbs := realPathOrSelf(config.OutputFile)
		rootAbs := realPathOrSelf(config.RootDir)
		if r, err := filepath.Rel(rootAbs, outAbs); err == nil && !strings.HasPrefix(r, "..") {
			outRel = filepath.ToSlash(r)
		}
	}

	// 一次遍历建立 path → 该路径**最新一次出现**的 Change 索引。
	//
	// -G 下同一文件可能出现在多个提交里：正文只读一次，取最新那次，
	// 因为 readContent 读的是"当前工作区"，二进制/未跟踪这些判定也就
	// 必须用最新一次的元数据（cs.Commits 是 git log 顺序：新 → 旧，
	// 所以"第一次遇到"即最新）。渲染层同样按这个顺序写正文，两边一致。
	// （原来是每处理一个路径就把整个 changeset 线性扫一遍，O(路径 × 变更)。）
	lastByPath := make(map[string]*Change, 32)
	var order []string
	for _, grp := range cs.Commits {
		for _, ch := range grp.Changes {
			if _, ok := lastByPath[ch.Path]; !ok {
				order = append(order, ch.Path)
				lastByPath[ch.Path] = ch
			}
		}
	}

	filteredReason := map[string]string{}
	noContent := map[string]string{}
	var skipped []SkippedFile
	var filtered []SkippedFile
	for _, p := range order {
		ch := lastByPath[p]

		reason := ""
		if p == outRel {
			reason = "Sourcepack output file"
		} else {
			reason = changeFilterReason(p, *config, matcher)
		}
		if reason != "" {
			// 被规则排除的既要进"排除清单"，也要进 skipped（-v / --dry-run 会打印）
			filteredReason[p] = reason
			entry := SkippedFile{RelPath: p, Reason: reason}
			filtered = append(filtered, entry)
			skipped = append(skipped, entry)
			continue
		}

		full := filepath.Join(cs.RepoRoot, filepath.FromSlash(p))
		info, err := os.Stat(full)
		if err != nil {
			noContent[p] = "Not in working tree"
		} else if info.IsDir() {
			noContent[p] = "Submodule or directory"
		} else if info.Size() > config.MaxFileSize {
			noContent[p] = "Size limit"
		} else {
			// 正文是否输出**以对当前文件的探测为准**，不用 diff 里的二进制标记：
			// git 的标记是"base 与 target 任一侧为二进制"，所以"旧二进制 → 新文本"
			// 的文件会被误判成二进制而丢掉正文。探测标准已收紧到与 git 等价
			// （前 8000 字节含 NUL，或非法 UTF-8），不会漏。
			lineCount, isBinary, tokens, serr := scanFileStats(full, true)
			switch {
			case serr != nil:
				noContent[p] = "Read error"
			case isBinary:
				noContent[p] = "Binary file"
			default:
				fm := FileMetadata{RelPath: p, FullPath: full, Size: info.Size(), LineCount: lineCount}
				files = append(files, fm)
				accumulateStats(&stats, fm, tokens)

				if ch != nil && ch.Untracked && ch.Diff == "" {
					// 未跟踪文件在 git diff 里一行都没有：摘要里的 +N 与总量
					// 告警都只能靠数正文（--diff-only 下由合成补丁填）。
					ch.Added = lineCount
					ch.DiffFull = lineCount
				}
			}
		}

		if nc := noContent[p]; nc != "" {
			skipped = append(skipped, SkippedFile{RelPath: p, Reason: nc})
		}
	}

	// 未跟踪文件在 --diff-only 下没有任何 git diff 可用，
	// 这里补一个 git 风格的"新增文件"补丁，否则新文件会凭空消失。
	if config.DiffOnly {
		for _, p := range order {
			// noContent 非空 = 二进制 / 超限 / 不在工作区 / 子模块：
			// 与默认模式保持同一道 gate，否则 --diff-only 会把二进制裸字节
			// 和超限文件的全文（os.ReadFile 无上限）合成进 diff 输出。
			if filteredReason[p] != "" || noContent[p] != "" {
				continue
			}
			ch := lastByPath[p]
			if ch == nil || !ch.Untracked || ch.Diff != "" {
				continue
			}
			content, rerr := os.ReadFile(filepath.Join(cs.RepoRoot, filepath.FromSlash(p)))
			if rerr != nil || diffBodyLooksBinary(string(content)) {
				continue
			}
			diff, total, omitted := syntheticNewFileDiff(p, content, config.MaxDiffLines, config.MaxFileSize)
			ch.Diff = diff
			ch.DiffFull = total
			ch.DiffOmitted = omitted
			ch.Added = total
		}
		// 正文本来就不输出，没必要把"无正文"当成跳过报出来
		skipped = filtered
	}

	// 标记要打到该路径的每一处出现上（-G 下一个文件会出现多次）
	for _, grp := range cs.Commits {
		for _, ch := range grp.Changes {
			if r := filteredReason[ch.Path]; r != "" {
				ch.Filtered = r
			}
			if nc := noContent[ch.Path]; nc != "" {
				ch.NoContent = nc
				if nc == "Binary file" {
					// 未跟踪文件没有 git diff 可判断，Binary 要靠我们的探测补上，
					// 否则标记与渲染文案都会说成"文本"
					ch.Binary = true
				}
			}
		}
	}

	cs.Excluded = filtered
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].RelPath < filtered[j].RelPath })
	sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })

	// 过滤结果会影响统计，标记完成后重算一次
	summarizeChangeSet(cs)
	return files, stats, skipped
}

// realPathOrSelf 取真实路径（解析符号链接）；失败时原样返回。
// 输出文件可能还不存在，所以不能依赖 EvalSymlinks 成功。
func realPathOrSelf(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	if abs, err := filepath.Abs(p); err == nil {
		if dir, base := filepath.Split(abs); dir != "" {
			if rdir, err := filepath.EvalSymlinks(filepath.Clean(dir)); err == nil {
				return filepath.Join(rdir, base)
			}
		}
		return abs
	}
	return p
}

// changeFilterReason 判定一个变更路径是否被过滤，返回原因（"" = 保留）。
func changeFilterReason(relPath string, config Config, matcher *gitignoreMatcher) string {
	if !config.NoDefaultIgnore {
		parts := strings.Split(relPath, "/")
		for _, part := range parts[:len(parts)-1] {
			if ignoreDirs[part] {
				return "Default ignore dir: " + part
			}
		}
	}
	return shouldIgnoreFileReason(relPath, config, matcher)
}

// ============================================================
//  终端摘要
// ============================================================

func printChangeSetTerminal(cs *ChangeSet) {
	fmt.Printf("\n📦 Git changes: %s\n", cs.Spec)
	fmt.Printf("  %-20s %s\n", "Repository:", cs.RepoRoot)
	fmt.Printf("  %-20s %s\n", "Branch:", cs.Branch)
	if cs.Head != "" {
		fmt.Printf("  %-20s %s %s\n", "HEAD:", shortSHA(cs.Head), cs.HeadSubject)
	}
	fmt.Printf("  %-20s %d (+%d −%d)\n", "Files changed:", cs.UniquePaths, cs.TotalAdded, cs.TotalDeleted)
	if cs.Mode == "history" {
		fmt.Printf("  %-20s %d\n", "Commits:", len(cs.Commits))
	} else {
		fmt.Printf("  %-20s staged %d · unstaged %d · untracked %d\n", "", cs.StagedCount, cs.UnstagedCount, cs.UntrackedCount)
	}
	if n := len(cs.Excluded); n > 0 {
		fmt.Printf("  %-20s %d (listed in the output)\n", "Filtered out:", n)
	}
	for _, wn := range cs.Warnings {
		fmt.Printf("  ⚠ %s\n", wn)
	}
	if cs.TotalDiffLines > warnDiffLines {
		fmt.Printf("  ⚠ %d changed lines (diffs + new files) — consider --diff-only or --max-diff-lines\n", cs.TotalDiffLines)
	}
	fmt.Println()
}

// ============================================================
//  变更快照渲染
// ============================================================

func writeChangeContent(config Config, files []FileMetadata, stats Stats, cs *ChangeSet, w io.Writer) error {
	bw := bufio.NewWriter(w)

	byPath := make(map[string]FileMetadata, len(files))
	for _, f := range files {
		byPath[f.RelPath] = f
	}

	fmt.Fprintf(bw, "# Git Change Snapshot: %s\n\n", filepath.Base(cs.RepoRoot))
	fmt.Fprintf(bw, "> Generated by [Sourcepack](https://github.com/yuanguangshan/sourcepack) on %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(bw, "> Source: `%s`\n\n", cs.Spec)
	if config.DiffOnly {
		fmt.Fprintln(bw, "> Mode: **diff only** — full file contents are not included (`--diff-only`).")
		fmt.Fprintln(bw)
	}
	if cs.PatchClipped {
		fmt.Fprintln(bw, "> ⚠ Diff output exceeded 64 MB and was truncated at a file boundary.")
		fmt.Fprintln(bw)
	}
	for _, wn := range cs.Warnings {
		fmt.Fprintf(bw, "> ⚠ %s\n", wn)
		fmt.Fprintln(bw)
	}
	if cs.TotalDiffLines > warnDiffLines {
		fmt.Fprintf(bw, "> ⚠ %d changed lines total (diffs + new-file content) — this snapshot is large; consider `--diff-only` or a lower `--max-diff-lines`.\n\n", cs.TotalDiffLines)
	}

	// ---- Repository ----
	fmt.Fprintln(bw, "## Repository")
	fmt.Fprintf(bw, "- Root: `%s`\n", cs.RepoRoot)
	fmt.Fprintf(bw, "- Branch: `%s`\n", cs.Branch)
	if cs.Head != "" {
		fmt.Fprintf(bw, "- HEAD: `%s` %s\n", shortSHA(cs.Head), mdCell(cs.HeadSubject))
	} else {
		fmt.Fprintln(bw, "- HEAD: (none)")
	}
	fmt.Fprintf(bw, "- Mode: %s\n", cs.Mode)
	if cs.Pathspec != "" {
		fmt.Fprintf(bw, "- Scope: `%s` (restricted by -d/--dir)\n", cs.Pathspec)
	}
	if cs.Mode == "history" {
		fmt.Fprintf(bw, "- Commits: %d\n", len(cs.Commits))
	} else {
		tree := "dirty"
		if cs.UniquePaths == 0 {
			tree = "clean"
		}
		fmt.Fprintf(bw, "- Working tree: %s\n", tree)
	}
	fmt.Fprintf(bw, "- Files changed: %d\n", cs.UniquePaths)
	if len(cs.Excluded) > 0 {
		fmt.Fprintf(bw, "- Excluded by rules: %d\n", len(cs.Excluded))
	}
	fmt.Fprintf(bw, "- Lines: +%d −%d\n", cs.TotalAdded, cs.TotalDeleted)
	if cs.Mode != "history" {
		fmt.Fprintf(bw, "- Index/worktree: staged %d · unstaged %d · untracked %d\n", cs.StagedCount, cs.UnstagedCount, cs.UntrackedCount)
	}

	// ---- Change Summary ----
	rows := summarizeRows(cs)
	fmt.Fprintln(bw, "\n## Change Summary")
	if len(rows) == 0 {
		if len(cs.Excluded) > 0 {
			fmt.Fprintln(bw, "_All changes were excluded by the active filter/ignore rules — see below._")
		} else {
			fmt.Fprintln(bw, "_No changes._")
		}
	} else {
		fmt.Fprintln(bw, "| # | File | Status | Lines | Flags |")
		fmt.Fprintln(bw, "| ---: | :--- | :--- | ---: | :--- |")
		for i, r := range rows {
			fmt.Fprintf(bw, "| %d | %s | %s | +%d −%d | %s |\n",
				i+1, mdCell(mdCode(r.path)), r.status, r.added, r.deleted, mdCell(strings.Join(r.flags, ", ")))
		}
		if cs.Mode == "history" {
			fmt.Fprintf(bw, "\nTotals: %d files, +%d −%d (line counts summed across the %d packaged commits).\n",
				len(rows), cs.TotalAdded, cs.TotalDeleted, len(cs.Commits))
		} else {
			fmt.Fprintf(bw, "\nTotals: %d files, +%d −%d.\n", len(rows), cs.TotalAdded, cs.TotalDeleted)
		}
	}

	// ---- Commits ----
	if cs.Mode == "history" && len(cs.Commits) > 0 {
		fmt.Fprintln(bw, "\n## Commits")
		fmt.Fprintln(bw, "| Commit | Date | Author | Subject |")
		fmt.Fprintln(bw, "| :--- | :--- | :--- | :--- |")
		for _, grp := range cs.Commits {
			subj := grp.Subject
			if grp.IsMerge {
				subj += " _(merge)_"
			}
			fmt.Fprintf(bw, "| `%s` | %s | %s | %s |\n", grp.Short, grp.Date, mdCell(grp.Author), mdCell(subj))
		}
	}

	// ---- 清单类小节 ----
	if len(cs.Excluded) > 0 {
		fmt.Fprintln(bw, "\n## Excluded by rules")
		fmt.Fprintln(bw, "These paths appear in the change set but were filtered out. Adjust the filters (or use `-a`) if you need them.")
		fmt.Fprintln(bw)
		fmt.Fprintln(bw, "| File | Reason |")
		fmt.Fprintln(bw, "| :--- | :--- |")
		for _, s := range cs.Excluded {
			fmt.Fprintf(bw, "| %s | %s |\n", mdCell(mdCode(s.RelPath)), mdCell(s.Reason))
		}
	}
	if list := collectPaths(cs, func(c *Change) bool { return c.Filtered == "" && c.Status != "" && c.Status[0] == 'D' }); len(list) > 0 {
		fmt.Fprintln(bw, "\n## Deleted Files")
		writePathList(bw, list)
	}
	if list := collectPaths(cs, func(c *Change) bool { return c.Filtered == "" && c.OldPath != "" }); len(list) > 0 {
		fmt.Fprintln(bw, "\n## Renamed / Copied Files")
		writeRenameList(bw, cs)
	}
	if list := collectPaths(cs, func(c *Change) bool { return c.Filtered == "" && c.Binary }); len(list) > 0 {
		fmt.Fprintln(bw, "\n## Binary Files (no textual diff)")
		writePathList(bw, list)
	}

	// ---- 逐文件明细 ----
	fmt.Fprintln(bw, "\n## Changes")
	contentWritten := map[string]bool{}
	number := 0
	for _, grp := range cs.Commits {
		visible := make([]*Change, 0, len(grp.Changes))
		for _, ch := range grp.Changes {
			if ch.Filtered == "" {
				visible = append(visible, ch)
			}
		}
		if len(visible) == 0 {
			continue
		}
		level := 3
		if cs.Mode == "history" {
			level = 4
			subject := grp.Subject
			if grp.IsMerge {
				subject += " (merge — diff vs first parent)"
			}
			fmt.Fprintf(bw, "\n### `%s` %s\n\n", grp.Short, mdCell(subject))
			meta := []string{grp.Date}
			if grp.Author != "" {
				meta = append(meta, grp.Author)
			}
			fmt.Fprintf(bw, "_%s_\n", strings.Join(meta, " · "))
		}
		for _, ch := range visible {
			number++
			heading := strings.Repeat("#", level)
			title := mdCode(ch.Path)
			if ch.OldPath != "" && ch.Status != "" && (ch.Status[0] == 'R' || ch.Status[0] == 'C') {
				title = mdCode(ch.OldPath) + " → " + mdCode(ch.Path)
			}
			fmt.Fprintf(bw, "\n%s %d. %s — %s (+%d −%d)\n", heading, number, title, ch.StatusLabel(), ch.Added, ch.Deleted)
			if fl := ch.Flags(); len(fl) > 0 {
				fmt.Fprintf(bw, "\n_%s_\n", strings.Join(fl, ", "))
			}

			// diff 块（可能没有）与"正文状况"分成两段，后者每次出现只说一遍：
			// 之前二进制文件会被 switch 和 !ok 兜底各说一次（自相矛盾的重复文案）。
			if ch.Diff != "" {
				fmt.Fprintln(bw)
				writeFenced(bw, "diff", strings.TrimRight(ch.Diff, "\n"))
			}
			switch {
			case ch.Binary || ch.NoContent == "Binary file":
				fmt.Fprintln(bw, "\n_Binary file — content not emitted._")
			case ch.NoContent != "":
				// 超限 / 不在工作区 / 子模块 / 读失败：如实说明，别假装"没有变更"
				fmt.Fprintf(bw, "\n_Content omitted: %s._\n", ch.NoContent)
			case ch.Diff == "" && ch.Untracked && config.DiffOnly:
				fmt.Fprintln(bw, "\n_New untracked file — no diff available in --diff-only mode._")
			case ch.Diff == "" && ch.Untracked:
				fmt.Fprintln(bw, "\n_New untracked file — full content included below._")
			case ch.Diff == "" && ch.Status != "" && ch.Status[0] == 'D':
				fmt.Fprintln(bw, "\n_File deleted._")
			case ch.Diff == "":
				fmt.Fprintln(bw, "\n_No textual diff (mode- or rename-only change)._")
			}

			if contentWritten[ch.Path] {
				continue
			}
			contentWritten[ch.Path] = true
			if config.DiffOnly {
				continue
			}
			fm, ok := byPath[ch.Path]
			if !ok {
				// 没有正文的原因上面已经说过一次，这里不再重复
				continue
			}
			label := "Full content"
			if cs.Mode == "history" {
				label = "Full content (current working tree)"
			}
			fmt.Fprintf(bw, "\n%s %s\n\n", strings.Repeat("#", level+1), label)
			if err := writeFileBody(bw, fm); err != nil {
				return err
			}
		}
	}

	return bw.Flush()
}

// writeFenced 写一个代码块，围栏长度按正文里最长反引号串动态加长，
// 否则 diff 正文里出现 ``` 会把 Markdown 结构截断。
func writeFenced(bw *bufio.Writer, lang, body string) {
	maxRun := scanBackticksInData([]byte(body))
	fence := "```"
	if maxRun >= 3 {
		fence = strings.Repeat("`", maxRun+1)
	}
	fmt.Fprintf(bw, "%s%s\n", fence, lang)
	bw.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		bw.WriteByte('\n')
	}
	fmt.Fprintf(bw, "%s\n", fence)
}

type summaryRow struct {
	path    string
	status  string
	added   int
	deleted int
	commits int
	flags   []string
}

// summarizeRows 按路径聚合变更（-G 下一个文件可能出现在多个提交里）。
func summarizeRows(cs *ChangeSet) []summaryRow {
	type rowState struct {
		row   summaryRow
		flags map[string]bool
	}
	byPath := map[string]*rowState{}
	var order []string
	for _, grp := range cs.Commits {
		for _, ch := range grp.Changes {
			if ch.Filtered != "" {
				continue
			}
			st, ok := byPath[ch.Path]
			if !ok {
				st = &rowState{row: summaryRow{path: ch.Path}, flags: map[string]bool{}}
				byPath[ch.Path] = st
				order = append(order, ch.Path)
			}
			// 状态取"最新一次出现"（-G 下 git log 是新→旧，首次遇到即最新），
			// 否则一个先新增后删除的文件会被标成 added。
			if st.row.status == "" {
				st.row.status = ch.StatusLabel()
			}
			st.row.added += ch.Added
			st.row.deleted += ch.Deleted
			st.row.commits++
			addFlag := func(f string) {
				if f != "" && !st.flags[f] {
					st.flags[f] = true
					st.row.flags = append(st.row.flags, f)
				}
			}
			for _, f := range ch.Flags() {
				addFlag(f)
			}
			if ch.DiffOmitted > 0 {
				addFlag("diff truncated")
			}
			if ch.NoContent == "Size limit" {
				addFlag("content skipped (size)")
			}
		}
	}
	sort.Strings(order)
	out := make([]summaryRow, 0, len(order))
	for _, p := range order {
		st := byPath[p]
		// "出现在几个提交里"必须等聚合结束再算，否则会写成 in 2 commits, in 3 commits
		if st.row.commits > 1 {
			st.row.flags = append(st.row.flags, fmt.Sprintf("in %d commits", st.row.commits))
		}
		out = append(out, st.row)
	}
	return out
}

func collectPaths(cs *ChangeSet, pred func(*Change) bool) []string {
	seen := map[string]bool{}
	var out []string
	for _, grp := range cs.Commits {
		for _, ch := range grp.Changes {
			if !pred(ch) || seen[ch.Path] {
				continue
			}
			seen[ch.Path] = true
			out = append(out, ch.Path)
		}
	}
	sort.Strings(out)
	return out
}

func writePathList(bw *bufio.Writer, paths []string) {
	for _, p := range paths {
		fmt.Fprintf(bw, "- %s\n", mdCode(p))
	}
}

func writeRenameList(bw *bufio.Writer, cs *ChangeSet) {
	type pair struct{ from, to, status string }
	seen := map[string]bool{}
	var pairs []pair
	for _, grp := range cs.Commits {
		for _, ch := range grp.Changes {
			if ch.Filtered != "" || ch.OldPath == "" {
				continue
			}
			key := ch.OldPath + "\x00" + ch.Path
			if seen[key] {
				continue
			}
			seen[key] = true
			pairs = append(pairs, pair{ch.OldPath, ch.Path, ch.StatusLabel()})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].to < pairs[j].to })
	for _, p := range pairs {
		fmt.Fprintf(bw, "- %s → %s (%s)\n", mdCode(p.from), mdCode(p.to), p.status)
	}
}

// mdCode 用反引号包住路径；控制字符先转成可见转义。
//
// git 允许路径含换行（unquoteGitPath 会把 \n 还原回来），原样输出会把
// Markdown 结构（尤其是 inline code 与表格）截断。
func mdCode(s string) string {
	if s == "" {
		return ""
	}
	s = escapeControlForInline(s)
	if strings.ContainsRune(s, '`') {
		// 反引号在 Markdown 里无法内联转义，退化为纯文本；
		// 此时控制字符已经转义，不会破坏结构
		return s
	}
	return "`" + s + "`"
}

// escapeControlForInline 把换行/制表/控制字符转成可见的转义写法。
func escapeControlForInline(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, "\\x%02x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// mdCell 转义 Markdown 表格单元里的竖线与换行。
func mdCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.ReplaceAll(s, "\n", " ")
}
