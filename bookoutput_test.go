package main

import (
	"bytes"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/spf13/pflag"
)

// ============================================================
//  书排版（--book）测试
// ============================================================

// bookFixture 造一棵"什么样的文件都有"的小项目：带 H1 的散文、没有 H1 的
// Markdown、多级子目录、代码文件，以及一段"代码块里以 # 开头"的内容。
func bookFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "src", "deep"))
	mustMkdir(t, filepath.Join(root, "docs"))
	mustWrite(t, filepath.Join(root, "README.md"),
		"# 说明\n\n> 引言\n\n## 第一节\n\n正文\n\n### 小节\n\n```bash\n# 这是代码里的注释，不是标题\nls\n```\n")
	mustWrite(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}\n")
	mustWrite(t, filepath.Join(root, "src", "util.go"), "package src\n")
	mustWrite(t, filepath.Join(root, "src", "deep", "a.py"), "print('x')\n")
	mustWrite(t, filepath.Join(root, "docs", "中文说明.md"), "中文内容\n")
	return root
}

func renderBook(t *testing.T, root string) string {
	t.Helper()
	cfg := Config{RootDir: root, OutputFile: "project_book.md", MaxFileSize: 500 * 1024, Book: true}
	files, stats, _ := scanDirectory(cfg)
	var buf bytes.Buffer
	if err := writeBookContent(cfg, files, stats, &buf); err != nil {
		t.Fatalf("writeBookContent: %v", err)
	}
	return buf.String()
}

var (
	bookAnchorRe = regexp.MustCompile(`<a id="([^"]+)"></a>`)
	bookLinkRe   = regexp.MustCompile(`\]\(#([^)]+)\)`)
)

// assertBookAnchorsResolve 钉住两件事：锚点不重复、每个目录链接都有落点。
//
// 书排版全靠显式 <a id> + `](#id)` 跳转，一旦章号/锚点生成错位，读者点目录
// 只会跳到文首——而这种坏掉是"看起来正常"的，必须靠测试守。
// 注意：这里要求正文里没有别的 `](#…)` 链接，fixture 已按此构造。
func assertBookAnchorsResolve(t *testing.T, out string) {
	t.Helper()
	ids := map[string]int{}
	for _, m := range bookAnchorRe.FindAllStringSubmatch(out, -1) {
		ids[m[1]]++
	}
	if len(ids) == 0 {
		t.Fatal("书排版里没有任何 <a id> 锚点")
	}
	for id, n := range ids {
		if n > 1 {
			t.Errorf("锚点重复：%s ×%d（目录会跳到第一个，后面的章无法直达）", id, n)
		}
	}
	links := bookLinkRe.FindAllStringSubmatch(out, -1)
	if len(links) == 0 {
		t.Fatal("书排版里没有任何目录链接")
	}
	for _, m := range links {
		if ids[m[1]] == 0 {
			t.Errorf("目录链接指向不存在的锚点：%s", m[1])
		}
	}
}

