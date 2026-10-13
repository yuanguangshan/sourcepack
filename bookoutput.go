package main

// ============================================================
//  书排版（--book）
// ============================================================
//
// 默认产物（project_snapshot.md）是给模型读的：结构树 + 文件清单 + 围栏代码，
// 追求"密度高、机器好切"。--book 换一套版式，追求"人能在 Markdown 阅读器里
// 从头读到尾"：
//
//	封面      书名 + 一句话说明 + 「关于本书」的表（规模/体积/主要语言）
//	目录      部/章两级，配显式锚点，点击可跳
//	正文      按顶层目录分「部」；每个文件一章，章前标语言/行数/体积/修改时间
//	          **Markdown 文件按正文排版**（标题下沉、不再是代码块），
//	          其余文件仍是完整的围栏代码
//	附录      A 目录结构  B 统计  C 打包参数（这份书是怎么来的）
//
// 层级：书名/部/附录 = H1，章 = H2，章内小节 = H3 起。这与本仓库 book/ 那本书
// 的产物（book/dist/BOOK.md）一致：部与章两级、"第 N 部/第 N 章"的读法、
// 附录排在最后、顶部用 HTML 注释记来源。两份产物在同一个阅读器里观感统一。
//
// 章标题的来源，按"内容优先"取：
//   - Markdown 文件用它自己的第一个 H1（如 `# 第 3 章　对象模型`）——书稿
//     自己编排的章号/标题最权威，工具不该另起一套编号（改了会跟正文里的
//     `1.1`、`3.7` 这类小节号对不上）；
//   - 其余文件（代码）没有标题，就用文件路径。
//
// 与默认快照的关系：**同一批 files/stats**，只是排版不同；因此 -s（统计）、
// -c（剪贴板）、-p（推送）、--icloud 这些输出目的地全部照旧可用。

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 层级常量。正文里的 Markdown 标题会整体下沉，让"最小标题级别"落在
// bookChapterLevel+1，这样阅读器的 outline 里"章 → 节"的从属关系成立。
const (
	bookPartLevel     = 1
	bookChapterLevel  = 2
	bookAppendixLevel = 1
)

// 附录锚点固定写死（ASCII、稳定、不依赖标题）。章锚点是 ch-01、ch-02…，
// 部锚点是 part-1、part-2…：编号型锚点在任何阅读器里都能跳，也不会因为
// 文件名里带 CJK / 标点而生成出没法用的 id。
const (
	bookAppendixStructure = "appendix-a"
	bookAppendixStats     = "appendix-b"
	bookAppendixColophon  = "appendix-c"
)

type bookChapter struct {
	no      int // 全书连续章序（只用于锚点，不印在正文里）
	relPath string
	title   string // 内容里的 H1；没有则留空，用文件路径兜底
	anchor  string
	lines   int
	size    int64
	modTime time.Time
}

// displayTitle 章标题：Markdown 自己的 H1 优先，否则用文件路径。
func (c bookChapter) displayTitle() string {
	if c.title != "" {
		return c.title
	}
	return mdCode(c.relPath)
}

type bookPart struct {
	anchor   string
	title    string
	lead     string
	chapters []bookChapter
}

