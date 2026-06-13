package fs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitcode.com/mindspore/mscli/tools"
)

func TestReadTool_Execute_LargeFileOverflow(t *testing.T) {
	tmp := t.TempDir()
	largeFile := filepath.Join(tmp, "large.txt")
	content := strings.Repeat("x", tools.DefaultMaxResultBytes+200)
	if err := os.WriteFile(largeFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	tool := NewReadTool(tmp)
	result, err := tool.Execute(context.Background(), []byte(`{"path":"large.txt"}`))
	if err != nil {
		t.Fatalf("execute read tool: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}

	if !strings.Contains(result.Content, "Result too large") {
		t.Errorf("expected overflow notice, got: %s", result.Content)
	}
	if !strings.Contains(result.Summary, "truncated") {
		t.Errorf("expected truncated summary, got: %s", result.Summary)
	}
}

func TestReadTool_Execute_SmallFileNoOverflow(t *testing.T) {
	tmp := t.TempDir()
	smallFile := filepath.Join(tmp, "small.txt")
	content := "hello world\n"
	if err := os.WriteFile(smallFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	tool := NewReadTool(tmp)
	result, err := tool.Execute(context.Background(), []byte(`{"path":"small.txt"}`))
	if err != nil {
		t.Fatalf("execute read tool: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}

	if result.Content != "hello world" {
		t.Errorf("content = %q, want %q", result.Content, "hello world")
	}
	if strings.Contains(result.Summary, "truncated") {
		t.Errorf("expected non-truncated summary, got: %s", result.Summary)
	}
}