func TestWriteBookContentStructure(t *testing.T) {
	root := bookFixture(t)
	out := renderBook(t, root)
	base := filepath.Base(root)

	for _, want := range []string{
		"<!--", "个文件 ·", // 顶部溯源注释
		`<a id="book-title"></a>`,
		"# " + base,
		"## 可以直接读的完整快照",
		"## 关于本书",
		"| **书名** | " + base + " |",
		"| **主要语言** |",
		"## 目录",
		// 部 = H1，章 = H2
		"# 第 1 部　根目录",
		"## 说明", // README.md 的 H1 成了章标题
		"## `main.go`",
		"# 第 2 部　`docs/`",
		"## `docs/中文说明.md`",
		"# 第 3 部　`src/`",
		"## `src/deep/a.py`",
		"## `src/util.go`",
		"# 附录 A　目录结构",
		"# 附录 B　统计",
		"# 附录 C　打包参数",
		"```go",
		"package main",
		"```python",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("书排版缺少 %q\n----\n%s", want, out)
		}
	}

	// Markdown 章：标题来自正文 H1，正文标题下沉一级（H2→H3、H3→H4），
	// 且代码块里的 "# 注释" 必须原样保留（不能被当成标题下沉）。
	for _, want := range []string{"### 第一节", "#### 小节", "# 这是代码里的注释，不是标题"} {
		if !strings.Contains(out, want) {
			t.Errorf("Markdown 正文排版不对，缺少 %q\n----\n%s", want, out)
		}
	}
	if strings.Contains(out, "```markdown") {
		t.Errorf("Markdown 文件不该被塞进代码块：\n%s", out)
	}

	// 章前那行说明：代码章不重复写路径（标题就是路径），Markdown 章要写路径
	if !regexp.MustCompile("> 📄 Go · \\d+ 行 · [\\d.]+ (B|KB|MB) · 最后修改 \\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}").MatchString(out) {
		t.Errorf("代码章首说明格式不对\n----\n%s", out)
	}
	if !regexp.MustCompile("> 📄 Markdown · \\d+ 行 · [\\d.]+ (B|KB|MB) · `README.md` · 最后修改 ").MatchString(out) {
		t.Errorf("Markdown 章首说明应带上文件路径\n----\n%s", out)
	}

	if !strings.Contains(out, "> 直接放在项目根目录下的 2 个文件。") {
		t.Errorf("第 1 部导语不对\n----\n%s", out)
	}

	// 目录里部/章齐全：3 部 + 5 章 + 3 个附录
	if got, want := strings.Count(out, "\n- [")+strings.Count(out, "\n  - ["), 11; got != want {
		t.Errorf("目录条目数 = %d, want %d", got, want)
	}

	assertBookAnchorsResolve(t, out)

	// 产物里不能出现本机绝对路径（仓库对"绝对路径泄漏"有明确要求）
	if strings.Contains(out, root) {
		t.Errorf("产物里泄漏了绝对路径 %s", root)
	}
	if !utf8.ValidString(out) {
		t.Error("产物不是合法 UTF-8")
	}
	if strings.ContainsRune(out, 0) {
		t.Error("产物含 NUL 字节")
	}
}

// Markdown 按正文排版，其余文件仍是代码块（含围栏升级）。
func TestBookInlinesMarkdownButNotOtherFiles(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "article.md"), "# 我的文章\n\n## 第一节\n\n正文\n")
	mustWrite(t, filepath.Join(root, "notes.txt"), "# 这只是文本里的井号\n")
	out := renderBook(t, root)

	if n := strings.Count(out, "\n## 我的文章\n"); n != 1 {
		t.Errorf("文章 H1 应恰好作为章标题出现一次，实际 %d 次\n%s", n, out)
	}
	if !strings.Contains(out, "### 第一节") {
		t.Errorf("正文小节应下沉到 H3：\n%s", out)
	}
	// 非 Markdown：保持代码块原样，不参与标题体系
	if !strings.Contains(out, "```\n# 这只是文本里的井号") {
		t.Errorf(".txt 应保持代码块：\n%s", out)
	}
	if strings.Contains(out, "## 这只是文本里的井号") {
		t.Errorf(".txt 的内容不该变成标题：\n%s", out)
	}
}

// 正文里带 ``` 的文件，围栏必须自动加长，否则 Markdown 结构会被截断。
func TestBookFenceEscalation(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "code.txt"), "正文\n\n```\ninner\n```\n")
	out := renderBook(t, root)
	if !strings.Contains(out, "````\n") {
		t.Errorf("含 ``` 的文件未升级围栏：\n%s", out)
	}
}

func TestBookEmptyPackageIsHonest(t *testing.T) {
	root := t.TempDir()
	out := renderBook(t, root)
	if !strings.Contains(out, "没有纳入任何文件") {
		t.Errorf("空包应在封面上说明：\n%s", out)
	}
	if !strings.Contains(out, "（没有文件被纳入，正文为空。）") {
		t.Errorf("空包的目录应如实说明：\n%s", out)
	}
}