func writeBookContent(config Config, files []FileMetadata, stats Stats, w io.Writer) error {
	bw := bufio.NewWriter(w)

	rootName := filepath.Base(config.RootDir)
	bookTitle := config.BookTitle
	if bookTitle == "" {
		bookTitle = rootName
	}
	now := time.Now()
	parts := buildBookParts(files)

	byPath := make(map[string]FileMetadata, len(files))
	for _, f := range files {
		byPath[f.RelPath] = f
	}

	// 顶部注释：和 BOOK.md 一样，把"谁生成、什么时候、正文来自哪"记在不会
	// 渲染出来的那一行里——读者看不到，但 diff、溯源和 grep 时很有用。
	fmt.Fprintf(bw, "<!-- %s · %d 个文件 · 由 sourcepack %s 生成于 %s -->\n\n",
		bookTitle, len(files), versionStr, now.Format("2006-01-02 15:04"))

	fmt.Fprintln(bw, `<a id="book-title"></a>`)
	fmt.Fprintln(bw)
	fmt.Fprintf(bw, "# %s\n\n", bookTitle)
	fmt.Fprintln(bw, "## 可以直接读的完整快照")
	fmt.Fprintln(bw)
	if len(files) == 0 {
		fmt.Fprintln(bw, "> ⚠ 这次打包没有纳入任何文件——检查 `-i/-x/-m/-X` 过滤条件和忽略规则。")
		fmt.Fprintln(bw)
	} else {
		fmt.Fprintf(bw, "> **这份产物是给人读的**：%d 个文件按顶层目录分成 %d 部、共 %d 章，每章前标出语言、行数、体积与修改时间。\n",
			len(files), len(parts), len(files))
		fmt.Fprintln(bw, "> Markdown 文件按正文排版（标题自动下沉一级，不再是代码块），其余文件以围栏代码完整保留。")
		fmt.Fprintln(bw, "> 需要喂给模型的紧凑版本，用不带 `--book` 的 `sourcepack`（默认产物 `project_snapshot.md`）。")
		fmt.Fprintln(bw)
	}

	writeBookAbout(bw, bookTitle, rootName, stats, len(parts), len(files), now)
	writeBookTOC(bw, parts, len(files))

	for i, part := range parts {
		fmt.Fprintln(bw, "---")
		fmt.Fprintln(bw)
		fmt.Fprintf(bw, `<a id="%s"></a>`+"\n\n", part.anchor)
		fmt.Fprintf(bw, "%s 第 %d 部　%s\n\n", strings.Repeat("#", bookPartLevel), i+1, part.title)
		if part.lead != "" {
			fmt.Fprintf(bw, "> %s\n\n", part.lead)
		}
		for ci, ch := range part.chapters {
			// 章与章之间给一道分隔线（部与部之间已经有了），书页感更清楚。
			if ci > 0 {
				fmt.Fprintln(bw, "---")
				fmt.Fprintln(bw)
			}
			fmt.Fprintf(bw, `<a id="%s"></a>`+"\n\n", ch.anchor)
			fmt.Fprintf(bw, "%s %s\n\n", strings.Repeat("#", bookChapterLevel), ch.displayTitle())

			fm, ok := byPath[ch.relPath]
			if !ok {
				continue
			}
			inlined := isMarkdownPath(ch.relPath)
			fmt.Fprintf(bw, "%s\n\n", bookChapterMeta(ch, inlined))

			if inlined {
				// 正文里原本的 H1 已经提出来当章标题了，这里只写剩下的部分。
				_, body, err := markdownChapterBody(fm.FullPath)
				if err != nil {
					return fmt.Errorf("reading %s: %w", ch.relPath, err)
				}
				if body != "" {
					fmt.Fprint(bw, demoteHeadings(body, bookChapterLevel+1))
					if !strings.HasSuffix(body, "\n") {
						fmt.Fprintln(bw)
					}
					fmt.Fprintln(bw)
				}
				continue
			}
			if err := writeFileBody(bw, fm); err != nil {
				return err
			}
			fmt.Fprintln(bw)
		}
	}

	writeBookStructureAppendix(bw, files, rootName)
	writeBookStatsAppendix(bw, stats)
	writeBookColophonAppendix(bw, config, stats, rootName, now)

	return bw.Flush()
}

// writeBookAbout 写封面上的「关于本书」表。
func writeBookAbout(bw *bufio.Writer, bookTitle, rootName string, stats Stats, partCount, fileCount int, now time.Time) {
	fmt.Fprintln(bw, "---")
	fmt.Fprintln(bw)
	fmt.Fprintln(bw, `<a id="book-about"></a>`)
	fmt.Fprintln(bw)
	fmt.Fprintln(bw, "## 关于本书")
	fmt.Fprintln(bw)
	fmt.Fprintln(bw, "| 项目 | 内容 |")
	fmt.Fprintln(bw, "| :--- | :--- |")
	fmt.Fprintf(bw, "| **书名** | %s |\n", mdCell(bookTitle))
	fmt.Fprintf(bw, "| **成书日期** | %s |\n", now.Format("2006-01-02 15:04"))
	fmt.Fprintf(bw, "| **根目录** | %s |\n", mdCell(mdCode(rootName)))
	fmt.Fprintf(bw, "| **正文规模** | %d 部 · %d 章 · %s 行 |\n", partCount, fileCount, humanInt(stats.TotalLines))
	fmt.Fprintf(bw, "| **体积** | %s（估算 ~%s token） |\n", humanSize(stats.TotalSize), humanInt(stats.TotalTokens))
	if langs := bookLanguageSummary(stats, 3); langs != "" {
		fmt.Fprintf(bw, "| **主要语言** | %s |\n", langs)
	}
	fmt.Fprintf(bw, "| **成书工具** | [Sourcepack](https://github.com/yuanguangshan/sourcepack) %s |\n", mdCell(versionStr))
	fmt.Fprintln(bw)
}

