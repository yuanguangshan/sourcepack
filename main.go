package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/pflag"
)

// ============================================================
//  Configuration & Data Types
// ============================================================

var versionStr string // set via -ldflags "-X main.versionStr=vX.Y.Z" or auto-detected

func init() {
	if versionStr != "" {
		return
	}
	versionStr = "dev"
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			versionStr = info.Main.Version
		}
	}
}

type Config struct {
	RootDir           string
	OutputFile        string
	IncludeExts       []string
	IncludeMatches    []string
	ExcludeExts       []string
	ExcludeMatches    []string
	MaxFileSize       int64
	NoSubdirs         bool
	Verbose           bool
	Version           bool
	ShowStats         bool
	All               bool // shortcut: disable all ignore rules
	DryRun            bool
	NoDefaultIgnore   bool
	NoGitignore       bool
	AdditionalIgnores []string
	Copy              bool
	Push              bool
	PushURL           string
	AuthKey           string
	ICloud            bool

	// Git 模式（-g / -G）：打包变更而不是全量快照。
	// 这些参数全部与既有的筛选/输出参数正交，可自由组合。
	GitDiff      bool   // -g：工作区 + 索引 相对 GitRev 的变更
	GitLog       int    // -G N：最近 N 个提交
	GitRev       string // --rev：-g 的比较基准（默认 HEAD）
	GitStaged    bool   // --staged：只看暂存区
	GitNoMerges  bool   // --no-merges：-G 时排除合并提交
	DiffOnly     bool   // --diff-only：只输出 diff，不输出变更文件全文
	MaxDiffLines int    // --max-diff-lines：单文件 diff 行数上限（0 = 不限）
	Help         bool   // -h / --help

	// parseErr 记录参数解析失败（只会在 ContinueOnError 的 FlagSet 上出现，
	// 生产路径的 pflag.CommandLine 是 ExitOnError，出错时已自行退出）。
	// 由 validateConfig 负责把它抛出来，避免单测里参数写错被静默吞掉。
	parseErr error

	// changeSet 由 run() 在 git 模式下填充，供 writeContent 分支渲染。
	changeSet *ChangeSet
}

func (c Config) gitMode() bool { return c.GitDiff || c.GitLog > 0 }

type FileMetadata struct {
	RelPath   string
	FullPath  string
	Size      int64
	LineCount int
}

type Stats struct {
	PotentialMatches   int
	ExplicitlyExcluded int
	FileCount          int
	TotalSize          int64
	TotalLines         int
	TotalTokens        int
	Skipped            int
	DirCount           int

	// 被内置忽略规则整目录跳过的目录（不改列表本身，至少让用户看得见）
	IgnoredDirs     int
	IgnoredDirNames []string

	DirMap map[string]*DirStats
	ExtMap map[string]*ExtStats
}

type SkippedFile struct {
	RelPath string
	Reason  string
}

type DirStats struct {
	Path       string
	FileCount  int
	TotalSize  int64
	TotalLines int
}

type ExtStats struct {
	Ext        string
	FileCount  int
	TotalSize  int64
	TotalLines int
}

// ============================================================
//  Ignore Rules & Language Map
// ============================================================

var languageMap = map[string]string{
	".go": "go", ".js": "javascript", ".ts": "typescript", ".py": "python",
	".c": "c", ".cpp": "cpp", ".h": "cpp", ".hpp": "cpp", ".cc": "cpp",
	".java": "java", ".rb": "ruby", ".php": "php", ".rs": "rust",
	".swift": "swift", ".kt": "kotlin", ".m": "objectivec", ".mm": "objectivec",
	".sh": "bash", ".zsh": "bash", ".bash": "bash", ".fish": "fish",
	".yml": "yaml", ".yaml": "yaml", ".json": "json", ".xml": "xml",
	".html": "html", ".css": "css", ".scss": "scss", ".sass": "sass", ".less": "less",
	".md": "markdown", ".sql": "sql", ".graphql": "graphql", ".proto": "protobuf",
	".dockerfile": "dockerfile", ".makefile": "makefile", ".cmake": "cmake",
	".vue": "vue", ".svelte": "svelte", ".dart": "dart", ".lua": "lua",
	".pl": "perl", ".ex": "elixir", ".erl": "erlang", ".hs": "haskell",
	".ml": "ocaml", ".clj": "clojure", ".tf": "hcl",
}

// ============================================================
//  Main Entry
// ============================================================

func main() {
	config := parseFlags()

	if config.Help {
		// 自己处理 -h：pflag 的内置 help 会走 ErrHelp，在 ExitOnError 下
		// 打印 "pflag: help requested" 并以 2 退出；这里给出正常的 0 退出。
		printUsage(os.Stdout, pflag.CommandLine)
		return
	}

	if config.Version {
		fmt.Printf("sourcepack %s\n", versionStr)
		return
	}

	if err := run(config); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
}

func run(config Config) error {
	if err := validateConfig(config, pflag.CommandLine); err != nil {
		return err
	}

	if config.Verbose {
		printConfigSummary(config)
	}

	if config.All {
		config.NoDefaultIgnore = true
		config.NoGitignore = true
	}

	if !config.DryRun {
		fmt.Println("▶ Sourcepack Started")
	} else {
		fmt.Println("▶ Sourcepack Dry-Run Mode")
	}

	startTime := time.Now()

	// 1. Scan and filter
	//    git 模式（-g / -G）下"扫描"变成"取 changeset"，findings 之后
	//    完全共用同一条 pipeline：过滤、忽略、敏感信息、体积、渲染、
	//    剪贴板、推送、统计都不需要另写一套。
	var files []FileMetadata
	var stats Stats
	var skipped []SkippedFile
	if config.gitMode() {
		cs, err := collectChangeSet(&config)
		if err != nil {
			return err
		}
		config.changeSet = cs
		files, stats, skipped = buildChangeFiles(&config, cs)
		printChangeSetTerminal(cs)
	} else {
		files, stats, skipped = scanDirectory(config)
	}

	if config.Verbose && len(skipped) > 0 {
		fmt.Printf("\n⏭  Skipped %d files:\n", len(skipped))
		for _, s := range skipped {
			fmt.Printf("  - %-40s [%s]\n", s.RelPath, s.Reason)
		}
	}

	// 2. Dry-run: terminal output only
	if config.DryRun {
		printDryRun(files, stats, skipped)
		printStatsTerminal(files, stats)
		return nil
	}

	// 3. Stats-only mode
	if config.ShowStats {
		printStatsTerminal(files, stats)
		if config.Push {
			if config.PushURL == "" {
				return fmt.Errorf("SOURCEPACK_PUSH_URL env is required for push")
			}
			if err := pushStatsToRemote(config, files, stats); err != nil {
				return fmt.Errorf("failed to push stats: %w", err)
			}
		}
		fmt.Printf("\n✨ Done! in %v\n", time.Since(startTime))
		return nil
	}

	// 4. Full output modes
	if config.Copy {
		if err := copyToClipboardStreaming(config, files, stats); err != nil {
			return fmt.Errorf("failed to copy: %w", err)
		}
		fmt.Printf("\n✨ Done! Copied to clipboard in %v\n", time.Since(startTime))
		return nil
	}

	if config.Push {
		if config.PushURL == "" {
			return fmt.Errorf("SOURCEPACK_PUSH_URL env is required for push")
		}
		if err := pushToRemoteStreaming(config, files, stats); err != nil {
			return fmt.Errorf("failed to push: %w", err)
		}
		fmt.Printf("\n✨ Done! Pushed to %s in %v\n", config.PushURL, time.Since(startTime))
		return nil
	}

	if config.ICloud {
		return saveToICloud(config, files, stats, startTime)
	}

	// Default: write to output file
	outFile, err := os.Create(config.OutputFile)
	if err != nil {
		return fmt.Errorf("creating %s: %w", config.OutputFile, err)
	}
	if err := writeContent(config, files, stats, outFile); err != nil {
		outFile.Close()
		return fmt.Errorf("writing content: %w", err)
	}
	outFile.Close()
	fmt.Printf("\n✨ Done! Generated %s in %v\n", config.OutputFile, time.Since(startTime))
	return nil
}