// 分组与编号：根目录永远第一部，其余按目录名升序；章序全书连续；
// 每个部内部按路径升序（这样目录里的顺序和正文一致）。
func TestBuildBookPartsGroupingAndNumbering(t *testing.T) {
	files := []FileMetadata{
		{RelPath: "z.txt"},
		{RelPath: "src/b.go"},
		{RelPath: "a.go"},
		{RelPath: "docs/x.md"},
		{RelPath: "src/deep/c.go"},
	}
	parts := buildBookParts(files)

	if len(parts) != 3 {
		t.Fatalf("应分成 3 部，实际 %d：%+v", len(parts), parts)
	}
	if parts[0].title != "根目录" || parts[1].title != "`docs/`" || parts[2].title != "`src/`" {
		t.Errorf("部的顺序/标题不对：%q %q %q", parts[0].title, parts[1].title, parts[2].title)
	}
	if parts[0].anchor != "part-1" || parts[2].anchor != "part-3" {
		t.Errorf("部锚点不对：%q %q", parts[0].anchor, parts[2].anchor)
	}
	if got := parts[2].chapters[0].relPath; got != "src/b.go" {
		t.Errorf("src 部第一个文件应为 src/b.go（路径升序），实际 %s", got)
	}

	// 没有正文可读（FullPath 为空）时，章标题退化成路径
	if got := parts[0].chapters[0].displayTitle(); got != "`a.go`" {
		t.Errorf("无 H1 的章标题应为路径，实际 %q", got)
	}

	wantOrder := []string{"a.go", "z.txt", "docs/x.md", "src/b.go", "src/deep/c.go"}
	var gotOrder []string
	seen := map[string]bool{}
	for _, p := range parts {
		for _, ch := range p.chapters {
			gotOrder = append(gotOrder, ch.relPath)
			if ch.no != len(gotOrder) {
				t.Errorf("章序应全书连续：%s 是第 %d 个，却编成第 %d", ch.relPath, len(gotOrder), ch.no)
			}
			if seen[ch.anchor] {
				t.Errorf("章锚点重复：%s", ch.anchor)
			}
			seen[ch.anchor] = true
		}
	}
	if strings.Join(gotOrder, ",") != strings.Join(wantOrder, ",") {
		t.Errorf("章顺序 = %v, want %v", gotOrder, wantOrder)
	}
}

func TestSplitMarkdownTitle(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantTitle string
		wantBody  string
	}{
		{"普通 H1", "# 标题\n\n正文\n", "标题", "正文\n"},
		{"前置 HTML 注释", "<!-- 溯源 -->\n\n# 标题\n正文\n", "标题", "正文\n"},
		{"YAML front matter", "---\ntitle: x\ntags: [a]\n---\n\n# 标题\n正文\n", "标题", "正文\n"},
		{"开头的水平线不是 front matter", "---\n正文\n", "", "---\n正文\n"},
		{"没有 H1", "正文\n## 小节\n", "", "正文\n## 小节\n"},
		{"只有 H1", "# 标题\n", "标题", ""},
		{"H2 开头不算章标题", "## 小节\n正文\n", "", "## 小节\n正文\n"},
		{"# 后没有空格不是标题", "#标题\n", "", "#标题\n"},
		{"空文件", "", "", ""},
	}
	for _, tt := range tests {
		title, body := splitMarkdownTitle(tt.in)
		if title != tt.wantTitle || body != tt.wantBody {
			t.Errorf("%s: splitMarkdownTitle(%q) = (%q, %q), want (%q, %q)",
				tt.name, tt.in, title, body, tt.wantTitle, tt.wantBody)
		}
	}
}