// writeBookTOC 写目录：部不缩进、章缩进一级（纸质书目录的样子），附录与部同级。
func writeBookTOC(bw *bufio.Writer, parts []bookPart, fileCount int) {
	fmt.Fprintln(bw, "---")
	fmt.Fprintln(bw)
	fmt.Fprintln(bw, `<a id="book-toc"></a>`)
	fmt.Fprintln(bw)
	fmt.Fprintln(bw, "## 目录")
	fmt.Fprintln(bw)
	if fileCount == 0 {
		fmt.Fprintln(bw, "_（没有文件被纳入，正文为空。）_")
	}
	for _, part := range parts {
		fmt.Fprintf(bw, "- [%s](#%s)\n", tocText(part.title), part.anchor)
		for _, ch := range part.chapters {
			fmt.Fprintf(bw, "  - [%s](#%s)\n", tocText(ch.displayTitle()), ch.anchor)
		}
	}
	fmt.Fprintf(bw, "- [附录 A　目录结构](#%s)\n", bookAppendixStructure)
	fmt.Fprintf(bw, "- [附录 B　统计](#%s)\n", bookAppendixStats)
	fmt.Fprintf(bw, "- [附录 C　打包参数](#%s)\n", bookAppendixColophon)
	fmt.Fprintln(bw)
}

// writeBookStructureAppendix 把结构树放在书末（正文之前已经有目录了，
// 树放在前面会把"目录"和"清单"两件事挤在一起）。
func writeBookStructureAppendix(bw *bufio.Writer, files []FileMetadata, rootName string) {
	fmt.Fprintln(bw, "---")
	fmt.Fprintln(bw)
	fmt.Fprintf(bw, `<a id="%s"></a>`+"\n\n", bookAppendixStructure)
	fmt.Fprintf(bw, "%s 附录 A　目录结构\n\n", strings.Repeat("#", bookAppendixLevel))
	fmt.Fprintln(bw, "括号内是行数。同一棵树也出现在默认快照里，这里放在书末，方便回查某个文件在哪一层。")
	fmt.Fprintln(bw)
	fmt.Fprintln(bw, "```")
	fmt.Fprint(bw, buildTreeString(files, rootName))
	fmt.Fprintln(bw, "```")
	fmt.Fprintln(bw)
}

// writeBookStatsAppendix 写统计附录。总览 + 按目录 + 按语言三段，
// 与全量快照 -s 的报告共用同一套排序口径（sortedDirStats / sortedExtStats）。
func writeBookStatsAppendix(bw *bufio.Writer, stats Stats) {
	fmt.Fprintln(bw, "---")
	fmt.Fprintln(bw)
	fmt.Fprintf(bw, `<a id="%s"></a>`+"\n\n", bookAppendixStats)
	fmt.Fprintf(bw, "%s 附录 B　统计\n\n", strings.Repeat("#", bookAppendixLevel))
	fmt.Fprintln(bw, "### 总览")
	fmt.Fprintln(bw)
	fmt.Fprintln(bw, "| 指标 | 数值 |")
	fmt.Fprintln(bw, "| :--- | ---: |")
	fmt.Fprintf(bw, "| 文件 | %s |\n", humanInt(stats.FileCount))
	fmt.Fprintf(bw, "| 行数 | %s |\n", humanInt(stats.TotalLines))
	fmt.Fprintf(bw, "| 体积 | %s |\n", humanSize(stats.TotalSize))
	fmt.Fprintf(bw, "| 估算 Token | ~%s |\n", humanInt(stats.TotalTokens))
	if stats.DirCount > 0 {
		fmt.Fprintf(bw, "| 目录 | %s |\n", humanInt(stats.DirCount))
	}
	if stats.Skipped > 0 {
		fmt.Fprintf(bw, "| 未纳入的文件 | %s |\n", humanInt(stats.Skipped))
	}
	if stats.IgnoredDirs > 0 {
		fmt.Fprintf(bw, "| 整目录跳过 | %s |\n", humanInt(stats.IgnoredDirs))
	}
	fmt.Fprintln(bw)

	if dirs := sortedDirStats(stats); len(dirs) > 0 {
		fmt.Fprintln(bw, "### 按目录")
		fmt.Fprintln(bw)
		fmt.Fprintln(bw, "| 目录 | 文件 | 行数 | 行数占比 | 体积 | 体积占比 |")
		fmt.Fprintln(bw, "| :--- | ---: | ---: | ---: | ---: | ---: |")
		for _, d := range dirs {
			fmt.Fprintf(bw, "| %s | %s | %s | %.1f%% | %s | %.1f%% |\n",
				mdCell(mdCode(d.Path)), humanInt(d.FileCount), humanInt(d.TotalLines),
				ratio(int64(d.TotalLines), int64(stats.TotalLines)), humanSize(d.TotalSize),
				ratio(d.TotalSize, stats.TotalSize))
		}
		fmt.Fprintln(bw)
	}

	if exts := sortedExtStats(stats); len(exts) > 0 {
		fmt.Fprintln(bw, "### 按语言")
		fmt.Fprintln(bw)
		fmt.Fprintln(bw, "| 语言 | 扩展名 | 文件 | 行数 | 行数占比 |")
		fmt.Fprintln(bw, "| :--- | :--- | ---: | ---: | ---: |")
		for _, e := range exts {
			fmt.Fprintf(bw, "| %s | %s | %s | %s | %.1f%% |\n",
				languageLabelForExt(e.Ext), mdCell(mdCode(e.Ext)), humanInt(e.FileCount), humanInt(e.TotalLines),
				ratio(int64(e.TotalLines), int64(stats.TotalLines)))
		}
		fmt.Fprintln(bw)
	}
}

