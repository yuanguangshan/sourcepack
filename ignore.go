package main

// ============================================================
//  ignore 判定（gitignore 语义）
// ============================================================
//
// 这里同时承担四种"排除来源"：
//  1. .gitignore —— 每一层目录的 .gitignore 都叠加生效（嵌套）；
//  2. .gdignore  —— Sourcepack 自己的忽略文件，语法与 .gitignore 相同；
//  3. 内置忽略表 —— 目录名 / 文件名 / 敏感文件；
//  4. 命令行 -I/--ignore 追加的模式。
//
// **为什么自己实现**：.gdignore 是项目自己的机制，git 不认识它；全量扫描还必须
// 在"根本不是 git 仓库"的目录里工作（所以不能全靠 git 给清单）。而旧实现只支持
// 简化 glob，实测 **/*.log、src/**/*.go 这类最常用的写法**全部失效**——README
// 却写着"支持 ** 通配等完整 gitignore 语法"；且它用 filepath.Match，在 Windows
// 上 * 会跨过 /。这里按 git 的规则重写，并以 `git check-ignore` 为真值做对照测试
// （见 ignore_test.go），让"文档承诺的语法"和"真正实现的语义"对齐。
//
// 语义要点（与 git 一致）：
//   - 空行、# 开头为注释；行尾未转义的空格剥掉；\# 与 \! 表示字面量
//   - ! 前缀是负向模式（重新纳入），按"最后一条命中的规则生效"
//   - 末尾 / 表示只匹配目录，其下内容一并忽略
//   - 模式含 /（除末尾）时相对该 ignore 文件所在目录生效；否则匹配任意层级的名字
//   - * 与 ? 不跨过 /；[a-z] 字符组，[!...] 与 [^...] 都表示取反
//   - **/ 匹配零层或多层目录；abc/** 匹配 abc 内部的全部内容（不含 abc 自身）
//
// 优先级（低 → 高）：浅层 .gitignore → 深层 .gitignore → 浅层 .gdignore →
// 深层 .gdignore → 命令行 -I。也就是 .gdignore 永远压过 .gitignore。

