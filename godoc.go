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
}

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

var ignoreDirs = map[string]bool{
	".git": true, ".idea": true, ".vscode": true, ".svn": true, ".hg": true,
	"node_modules": true, "vendor": true, "dist": true, "build": true,
	"target": true, "bin": true, "out": true, "release": true, "debug": true,
	"__pycache__": true, ".pytest_cache": true, ".tox": true,
	".env": true, ".venv": true, "venv": true, "env": true,
	"Pods": true, "Carthage": true, "CocoaPods": true,
	"obj": true, "ipch": true, "Debug": true, "Release": true,
	"x64": true, "x86": true, "arm64": true,
	".gradle": true, ".sonar": true, ".scannerwork": true,
	"logs": true, "tmp": true, "temp": true, "cache": true,
	".history": true, ".nyc_output": true, ".coverage": true,
}

var ignoreFiles = map[string]bool{
	"package-lock.json": true, "yarn.lock": true, "go.sum": true,
	"composer.lock": true, "Gemfile.lock": true,
	"tags": true, "TAGS": true, ".DS_Store": true,
	"coverage.xml": true, "thumbs.db": true,
}

var knownTextFiles = map[string]bool{
	"Makefile": true, "Dockerfile": true, "Rakefile": true, "Gemfile": true,
	"CMakeLists.txt": true, "Vagrantfile": true, "Jenkinsfile": true,
	"README": true, "LICENSE": true, "CHANGELOG": true, "CONTRIBUTING": true,
}

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
//  Gitignore Pattern Matching
// ============================================================

type gitignoreMatcher struct {
	patterns []gitignorePattern
}

type gitignorePattern struct {
	negate   bool   // ! prefix
	dirOnly  bool   // trailing /
	anchored bool   // leading /
	starStar bool   // contains **
	raw      string // pattern text after stripping markers
	literal  string // longest literal prefix (before any glob char)
}

// loadGitignore reads and parses a .gitignore file.
func loadGitignore(root string) *gitignoreMatcher {
	return loadIgnoreFile(filepath.Join(root, ".gitignore"))
}

// loadGdignore reads and parses a .gdignore file — a sourcepack-specific
// ignore file with the same syntax as .gitignore, letting you exclude files
// from snapshots without modifying your project's .gitignore.
func loadGdignore(root string) *gitignoreMatcher {
	return loadIgnoreFile(filepath.Join(root, ".gdignore"))
}

func loadIgnoreFile(path string) *gitignoreMatcher {
	data, err := os.ReadFile(path)
	if err != nil {
		return &gitignoreMatcher{}
	}
	var m gitignoreMatcher
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		if p, ok := parseGitignoreLine(scanner.Text()); ok {
			m.patterns = append(m.patterns, p)
		}
	}
	return &m
}

func parseGitignoreLine(line string) (gitignorePattern, bool) {
	raw := strings.TrimSpace(line)
	if raw == "" || strings.HasPrefix(raw, "#") {
		return gitignorePattern{}, false
	}

	p := gitignorePattern{}
	if strings.HasPrefix(raw, "!") {
		p.negate = true
		raw = raw[1:]
	}
	if strings.HasSuffix(raw, "/") {
		p.dirOnly = true
		raw = strings.TrimSuffix(raw, "/")
	}
	if strings.HasPrefix(raw, "/") {
		p.anchored = true
		raw = raw[1:]
	}

	p.starStar = strings.Contains(raw, "**")

	// Extract literal prefix (text before first * / ? / [)
	for i, c := range raw {
		if c == '*' || c == '?' || c == '[' {
			p.literal = raw[:i]
			break
		}
	}
	if !strings.ContainsAny(raw, "*?[") {
		p.literal = raw
	}

	p.raw = raw
	return p, true
}

// ShouldIgnore returns (applies, ignoreLast) — the first bool says whether
// the pattern applies to this path at all (dir-only patterns skip files),
// and the second bool is the matched result adjusted for negation
// (normal match → ignore, negated match → don't ignore).
func (m *gitignoreMatcher) ShouldIgnore(path string, isDir bool) (applies bool, shouldIgnore bool) {
	var matched bool
	for _, p := range m.patterns {
		if p.dirOnly && !isDir {
			continue
		}
		if p.matches(path) {
			matched = true
			shouldIgnore = !p.negate
		}
	}
	return matched, shouldIgnore
}