// writeBookColophonAppendix 写"这本书是怎么生成的"：过滤条件、忽略规则、
// 被挡掉多少东西。默认快照不写这些（模型不需要），但人读的时候需要知道
// "它到底看了哪些文件、漏了哪些"。
func writeBookColophonAppendix(bw *bufio.Writer, config Config, stats Stats, rootName string, now time.Time) {
	fmt.Fprintln(bw, "---")
	fmt.Fprintln(bw)
	fmt.Fprintf(bw, `<a id="%s"></a>`+"\n\n", bookAppendixColophon)
	fmt.Fprintf(bw, "%s 附录 C　打包参数\n\n", strings.Repeat("#", bookAppendixLevel))
	fmt.Fprintln(bw, "| 项目 | 内容 |")
	fmt.Fprintln(bw, "| :--- | :--- |")
	fmt.Fprintf(bw, "| **生成器** | [Sourcepack](https://github.com/yuanguangshan/sourcepack) %s（`--book` 书排版） |\n", mdCell(versionStr))
	fmt.Fprintf(bw, "| **生成时间** | %s |\n", now.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(bw, "| **根目录** | %s |\n", mdCell(mdCode(rootName)))
	if config.NoSubdirs {
		fmt.Fprintln(bw, "| **扫描范围** | 只看根目录（`-n`） |")
	} else {
		fmt.Fprintln(bw, "| **扫描范围** | 递归扫描子目录 |")
	}
	fmt.Fprintf(bw, "| **单文件上限** | %d KB（`--max-size`） |\n", config.MaxFileSize/1024)
	if cell := listCell(config.IncludeExts, 8); cell != "" {
		fmt.Fprintf(bw, "| **只收扩展名** | %s（`-i`） |\n", cell)
	}
	if cell := listCell(config.ExcludeExts, 8); cell != "" {
		fmt.Fprintf(bw, "| **排除扩展名** | %s（`-x`） |\n", cell)
	}
	if cell := listCell(config.IncludeMatches, 8); cell != "" {
		fmt.Fprintf(bw, "| **路径必须包含** | %s（`-m`） |\n", cell)
	}
	if cell := listCell(config.ExcludeMatches, 8); cell != "" {
		fmt.Fprintf(bw, "| **路径排除** | %s（`-X`） |\n", cell)
	}
	if cell := listCell(config.AdditionalIgnores, 8); cell != "" {
		fmt.Fprintf(bw, "| **额外忽略** | %s（`-I`） |\n", cell)
	}
	if config.NoGitignore {
		fmt.Fprintln(bw, "| **`.gitignore`** | 未读取（`--no-gitignore` / `-a`） |")
	} else {
		fmt.Fprintln(bw, "| **`.gitignore`** | 生效 |")
	}
	if config.NoDefaultIgnore {
		fmt.Fprintln(bw, "| **内置忽略规则** | 已关闭（`--no-default-ignore` / `-a`） |")
	} else {
		fmt.Fprintln(bw, "| **内置忽略规则** | 生效（`node_modules`、`.git` 等） |")
	}
	omitted := stats.Skipped + stats.ExplicitlyExcluded
	if omitted > 0 {
		fmt.Fprintf(bw, "| **未纳入的文件** | %s 个（体积超限 / 二进制 / 敏感文件 / 被规则排除） |\n", humanInt(omitted))
	}
	if stats.IgnoredDirs > 0 {
		names := stats.IgnoredDirNames
		shown := names
		suffix := ""
		if len(names) > 8 {
			shown, suffix = names[:8], fmt.Sprintf(" 等 %d 个", len(names))
		}
		fmt.Fprintf(bw, "| **整目录跳过** | %s 个：%s%s |\n", humanInt(stats.IgnoredDirs), mdCell(strings.Join(shown, "、")), suffix)
	}
	fmt.Fprintln(bw)
	fmt.Fprintln(bw, "> 为免产物里带出本机绝对路径，这里只记目录名，不记完整路径。")
	fmt.Fprintln(bw)
	fmt.Fprintln(bw, "复现这份书的命令：")
	fmt.Fprintln(bw)
	fmt.Fprintln(bw, "```bash")
	fmt.Fprintln(bw, bookCommandLine(config))
	fmt.Fprintln(bw, "```")
}

// bookCommandLine 尽量忠实地还原"这份书是怎么跑出来的"。
//
// 不还原 -d / -o：它们通常是本机绝对路径，写进产物就泄漏了目录结构，
// 改成 `cd <目录名>` 交代来源；-o 是用户自己的选择，与书的内容无关。
// 其余会影响"书里有什么"的参数全部还原，否则这份附录就是在骗人。
func bookCommandLine(config Config) string {
	args := []string{"sourcepack", "--book"}
	if config.BookTitle != "" {
		args = append(args, "--book-title", shellQuote(config.BookTitle))
	}
	if len(config.IncludeExts) > 0 {
		args = append(args, "-i", shellQuoteIfNeeded(bareExtList(config.IncludeExts)))
	}
	if len(config.ExcludeExts) > 0 {
		args = append(args, "-x", shellQuoteIfNeeded(bareExtList(config.ExcludeExts)))
	}
	if len(config.IncludeMatches) > 0 {
		args = append(args, "-m", shellQuote(strings.Join(config.IncludeMatches, ",")))
	}
	if len(config.ExcludeMatches) > 0 {
		args = append(args, "-X", shellQuote(strings.Join(config.ExcludeMatches, ",")))
	}
	if len(config.AdditionalIgnores) > 0 {
		args = append(args, "-I", shellQuote(strings.Join(config.AdditionalIgnores, ",")))
	}
	if config.NoSubdirs {
		args = append(args, "-n")
	}
	if config.NoGitignore {
		args = append(args, "--no-gitignore")
	}
	if config.NoDefaultIgnore {
		args = append(args, "--no-default-ignore")
	}
	if config.MaxFileSize != 500*1024 {
		args = append(args, "--max-size", strconv.FormatInt(config.MaxFileSize/1024, 10))
	}
	return "cd " + filepath.Base(config.RootDir) + " && " + strings.Join(args, " ")
}

// bareExtList 把内部存成 ".go,.md" 的扩展名列表还原成命令行里更常见的 "go,md"。
func bareExtList(exts []string) string {
	out := make([]string, 0, len(exts))
	for _, e := range exts {
		out = append(out, strings.TrimPrefix(e, "."))
	}
	return strings.Join(out, ",")
}

// shellQuote 给要原样传给 shell 的值加单引号（glob 如 `**/*.log` 不加引号
// 会被 shell 先展开，命令就复现不出来了）。
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// shellQuoteIfNeeded 只在值里含 shell 特殊字符时才加引号——像 `-i md` 这种
// 常见写法保持原样更好读，而 `-i 'src/*.go'` 这种必须加引号才不会被展开。
func shellQuoteIfNeeded(s string) string {
	if s == "" {
		return s
	}
	for _, r := range s {
		if !(r == ',' || r == '.' || r == '_' || r == '+' || r == '-' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return shellQuote(s)
		}
	}
	return s
}

// buildBookParts 把文件按顶层目录分「部」，并给每一章编上全书连续的章序
// （只用于锚点，不印在正文里——章标题要么来自内容，要么就是文件路径）。
//
// 分组的顺序：根目录文件永远第一部，其余按目录名升序——读一本书时
// "先看根目录有什么，再一个目录一个目录看下去"是最自然的顺序。
func buildBookParts(files []FileMetadata) []bookPart {
	sorted := append([]FileMetadata(nil), files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].RelPath < sorted[j].RelPath })

	grouped := map[string][]FileMetadata{}
	segments := []string{}
	for _, f := range sorted {
		seg := topSegment(f.RelPath)
		if _, ok := grouped[seg]; !ok {
			segments = append(segments, seg)
		}
		grouped[seg] = append(grouped[seg], f)
	}
	sort.Slice(segments, func(i, j int) bool {
		if (segments[i] == "") != (segments[j] == "") {
			return segments[i] == ""
		}
		return segments[i] < segments[j]
	})

	parts := make([]bookPart, 0, len(segments))
	no := 0
	for _, seg := range segments {
		group := grouped[seg]
		part := bookPart{
			anchor: fmt.Sprintf("part-%d", len(parts)+1),
			title:  mdCode(seg + "/"),
			lead:   fmt.Sprintf("`%s/` 目录下的 %d 个文件（含其子目录）。", seg, len(group)),
		}
		if seg == "" {
			part.title = "根目录"
			part.lead = fmt.Sprintf("直接放在项目根目录下的 %d 个文件。", len(group))
		}
		for _, f := range group {
			no++
			ch := bookChapter{
				no:      no,
				relPath: f.RelPath,
				anchor:  fmt.Sprintf("ch-%02d", no),
				lines:   f.LineCount,
				size:    f.Size,
				modTime: fileModTime(f.FullPath),
			}
			// 目录要用章标题，所以标题得先读出来。这里多读一遍 Markdown：
			// 正文渲染时必须流式（可能几十 MB），不能把全文缓存在内存里等目录。
			// 读失败不在这里报（下一遍渲染正文时会带着路径报错），这里只留空标题。
			if isMarkdownPath(f.RelPath) {
				if title, _, err := markdownChapterBody(f.FullPath); err == nil {
					ch.title = title
				}
			}
			part.chapters = append(part.chapters, ch)
		}
		parts = append(parts, part)
	}
	return parts
}

