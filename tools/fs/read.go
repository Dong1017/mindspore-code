package fs

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mindspore-lab/mindspore-cli/integrations/llm"
	"github.com/mindspore-lab/mindspore-cli/internal/pathpolicy"
	"github.com/mindspore-lab/mindspore-cli/tools"
)

// ReadTool reads file contents.
type ReadTool struct {
	resolver *pathpolicy.Resolver
}

// NewReadTool creates a new read tool.
func NewReadTool(workDir string) *ReadTool {
	return NewReadToolWithResolver(newWorkspaceResolver(workDir))
}

func NewReadToolWithResolver(resolver *pathpolicy.Resolver) *ReadTool {
	return &ReadTool{resolver: resolver}
}

// Name returns the tool name.
func (t *ReadTool) Name() string {
	return "read"
}

// Description returns the tool description.
func (t *ReadTool) Description() string {
	return "Read the contents of a file. Use this when you need to examine file contents."
}

// Schema returns the tool parameter schema.
func (t *ReadTool) Schema() llm.ToolSchema {
	return llm.ToolSchema{
		Type: "object",
		Properties: map[string]llm.Property{
			"path": {
				Type:        "string",
				Description: "Path to the file to read. Relative paths resolve under the workspace; absolute paths require workspace or external_read_roots access",
			},
			"offset": {
				Type:        "integer",
				Description: "Line number to start reading from (1-indexed, 0 means from start)",
			},
			"limit": {
				Type:        "integer",
				Description: "Maximum number of lines to read (0 means no limit)",
			},
		},
		Required: []string{"path"},
	}
}

type readParams struct {
	Path   string `json:"path"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

// Execute executes the read tool.
func (t *ReadTool) Execute(ctx context.Context, params json.RawMessage) (*tools.Result, error) {
	var p readParams
	if err := tools.ParseParams(params, &p); err != nil {
		return tools.ErrorResult(err), nil
	}

	fullPath, denial, err := t.resolver.ResolveReadablePathForOperation("read", p.Path, pathpolicy.ResolveOptionsFromContext(ctx))
	if err != nil {
		return tools.ErrorResult(err), nil
	}
	if denial != nil {
		return pathpolicy.NewPathDenialResult(denial), nil
	}

	// Check if file exists
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return tools.ErrorResultf("file not found: %s", p.Path), nil
		}
		return tools.ErrorResultf("stat file: %w", err), nil
	}

	if info.IsDir() {
		return tools.ErrorResultf("path is a directory: %s", p.Path), nil
	}

	// Read file
	content, err := t.readFile(fullPath, p.Offset, p.Limit)
	if err != nil {
		return tools.ErrorResult(err), nil
	}

	// Count lines for summary
	lines := strings.Count(content, "\n")
	if !strings.HasSuffix(content, "\n") && content != "" {
		lines++
	}

	summary := fmt.Sprintf("%d lines", lines)
	if p.Offset > 0 || p.Limit > 0 {
		summary = fmt.Sprintf("%d lines (offset=%d, limit=%d)", lines, p.Offset, p.Limit)
	}

	return tools.StringResultWithSummary(content, summary), nil
}

func (t *ReadTool) readFile(path string, offset, limit int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var lines []string
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if offset > 0 && lineNum < offset {
			continue
		}
		lines = append(lines, scanner.Text())
		if limit > 0 && len(lines) >= limit {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return strings.Join(lines, "\n"), nil
}
