package main

import (
	"os"
	"strings"
	"testing"
)

func TestWriteContentOutput(t *testing.T) {
	cfg := Config{
		RootDir:         ".",
		ShowStats:       false,
		NoSubdirs:       false,
		NoDefaultIgnore: true,
		NoGitignore:     true,
		MaxFileSize:     500 * 1024,
	}
	files, stats, _ := scanDirectory(cfg)
	if len(files) == 0 {
		t.Fatal("no files found")
	}

	f, _ := os.CreateTemp("", "sp_*.md")
	defer os.Remove(f.Name())
	if err := writeContent(cfg, files, stats, f); err != nil {
		t.Fatalf("writeContent error: %v", err)
	}
	f.Close()

	data, _ := os.ReadFile(f.Name())
	content := string(data)

	// Verify structure
	for _, want := range []string{
		"# Project Snapshot:",
		"## Project Structure",
		"## Table of Contents",
		"---",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("missing section: %s", want)
		}
	}

	// Verify each included file has a heading and at least one code fence
	for _, file := range files {
		if !strings.Contains(content, "## "+file.RelPath) {
			t.Errorf("missing file heading: %s", file.RelPath)
		}
		// Extract content after this file's heading and check it has a fence
		idx := strings.Index(content, "## "+file.RelPath)
		if idx >= 0 {
			// Look for next file heading or end of content
			rest := content[idx+len("## "+file.RelPath):]
			nextIdx := strings.Index(rest, "\n## ")
			if nextIdx > 0 {
				rest = rest[:nextIdx]
			}
			fenceCount := strings.Count(rest, "\n```")
			if fenceCount == 0 {
				t.Errorf("no code fence for file: %s", file.RelPath)
			}
		}
	}
}

func TestScanMaxBackticks(t *testing.T) {
	tmp := t.TempDir()

	// No backticks
	p1 := tmp + "/n1.txt"
	os.WriteFile(p1, []byte("hello world"), 0644)
	got, _ := scanMaxBackticks(p1)
	if got != 0 {
		t.Errorf("no backticks: got %d, want 0", got)
	}

	// Some backticks
	p2 := tmp + "/n2.txt"
	os.WriteFile(p2, []byte("some `inline` code"), 0644)
	got, _ = scanMaxBackticks(p2)
	if got != 1 {
		t.Errorf("single backticks: got %d, want 1", got)
	}

	// Triple backticks
	p3 := tmp + "/n3.txt"
	os.WriteFile(p3, []byte("```\ncode\n```"), 0644)
	got, _ = scanMaxBackticks(p3)
	if got != 3 {
		t.Errorf("triple backticks: got %d, want 3", got)
	}

	// Six backticks
	p4 := tmp + "/n4.txt"
	os.WriteFile(p4, []byte("``````"), 0644)
	got, _ = scanMaxBackticks(p4)
	if got != 6 {
		t.Errorf("six backticks: got %d, want 6", got)
	}

	// Large file (exceeds 32KB buffer)
	p5 := tmp + "/n5.txt"
	large := strings.Repeat("a", 40000) + "```" + strings.Repeat("b", 40000)
	os.WriteFile(p5, []byte(large), 0644)
	got, _ = scanMaxBackticks(p5)
	if got != 3 {
		t.Errorf("large file: got %d, want 3", got)
	}
}
