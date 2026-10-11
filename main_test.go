package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		path    string
		pattern string
		want    bool
	}{
		// Basename matching (no path separator in pattern)
		{"snapshot.go", "snapshot.go", true},
		{"snapshot.go", "snapshot", false}, // exact basename match only, no prefix
		{"src/snapshot.go", "snapshot.go", true},
		{"src/snapshot.go", "snapshot", false},
		{"foo.go", "*.go", true},
		{"foo.txt", "*.go", false},
		{"README.md", "*.md", true},

		// Wildcard patterns
		{"test.txt", "test.*", true},
		{"test.go", "test.*", true},
		{"main.go", "test.*", false},

		// Path-based patterns (contain /)
		{"src/main.go", "src/*.go", true},
		{"src/main.go", "pkg/*.go", false},
		{"src/sub/main.go", "src/*.go", false}, // filepath.Match does not match nested
	}

	for _, tt := range tests {
		got := matchPattern(tt.path, tt.pattern)
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.path, tt.pattern, got, tt.want)
		}
	}
}

func TestScanBackticksInData(t *testing.T) {
	tests := []struct {
		content string
		want    int
	}{
		{"hello world", 0},
		{"no backticks here", 0},
		{"some `inline` code", 1},
		{"``double``", 2},
		{"```code block```", 3},
		{"````four````", 4},
		{"``````", 6},
	}
	for _, tt := range tests {
		got := scanBackticksInData([]byte(tt.content))
		if got != tt.want {
			t.Errorf("scanBackticksInData(%q) = %d, want %d", tt.content, got, tt.want)
		}
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"main.go", "go"},
		{"app.ts", "typescript"},
		{"index.js", "javascript"},
		{"style.css", "css"},
		{"README.md", "markdown"},
		{"config.yaml", "yaml"},
		{"Dockerfile", "dockerfile"},
		{"Makefile", "makefile"},
		{"unknown.xyz", ""},
	}
	for _, tt := range tests {
		got := detectLanguage(tt.path)
		if got != tt.want {
			t.Errorf("detectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestIsBinaryBuffer(t *testing.T) {
	// Text file
	if isBinaryBuffer([]byte("hello world\nline 2\n")) {
		t.Error("text buffer should not be detected as binary")
	}

	// Binary buffer: invalid UTF-8 containing NULL byte
	if !isBinaryBuffer([]byte("\xff\x00\xfe\x00")) {
		t.Error("buffer with invalid UTF-8 and NULL bytes should be detected as binary")
	}

	// Empty buffer
	if isBinaryBuffer([]byte{}) {
		t.Error("empty buffer should not be detected as binary")
	}

	// 本版收紧：非法 UTF-8（无 NUL）也算二进制，否则裸字节会写进 Markdown
	if !isBinaryBuffer([]byte("\x80\x81\x82\xff")) {
		t.Error("invalid UTF-8 without NUL should be detected as binary")
	}

	// 本版收紧：合法 UTF-8 + NUL。旧实现只在"非法 UTF-8"时才去找 NUL，
	// 于是 "abc\x00def" 被当成文本，裸 NUL 直接进了产物。
	if !isBinaryBuffer([]byte("abc\x00def\n")) {
		t.Error("valid UTF-8 containing a NUL should be detected as binary")
	}

	// 中文等多字节 UTF-8 不能被误判
	if isBinaryBuffer([]byte("中文内容\n第二行\n")) {
		t.Error("UTF-8 中文不该被判成二进制")
	}
}

// git 会为"前 8000 字节干净、后面才有 NUL"或无 NUL 的非法 UTF-8 文件
// 输出逐行文本 diff，正文里是原始字节——这层安全网负责把它们挡在产物外。
func TestDiffBodyLooksBinary(t *testing.T) {
	if diffBodyLooksBinary("@@ -1 +1 @@\n-a\n+b\n") {
		t.Error("普通 diff 正文不该被判成二进制")
	}
	if !diffBodyLooksBinary("@@ -1 +1 @@\n-\x00\n+\x01\n") {
		t.Error("含 NUL 的 diff 正文应判为二进制")
	}
	if !diffBodyLooksBinary("@@ -1 +1 @@\n-\x80\x81\n+\xff\n") {
		t.Error("含非法 UTF-8 的 diff 正文应判为二进制")
	}
}

// 扩展名不能作为"内容是文本"的保证：一个含 NUL 字节的 .md 会让整个
// 快照变成非法 UTF-8。这条守的是"一律按内容探测，不看扩展名"。
func TestScanDirectorySkipsBinaryByContentNotExtension(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "ok.md"), "纯文本\n第二行\n")
	mustWrite(t, filepath.Join(root, "stray_nul.md"), "text\nstray\x00nul\n")
	mustWrite(t, filepath.Join(root, "bad.go"), "package main\n\x80\x81\n")

	cfg := Config{RootDir: root, OutputFile: "out.md", MaxFileSize: 500 * 1024}
	files, _, skipped := scanDirectory(cfg)

	if len(files) != 1 || files[0].RelPath != "ok.md" {
		t.Errorf("只应留下纯文本文件，实际 %+v", files)
	}
	reasons := map[string]string{}
	for _, sk := range skipped {
		reasons[sk.RelPath] = sk.Reason
	}
	for _, p := range []string{"stray_nul.md", "bad.go"} {
		if reasons[p] != "Binary file" {
			t.Errorf("%s 应因内容被判为二进制，实际 reason=%q", p, reasons[p])
		}
	}
}