// topSegment 取相对路径的顶层目录名；直接放在根目录下的文件返回 ""。
func topSegment(relPath string) string {
	if i := strings.IndexByte(relPath, '/'); i >= 0 {
		return relPath[:i]
	}
	return ""
}

// fileModTime 取文件的最后修改时间；取不到返回零值（调用方省略这一段）。
//
// 这里单独 stat 一次：扫描阶段填进 FileMetadata 的只有体积与行数，
// 而书排版要在每章开头标"最后修改"。相对"把每个文件完整读一遍"，这点开销可以忽略。
func fileModTime(fullPath string) time.Time {
	fi, err := os.Stat(fullPath)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}

// bookChapterMeta 拼一章开头那行说明，如：
//
//	> 📄 Markdown · 1,588 行 · 55.8 KB · `chapter-1.md` · 最后修改 2026-10-13 01:31
//
// Markdown 章（showPath=true）的标题来自正文的 H1，标题里没有文件路径，
// 所以把路径补在这行里，保证"这一章对应哪个文件"永远查得到。
func bookChapterMeta(ch bookChapter, showPath bool) string {
	bits := []string{languageLabel(ch.relPath), humanInt(ch.lines) + " 行", humanSize(ch.size)}
	if showPath {
		bits = append(bits, mdCode(ch.relPath))
	}
	if !ch.modTime.IsZero() {
		bits = append(bits, "最后修改 "+ch.modTime.Format("2006-01-02 15:04"))
	}
	return "> 📄 " + strings.Join(bits, " · ")
}

