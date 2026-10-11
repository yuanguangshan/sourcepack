package main

import (
	"path/filepath"
	"testing"
)

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		path    string
		pattern string
		want    bool
	}{
		// Basename matching (no path separator in pattern)
		{"godoc.go", "godoc.go", true},
		{"godoc.go", "godoc", false}, // exact basename match only, no prefix
		{"src/godoc.go", "godoc.go", true},
		{"src/godoc.go", "godoc", false},
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

	// v1.8.1 收紧：非法 UTF-8（无 NUL）也算二进制，否则裸字节会写进 Markdown
	if !isBinaryBuffer([]byte("\x80\x81\x82\xff")) {
		t.Error("invalid UTF-8 without NUL should be detected as binary")
	}

	// v1.8.1 收紧：合法 UTF-8 + NUL。旧实现只在"非法 UTF-8"时才去找 NUL，
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

func TestTokensApproximation(t *testing.T) {
	content := "hello world"
	tokens := len(content) / 4
	expected := 2 // 11 / 4 = 2
	if tokens != expected {
		t.Errorf("token approximation = %d, want %d", tokens, expected)
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