// printUsage 打印用法说明：pflag 的 flag 列表 + 三段项目自己的说明。
//
// PrintDefaults 写的是 FlagSet 自己的输出目标，所以这里临时切到 w，
// 打完恢复成调用前那个目标（pflag ≥1.0.6 才有 Output()；go.mod 已升到 1.0.7）。
func printUsage(w io.Writer, fs *pflag.FlagSet) {
	prev := fs.Output()
	fs.SetOutput(w)
	defer fs.SetOutput(prev)

	fmt.Fprintf(w, "Usage of %s:\n", programName())
	fs.PrintDefaults()
	fmt.Fprintln(w, "\nConvenience shortcuts:")
	fmt.Fprintln(w, "  -a              Include all files (disable .gitignore + built-in ignores; .gdignore still applies)")
	fmt.Fprintln(w, "\nCombinations:")
	fmt.Fprintln(w, "  -s -p           Show stats and push the stats-only markdown (the only allowed pair)")
	fmt.Fprintln(w, "  other pairs     Error: -s / -c / -p / --icloud are mutually exclusive destinations")
	fmt.Fprintf(w, "\nEnvironment variables:\n")
	fmt.Fprintf(w, "  SOURCEPACK_PUSH_URL    Remote URL for push (e.g. https://host/submit)\n")
	fmt.Fprintf(w, "  SOURCEPACK_AUTH_KEY    X-Auth-Key for push authentication\n")
}

// programName 用法说明里的程序名。
//
// npm 包装器（index.js）直接 spawn 平台二进制，所以 os.Args[0] 会是
// ".../sourcepack-darwin-arm64" 这样的完整路径——取 basename，
// 再把平台后缀归一成 sourcepack，输出才像人话。
func programName() string {
	name := filepath.Base(os.Args[0])
	if strings.HasPrefix(name, "sourcepack-") {
		return "sourcepack"
	}
	return name
}

func initFlags(fs *pflag.FlagSet) {
	fs.Usage = func() { printUsage(os.Stderr, fs) }
	if fs == pflag.CommandLine {
		// (*FlagSet).usage() 在 f == CommandLine 时调的是**包级** pflag.Usage，
		// 只设 fs.Usage 会让上面这段自定义说明整段丢失（v1.8 重构踩过）。
		pflag.Usage = fs.Usage
	}
}

// parseFlags 解析命令行。pflag.CommandLine 是 ExitOnError 的，
// 所以参数写错时会打印用法并以 2 退出（与历史行为一致）。
func parseFlags() Config {
	return parseFlagsInto(pflag.CommandLine, os.Args[1:])
}

// parseFlagsInto 与 parseFlags 等价，但允许传入自定义 FlagSet，
// 这样参数解析与组合校验都能在单测里覆盖。
func parseFlagsInto(fs *pflag.FlagSet, args []string) Config {
	var c Config

	initFlags(fs)

	fs.StringVarP(&c.RootDir, "dir", "d", ".", "Root directory to scan")
	fs.StringVarP(&c.OutputFile, "out", "o", "project_snapshot.md", "Output markdown file")

	var incExts, incMatches, excExts, excMatches, addIgnores string
	fs.StringVarP(&incExts, "include", "i", "", "Only include these extensions (e.g. go,md)")
	fs.StringVarP(&incMatches, "match", "m", "", "Only include paths containing these keywords")
	fs.StringVarP(&excExts, "exclude", "x", "", "Exclude these extensions (e.g. exe,bin)")
	fs.StringVarP(&excMatches, "exclude-match", "X", "", "Exclude paths containing these keywords")
	fs.StringVarP(&addIgnores, "ignore", "I", "", "Additional gitignore-style patterns (comma separated)")

	fs.Int64Var(&c.MaxFileSize, "max-size", 500, "Max file size in KB")
	fs.BoolVarP(&c.NoSubdirs, "no-subdirs", "n", false, "Do not scan subdirectories")
	fs.BoolVarP(&c.Verbose, "verbose", "v", false, "Verbose output")
	fs.BoolVar(&c.Version, "version", false, "Show version")
	fs.BoolVarP(&c.ShowStats, "stats", "s", false, "Show detailed multi-dimensional statistics")
	fs.BoolVar(&c.DryRun, "dry-run", false, "Dry run mode (no file write)")
	fs.BoolVar(&c.NoDefaultIgnore, "no-default-ignore", false, "Disable default ignore rules")
	fs.BoolVar(&c.NoGitignore, "no-gitignore", false, "Do not load .gitignore")
	fs.BoolVarP(&c.Copy, "copy", "c", false, "Copy output to clipboard instead of file")
	fs.BoolVarP(&c.Push, "push", "p", false, "Push output to remote (requires --push-url or SOURCEPACK_PUSH_URL env)")
	fs.StringVarP(&c.PushURL, "push-url", "u", "", "Remote URL for push (e.g. https://host/submit). Overrides SOURCEPACK_PUSH_URL env")
	fs.StringVar(&c.AuthKey, "auth-key", "", "X-Auth-Key for push auth (or env SOURCEPACK_AUTH_KEY)")
	fs.BoolVarP(&c.Help, "help", "h", false, "Show this help")
	fs.BoolVar(&c.ICloud, "icloud", false, "Save output to iCloud Documents folder")
	fs.BoolVarP(&c.All, "all", "a", false, "Include all files (disable .gitignore + built-in ignores; .gdignore still applies)")

	// --- Git 变更打包（与上面的筛选/输出参数正交，可自由组合）---
	fs.BoolVarP(&c.GitDiff, "git-diff", "g", false, "Package uncommitted changes: working tree + index vs --rev (default HEAD)")
	fs.StringVar(&c.GitRev, "rev", "HEAD", "Revision to diff against for -g/--git-diff (any git revspec)")
	fs.BoolVar(&c.GitStaged, "staged", false, "With -g: only staged changes (git diff --cached)")
	fs.IntVarP(&c.GitLog, "commits", "G", 0, "Package the last N commits (git log -N)")
	fs.BoolVar(&c.GitNoMerges, "no-merges", false, "With -G: exclude merge commits")
	fs.BoolVar(&c.DiffOnly, "diff-only", false, "With -g/-G: emit diffs only, no full file contents")
	fs.IntVar(&c.MaxDiffLines, "max-diff-lines", 2000, "With -g/-G: max diff lines kept per file (0 = unlimited)")

	if err := fs.Parse(args); err != nil {
		// ExitOnError 的 CommandLine 在出错时已退出；能走到这里说明是
		// ContinueOnError 的 FlagSet（单测）。错误记下来交给 validateConfig，
		// 否则参数写错会静默拿到一份零值 Config。
		c.parseErr = err
	}

	if c.PushURL == "" {
		c.PushURL = os.Getenv("SOURCEPACK_PUSH_URL")
	}
	if c.AuthKey == "" {
		c.AuthKey = os.Getenv("SOURCEPACK_AUTH_KEY")
	}

	if incExts != "" {
		c.IncludeExts = cleanList(incExts)
	}
	if incMatches != "" {
		c.IncludeMatches = cleanPathList(incMatches)
	}
	if excExts != "" {
		c.ExcludeExts = cleanList(excExts)
	}
	if excMatches != "" {
		c.ExcludeMatches = cleanPathList(excMatches)
	}
	if addIgnores != "" {
		c.AdditionalIgnores = cleanPathList(addIgnores)
	}

	c.MaxFileSize *= 1024
	if absRoot, err := filepath.Abs(c.RootDir); err == nil {
		c.RootDir = absRoot
	}

	// git 模式下默认产物名与全量快照区分开（用户显式给 -o 时不覆盖）。
	if c.gitMode() && !fs.Changed("out") {
		c.OutputFile = "git_changes.md"
	}

	return c
}