// bookLanguageSummary 给封面拼"主要语言"一行（按行数占比取前 limit 种）。
func bookLanguageSummary(stats Stats, limit int) string {
	exts := sortedExtStats(stats)
	if len(exts) == 0 {
		return ""
	}
	bits := make([]string, 0, limit)
	for _, e := range exts {
		if len(bits) >= limit {
			break
		}
		if e.TotalLines == 0 {
			continue
		}
		bits = append(bits, fmt.Sprintf("%s %.1f%%", languageLabelForExt(e.Ext), ratio(int64(e.TotalLines), int64(stats.TotalLines))))
	}
	return strings.Join(bits, " · ")
}

// listCell 把参数里的字符串列表拼成表格单元（超长时截断并标出总数）。
func listCell(items []string, limit int) string {
	if len(items) == 0 {
		return ""
	}
	shown, suffix := items, ""
	if len(items) > limit {
		shown, suffix = items[:limit], fmt.Sprintf(" 等 %d 项", len(items))
	}
	parts := make([]string, 0, len(shown))
	for _, it := range shown {
		parts = append(parts, mdCode(it))
	}
	return strings.Join(parts, "、") + suffix
}

// tocText 目录链接的文字里不能出现裸的 [ ]（会把链接截断）。
func tocText(s string) string {
	return strings.NewReplacer("[", "\\[", "]", "\\]").Replace(s)
}

// isMarkdownPath 判断一个文件是否按"正文"而不是"代码块"排进书里。
func isMarkdownPath(relPath string) bool {
	switch strings.ToLower(filepath.Ext(relPath)) {
	case ".md", ".markdown":
		return true
	}
	return false
}