// 英文/代码按 4 字节 1 token；CJK 按 1 字 1 token（旧实现一律 size/4，
// 对纯中文只有 0.75 token/字，明显偏低）。
func TestCountTokensBuffer(t *testing.T) {
	if got := countTokensBuffer([]byte(strings.Repeat("a", 400))); got != 100 {
		t.Errorf("400 字节英文应约 100 token，实际 %d", got)
	}
	cn := strings.Repeat("中", 100) // 300 字节 = 100 个汉字
	if got := countTokensBuffer([]byte(cn)); got != 100 {
		t.Errorf("100 个汉字应约 100 token，实际 %d", got)
	}
	mixed := strings.Repeat("中", 50) + strings.Repeat("a", 100)
	if got := countTokensBuffer([]byte(mixed)); got != 75 {
		t.Errorf("50 汉字 + 100 字节英文 = 75 token，实际 %d", got)
	}
	if countTokensBuffer([]byte(cn)) <= len(cn)/4 {
		t.Error("中文估算必须明显高于旧的 size/4")
	}
	if got := countTokensBuffer(nil); got != 0 {
		t.Errorf("空内容应为 0，实际 %d", got)
	}
}

// 分块读取时末尾可能截在多字节字符中间，那不是"内容非法"。
func TestIncompleteUTF8Tail(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want int
	}{
		{"纯 ASCII", []byte("abc"), 0},
		{"完整汉字", []byte("中文"), 0},
		{"汉字被截断 2 字节", []byte{0xE4, 0xB8}, 2},
		{"汉字被截断 1 字节", []byte{0xE4}, 1},
		{"emoji 被截断 3 字节", []byte{0xF0, 0x9F, 0x98}, 3},
		{"真非法字节（不是截断）", []byte("a\xff"), 0},
		{"结尾是续字节但前面没有起始字节", []byte("a\x80\x80\x80\x80"), 0},
	}
	for _, c := range cases {
		if got := incompleteUTF8Tail(c.in); got != c.want {
			t.Errorf("%s: incompleteUTF8Tail = %d，期望 %d", c.name, got, c.want)
		}
	}
}

// 回归：按字节截断的窗口会把"切在多字节字符中间"的合法 UTF-8 判成非法，
// 于是任何 >8KB 且含中文的文件都会被误杀成二进制（实测 11KB 的 .go 中招）。
func TestScanFileStatsLargeUTF8FileIsNotBinary(t *testing.T) {
	p := filepath.Join(t.TempDir(), "big_cn.go")
	// 开头放一长串连续汉字：8000 % 3 == 2，保证"前 8000 字节"这个旧窗口
	// 必定切在一个汉字的中间（如果只用整行文本，8000 有可能正好落在行首/ASCII 边界，
	// 那样测不出这个 bug——第一版夹具就踩了这个坑）
	content := strings.Repeat("中", 3000) + "\n" + strings.Repeat("// 中文注释说明这个函数做什么\n", 200)
	mustWrite(t, p, content)
	if len(content) <= 8000 {
		t.Fatalf("夹具需超过 8000 字节才有意义，实际 %d", len(content))
	}
	if len(content[:8000])%3 == 0 {
		t.Log("提示：截断点恰好落在字符边界，本用例对该路径不敏感")
	}

	lineCount, isBinary, tokens, err := scanFileStats(p, true)
	if err != nil {
		t.Fatal(err)
	}
	if isBinary {
		t.Error("含中文的大文件不该被判成二进制（这是窗口截断误判）")
	}
	if lineCount != 201 {
		t.Errorf("行数 = %d，期望 201", lineCount)
	}
	if tokens <= len(content)/4 {
		t.Errorf("中文 token 估算应高于 size/4：tokens=%d size/4=%d", tokens, len(content)/4)
	}
}