func (p gitignorePattern) matches(path string) bool {
	target := path
	// Non-anchored pattern without a / matches basename only.
	if !p.anchored && !strings.Contains(p.raw, "/") {
		target = filepath.Base(path)
	}
	if p.starStar {
		return matchWithStarStar(p.raw, target)
	}
	ok, _ := filepath.Match(p.raw, target)
	return ok
}

// matchWithStarStar handles gitignore-style ** patterns.
// It splits the pattern at ** and verifies each segment appears in order.
func matchWithStarStar(pattern, path string) bool {
	parts := strings.Split(pattern, "**")
	remaining := path

	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == 0 {
			// First segment must be a prefix (glob‑compatible)
			if len(remaining) < len(part) {
				return false
			}
			ok, _ := filepath.Match(part, remaining[:len(part)])
			if !ok && !strings.HasPrefix(remaining, part) {
				return false
			}
			remaining = remaining[len(part):]
		} else if i == len(parts)-1 {
			// Last segment must be a suffix
			if len(remaining) < len(part) {
				return false
			}
			suffix := remaining[len(remaining)-len(part):]
			ok, _ := filepath.Match(part, suffix)
			return ok
		} else {
			// Middle segment must appear somewhere
			idx := strings.Index(remaining, part)
			if idx < 0 {
				return false
			}
			remaining = remaining[idx+len(part):]
		}
	}
	return true
}

// matchPattern is a simpler glob match used for AdditionalIgnores and some tests.
// Patterns without a path separator match against basename.
func matchPattern(path, pattern string) bool {
	if !strings.ContainsAny(pattern, "/\\") {
		m, _ := filepath.Match(pattern, filepath.Base(path))
		return m
	}
	m, _ := filepath.Match(pattern, path)
	return m
}

// ============================================================
//  Main Entry
// ============================================================

func main() {
	config := parseFlags()

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
	if config.Verbose {
		printConfigSummary(config)
	}

	if config.All {
		config.NoDefaultIgnore = true
		config.NoGitignore = true
	}

	{
		n := 0
		if config.Copy {
			n++
		}
		if config.Push {
			n++
		}
		if config.ICloud {
			n++
		}
		if config.ShowStats {
			n++
		}
		if n > 1 {
			fmt.Println("⚠  Multiple output modes detected (using priority: clipboard > push > icloud > stats > file)")
		}
	}

	if !config.DryRun {
		fmt.Println("▶ Sourcepack Started")
	} else {
		fmt.Println("▶ Sourcepack Dry-Run Mode")
	}

	startTime := time.Now()

	// 1. Scan and filter
	files, stats, skipped := scanDirectory(config)

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

func initFlags() {
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		pflag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nConvenience shortcuts:")
		fmt.Fprintln(os.Stderr, "  -a              Include all files (disable ignores, no .gitignore)")
		fmt.Fprintln(os.Stderr, "\nCombinations:")
		fmt.Fprintln(os.Stderr, "  -s -p           Show stats and push stats-only markdown to remote")
		fmt.Fprintln(os.Stderr, "  -c -p           Not supported (-c takes priority)")
		fmt.Fprintf(os.Stderr, "\nEnvironment variables:\n")
		fmt.Fprintf(os.Stderr, "  SOURCEPACK_PUSH_URL    Remote URL for push (e.g. https://host/submit)\n")
		fmt.Fprintf(os.Stderr, "  SOURCEPACK_AUTH_KEY    X-Auth-Key for push authentication\n")
	}
}