// ============================================================
//  Markdown 正文处理
// ============================================================

// markdownChapterBody 读一篇 Markdown，拆成"章标题"与"正文"。
//
// 标题取文件开头的第一个 H1（跳过 BOM、前置 HTML 注释与 YAML front matter）。
// 找不到 H1 就返回空标题 + 全文，调用方退化成"用文件路径当标题"。
func markdownChapterBody(fullPath string) (string, string, error) {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", "", err
	}
	title, body := splitMarkdownTitle(strings.TrimPrefix(string(data), "\ufeff"))
	return title, body, nil
}

// splitMarkdownTitle 是 markdownChapterBody 的纯函数部分（便于单测）。
func splitMarkdownTitle(content string) (string, string) {
	lines := strings.Split(content, "\n")

	start := -1
	for i := 0; i < len(lines); {
		trimmed := strings.TrimSpace(lines[i])
		switch {
		case trimmed == "":
			i++
		case strings.HasPrefix(trimmed, "<!--"):
			// 前置 HTML 注释（本工具自己也会在书顶写一行溯源注释）
			for i < len(lines) && !strings.Contains(lines[i], "-->") {
				i++
			}
			i++
		case trimmed == "---" && looksLikeFrontMatter(lines, i):
			// YAML front matter：跳到配对的那一行之后
			j := i + 1
			for j < len(lines) && strings.TrimSpace(lines[j]) != "---" && strings.TrimSpace(lines[j]) != "..." {
				j++
			}
			i = j + 1
		default:
			start = i
			i = len(lines) // 跳出
		}
	}
	if start < 0 {
		return "", content
	}
	if title, ok := atxHeadingText(lines[start], 1); ok {
		body := strings.Join(lines[start+1:], "\n")
		return title, strings.TrimLeft(body, "\n")
	}
	return "", content
}