import (
	"bufio"
	"bytes"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ignoreRule 是一条已经解析好的 ignore 规则。
type ignoreRule struct {
	negate   bool     // ! 前缀
	dirOnly  bool     // 末尾 /
	anchored bool     // 含 /（除末尾），相对规则所在目录
	segs     []string // 按 / 切开的模式段，"**" 为特殊段
	raw      string   // 去掉修饰符后的模式原文（诊断用）
}

// ignoreMatcher 按目录层级叠加 ignore 文件，并对给定路径给出最终判定。
type ignoreMatcher struct {
	root  string
	names []string // 要读取的 ignore 文件名，按优先级从低到高
	cache map[string][]ignoreRule
	cli   []ignoreRule // -I/--ignore，优先级最高

	// 目录判定结果缓存。遍历是"先目录、后其内容"，所以每个目录只会真正
	// 计算一次；文件查"父目录是否被忽略"时直接命中，避免深目录变成 O(深度²)。
	dirCache map[string]dirVerdict
}

type dirVerdict struct {
	applies bool
	ignored bool
	source  string
}

func newIgnoreMatcher(root string, names ...string) *ignoreMatcher {
	return &ignoreMatcher{
		root:     root,
		names:    names,
		cache:    make(map[string][]ignoreRule),
		dirCache: make(map[string]dirVerdict),
	}
}

// addCLIPatterns 追加命令行模式（相对 root 生效，优先级最高）。
func (m *ignoreMatcher) addCLIPatterns(patterns []string) {
	for _, p := range patterns {
		if r, ok := parseIgnoreLine(p); ok {
			m.cli = append(m.cli, r)
		}
	}
}

// ShouldIgnore 判定一条路径。
//
// 返回 (是否有规则命中, 是否应忽略, 命中的规则来自哪个 ignore 文件)。
// 多条规则命中时"最后一条生效"，与 git 一致；来源字符串用于把
// "被谁排除的"如实告诉用户（-v / git 模式的排除清单）。
func (m *ignoreMatcher) ShouldIgnore(relPath string, isDir bool) (applies, ignored bool, source string) {
	relPath = strings.TrimPrefix(filepath.ToSlash(relPath), "./")
	if isDir {
		v := m.dirVerdict(relPath)
		return v.applies, v.ignored, v.source
	}

	applies, ignored, source = m.decide(relPath, false)
	if ignored {
		return applies, ignored, source
	}
	// 祖先目录被忽略 → 本路径也被忽略，而且**无法用 ! 重新纳入**
	// （git 的原话：不能重新纳入"父目录已被排除"的文件）。
	// 遍历模式靠 SkipDir 达到同样效果，但 git 变更模式没有遍历，必须在这里兜住。
	// 例：.gitignore 里 a/**/b，则 a/b/c 被忽略，因为 a/b 这一级是目录且命中了规则。
	comps := strings.Split(relPath, "/")
	for i := 1; i < len(comps); i++ {
		if v := m.dirVerdict(strings.Join(comps[:i], "/")); v.ignored {
			return true, true, v.source
		}
	}
	return applies, false, source
}

// dirVerdict 判定一个目录是否被忽略（含"祖先目录被忽略"的传递），并缓存。
func (m *ignoreMatcher) dirVerdict(dir string) dirVerdict {
	if v, ok := m.dirCache[dir]; ok {
		return v
	}
	a, ignored, source := m.decide(dir, true)
	if !ignored {
		comps := strings.Split(dir, "/")
		for i := 1; i < len(comps); i++ {
			if parent := m.dirVerdict(strings.Join(comps[:i], "/")); parent.ignored {
				a, ignored, source = true, true, parent.source
				break
			}
		}
	}
	v := dirVerdict{applies: a, ignored: ignored, source: source}
	m.dirCache[dir] = v
	return v
}

// decide 只判定"路径自身"，不考虑祖先目录（祖先由 ShouldIgnore 兜）。
func (m *ignoreMatcher) decide(relPath string, isDir bool) (applies, ignored bool, source string) {
	// 先按"文件类型"（.gitignore 全部走完，再走 .gdignore），再按目录浅→深：
	// 这样 .gdignore 永远压过 .gitignore，而同一种文件里更深目录优先。
	for _, name := range m.names {
		for _, dir := range ancestorDirs(relPath, isDir) {
			for _, r := range m.rulesFor(dir, name) {
				if r.matches(relTo(relPath, dir), isDir) {
					applies = true
					ignored = !r.negate
					source = path.Join(dir, name)
				}
			}
		}
	}
	for _, r := range m.cli {
		if r.matches(relPath, isDir) {
			applies = true
			ignored = !r.negate
			source = "-I/--ignore"
		}
	}
	return applies, ignored, source
}

// MatchCLI 只判定命令行模式（-I），供"先看 -I 再走其它 gate"的顺序使用。
func (m *ignoreMatcher) MatchCLI(relPath string, isDir bool) (matched, ignored bool) {
	relPath = strings.TrimPrefix(filepath.ToSlash(relPath), "./")
	for _, r := range m.cli {
		if r.matches(relPath, isDir) {
			matched = true
			ignored = !r.negate
		}
	}
	return matched, ignored
}

// IgnoreFileCount 返回实际读到的（非空的）ignore 文件数，供 -v 展示。
func (m *ignoreMatcher) IgnoreFileCount() int {
	n := 0
	for _, rules := range m.cache {
		if len(rules) > 0 {
			n++
		}
	}
	return n
}

// rulesFor 读取并缓存某个目录下的某个 ignore 文件。
func (m *ignoreMatcher) rulesFor(dir, name string) []ignoreRule {
	key := dir + "\x00" + name
	if rules, ok := m.cache[key]; ok {
		return rules
	}
	p := name
	if dir != "" {
		p = filepath.Join(filepath.FromSlash(dir), name)
	}
	data, err := os.ReadFile(filepath.Join(m.root, p))
	var rules []ignoreRule
	if err == nil {
		rules = parseIgnoreLines(data)
	}
	m.cache[key] = rules
	return rules
}

// relTo 把 rel 换算成"相对 dir"的路径（dir 为空表示根）。
func relTo(rel, dir string) string {
	if dir == "" {
		return rel
	}
	sub := strings.TrimPrefix(rel, dir)
	sub = strings.TrimPrefix(sub, "/")
	if sub == "" {
		return "."
	}
	return sub
}

// ancestorDirs 返回路径的各层目录（相对 root，浅 → 深，首元素 "" 表示根）。
// isDir 为真时包含路径自身，因为目录自己的 ignore 文件也可能命中它。
func ancestorDirs(rel string, isDir bool) []string {
	comps := strings.Split(rel, "/")
	end := len(comps)
	if !isDir {
		end--
	}
	dirs := make([]string, 0, end+1)
	dirs = append(dirs, "")
	for i := 1; i <= end; i++ {
		dirs = append(dirs, strings.Join(comps[:i], "/"))
	}
	return dirs
}

// parseIgnoreLines 解析一个 ignore 文件的全部规则。
func parseIgnoreLines(data []byte) []ignoreRule {
	var rules []ignoreRule
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if r, ok := parseIgnoreLine(scanner.Text()); ok {
			rules = append(rules, r)
		}
	}
	return rules
}