// validateConfig 校验参数组合。宁可明确报错，也不要静默忽略用户给错的参数。
//
// fs 必须是"解析这组参数时用的那个 FlagSet"：--rev / --diff-only / -n
// 在 git 模式下的含义要靠 fs.Changed() 区分"用户显式给了"与"取了默认值"，
// 传一个干净的 FlagSet 会漏报。
func validateConfig(c Config, fs *pflag.FlagSet) error {
	if c.parseErr != nil {
		return fmt.Errorf("invalid arguments: %w", c.parseErr)
	}
	if c.GitDiff && c.GitLog > 0 {
		return fmt.Errorf("-g/--git-diff and -G/--commits are mutually exclusive (pick one)")
	}
	if c.GitLog < 0 {
		return fmt.Errorf("-G/--commits must be >= 1")
	}
	if fs.Changed("commits") && c.GitLog == 0 {
		return fmt.Errorf("-G/--commits must be >= 1")
	}
	if c.GitStaged && !c.GitDiff {
		return fmt.Errorf("--staged only applies to -g/--git-diff")
	}
	if fs.Changed("no-merges") && c.GitLog == 0 {
		return fmt.Errorf("--no-merges only applies to -G/--commits")
	}
	if fs.Changed("rev") && !c.GitDiff {
		return fmt.Errorf("--rev only applies to -g/--git-diff")
	}
	if fs.Changed("diff-only") && !c.gitMode() {
		return fmt.Errorf("--diff-only only applies to -g/--git-diff or -G/--commits")
	}
	if fs.Changed("max-diff-lines") && !c.gitMode() {
		return fmt.Errorf("--max-diff-lines only applies to -g/--git-diff or -G/--commits")
	}
	if c.MaxDiffLines < 0 {
		return fmt.Errorf("--max-diff-lines must be >= 0")
	}
	if fs.Changed("no-subdirs") && c.gitMode() {
		return fmt.Errorf("-n/--no-subdirs has no effect in git mode; use -m/--match or -d <subdir> instead")
	}

	// 输出目的地互斥。原实现只是打印一句警告，然后按"实际代码顺序"悄悄取一个
	// ——而那句警告写的优先级（clipboard > push > icloud > stats > file）与真实
	// 顺序相反（stats 分支在 copy 之前 return）。与其撒谎，不如明确报错。
	// 唯一例外是文档化的 -s -p：-s 生成的那份报告正是被推送的内容。
	var dests []string
	if c.ShowStats && !c.Push {
		dests = append(dests, "-s/--stats")
	}
	if c.Copy {
		dests = append(dests, "-c/--copy")
	}
	if c.Push {
		dests = append(dests, "-p/--push")
	}
	if c.ICloud {
		dests = append(dests, "--icloud")
	}
	if len(dests) > 1 {
		return fmt.Errorf("output destinations are mutually exclusive, but got %s\n  (the only allowed pair is -s -p: the stats report is what gets pushed)",
			strings.Join(dests, " + "))
	}
	return nil
}

func cleanList(s string) []string {
	parts := strings.Split(s, ",")
	var res []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, ".") && !strings.ContainsAny(trimmed, "/\\") {
			res = append(res, "."+trimmed)
		} else {
			res = append(res, trimmed)
		}
	}
	return res
}

// cleanPathList splits a comma-separated list of path keywords or
// gitignore-style patterns, trimming whitespace, without adding any
// prefix. Used for --match/--exclude-match/--ignore, where a leading
// dot would silently break the match (e.g. "_COMPLETE_BOOK" would
// become "._COMPLETE_BOOK" and never match).
func cleanPathList(s string) []string {
	parts := strings.Split(s, ",")
	var res []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		res = append(res, trimmed)
	}
	return res
}

// ============================================================
//  Scanning Logic
// ============================================================