func TestDemoteHeadings(t *testing.T) {
	in := "## B\n\n```bash\n# 注释不是标题\n## 也不是\n```\n\n~~~\n# 同样不是\n~~~\n\n### C\n"
	want := "### B\n\n```bash\n# 注释不是标题\n## 也不是\n```\n\n~~~\n# 同样不是\n~~~\n\n#### C\n"
	if got := demoteHeadings(in, 3); got != want {
		t.Errorf("demoteHeadings:\n got %q\nwant %q", got, want)
	}

	// 只下沉、不上浮：最小级别已经比目标深时原样返回
	deep := "#### D\n\n##### E\n"
	if got := demoteHeadings(deep, 3); got != deep {
		t.Errorf("不该把深标题上浮：%q", got)
	}
	// 封顶 H6
	if got := demoteHeadings("### X\n", 6); got != "###### X\n" {
		t.Errorf("下沉应封顶在 H6，实际 %q", got)
	}
	// 没有标题时原样返回
	plain := "正文\n\n```\n# x\n```\n"
	if got := demoteHeadings(plain, 3); got != plain {
		t.Errorf("没有标题时不该改动：%q", got)
	}
}

// -book / --book 都要能用：用户敲的就是单横线写法，而 pflag 默认只认 --book
// （单横线会被当成短参数簇，要么报 unknown shorthand，要么静默解析成别的参数）。
func TestNormalizeSingleDashLongFlags(t *testing.T) {
	tests := []struct {
		arg  string
		want string
	}{
		{"-book", "--book"}, // 本次需求的主角
		{"-book=false", "--book=false"},
		{"-dir=src", "--dir=src"}, // 带等号的长参数
		{"-dry-run", "--dry-run"}, // 旧行为是 -d "ry-run"（静默错误）
		{"-stats", "--stats"},     // 旧行为是 unknown shorthand 't'
		{"--book", "--book"},      // 双横线原样
		{"-s", "-s"},              // 单字符短参数
		{"-sp", "-sp"},            // 真正的短参数簇：不是长参数名
		{"-x", "-x"},
		{"-", "-"},                     // stdin 约定
		{"-nosuchflag", "-nosuchflag"}, // 不认识的名字不改写，交给 pflag 报错
		{"plain", "plain"},
	}
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	initFlags(fs)
	// 注册几个代表性的参数，让 Lookup 有东西可查
	var book, dryRun, statsFlag bool
	var dir, out string
	fs.BoolVar(&book, "book", false, "")
	fs.BoolVar(&dryRun, "dry-run", false, "")
	fs.BoolVar(&statsFlag, "stats", false, "")
	fs.StringVar(&dir, "dir", ".", "")
	fs.StringVar(&out, "out", "", "")

	for _, tt := range tests {
		got := normalizeSingleDashLongFlags(fs, []string{tt.arg})
		if len(got) != 1 || got[0] != tt.want {
			t.Errorf("normalize(%q) = %v, want [%s]", tt.arg, got, tt.want)
		}
	}

	// "--" 之后是位置参数，一个都不许动
	got := normalizeSingleDashLongFlags(fs, []string{"--", "-book", "-dir"})
	if strings.Join(got, " ") != "-- -book -dir" {
		t.Errorf(`"--" 之后不应改写，实际 %v`, got)
	}
}

func TestParseFlagsBookDefaults(t *testing.T) {
	c := parseFlagsInto(newTestFlagSet(), []string{"-book"})
	if !c.Book {
		t.Fatalf("-book 未开启书排版: %+v", c)
	}
	if c.OutputFile != "project_book.md" {
		t.Errorf("书排版默认输出名应为 project_book.md，实际 %q", c.OutputFile)
	}
	if err := validateConfig(c, newTestFlagSet()); err != nil {
		t.Errorf("默认组合不该报错: %v", err)
	}

	// 显式 -o 优先
	c = parseFlagsInto(newTestFlagSet(), []string{"--book", "-o", "my-book.md"})
	if c.OutputFile != "my-book.md" {
		t.Errorf("显式 -o 应覆盖默认名，实际 %q", c.OutputFile)
	}

	// 不写 -book 时默认名不变（默认快照与书排版互不覆盖）
	c = parseFlagsInto(newTestFlagSet(), []string{"-i", "go"})
	if c.Book || c.OutputFile != "project_snapshot.md" {
		t.Errorf("非书排版模式被改动: book=%v out=%q", c.Book, c.OutputFile)
	}
}