func parseIgnoreLine(line string) (ignoreRule, bool) {
	line = strings.TrimSuffix(line, "\r") // CRLF 检出的仓库
	if line == "" || strings.HasPrefix(line, "#") {
		return ignoreRule{}, false
	}
	line = trimTrailingUnescapedSpaces(line)
	if line == "" {
		return ignoreRule{}, false
	}

	r := ignoreRule{}
	switch {
	case strings.HasPrefix(line, "\\#"), strings.HasPrefix(line, "\\!"):
		line = line[1:] // 字面量的 # 或 !
	case strings.HasPrefix(line, "!"):
		r.negate = true
		line = line[1:]
	}
	if strings.HasSuffix(line, "/") {
		r.dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}
	if line == "" {
		return ignoreRule{}, false
	}
	if strings.Contains(line, "/") {
		r.anchored = true
		line = strings.TrimPrefix(line, "/")
	}

	r.raw = line
	r.segs = strings.Split(line, "/")
	// abc/** 表示"abc 内部的内容"（不含 abc 自身）：补一个 * 段要求至少一层，
	// 否则 ** 会匹配零层、把 abc 自己也吞掉。**/ 开头则必须允许零层。
	if len(r.segs) > 1 && r.segs[len(r.segs)-1] == "**" {
		r.segs = append(r.segs, "*")
	}
	return r, true
}