func scanDirectory(config Config) ([]FileMetadata, Stats, []SkippedFile) {
	var files []FileMetadata
	stats := Stats{
		DirMap: make(map[string]*DirStats),
		ExtMap: make(map[string]*ExtStats),
	}
	var skipped []SkippedFile

	// .gitignore（含各级子目录的嵌套）与 .gdignore 都按 gitignore 语义生效。
	// -a / --no-gitignore 只是不读 .gitignore，.gdignore 仍然生效。
	var ignoreNames []string
	if !config.NoGitignore {
		ignoreNames = append(ignoreNames, ".gitignore")
	}
	ignoreNames = append(ignoreNames, ".gdignore")
	matcher := newIgnoreMatcher(config.RootDir, ignoreNames...)
	matcher.addCLIPatterns(config.AdditionalIgnores)

	if config.Verbose {
		var present []string
		for _, n := range ignoreNames {
			if _, err := os.Stat(filepath.Join(config.RootDir, n)); err == nil {
				present = append(present, n)
			}
		}
		if len(present) > 0 {
			fmt.Printf("  Ignore files:        %s (子目录中的同名文件同样生效)\n", strings.Join(present, ", "))
		}
	}

	walkErr := filepath.WalkDir(config.RootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if config.Verbose {
				fmt.Printf("  ! Warning: cannot access %s: %v\n", path, err)
			}
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		relPath, err := filepath.Rel(config.RootDir, path)
		if err != nil {
			if config.Verbose {
				fmt.Printf("  ! Warning: cannot compute relative path %s: %v\n", path, err)
			}
			return nil
		}
		// 统一为 "/" 分隔：gitignore 规则、树形图、TOC 全部以 "/" 风格匹配，
		// 避免 Windows 上 "\" 与规则里的 "/" 不一致导致 ignore 失效。
		relPath = filepath.ToSlash(relPath)
		if relPath == "." {
			return nil
		}

		if d.IsDir() {
			if shouldIgnoreDir(relPath, config, matcher) {
				// 被内置规则整目录跳过：以前完全静默，用户无从得知。
				// 这里只做记录与报告，不改默认列表（收窄列表是行为变更，另议）。
				stats.IgnoredDirs++
				if len(stats.IgnoredDirNames) < 10 {
					stats.IgnoredDirNames = append(stats.IgnoredDirNames, relPath)
				}
				return filepath.SkipDir
			}
			// NoSubdirs：只扫根目录，遇到任何子目录就 SkipDir。
			//
			// 注意：走到这里说明 relPath != "."（根目录已在上面 return），
			// 因此"任何目录"就是"任何子目录"。
			//
			// 原实现用 strings.Contains(relPath, "/") 判断，是错的：
			// 一级子目录的 relPath 是 "src"（不含 "/"），条件永不成立，
			// 于是 -n 完全失效；而到 "src/deep" 时才含 "/"，那时父目录
			// 早已被遍历，SkipDir 已经没有意义。
			if config.NoSubdirs {
				return filepath.SkipDir
			}
			stats.DirCount++
			return nil
		}

		stats.PotentialMatches++
		if shouldIgnoreFile(relPath, config, matcher) {
			stats.ExplicitlyExcluded++
			return nil
		}

		// Skip output file（统一为 "/" 风格再比较）
		outRel := filepath.ToSlash(config.OutputFile)
		if filepath.IsAbs(config.OutputFile) {
			if r, err := filepath.Rel(config.RootDir, config.OutputFile); err == nil {
				outRel = filepath.ToSlash(r)
			}
		}
		if relPath == outRel {
			return nil
		}

		// 快筛：超限文件直接跳过，不读内容
		info, err := d.Info()
		if err != nil {
			skipped = append(skipped, SkippedFile{relPath, "Stat error"})
			stats.Skipped++
			return nil
		}
		if info.Size() > config.MaxFileSize {
			skipped = append(skipped, SkippedFile{relPath, "Size limit"})
			stats.Skipped++
			return nil
		}

		// 流式扫描：前 1KB 判二进制 + 全文数行，全程不把文件读进内存
		// 一律探测二进制：扩展名不能作为"内容是文本"的保证——实测一个
		// 含 NUL 字节的 .md 会被当文本写出，整个产物变成非法 UTF-8。
		// 探测复用同一个 bufio 缓冲，成本约等于 0。
		lineCount, isBinary, tokens, err := scanFileStats(path, true)
		if err != nil {
			skipped = append(skipped, SkippedFile{relPath, "Read error"})
			stats.Skipped++
			return nil
		}
		if isBinary {
			skipped = append(skipped, SkippedFile{relPath, "Binary file"})
			stats.Skipped++
			return nil
		}

		fMeta := FileMetadata{RelPath: relPath, FullPath: path, Size: info.Size(), LineCount: lineCount}
		files = append(files, fMeta)

		if config.Verbose {
			fmt.Printf("  + %-40s %d lines\n", relPath, lineCount)
		}

		accumulateStats(&stats, fMeta, tokens)

		return nil
	})
	if walkErr != nil && config.Verbose {
		fmt.Printf("  ! Walk error: %v\n", walkErr)
	}

	return files, stats, skipped
}

// accumulateStats 把一个文件计入统计。全量快照与 git 变更模式共用，
// 保证两种模式下的 -s 输出结构一致。
func accumulateStats(stats *Stats, fMeta FileMetadata, tokens int) {
	stats.FileCount++
	stats.TotalSize += fMeta.Size
	stats.TotalLines += fMeta.LineCount
	stats.TotalTokens += tokens

	dir := filepath.Dir(fMeta.RelPath)
	if _, ok := stats.DirMap[dir]; !ok {
		stats.DirMap[dir] = &DirStats{Path: dir}
	}
	stats.DirMap[dir].FileCount++
	stats.DirMap[dir].TotalSize += fMeta.Size
	stats.DirMap[dir].TotalLines += fMeta.LineCount

	ext := strings.ToLower(filepath.Ext(fMeta.RelPath))
	if ext == "" {
		ext = "[no ext]"
	}
	if _, ok := stats.ExtMap[ext]; !ok {
		stats.ExtMap[ext] = &ExtStats{Ext: ext}
	}
	stats.ExtMap[ext].FileCount++
	stats.ExtMap[ext].TotalSize += fMeta.Size
	stats.ExtMap[ext].TotalLines += fMeta.LineCount
}

// isBinaryBuffer 判断一段字节是否应按二进制处理。
//
// 判定标准（本版起收紧；旧实现只在"非法 UTF-8"时才去找 NUL，
// 于是 "abc\x00def" 这种合法 UTF-8 + NUL 的文件被当成文本，裸字节
// 直接写进了 Markdown，整个产物变成非法 UTF-8）：
//   - 含 NUL 字节 → 二进制（与 git 的 buffer_is_binary 同标准）；
//   - 不是合法 UTF-8 → 也按二进制处理，否则产物里会混入裸字节。
//
// 注意：这个函数要求 buf 是**完整内容**。窗口/分块场景请用
// incompleteUTF8Tail() 把末端的半截字符排除后再判断——直接把
// 按字节截断的缓冲区丢进来，会把"窗口切在多字节字符中间"误判成非法。
func isBinaryBuffer(buf []byte) bool {
	if len(buf) == 0 {
		return false
	}
	if bytes.IndexByte(buf, 0) >= 0 {
		return true
	}
	return !utf8.Valid(buf)
}

// incompleteUTF8Tail 返回末尾那段"不完整的多字节序列"的长度（0–3 字节）。
//
// 用于流式校验：分块读取时末尾可能正好截在一个字符中间，那不是内容非法，
// 只是这一块不完整。返回值是应该留到下一块、不参与本次校验的字节数。
func incompleteUTF8Tail(b []byte) int {
	for back := 1; back <= 3 && back <= len(b); back++ {
		start := len(b) - back
		c := b[start]
		if c < 0x80 {
			return 0 // ASCII 收尾，末尾完整
		}
		if c >= 0xC0 && c <= 0xF4 {
			need := 1
			switch {
			case c >= 0xF0:
				need = 3
			case c >= 0xE0:
				need = 2
			}
			if back-1 >= need {
				return 0 // 续字节齐了
			}
			for _, cc := range b[start+1:] {
				if cc < 0x80 || cc > 0xBF {
					return 0 // 后面不是续字节：不是"被截断"，是真的非法
				}
			}
			return back
		}
	}
	return 0
}