// 判定范围是整个文件：第 8000 字节之后的 NUL / 非法 UTF-8 同样要拦住
// （否则裸字节会随正文写进 Markdown）。
func TestScanFileStatsDetectsBinaryBeyondProbeWindow(t *testing.T) {
	dir := t.TempDir()
	late := filepath.Join(dir, "late_bad.bin")
	mustWrite(t, late, strings.Repeat("a", 9000)+"\xff\xfe")
	if _, isBinary, _, err := scanFileStats(late, true); err != nil || !isBinary {
		t.Errorf("第 9000 字节后的非法 UTF-8 应判为二进制（err=%v binary=%v）", err, isBinary)
	}

	lateNul := filepath.Join(dir, "late_nul.bin")
	mustWrite(t, lateNul, strings.Repeat("a", 9000)+"\x00")
	if _, isBinary, _, err := scanFileStats(lateNul, true); err != nil || !isBinary {
		t.Errorf("第 9000 字节后的 NUL 应判为二进制（err=%v binary=%v）", err, isBinary)
	}

	// 整文件合法则不能误杀（跨块边界的中文最容易出问题）
	big := filepath.Join(dir, "big_ok.txt")
	mustWrite(t, big, strings.Repeat("中", 5000)) // 15000 字节，跨多个块边界
	if _, isBinary, _, err := scanFileStats(big, true); err != nil || isBinary {
		t.Errorf("整文件合法 UTF-8 不该判为二进制（err=%v binary=%v）", err, isBinary)
	}
}

func TestCountLinesBuffer(t *testing.T) {
	lines := countLinesBuffer([]byte("line1\nline2\nline3\n"))
	if lines != 3 {
		t.Errorf("countLinesBuffer() = %d, want 3", lines)
	}
}

func TestCleanList(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"go,ts,py", []string{".go", ".ts", ".py"}},
		{".go, .ts", []string{".go", ".ts"}},
		{"go", []string{".go"}},
		{"src/main", []string{"src/main"}},
	}
	for _, tt := range tests {
		got := cleanList(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("cleanList(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("cleanList(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestCleanPathList(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"_COMPLETE_BOOK", []string{"_COMPLETE_BOOK"}},
		{"chapters, materials", []string{"chapters", "materials"}},
		{"黑莓,诺基亚", []string{"黑莓", "诺基亚"}},
		{"src/main,*.tmp", []string{"src/main", "*.tmp"}},
		{"go, ts", []string{"go", "ts"}},
		{"  , ", nil},
	}
	for _, tt := range tests {
		got := cleanPathList(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("cleanPathList(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("cleanPathList(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestGenerateAnchor(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"src/main.go", "src-main-go"},
		{"README.md", "readme-md"},
		{"pkg\\util.js", "pkg-util-js"},
	}
	for _, tt := range tests {
		got := generateAnchor(tt.input)
		if got != tt.want {
			t.Errorf("generateAnchor(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildTreeString(t *testing.T) {
	files := []FileMetadata{
		{RelPath: "cmd/app/main.go"},
		{RelPath: "cmd/app/util.go"},
		{RelPath: "go.mod"},
		{RelPath: "internal/handler.go"},
		{RelPath: "README.md"},
	}

	got := buildTreeString(files, "myproject")
	want := `myproject/
├── cmd/
│   └── app/
│       ├── main.go  (0 lines)
│       └── util.go  (0 lines)
├── internal/
│   └── handler.go  (0 lines)
├── README.md  (0 lines)
└── go.mod  (0 lines)
`
	if got != want {
		t.Errorf("buildTreeString() =\n%s\nwant:\n%s", got, want)
	}
}

func TestIsSensitiveFile(t *testing.T) {
	sensitive := []string{
		".env", ".env.local", ".env.production", ".ENV",
		"id_rsa", "id_ed25519", ".npmrc", ".netrc",
		"credentials.json", "secrets.yaml", "server.pem", "private.key",
	}
	for _, name := range sensitive {
		if !isSensitiveFile(name) {
			t.Errorf("isSensitiveFile(%q) = false, want true", name)
		}
	}
	normal := []string{
		"main.go", "envelope.go", "environment.ts", "provider.go",
		"README.md", "docker-compose.yaml", "Makefile",
	}
	for _, name := range normal {
		if isSensitiveFile(name) {
			t.Errorf("isSensitiveFile(%q) = true, want false", name)
		}
	}
}

func TestShouldIgnoreFileSensitiveDefaults(t *testing.T) {
	cfg := Config{}
	matcher := (*gitignoreMatcher)(nil)

	if !shouldIgnoreFile(".env", cfg, matcher) {
		t.Error(".env 应被默认忽略")
	}
	if !shouldIgnoreFile("config/.env.local", cfg, matcher) {
		t.Error("config/.env.local 应被默认忽略")
	}
	if shouldIgnoreFile("main.go", cfg, matcher) {
		t.Error("main.go 不应被忽略")
	}
	// --no-default-ignore 时保留显式放行能力
	cfgOptOut := Config{NoDefaultIgnore: true}
	if shouldIgnoreFile(".env", cfgOptOut, matcher) {
		t.Error("--no-default-ignore 下 .env 不应被默认忽略")
	}
}
