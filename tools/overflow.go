package tools

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// DefaultMaxResultBytes is the default max size for a tool result before overflow.
const DefaultMaxResultBytes = 100_000

// DefaultSpillDir returns the canonical spill directory for a workspace.
// Path: ~/.mscli/projects/<workspace-key>/tool-results
// Returns empty string if home directory cannot be resolved.
func DefaultSpillDir(workDir string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	workspaceAbs, err := filepath.Abs(workDir)
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".mscli", "projects", workspaceKey(workspaceAbs), "tool-results")
}

// SpillResult writes content to disk if it exceeds maxBytes and returns a notice
// string containing the file path and a preview. If write fails, it returns an
// in-memory truncated notice. spillDir must exist before calling; callers should
// create it with os.MkdirAll immediately prior to the call.
func SpillResult(content string, maxBytes int, spillDir, toolName string) string {
	if len(content) <= maxBytes {
		return content
	}
	path, err := writeSpillFile(content, spillDir, toolName)
	if err != nil {
		preview := TruncateString(content, 500)
		return fmt.Sprintf("Result too large (%d bytes). Failed to save to disk: %v.\nPreview:\n%s",
			len(content), err, preview)
	}
	preview := TruncateString(content, 500)
	return fmt.Sprintf("Result too large (%d bytes). Output saved to %s.\nPreview:\n%s",
		len(content), path, preview)
}

// TruncateString truncates s to at most maxLen bytes without breaking a UTF-8 rune.
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Try to break at newline
	if idx := strings.LastIndex(s[:maxLen], "\n"); idx > maxLen/2 {
		return s[:idx] + "\n..."
	}
	// Walk back to avoid splitting a rune
	for maxLen > 0 && !utf8.ValidString(s[:maxLen]) {
		maxLen--
	}
	return s[:maxLen] + "..."
}

// SpillFileName generates a unique filename for a spill file.
func SpillFileName(toolName string) string {
	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)
	return fmt.Sprintf("%s-%d-%x.txt", safeFileName(toolName), time.Now().UnixMilli(), suffix)
}

func writeSpillFile(content, spillDir, toolName string) (string, error) {
	name := SpillFileName(toolName)
	path := filepath.Join(spillDir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}

func workspaceKey(workspace string) string {
	key := strings.ReplaceAll(workspace, string(filepath.Separator), "-")
	key = strings.ReplaceAll(key, ":", "-")
	key = strings.Trim(key, "-")
	if key == "" {
		return "workspace"
	}
	return key
}

func safeFileName(name string) string {
	name = strings.TrimSpace(name)
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	result := strings.Trim(sb.String(), "_")
	if result == "" {
		return "tool"
	}
	return result
}
