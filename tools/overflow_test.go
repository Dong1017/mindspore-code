package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultSpillDir(t *testing.T) {
	dir := DefaultSpillDir("/tmp/test-workspace")
	if dir == "" {
		t.Fatal("expected non-empty spill dir")
	}
	if !strings.Contains(dir, ".mscli") {
		t.Errorf("expected .mscli in path, got %q", dir)
	}
	if !strings.Contains(dir, "tool-results") {
		t.Errorf("expected tool-results in path, got %q", dir)
	}
}

func TestSpillResult_UnderLimit(t *testing.T) {
	tmp := t.TempDir()
	content := "small content"
	out := SpillResult(content, 100, tmp, "test")
	if out != content {
		t.Errorf("content changed unexpectedly: got %q", out)
	}
	files, _ := os.ReadDir(tmp)
	if len(files) > 0 {
		t.Errorf("expected no spill files, got %d", len(files))
	}
}

func TestSpillResult_OverLimit(t *testing.T) {
	tmp := t.TempDir()
	content := strings.Repeat("x", DefaultMaxResultBytes+200)
	out := SpillResult(content, DefaultMaxResultBytes, tmp, "test")

	if !strings.Contains(out, "Result too large") {
		t.Errorf("expected overflow notice, got: %s", out)
	}
	if !strings.Contains(out, "Preview:") {
		t.Errorf("expected preview, got: %s", out)
	}

	// Extract path from notice
	path := extractPathFromNotice(out)
	if path == "" {
		t.Fatal("could not extract spill path from notice")
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("spill file not created at %s: %v", path, statErr)
	}
	data, _ := os.ReadFile(path)
	if string(data) != content {
		t.Errorf("spill file content mismatch")
	}

	// Preview should be ≤500 bytes
	previewPrefix := "Preview:\n"
	previewIdx := strings.Index(out, previewPrefix)
	if previewIdx == -1 {
		t.Fatal("could not find preview in notice")
	}
	preview := out[previewIdx+len(previewPrefix):]
	if len(preview) > 500+3 { // 500 bytes + "..."
		t.Errorf("preview too long: %d bytes", len(preview))
	}
}

func TestSpillResult_WriteFailure(t *testing.T) {
	// Pass a non-existent nested path that MkdirAll won't fix
	badDir := filepath.Join(t.TempDir(), "nonexistent", "nested")
	content := strings.Repeat("y", 200)
	out := SpillResult(content, 100, badDir, "test")

	if !strings.Contains(out, "Failed to save to disk") {
		t.Errorf("expected disk-failure notice, got: %s", out)
	}
	if !strings.Contains(out, "Preview:") {
		t.Errorf("expected preview, got: %s", out)
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello wo..."},
		{"line1\nline2\nline3", 10, "line1\nline..."},
		{"exact", 5, "exact"},
		{"boundary", 8, "boundary"},
	}
	for _, tt := range tests {
		got := TruncateString(tt.input, tt.max)
		if got != tt.want {
			t.Errorf("TruncateString(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
		}
	}
}

func TestTruncateString_RuneBoundary(t *testing.T) {
	// Chinese characters are 3 bytes each in UTF-8
	input := "你好世界"
	// Truncate at 4 bytes — should walk back to 3 bytes (one complete rune)
	got := TruncateString(input, 4)
	want := "你..."
	if got != want {
		t.Errorf("TruncateString(%q, 4) = %q, want %q", input, got, want)
	}
}

func TestSpillFileName_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		name := SpillFileName("test")
		if seen[name] {
			t.Fatalf("duplicate spill file name: %s", name)
		}
		seen[name] = true
	}
}

func extractPathFromNotice(notice string) string {
	prefix := "Output saved to "
	suffix := ".\nPreview:"
	idx := strings.Index(notice, prefix)
	if idx == -1 {
		return ""
	}
	rest := notice[idx+len(prefix):]
	endIdx := strings.Index(rest, suffix)
	if endIdx == -1 {
		return ""
	}
	return rest[:endIdx]
}
