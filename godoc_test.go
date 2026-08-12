package main

import (
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