// --book 只对全量快照有意义；-g/-G 的产物是变更集，套书版式只会得到四不像。
func TestValidateConfigRejectsBookWithGitMode(t *testing.T) {
	for _, args := range [][]string{{"-g", "-book"}, {"-G", "3", "--book"}} {
		fs := newTestFlagSet()
		c := parseFlagsInto(fs, args)
		err := validateConfig(c, fs)
		if err == nil || !strings.Contains(err.Error(), "--book") {
			t.Errorf("%v 应明确报错，实际 err=%v", args, err)
		}
	}
}

func TestArtifactBaseNameBookEdition(t *testing.T) {
	if got := artifactBaseName(Config{RootDir: "/tmp/some-repo", Book: true}); got != "some-repo_book" {
		t.Errorf("书排版的推送前缀 = %q, want some-repo_book", got)
	}
}

func TestHumanIntAndHumanSize(t *testing.T) {
	ints := map[int]string{0: "0", 7: "7", 999: "999", 1000: "1,000", 12345: "12,345", 1234567: "1,234,567", -4321: "-4,321"}
	for in, want := range ints {
		if got := humanInt(in); got != want {
			t.Errorf("humanInt(%d) = %q, want %q", in, got, want)
		}
	}
	sizes := map[int64]string{0: "0 B", 512: "512 B", 2048: "2.0 KB", 5 * 1024 * 1024: "5.0 MB"}
	for in, want := range sizes {
		if got := humanSize(in); got != want {
			t.Errorf("humanSize(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestLanguageLabel(t *testing.T) {
	tests := map[string]string{
		"a.go":         "Go",
		"a.md":         "Markdown",
		"a.py":         "Python",
		"a.sh":         "Shell",
		"a.cpp":        "C++",
		"a.yaml":       "YAML",
		"a.unknownext": "UNKNOWNEXT",
		"LICENSE":      "纯文本",
		"Dockerfile":   "Dockerfile",
		"docs/中文说明.md": "Markdown",
	}
	for in, want := range tests {
		if got := languageLabel(in); got != want {
			t.Errorf("languageLabel(%q) = %q, want %q", in, got, want)
		}
	}
	if got := languageLabelForExt("[no ext]"); got != "无扩展名" {
		t.Errorf("无扩展名应显示为「无扩展名」，实际 %q", got)
	}
}

// 书排版走的是同一批 files/stats，因此 -c（剪贴板）与 -p（推送）拿到的
// 必须是书排版的内容，而不是默认快照。这里用 writeContent 的分发直接钉住。
func TestWriteContentDispatchesBook(t *testing.T) {
	root := bookFixture(t)
	cfg := Config{RootDir: root, OutputFile: "project_book.md", MaxFileSize: 500 * 1024, Book: true}
	files, stats, _ := scanDirectory(cfg)

	var buf bytes.Buffer
	if err := writeContent(cfg, files, stats, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "可以直接读的完整快照") {
		t.Error("--book 没有走书排版")
	}
	if strings.Contains(out, "# Project Snapshot:") {
		t.Error("--book 不应同时输出默认快照版式")
	}
}

// 书名可覆盖：默认用目录名，--book-title 给了就用给的。
func TestBookTitleOverride(t *testing.T) {
	root := bookFixture(t)
	base := filepath.Base(root)

	render := func(cfg Config) string {
		t.Helper()
		files, stats, _ := scanDirectory(cfg)
		var buf bytes.Buffer
		if err := writeBookContent(cfg, files, stats, &buf); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}

	custom := render(Config{RootDir: root, MaxFileSize: 500 * 1024, Book: true, BookTitle: "《测试书》"})
	for _, want := range []string{
		"<!-- 《测试书》 ·",
		"\n# 《测试书》\n",
		"| **书名** | 《测试书》 |",
		"| **根目录** | `" + base + "` |",
	} {
		if !strings.Contains(custom, want) {
			t.Errorf("自定义书名缺少 %q\n----\n%s", want, custom)
		}
	}

	fallback := render(Config{RootDir: root, MaxFileSize: 500 * 1024, Book: true})
	if !strings.Contains(fallback, "\n# "+base+"\n") {
		t.Errorf("未给书名时应回落到目录名\n----\n%s", fallback)
	}
}

func TestValidateConfigRejectsBookTitleWithoutBook(t *testing.T) {
	fs := newTestFlagSet()
	c := parseFlagsInto(fs, []string{"--book-title", "x"})
	if err := validateConfig(c, fs); err == nil || !strings.Contains(err.Error(), "--book-title") {
		t.Errorf("--book-title 不给 --book 应明确报错，实际 err=%v", err)
	}
}

// 附录里的"复现命令"必须真的能复现（只影响内容的参数都还原），
// 且不能带出本机绝对路径。
func TestBookCommandLine(t *testing.T) {
	cfg := Config{
		RootDir: "/tmp/my-book", Book: true, BookTitle: "《X》",
		IncludeExts: []string{".md"}, AdditionalIgnores: []string{"**/*.log"},
		MaxFileSize: 1024 * 1024, NoGitignore: true,
	}
	got := bookCommandLine(cfg)
	for _, want := range []string{
		"cd my-book &&", "sourcepack --book", "--book-title '《X》'",
		"-i md", "-I '**/*.log'", "--max-size 1024", "--no-gitignore",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("复现命令缺少 %q：%s", want, got)
		}
	}
	if strings.Contains(got, "/tmp") {
		t.Errorf("复现命令泄漏了绝对路径：%s", got)
	}

	if plain := bookCommandLine(Config{RootDir: "/tmp/x", Book: true, MaxFileSize: 500 * 1024}); plain != "cd x && sourcepack --book" {
		t.Errorf("默认参数不该出现在命令里，实际 %q", plain)
	}
}

// 统计表的排序必须确定：并列项（行数相同）以前会随运行给出不同顺序。
func TestSortedStatsAreDeterministic(t *testing.T) {
	stats := Stats{
		TotalLines: 4,
		ExtMap: map[string]*ExtStats{
			".py":   {Ext: ".py", TotalLines: 1},
			".txt":  {Ext: ".txt", TotalLines: 1},
			".go":   {Ext: ".go", TotalLines: 2},
			".json": {Ext: ".json", TotalLines: 1},
		},
		DirMap: map[string]*DirStats{
			"src": {Path: "src", TotalLines: 2},
			"doc": {Path: "doc", TotalLines: 2},
		},
	}
	for i := 0; i < 20; i++ {
		var got []string
		for _, e := range sortedExtStats(stats) {
			got = append(got, e.Ext)
		}
		if strings.Join(got, ",") != ".go,.json,.py,.txt" {
			t.Fatalf("扩展名排序不确定：%v", got)
		}
	}
	var dirs []string
	for _, d := range sortedDirStats(stats) {
		dirs = append(dirs, d.Path)
	}
	if strings.Join(dirs, ",") != "doc,src" {
		t.Errorf("目录排序应为路径升序：%v", dirs)
	}
}

func TestShellQuoteIfNeeded(t *testing.T) {
	tests := map[string]string{
		"md":       "md",
		"go,md":    "go,md",
		"src/*.go": "'src/*.go'",
		"**/*.log": "'**/*.log'",
		"a b":      "'a b'",
		"":         "",
	}
	for in, want := range tests {
		if got := shellQuoteIfNeeded(in); got != want {
			t.Errorf("shellQuoteIfNeeded(%q) = %q, want %q", in, got, want)
		}
	}
}