// scanFileStats 流式获取文件的行数与 token 估算值，并判断内容能否安全
// 写进 Markdown（含 NUL 或整文件不是合法 UTF-8 → 二进制）。
//
// 只顺序读一遍文件即可完成三件事，不把文件读进内存。
// probeBinary 目前调用方一律传 true，保留参数是为了明确"这一步可被显式关掉"。
//
// 判定范围是**整个文件**，不是前 N 字节：窗口截断会把切在多字节字符
// 中间的合法 UTF-8 误判成非法（实测任何 >8KB 且含中文的文件都会被误杀）。
func scanFileStats(path string, probeBinary bool) (lineCount int, isBinary bool, tokens int, err error) {
	return scanFileStatsChunk(path, probeBinary, scanChunkSize)
}

// scanChunkSize 是流式扫描的默认块大小。
const scanChunkSize = 32 * 1024

// scanFileStatsChunk 是 scanFileStats 的可参数化版本。
//
// chunkSize 存在的唯一目的是**测试**：把块切到 1/2/3/5 字节，跨块边界的
// 情形就被穷举了，不再依赖"夹具要够大"这种隐式前提。历史上这里就是
// 因为夹具只有 15000 字节（小于 32KB 块）而漏掉过一个真 bug。
func scanFileStatsChunk(path string, probeBinary bool, chunkSize int) (lineCount int, isBinary bool, tokens int, err error) {
	if chunkSize <= 0 {
		chunkSize = scanChunkSize
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, false, 0, err
	}
	defer f.Close()

	reader := bufio.NewReaderSize(f, chunkSize)

	// 数行 / 估算 token / 校验文本三件事共用这一遍读取（不额外读盘）。
	//
	// carry 存的是上一块末尾那几个"不完整的多字节字符"的字节，本块开头
	// 必须先把它们拼回来再校验，否则块边界上的汉字会被当成非法 UTF-8。
	// 这里曾经写成 append(scan[:carryLen], chunk...)——切出来的是旧缓冲区
	// 的**开头**几个字节，carry 的内容从未被拼回，导致任何跨 32KB 边界且
	// 含中文的文件被整文件判成 Binary（纯汉字碰巧能凑成合法字符而幸存）。
	var carry [4]byte
	carryLen := 0
	scan := make([]byte, 0, chunkSize+4) // 复用底层数组，避免每块分配
	buf := make([]byte, chunkSize)
	for {
		n, rerr := reader.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			lineCount += countLinesBuffer(chunk)
			tokens += countTokensBuffer(chunk)

			if probeBinary && !isBinary {
				scan = append(scan[:0], carry[:carryLen]...)
				scan = append(scan, chunk...)
				if bytes.IndexByte(scan, 0) >= 0 {
					isBinary = true
				} else {
					tail := incompleteUTF8Tail(scan)
					if !utf8.Valid(scan[:len(scan)-tail]) {
						isBinary = true
					} else {
						carryLen = copy(carry[:], scan[len(scan)-tail:])
					}
				}
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return 0, false, 0, rerr
		}
	}
	if probeBinary && !isBinary && carryLen > 0 {
		// 文件以不完整的多字节序列收尾 → 不是合法 UTF-8
		isBinary = true
	}
	return lineCount, isBinary, tokens, nil
}

// countTokensBuffer 估算一段 UTF-8 字节对应的 LLM token 数。
//
// 经验法则：
//   - CJK 汉字/全角标点约 1 token/字（UTF-8 每字 3 字节）；
//   - 其余（英文、代码、空白）约 4 字节/token。
//
// 旧实现一律 size/4，对中文项目明显偏低（3 字节的汉字只算 0.75）。
// 这里只做字节级判断、不解码整个文件：UTF-8 里 U+3000–U+9FFF
// （CJK 标点/假名/汉字）是 3 字节序列，首字节落在 0xE3–0xE9，
// 命中即按一个字计 token 并跳过两个续字节。
//
// 注意这是跨模型都会"偏保守"的粗估：DeepSeek/Qwen 这类中文优化过的
// 分词器实际会更省，gpt 系会更费；-s 里的数字当量级看即可。
func countTokensBuffer(data []byte) int {
	cjkChars, otherBytes := 0, 0
	for i := 0; i < len(data); i++ {
		if b := data[i]; b >= 0xE3 && b <= 0xE9 && i+2 < len(data) {
			cjkChars++
			i += 2
			continue
		}
		otherBytes++
	}
	return cjkChars + otherBytes/4
}

func countLinesBuffer(data []byte) int {
	n := 0
	for _, b := range data {
		if b == '\n' {
			n++
		}
	}
	return n
}

// ============================================================
//  Statistics & Output
// ============================================================

// moreSuffix 在截断名单时补一个 "(+N more)"，避免读者以为只有列出来的这些。
func moreSuffix(total, shown int) string {
	if total > shown {
		return fmt.Sprintf(", +%d more", total-shown)
	}
	return ""
}

func printStatsTerminal(files []FileMetadata, stats Stats) {
	fmt.Printf("\n📊 Project Statistics Summary\n")
	fmt.Printf("  %-20s %d\n", "Files Processed:", stats.FileCount)
	fmt.Printf("  %-20s %d\n", "Total Lines:", stats.TotalLines)
	fmt.Printf("  %-20s %.2f KB\n", "Total Size:", float64(stats.TotalSize)/1024)
	fmt.Printf("  %-20s %d\n", "Directories:", stats.DirCount)
	fmt.Printf("  %-20s ~%d tokens\n", "Est. Tokens:", stats.TotalTokens)
	if stats.ExplicitlyExcluded > 0 {
		fmt.Printf("  %-20s %d\n", "Excluded by rules:", stats.ExplicitlyExcluded)
	}
	if stats.IgnoredDirs > 0 {
		fmt.Printf("  %-20s %d (%s%s)\n", "Ignored dirs:", stats.IgnoredDirs,
			strings.Join(stats.IgnoredDirNames, ", "), moreSuffix(stats.IgnoredDirs, len(stats.IgnoredDirNames)))
	}

	// 1. Top Files by Line Count
	sort.Slice(files, func(i, j int) bool { return files[i].LineCount > files[j].LineCount })
	fmt.Printf("\n%-45s %12s %12s\n", "🔝 Top Files (by Lines):", "Lines", "Size")
	fmt.Printf("%-45s %12s %12s\n", "---------------------------------------------", "-----", "----")
	for i := 0; i < len(files) && i < 5; i++ {
		fmt.Printf("%-45s %12d %11.2f KB\n", files[i].RelPath, files[i].LineCount, float64(files[i].Size)/1024)
	}

	// 2. Directory Dimension
	dirs := make([]*DirStats, 0, len(stats.DirMap))
	for _, d := range stats.DirMap {
		dirs = append(dirs, d)
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].TotalLines > dirs[j].TotalLines })
	fmt.Printf("\n%-30s %8s %12s %10s %12s %10s\n", "📁 Folder Dimension:", "Files", "Lines", "Lines %", "Size", "Size %")
	fmt.Printf("%-30s %8s %12s %10s %12s %10s\n", "------------------------------", "-------", "-----------", "-------", "-----------", "-------")
	for i := 0; i < len(dirs) && i < 10; i++ {
		linePct := 0.0
		if stats.TotalLines > 0 {
			linePct = float64(dirs[i].TotalLines) / float64(stats.TotalLines) * 100
		}
		sizePct := 0.0
		if stats.TotalSize > 0 {
			sizePct = float64(dirs[i].TotalSize) / float64(stats.TotalSize) * 100
		}
		fmt.Printf("%-30s %8d %12d %9.1f%% %11.2f KB %9.1f%%\n",
			dirs[i].Path, dirs[i].FileCount, dirs[i].TotalLines, linePct, float64(dirs[i].TotalSize)/1024, sizePct)
	}

	// 3. Language Dimension
	exts := make([]*ExtStats, 0, len(stats.ExtMap))
	for _, e := range stats.ExtMap {
		exts = append(exts, e)
	}
	sort.Slice(exts, func(i, j int) bool { return exts[i].TotalLines > exts[j].TotalLines })
	fmt.Printf("\n%-15s %8s %12s %10s %12s %10s\n", "📝 Language:", "Files", "Lines", "Lines %", "Size", "Size %")
	fmt.Printf("%-15s %8s %12s %10s %12s %10s\n", "---------------", "-------", "-----------", "-------", "-----------", "-------")
	for _, e := range exts {
		linePct := 0.0
		if stats.TotalLines > 0 {
			linePct = float64(e.TotalLines) / float64(stats.TotalLines) * 100
		}
		sizePct := 0.0
		if stats.TotalSize > 0 {
			sizePct = float64(e.TotalSize) / float64(stats.TotalSize) * 100
		}
		fmt.Printf("%-15s %8d %12d %9.1f%% %11.2f KB %9.1f%%\n",
			e.Ext, e.FileCount, e.TotalLines, linePct, float64(e.TotalSize)/1024, sizePct)
	}
}