func parseFlags() Config {
	var c Config

	initFlags()

	pflag.StringVarP(&c.RootDir, "dir", "d", ".", "Root directory to scan")
	pflag.StringVarP(&c.OutputFile, "out", "o", "project_snapshot.md", "Output markdown file")

	var incExts, incMatches, excExts, excMatches, addIgnores string
	pflag.StringVarP(&incExts, "include", "i", "", "Only include these extensions (e.g. go,md)")
	pflag.StringVarP(&incMatches, "match", "m", "", "Only include paths containing these keywords")
	pflag.StringVarP(&excExts, "exclude", "x", "", "Exclude these extensions (e.g. exe,bin)")
	pflag.StringVarP(&excMatches, "exclude-match", "X", "", "Exclude paths containing these keywords")
	pflag.StringVarP(&addIgnores, "ignore", "I", "", "Additional gitignore-style patterns (comma separated)")

	pflag.Int64Var(&c.MaxFileSize, "max-size", 500, "Max file size in KB")
	pflag.BoolVarP(&c.NoSubdirs, "no-subdirs", "n", false, "Do not scan subdirectories")
	pflag.BoolVarP(&c.Verbose, "verbose", "v", false, "Verbose output")
	pflag.BoolVar(&c.Version, "version", false, "Show version")
	pflag.BoolVarP(&c.ShowStats, "stats", "s", false, "Show detailed multi-dimensional statistics")
	pflag.BoolVar(&c.DryRun, "dry-run", false, "Dry run mode (no file write)")
	pflag.BoolVar(&c.NoDefaultIgnore, "no-default-ignore", false, "Disable default ignore rules")
	pflag.BoolVar(&c.NoGitignore, "no-gitignore", false, "Do not load .gitignore")
	pflag.BoolVarP(&c.Copy, "copy", "c", false, "Copy output to clipboard instead of file")
	pflag.BoolVarP(&c.Push, "push", "p", false, "Push output to remote (requires --push-url or SOURCEPACK_PUSH_URL env)")
	pflag.StringVarP(&c.PushURL, "push-url", "u", "", "Remote URL for push (e.g. https://host/submit). Overrides SOURCEPACK_PUSH_URL env")
	pflag.StringVar(&c.AuthKey, "auth-key", "", "X-Auth-Key for push auth (or env SOURCEPACK_AUTH_KEY)")
	pflag.BoolVar(&c.ICloud, "icloud", false, "Save output to iCloud Documents folder")
	pflag.BoolVarP(&c.All, "all", "a", false, "Include all files (disable all ignore rules)")

	pflag.Parse()

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

	return c
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

	var matcher = &gitignoreMatcher{}
	if !config.NoGitignore {
		matcher = loadGitignore(config.RootDir)
	}
	gdMatcher := loadGdignore(config.RootDir)
	matcher.patterns = append(matcher.patterns, gdMatcher.patterns...)

	if config.Verbose {
		if len(matcher.patterns) > 0 {
			fmt.Printf("  Loaded ignore patterns (%d)\n", len(matcher.patterns))
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
				return filepath.SkipDir
			}
			if config.NoSubdirs && strings.Contains(relPath, "/") {
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
		lineCount, isBinary, err := scanFileStats(path, !isKnownTextFile(relPath))
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

		tokens := int(info.Size() / 4)
		fMeta := FileMetadata{RelPath: relPath, FullPath: path, Size: info.Size(), LineCount: lineCount}
		files = append(files, fMeta)

		if config.Verbose {
			fmt.Printf("  + %-40s %d lines\n", relPath, lineCount)
		}

		// Accumulate Stats
		stats.FileCount++
		stats.TotalSize += fMeta.Size
		stats.TotalLines += fMeta.LineCount
		stats.TotalTokens += tokens

		dir := filepath.Dir(relPath)
		if _, ok := stats.DirMap[dir]; !ok {
			stats.DirMap[dir] = &DirStats{Path: dir}
		}
		stats.DirMap[dir].FileCount++
		stats.DirMap[dir].TotalSize += fMeta.Size
		stats.DirMap[dir].TotalLines += fMeta.LineCount

		ext := strings.ToLower(filepath.Ext(relPath))
		if ext == "" {
			ext = "[no ext]"
		}
		if _, ok := stats.ExtMap[ext]; !ok {
			stats.ExtMap[ext] = &ExtStats{Ext: ext}
		}
		stats.ExtMap[ext].FileCount++
		stats.ExtMap[ext].TotalSize += fMeta.Size
		stats.ExtMap[ext].TotalLines += fMeta.LineCount

		return nil
	})
	if walkErr != nil && config.Verbose {
		fmt.Printf("  ! Walk error: %v\n", walkErr)
	}

	return files, stats, skipped
}