// trimTrailingUnescapedSpaces 剥掉行尾空格，但 \  转义的空格保留（git 规则）。
func trimTrailingUnescapedSpaces(s string) string {
	for len(s) > 0 && s[len(s)-1] == ' ' {
		backslashes := 0
		for i := len(s) - 2; i >= 0 && s[i] == '\\'; i-- {
			backslashes++
		}
		if backslashes%2 == 1 {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}

// matches 判断规则是否命中路径。rel 是相对**规则所在目录**的路径。
func (r ignoreRule) matches(rel string, isDir bool) bool {
	if rel == "" || rel == "." {
		return false // 规则不作用于它自己所在的那个目录
	}
	if r.dirOnly {
		// 目录规则命中某级目录即命中：git 里"目录被忽略 → 其中内容一并被忽略"。
		// 遍历时会在目录上 SkipDir，但 git 变更模式没有遍历，必须在这里兜住。
		comps := strings.Split(rel, "/")
		n := len(comps)
		if !isDir {
			n-- // 文件本身不是目录，只看各级父目录
		}
		for i := 1; i <= n; i++ {
			if r.matchPath(strings.Join(comps[:i], "/")) {
				return true
			}
		}
		return false
	}
	return r.matchPath(rel)
}

func (r ignoreRule) matchPath(rel string) bool {
	if r.anchored {
		return matchSegments(r.segs, strings.Split(rel, "/"))
	}
	// 不含 "/" 的模式匹配任意层级的名字
	return matchSegments(r.segs, []string{path.Base(rel)})
}

// matchSegments 逐段匹配，段 "**" 匹配零个或多个段。
func matchSegments(pat, name []string) bool {
	pi, ni := 0, 0
	star, starName := -1, 0
	for ni < len(name) {
		switch {
		case pi < len(pat) && pat[pi] == "**":
			star, starName = pi, ni
			pi++
		case pi < len(pat) && segmentMatch(pat[pi], name[ni]):
			pi++
			ni++
		case star >= 0:
			starName++
			ni = starName
			pi = star + 1
		default:
			return false
		}
	}
	for pi < len(pat) && pat[pi] == "**" {
		pi++
	}
	return pi == len(pat)
}

// segmentMatch 匹配单个路径段（不含 /）。
//
// 用 path.Match 而不是 filepath.Match：后者在 Windows 上把 \ 当分隔符、把 /
// 当普通字符，* 于是会跨过 /（这正是 CI 在 windows-latest 上抓到的差异）。
func segmentMatch(pat, name string) bool {
	if pat == "*" {
		return true
	}
	// git 用 [!...] 表示取反，Go 的 path.Match 用 [^...]
	p := strings.ReplaceAll(pat, "[!", "[^")
	ok, err := path.Match(p, name)
	return err == nil && ok
}

// ============================================================
//  内置忽略表与敏感文件
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

var sensitiveFileExts = map[string]bool{
	".pem": true, ".key": true, ".p12": true, ".pfx": true,
	".jks": true, ".keystore": true,
}

// isSensitiveFile 按文件名判断敏感文件（.env、私钥、凭据等）默认排除。
//
// 注意：这里只看文件名，不看内容——写在 config.go / docker-compose.yml 里的
// token 不会被拦住（见 TODO.md 的"内容级密钥扫描"）。
func isSensitiveFile(name string) bool {
	lower := strings.ToLower(name)
	// .env 模板文件按约定不含真值，而它们恰恰是 LLM 理解"这个项目需要哪些
	// 配置项"最有用的文件，不能和真 .env 一起被当敏感文件杀掉
	switch lower {
	case ".env.example", ".env.sample", ".env.template", ".env.dist":
		return false
	}
	// .env 与 .env.* 变体（.env.local、.env.production 等）
	if lower == ".env" || strings.HasPrefix(lower, ".env.") {
		return true
	}
	switch lower {
	case "id_rsa", "id_dsa", "id_ecdsa", "id_ed25519",
		".npmrc", ".netrc", ".git-credentials",
		"credentials.json", "serviceaccountkey.json",
		"secrets.yaml", "secrets.yml", "secrets.json", "secrets.toml":
		return true
	}
	if ext := filepath.Ext(lower); sensitiveFileExts[ext] {
		return true
	}
	return false
}

// ============================================================
//  排除判定
// ============================================================

func shouldIgnoreDir(relPath string, config Config, matcher *ignoreMatcher) bool {
	name := filepath.Base(relPath)
	if !config.NoDefaultIgnore && ignoreDirs[name] {
		return true
	}
	if matcher != nil {
		if _, ignore := matcher.MatchCLI(relPath, true); ignore {
			return true
		}
		if _, ignore, _ := matcher.ShouldIgnore(relPath, true); ignore {
			return true
		}
	}
	return false
}

func shouldIgnoreFile(relPath string, config Config, matcher *ignoreMatcher) bool {
	return shouldIgnoreFileReason(relPath, config, matcher) != ""
}

// shouldIgnoreFileReason 与 shouldIgnoreFile 判定完全一致，但返回被排除的
// 原因（"" 表示保留）。git 变更模式需要把"被谁过滤掉了"如实列出来，
// 否则用户会以为变更丢了。
func shouldIgnoreFileReason(relPath string, config Config, matcher *ignoreMatcher) string {
	name := filepath.Base(relPath)
	ext := filepath.Ext(relPath)
	if !config.NoDefaultIgnore && ignoreFiles[name] {
		return "Default ignore file"
	}
	// 敏感文件（.env、私钥、凭据）默认一律排除，防止快照/推送泄露密钥
	if !config.NoDefaultIgnore && isSensitiveFile(name) {
		return "Sensitive file"
	}
	// 命令行 -I/--ignore：优先级最高，且命中就返回（CLI 明确要求排除）
	if matcher != nil {
		if matched, ignore := matcher.MatchCLI(relPath, false); matched && ignore {
			return "Ignored by -I/--ignore"
		}
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
			return "Not in -i/--include"
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
			return "Not in -m/--match"
		}
	}
	for _, e := range config.ExcludeExts {
		if strings.EqualFold(ext, e) {
			return "Excluded by -x/--exclude"
		}
	}
	for _, m := range config.ExcludeMatches {
		if strings.Contains(relPath, m) {
			return "Excluded by -X/--exclude-match"
		}
	}
	if matcher != nil {
		if applies, ignore, source := matcher.ShouldIgnore(relPath, false); applies && ignore {
			return "Ignored by " + source
		}
	}
	return ""
}