// looksLikeFrontMatter 只有"--- 后面紧跟 `key:` 形式的行"才当 front matter。
// 否则文档开头的 `---` 只是一条水平分隔线，跳过它会把正文吞掉。
func looksLikeFrontMatter(lines []string, i int) bool {
	if i+1 >= len(lines) {
		return false
	}
	next := strings.TrimSpace(lines[i+1])
	colon := strings.IndexByte(next, ':')
	if colon <= 0 {
		return false
	}
	for _, r := range next[:colon] {
		if !(r == '_' || r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// atxHeadingText 解析一行 ATX 标题，级别必须正好是 wantLevel。
func atxHeadingText(line string, wantLevel int) (string, bool) {
	level, rest, ok := parseATXHeading(line)
	if !ok || level != wantLevel {
		return "", false
	}
	text := strings.TrimSpace(strings.TrimRight(rest, "#"))
	if text == "" {
		return "", false
	}
	return text, true
}

// parseATXHeading 解析一行 ATX 标题：返回级别与"# 之后"的原始文本。
func parseATXHeading(line string) (int, string, bool) {
	s := strings.TrimLeft(line, " ")
	if len(line)-len(s) > 3 { // 缩进 ≥4 空格是代码块
		return 0, "", false
	}
	n := 0
	for n < len(s) && s[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return 0, "", false
	}
	if n < len(s) && s[n] != ' ' && s[n] != '\t' {
		return 0, "", false // "#tag" 不是标题
	}
	return n, s[n:], true
}

// demoteHeadings 把正文里的 ATX 标题整体下沉，让"章 → 节"的从属关系在阅读器
// 的 outline 里成立：正文里**最小的**标题级别对齐到 minTarget。
//
// 两个必须守住的点：
//   - 围栏代码块里的 `# 注释` 不是标题，必须原样保留。书稿里的 shell/Python
//     代码大量以 # 开头，漏掉这一步会把代码注释变成一堆标题（本书稿 129 行
//     以 # 开头，其中大部分在代码块里）。
//   - 只下沉、不上浮：正文最小的标题级别已经比 minTarget 深时保持原样，
//     免得把 `####` 抬成 `###` 反而打乱作者的分节意图。
func demoteHeadings(body string, minTarget int) string {
	if body == "" {
		return body
	}
	lines := strings.Split(body, "\n")

	minLevel := 0
	inFence, fenceChar, fenceLen := false, byte(0), 0
	for _, line := range lines {
		if inFence {
			if closesFence(line, fenceChar, fenceLen) {
				inFence = false
			}
			continue
		}
		if c, n, ok := fenceInfo(line); ok {
			inFence, fenceChar, fenceLen = true, c, n
			continue
		}
		if level, _, ok := parseATXHeading(line); ok && (minLevel == 0 || level < minLevel) {
			minLevel = level
		}
	}
	if minLevel == 0 {
		return body
	}
	delta := minTarget - minLevel
	if delta <= 0 {
		return body
	}

	out := make([]string, 0, len(lines))
	inFence, fenceChar, fenceLen = false, 0, 0
	for _, line := range lines {
		if inFence {
			out = append(out, line)
			if closesFence(line, fenceChar, fenceLen) {
				inFence = false
			}
			continue
		}
		if c, n, ok := fenceInfo(line); ok {
			inFence, fenceChar, fenceLen = true, c, n
			out = append(out, line)
			continue
		}
		if level, rest, ok := parseATXHeading(line); ok {
			newLevel := level + delta
			if newLevel > 6 {
				newLevel = 6
			}
			out = append(out, strings.Repeat("#", newLevel)+rest)
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// fenceInfo 判断一行是否是围栏标记（``` 或 ~~~），返回标记字符与长度。
func fenceInfo(line string) (byte, int, bool) {
	s := strings.TrimLeft(line, " ")
	if len(line)-len(s) > 3 {
		return 0, 0, false
	}
	if s == "" || (s[0] != '`' && s[0] != '~') {
		return 0, 0, false
	}
	c := s[0]
	n := 0
	for n < len(s) && s[n] == c {
		n++
	}
	if n < 3 {
		return 0, 0, false
	}
	return c, n, true
}

// closesFence 判断一行是否闭合了当前围栏：同种字符、不短于开启标记、
// 且后面没有别的内容（CommonMark 的闭合条件）。
func closesFence(line string, char byte, length int) bool {
	s := strings.TrimLeft(line, " ")
	if len(line)-len(s) > 3 || s == "" || s[0] != char {
		return false
	}
	n := 0
	for n < len(s) && s[n] == char {
		n++
	}
	return n >= length && strings.TrimSpace(s[n:]) == ""
}

// humanInt 给数字加千分位（"12345" → "12,345"）。
func humanInt(n int) string {
	digits := strconv.Itoa(n)
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}
	var b strings.Builder
	for i := 0; i < len(digits); i++ {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(digits[i])
	}
	return sign + b.String()
}

// humanSize 把字节数写成人话（书里的"体积"）。
func humanSize(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	default:
		return fmt.Sprintf("%.2f GB", float64(n)/(1024*1024*1024))
	}
}

// languageDisplay 把 languageMap 里的小写标识写成给人看的写法。
var languageDisplay = map[string]string{
	"go": "Go", "javascript": "JavaScript", "typescript": "TypeScript", "python": "Python",
	"c": "C", "cpp": "C++", "java": "Java", "ruby": "Ruby", "php": "PHP", "rust": "Rust",
	"swift": "Swift", "kotlin": "Kotlin", "objectivec": "Objective-C", "bash": "Shell",
	"fish": "Fish", "yaml": "YAML", "json": "JSON", "xml": "XML", "html": "HTML",
	"css": "CSS", "scss": "SCSS", "sass": "Sass", "less": "Less", "markdown": "Markdown",
	"sql": "SQL", "graphql": "GraphQL", "protobuf": "Protobuf", "dockerfile": "Dockerfile",
	"makefile": "Makefile", "cmake": "CMake", "vue": "Vue", "svelte": "Svelte",
	"dart": "Dart", "lua": "Lua", "perl": "Perl", "elixir": "Elixir", "erlang": "Erlang",
	"haskell": "Haskell", "ocaml": "OCaml", "clojure": "Clojure", "hcl": "HCL",
}

// languageLabel 给一个文件路径返回它的语言名（"Go"、"Markdown"…）。
func languageLabel(relPath string) string {
	if lang := detectLanguage(relPath); lang != "" {
		if disp, ok := languageDisplay[lang]; ok {
			return disp
		}
		return lang
	}
	return languageLabelForExt(filepath.Ext(relPath))
}

// languageLabelForExt 同上，但输入是扩展名（含点或不含点都行）。
// 认不出来的扩展名就大写成原样（".ygs" → "YGS"），不假装认识。
func languageLabelForExt(ext string) string {
	e := strings.ToLower(ext)
	if e == "[no ext]" {
		return "无扩展名"
	}
	if lang, ok := languageMap[e]; ok {
		if disp, ok := languageDisplay[lang]; ok {
			return disp
		}
		return lang
	}
	e = strings.TrimPrefix(e, ".")
	if e == "" {
		return "纯文本"
	}
	return strings.ToUpper(e)
}