func generateStatsContent(config Config, files []FileMetadata, stats Stats) string {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	fmt.Fprintf(w, "# Project Statistics: %s\n\n", filepath.Base(config.RootDir))
	fmt.Fprintf(w, "> Generated by [Sourcepack](https://github.com/yuanguangshan/sourcepack) on %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	// Overview
	fmt.Fprintln(w, "## Overview")
	fmt.Fprintf(w, "- Files: %d\n- Lines: %d\n- Size: %.2f KB\n- Directories: %d\n- Est. Tokens: ~%d\n\n", stats.FileCount, stats.TotalLines, float64(stats.TotalSize)/1024, stats.DirCount, stats.TotalTokens)
	if stats.ExplicitlyExcluded > 0 {
		fmt.Fprintf(w, "- Excluded by rules: %d\n", stats.ExplicitlyExcluded)
	}
	if stats.IgnoredDirs > 0 {
		fmt.Fprintf(w, "- Ignored dirs: %d (%s%s)\n", stats.IgnoredDirs,
			strings.Join(stats.IgnoredDirNames, ", "), moreSuffix(stats.IgnoredDirs, len(stats.IgnoredDirNames)))
	}
	fmt.Fprintln(w)

	// Top Files
	sort.Slice(files, func(i, j int) bool { return files[i].LineCount > files[j].LineCount })
	fmt.Fprintln(w, "## Top Files (by Lines)")
	fmt.Fprintln(w, "| File | Lines | Size |")
	fmt.Fprintln(w, "| :--- | ---: | ---: |")
	for i := 0; i < len(files) && i < 5; i++ {
		fmt.Fprintf(w, "| %s | %d | %.2f KB |\n", files[i].RelPath, files[i].LineCount, float64(files[i].Size)/1024)
	}
	fmt.Fprintln(w)

	// Directory Dimension
	dirs := make([]*DirStats, 0, len(stats.DirMap))
	for _, d := range stats.DirMap {
		dirs = append(dirs, d)
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].TotalLines > dirs[j].TotalLines })
	fmt.Fprintln(w, "## Directory Distribution")
	fmt.Fprintln(w, "| Directory | Files | Lines | Lines % | Size | Size % |")
	fmt.Fprintln(w, "| :--- | ---: | ---: | ---: | ---: | ---: |")
	for i := 0; i < len(dirs) && i < 10; i++ {
		linePct := 0.0
		if stats.TotalLines > 0 {
			linePct = float64(dirs[i].TotalLines) / float64(stats.TotalLines) * 100
		}
		sizePct := 0.0
		if stats.TotalSize > 0 {
			sizePct = float64(dirs[i].TotalSize) / float64(stats.TotalSize) * 100
		}
		fmt.Fprintf(w, "| %s | %d | %d | %.1f%% | %.2f KB | %.1f%% |\n", dirs[i].Path, dirs[i].FileCount, dirs[i].TotalLines, linePct, float64(dirs[i].TotalSize)/1024, sizePct)
	}
	fmt.Fprintln(w)

	// Language Dimension
	exts := make([]*ExtStats, 0, len(stats.ExtMap))
	for _, e := range stats.ExtMap {
		exts = append(exts, e)
	}
	sort.Slice(exts, func(i, j int) bool { return exts[i].TotalLines > exts[j].TotalLines })
	fmt.Fprintln(w, "## Language Breakdown")
	fmt.Fprintln(w, "| Extension | Files | Lines | Lines % | Size | Size % |")
	fmt.Fprintln(w, "| :--- | ---: | ---: | ---: | ---: | ---: |")
	for _, e := range exts {
		linePct := 0.0
		if stats.TotalLines > 0 {
			linePct = float64(e.TotalLines) / float64(stats.TotalLines) * 100
		}
		sizePct := 0.0
		if stats.TotalSize > 0 {
			sizePct = float64(e.TotalSize) / float64(stats.TotalSize) * 100
		}
		fmt.Fprintf(w, "| %s | %d | %d | %.1f%% | %.2f KB | %.1f%% |\n", e.Ext, e.FileCount, e.TotalLines, linePct, float64(e.TotalSize)/1024, sizePct)
	}

	w.Flush()
	return buf.String()
}