func shouldIgnoreDir(relPath string, config Config, matcher *gitignoreMatcher) bool {
	name := filepath.Base(relPath)
	if !config.NoDefaultIgnore && ignoreDirs[name] {
		return true
	}
	for _, p := range config.AdditionalIgnores {
		if matchPattern(relPath, p) {
			return true
		}
	}
	if matcher != nil {
		if applies, ignore := matcher.ShouldIgnore(relPath, true); applies && ignore {
			return true
		}
	}
	return false
}

func shouldIgnoreFile(relPath string, config Config, matcher *gitignoreMatcher) bool {
	name := filepath.Base(relPath)
	ext := filepath.Ext(relPath)
	if !config.NoDefaultIgnore && ignoreFiles[name] {
		return true
	}
	if len(config.IncludeExts) > 0 {
		match := false
		for _, e := range config.IncludeExts {
			if strings.EqualFold(ext, e) {
				match = true
				break
			}
		}
		if !match {
			return true
		}
	}
	if len(config.IncludeMatches) > 0 {
		match := false
		for _, m := range config.IncludeMatches {
			if strings.Contains(relPath, m) {
				match = true
				break
			}
		}
		if !match {
			return true
		}
	}
	for _, e := range config.ExcludeExts {
		if strings.EqualFold(ext, e) {
			return true
		}
	}
	for _, m := range config.ExcludeMatches {
		if strings.Contains(relPath, m) {
			return true
		}
	}
	if matcher != nil {
		if applies, ignore := matcher.ShouldIgnore(relPath, false); applies && ignore {
			return true
		}
	}
	return false
}

func isBinaryBuffer(buf []byte) bool {
	if len(buf) == 0 {
		return false
	}
	if !utf8.Valid(buf) {
		for _, b := range buf {
			if b == 0 {
				return true
			}
		}
	}
	return false
}

// binaryProbeSize 判断二进制时只探测文件头部字节（与 http.DetectContentType 思路一致）。
const binaryProbeSize = 1024

// scanFileStats 流式获取文件的行数，并按需探测头部判断是否二进制。
// 只顺序读一遍文件即可完成两项工作，避免将整个文件读入内存。
func scanFileStats(path string, probeBinary bool) (lineCount int, isBinary bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false, err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	if probeBinary {
		head, perr := reader.Peek(binaryProbeSize)
		if perr != nil && perr != io.EOF {
			return 0, false, perr
		}
		if isBinaryBuffer(head) {
			return 0, true, nil
		}
	}

	buf := make([]byte, 32*1024)
	for {
		n, rerr := reader.Read(buf)
		if n > 0 {
			lineCount += countLinesBuffer(buf[:n])
		}
		if rerr == io.EOF {
			return lineCount, false, nil
		}
		if rerr != nil {
			return 0, false, rerr
		}
	}
}

func isKnownTextFile(relPath string) bool {
	name := filepath.Base(relPath)
	if knownTextFiles[name] {
		return true
	}
	ext := strings.ToLower(filepath.Ext(relPath))
	_, ok := languageMap[ext]
	return ok
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

func printStatsTerminal(files []FileMetadata, stats Stats) {
	fmt.Printf("\n📊 Project Statistics Summary\n")
	fmt.Printf("  %-20s %d\n", "Files Processed:", stats.FileCount)
	fmt.Printf("  %-20s %d\n", "Total Lines:", stats.TotalLines)
	fmt.Printf("  %-20s %.2f KB\n", "Total Size:", float64(stats.TotalSize)/1024)
	fmt.Printf("  %-20s %d\n", "Directories:", stats.DirCount)
	fmt.Printf("  %-20s ~%d tokens\n", "Est. Tokens:", stats.TotalTokens)

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
			continue
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

	return uploadViaMultipart(config, tmpPath, filepath.Base(config.RootDir)+"_stats.md")
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

	return uploadViaMultipart(config, tmpPath, filepath.Base(config.RootDir)+"_snapshot.md")
}
