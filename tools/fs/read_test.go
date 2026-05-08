package fs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadToolReturnsPlaceholderForEmptyFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "empty.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	tool := NewReadTool(root)
	args, err := json.Marshal(map[string]string{"path": "empty.txt"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Content != "(file is empty)" {
		t.Fatalf("read content = %q, want empty-file placeholder", result.Content)
	}
	if result.Summary != "0 lines" {
		t.Fatalf("read summary = %q, want 0 lines", result.Summary)
	}
}