func writeContent(config Config, files []FileMetadata, stats Stats, w io.Writer) error {
	// git 模式（-g / -G）：换成变更快照的排版，但沿用同一批
	// files/stats（因此 -s / -c / -p / --icloud 全部照旧可用）。
	if config.changeSet != nil {
		return writeChangeContent(config, files, stats, config.changeSet, w)
	}

	bw := bufio.NewWriter(w)

	fmt.Fprintf(bw, "# Project Snapshot: %s\n\n", filepath.Base(config.RootDir))
	fmt.Fprintf(bw, "> Generated by [Sourcepack](https://github.com/yuanguangshan/sourcepack) on %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	// Project Structure Tree
	sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })
	tree := buildTreeString(files, filepath.Base(config.RootDir))
	fmt.Fprintln(bw, "## Project Structure")
	fmt.Fprintln(bw, "```")
	fmt.Fprint(bw, tree)
	fmt.Fprintln(bw, "```")
	fmt.Fprintln(bw)

	fmt.Fprintln(bw, "## Table of Contents")
	for _, f := range files {
		fmt.Fprintf(bw, "- [%s](#%s)\n", f.RelPath, generateAnchor(f.RelPath))
	}
	fmt.Fprintln(bw, "\n---")

	for _, f := range files {
		fmt.Fprintf(bw, "\n## %s\n\n", f.RelPath)
		if err := writeFileBody(bw, f); err != nil {
			return err
		}
	}

	if config.ShowStats {
		fmt.Fprintln(bw, "\n---")
		fmt.Fprintln(bw, "## Detailed Project Audit")

		fmt.Fprintln(bw, "### 📈 Overview")
		fmt.Fprintf(bw, "- Files: %d\n- Lines: %d\n- Size: %.2f KB\n- Est. Tokens: ~%d\n\n", stats.FileCount, stats.TotalLines, float64(stats.TotalSize)/1024, stats.TotalTokens)

		fmt.Fprintln(bw, "### 📁 Directory Distribution")
		fmt.Fprintln(bw, "| Directory | Files | Lines | Lines % | Size | Size % |")
		fmt.Fprintln(bw, "| :--- | :---: | :---: | :---: | :---: | :---: |")
		dirs := make([]*DirStats, 0, len(stats.DirMap))
		for _, d := range stats.DirMap {
			dirs = append(dirs, d)
		}
		sort.Slice(dirs, func(i, j int) bool { return dirs[i].Path < dirs[j].Path })
		for _, d := range dirs {
			linePct := 0.0
			if stats.TotalLines > 0 {
				linePct = float64(d.TotalLines) / float64(stats.TotalLines) * 100
			}
			sizePct := 0.0
			if stats.TotalSize > 0 {
				sizePct = float64(d.TotalSize) / float64(stats.TotalSize) * 100
			}
			fmt.Fprintf(bw, "| %s | %d | %d | %.1f%% | %.2f KB | %.1f%% |\n", d.Path, d.FileCount, d.TotalLines, linePct, float64(d.TotalSize)/1024, sizePct)
		}

		fmt.Fprintln(bw, "\n### 📝 Language Breakdown")
		fmt.Fprintln(bw, "| Extension | Files | Lines | Lines % | Size | Size % |")
		fmt.Fprintln(bw, "| :--- | :---: | :---: | :---: | :---: | :---: |")
		exts := make([]*ExtStats, 0, len(stats.ExtMap))
		for _, e := range stats.ExtMap {
			exts = append(exts, e)
		}
		sort.Slice(exts, func(i, j int) bool { return exts[i].TotalLines > exts[j].TotalLines })
		for _, e := range exts {
			linePct := 0.0
			if stats.TotalLines > 0 {
				linePct = float64(e.TotalLines) / float64(stats.TotalLines) * 100
			}
			sizePct := 0.0
			if stats.TotalSize > 0 {
				sizePct = float64(e.TotalSize) / float64(stats.TotalSize) * 100
			}
			fmt.Fprintf(bw, "| %s | %d | %d | %.1f%% | %.2f KB | %.1f%% |\n", e.Ext, e.FileCount, e.TotalLines, linePct, float64(e.TotalSize)/1024, sizePct)
		}
	}

	return bw.Flush()
}

// writeFileBody 输出一个文件的内容块（不含 Markdown 标题），
// 全量快照与变更快照共用同一套围栏/流式逻辑。
func writeFileBody(bw *bufio.Writer, f FileMetadata) error {
	// Stream file content: small files read once, large files scan + copy
	var content []byte
	var readErr error
	smallLimit := int64(32 * 1024)
	if f.Size <= smallLimit {
		content, readErr = os.ReadFile(f.FullPath)
		if readErr != nil {
			return fmt.Errorf("reading %s: %w", f.RelPath, readErr)
		}
	} else {
		// Large file: scan backticks via chunked read, then io.Copy
		maxBacktick, scanErr := scanMaxBackticks(f.FullPath)
		if scanErr != nil {
			return fmt.Errorf("scanning %s: %w", f.RelPath, scanErr)
		}
		fence := "```"
		if maxBacktick >= 3 {
			fence = strings.Repeat("`", maxBacktick+1)
		}
		lang := detectLanguage(f.RelPath)
		fmt.Fprintf(bw, "%s%s\n", fence, lang)

		src, openErr := os.Open(f.FullPath)
		if openErr != nil {
			return fmt.Errorf("opening %s: %w", f.RelPath, openErr)
		}
		if _, copyErr := io.Copy(bw, src); copyErr != nil {
			src.Close()
			return fmt.Errorf("copying %s: %w", f.RelPath, copyErr)
		}
		src.Close()

		bw.WriteByte('\n')
		fmt.Fprintf(bw, "%s\n", fence)
		return nil
	}
	maxBacktick := scanBackticksInData(content)
	fence := "```"
	if maxBacktick >= 3 {
		fence = strings.Repeat("`", maxBacktick+1)
	}
	lang := detectLanguage(f.RelPath)
	fmt.Fprintf(bw, "%s%s\n", fence, lang)
	bw.Write(content)

	// Ensure trailing newline before closing fence
	bw.WriteByte('\n')
	fmt.Fprintf(bw, "%s\n", fence)
	return nil
}

// scanMaxBackticks reads a file in 32KB chunks and returns the maximum
// consecutive backtick run found anywhere in the content.
func scanMaxBackticks(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	maxRun, curRun := 0, 0
	buf := make([]byte, 32*1024)
	for {
		n, readErr := f.Read(buf)
		if n == 0 {
			if readErr != nil {
				break
			}
			continue
		}
		for _, b := range buf[:n] {
			if b == '`' {
				curRun++
				if curRun > maxRun {
					maxRun = curRun
				}
			} else {
				curRun = 0
			}
		}
		if readErr != nil {
			break
		}
	}
	return maxRun, nil
}

// scanBackticksInData is the in-memory variant of scanMaxBackticks,
// used for small files that are already fully read.
func scanBackticksInData(data []byte) int {
	maxRun, curRun := 0, 0
	for _, b := range data {
		if b == '`' {
			curRun++
			if curRun > maxRun {
				maxRun = curRun
			}
		} else {
			curRun = 0
		}
	}
	return maxRun
}

func generateAnchor(p string) string {
	return strings.NewReplacer("/", "-", "\\", "-", ".", "-").Replace(strings.ToLower(p))
}

func detectLanguage(p string) string {
	ext := strings.ToLower(filepath.Ext(p))
	if l, ok := languageMap[ext]; ok {
		return l
	}
	base := strings.ToLower(filepath.Base(p))
	if base == "dockerfile" || base == "makefile" {
		return base
	}
	return ""
}

func printDryRun(files []FileMetadata, stats Stats, skipped []SkippedFile) {
	fmt.Printf("\n🔍 Files to be included (%d):\n", len(files))
	for _, f := range files {
		fmt.Printf("  - %-40s (%d lines)\n", f.RelPath, f.LineCount)
	}
	if len(skipped) > 0 {
		fmt.Printf("\n⏭  Skipped:\n")
		for _, s := range skipped {
			fmt.Printf("  - %-40s [%s]\n", s.RelPath, s.Reason)
		}
	}
}

func printConfigSummary(c Config) {
	fmt.Println("⚙  Configuration:")
	fmt.Printf("  %-20s %s\n", "Root:", c.RootDir)
	fmt.Printf("  %-20s %s\n", "Output:", c.OutputFile)
	fmt.Printf("  %-20s %d KB\n", "Max file size:", c.MaxFileSize/1024)
	if len(c.IncludeExts) > 0 {
		fmt.Printf("  %-20s %v\n", "Include exts:", c.IncludeExts)
	}
	if len(c.ExcludeExts) > 0 {
		fmt.Printf("  %-20s %v\n", "Exclude exts:", c.ExcludeExts)
	}
	if len(c.IncludeMatches) > 0 {
		fmt.Printf("  %-20s %v\n", "Include matches:", c.IncludeMatches)
	}
	if len(c.ExcludeMatches) > 0 {
		fmt.Printf("  %-20s %v\n", "Exclude matches:", c.ExcludeMatches)
	}
	if len(c.AdditionalIgnores) > 0 {
		fmt.Printf("  %-20s %v\n", "Extra ignores:", c.AdditionalIgnores)
	}
	fmt.Printf("  %-20s %v\n", "No subdirs:", c.NoSubdirs)
	fmt.Printf("  %-20s %v\n", "No default ignore:", c.NoDefaultIgnore)
	fmt.Printf("  %-20s %v\n", "No .gitignore:", c.NoGitignore)
	fmt.Printf("  %-20s %v\n", "Copy to clipboard:", c.Copy)
	fmt.Printf("  %-20s %v\n", "Push to remote:", c.Push)
	if c.Push {
		fmt.Printf("  %-20s %s\n", "Push URL:", c.PushURL)
	}
	if c.gitMode() {
		if c.GitDiff {
			fmt.Printf("  %-20s %s\n", "Git diff vs:", c.GitRev)
			fmt.Printf("  %-20s %v\n", "Staged only:", c.GitStaged)
		} else {
			fmt.Printf("  %-20s %d\n", "Git commits:", c.GitLog)
			fmt.Printf("  %-20s %v\n", "No merges:", c.GitNoMerges)
		}
		fmt.Printf("  %-20s %v\n", "Diff only:", c.DiffOnly)
		fmt.Printf("  %-20s %d\n", "Max diff lines:", c.MaxDiffLines)
	}
	fmt.Println()
}

// ============================================================
//  Tree Structure Generation
// ============================================================

type treeNode struct {
	children  map[string]*treeNode
	lineCount int // only set for files
}

func buildTreeString(files []FileMetadata, rootName string) string {
	root := &treeNode{children: make(map[string]*treeNode)}
	for _, f := range files {
		parts := strings.Split(f.RelPath, "/")
		node := root
		for _, part := range parts {
			if _, ok := node.children[part]; !ok {
				node.children[part] = &treeNode{children: make(map[string]*treeNode)}
			}
			node = node.children[part]
		}
		node.lineCount = f.LineCount
	}
	var sb strings.Builder
	sb.WriteString(rootName + "/\n")
	formatTree(&sb, root, "")
	return sb.String()
}

func formatTree(sb *strings.Builder, node *treeNode, prefix string) {
	// Sort: directories first, then files, alphabetically within each group
	names := make([]string, 0, len(node.children))
	for name := range node.children {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		iDir := len(node.children[names[i]].children) > 0
		jDir := len(node.children[names[j]].children) > 0
		if iDir != jDir {
			return iDir
		}
		return names[i] < names[j]
	})

	for i, name := range names {
		child := node.children[name]
		isDir := len(child.children) > 0
		isLast := i == len(names)-1
		connector := "├── "
		newPrefix := "│   "
		if isLast {
			connector = "└── "
			newPrefix = "    "
		}
		sb.WriteString(prefix + connector + name)
		if isDir {
			sb.WriteString("/")
		} else {
			sb.WriteString(fmt.Sprintf("  (%d lines)", child.lineCount))
		}
		sb.WriteString("\n")
		formatTree(sb, child, prefix+newPrefix)
	}
}

// ============================================================
//  Clipboard Support
// ============================================================

func saveToICloud(config Config, files []FileMetadata, stats Stats, startTime time.Time) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	icloudDir := filepath.Join(homeDir, "Library", "Mobile Documents", "com~apple~CloudDocs", "Documents")
	if err := os.MkdirAll(icloudDir, 0755); err != nil {
		return fmt.Errorf("cannot create iCloud directory: %w", err)
	}
	folderName := filepath.Base(config.RootDir)
	dateStr := time.Now().Format("2006-01-02")
	icloudPath := filepath.Join(icloudDir, folderName+"+"+dateStr+".md")
	outFile, err := os.Create(icloudPath)
	if err != nil {
		return fmt.Errorf("creating iCloud file %s: %w", icloudPath, err)
	}
	if err := writeContent(config, files, stats, outFile); err != nil {
		outFile.Close()
		return fmt.Errorf("writing content: %w", err)
	}
	outFile.Close()
	fmt.Printf("\n✨ Done! Saved to iCloud: %s in %v\n", icloudPath, time.Since(startTime))
	return nil
}

func copyToClipboardStreaming(config Config, files []FileMetadata, stats Stats) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard")
		} else {
			return fmt.Errorf("no clipboard tool found (install xclip or xsel)")
		}
	case "windows":
		cmd = exec.Command("clip")
	default:
		return fmt.Errorf("unsupported platform for clipboard: %s", runtime.GOOS)
	}

	pr, pw := io.Pipe()
	cmd.Stdin = pr

	if err := cmd.Start(); err != nil {
		pr.Close()
		return err
	}

	writeErr := writeContent(config, files, stats, pw)
	pw.Close()
	cmd.Wait()
	return writeErr
}

func uploadViaMultipart(config Config, filePath string, filename string) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, f); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest("POST", config.PushURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if config.AuthKey != "" {
		req.Header.Set("X-Auth-Key", config.AuthKey)
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func pushStatsToRemote(config Config, files []FileMetadata, stats Stats) error {
	tmp, err := os.CreateTemp("", "sourcepack-stats-*.md")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	content := generateStatsContent(config, files, stats)
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	return uploadViaMultipart(config, tmpPath, artifactBaseName(config)+"_stats.md")
}

// ============================================================
//  Remote Push Support (streaming via chunked transfer)
// ============================================================

func pushToRemoteStreaming(config Config, files []FileMetadata, stats Stats) error {
	tmp, err := os.CreateTemp("", "sourcepack-*.md")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	writeErr := writeContent(config, files, stats, tmp)
	tmp.Close()
	if writeErr != nil {
		return writeErr
	}

	return uploadViaMultipart(config, tmpPath, artifactBaseName(config)+"_snapshot.md")
}

// artifactBaseName 远端推送时的文件名前缀。git 模式用 _changes 后缀，
// 避免变更快照和全量快照在服务端互相覆盖。
func artifactBaseName(config Config) string {
	base := filepath.Base(config.RootDir)
	if config.gitMode() {
		return base + "_changes"
	}
	return base
}
