package loop

import (
	"context"
	"encoding/json"
	"testing"

	ctxmanager "github.com/mindspore-lab/mindspore-cli/agent/context"
	"github.com/mindspore-lab/mindspore-cli/integrations/llm"
	"github.com/mindspore-lab/mindspore-cli/internal/pathpolicy"
	"github.com/mindspore-lab/mindspore-cli/permission"
	"github.com/mindspore-lab/mindspore-cli/tools"
)

type pathDenialAuthorizer struct {
	decision PathAuthorizationDecision
	calls    int
}

func (a *pathDenialAuthorizer) RequestPathAuthorization(context.Context, *pathpolicy.PathDenial) (PathAuthorizationDecision, error) {
	a.calls++
	return a.decision, nil
}

type pathDenialTool struct {
	calls int
	mode  string
}

func (t *pathDenialTool) Name() string {
	if t.mode == PathAuthorizationModeWrite {
		return "edit"
	}
	return "read"
}

func (t *pathDenialTool) Description() string { return "path denial tool" }

func (t *pathDenialTool) Schema() llm.ToolSchema { return llm.ToolSchema{Type: "object"} }

func (t *pathDenialTool) Execute(ctx context.Context, _ json.RawMessage) (*tools.Result, error) {
	t.calls++
	opts := pathpolicy.ResolveOptionsFromContext(ctx)
	if t.mode == PathAuthorizationModeWrite {
		if len(opts.TemporaryWriteRoots) > 0 {
			return tools.StringResultWithSummary("write allowed", "ok"), nil
		}
		return pathpolicy.NewPathDenialResult(&pathpolicy.PathDenial{
			Kind:          string(pathpolicy.DenialKindExternalWrite),
			Operation:     "edit",
			InputPath:     "/external/file.txt",
			SuggestedRoot: "/external",
		}), nil
	}
	if len(opts.TemporaryReadRoots) > 0 {
		return tools.StringResultWithSummary("allowed", "ok"), nil
	}
	return pathpolicy.NewPathDenialResult(&pathpolicy.PathDenial{
		Kind:          string(pathpolicy.DenialKindExternalRead),
		Operation:     "read",
		InputPath:     "/external/file.txt",
		SuggestedRoot: "/external",
	}), nil
}

func TestExecuteToolCallRetriesPathDenialWithTemporaryReadRoot(t *testing.T) {
	args, err := json.Marshal(map[string]string{"path": "/external/file.txt"})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	tool := &pathDenialTool{}
	registry := tools.NewRegistry()
	registry.MustRegister(tool)

	engine := NewEngine(EngineConfig{ContextWindow: 4096}, nil, registry)
	engine.ctxManager = ctxmanager.NewManager(ctxmanager.ManagerConfig{ContextWindow: 4096, ReserveTokens: 100})
	authorizer := &pathDenialAuthorizer{decision: PathAuthorizationDecision{Scope: PathAuthorizationOnce, Root: "/external", Mode: "read"}}
	engine.SetPathAuthorizer(authorizer)
	engine.SetPermissionService(permission.NewNoOpPermissionService())

	ex := &executor{engine: engine}
	tc := llm.ToolCall{ID: "call-read", Type: "function", Function: llm.ToolCallFunc{Name: "read", Arguments: args}}

	if err := ex.executeToolCall(context.Background(), tc); err != nil {
		t.Fatalf("executeToolCall() error = %v", err)
	}
	if got, want := authorizer.calls, 1; got != want {
		t.Fatalf("authorizer calls = %d, want %d", got, want)
	}
	if got, want := tool.calls, 2; got != want {
		t.Fatalf("tool calls = %d, want %d", got, want)
	}
	msgs := engine.ctxManager.GetNonSystemMessages()
	if len(msgs) != 1 {
		t.Fatalf("tool messages = %d, want 1", len(msgs))
	}
	if got, want := msgs[0].Content, "allowed"; got != want {
		t.Fatalf("tool result = %q, want %q", got, want)
	}
}

func TestExecuteToolCallRetriesPathDenialWithTemporaryWriteRoot(t *testing.T) {
	args, err := json.Marshal(map[string]string{"path": "/external/file.txt"})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	tool := &pathDenialTool{mode: PathAuthorizationModeWrite}
	registry := tools.NewRegistry()
	registry.MustRegister(tool)

	engine := NewEngine(EngineConfig{ContextWindow: 4096}, nil, registry)
	engine.ctxManager = ctxmanager.NewManager(ctxmanager.ManagerConfig{ContextWindow: 4096, ReserveTokens: 100})
	authorizer := &pathDenialAuthorizer{decision: PathAuthorizationDecision{Scope: PathAuthorizationOnce, Root: "/external", Mode: PathAuthorizationModeWrite}}
	engine.SetPathAuthorizer(authorizer)
	engine.SetPermissionService(permission.NewNoOpPermissionService())

	ex := &executor{engine: engine}
	tc := llm.ToolCall{ID: "call-edit", Type: "function", Function: llm.ToolCallFunc{Name: "edit", Arguments: args}}

	if err := ex.executeToolCall(context.Background(), tc); err != nil {
		t.Fatalf("executeToolCall() error = %v", err)
	}
	if got, want := authorizer.calls, 1; got != want {
		t.Fatalf("authorizer calls = %d, want %d", got, want)
	}
	if got, want := tool.calls, 2; got != want {
		t.Fatalf("tool calls = %d, want %d", got, want)
	}
	msgs := engine.ctxManager.GetNonSystemMessages()
	if len(msgs) != 1 {
		t.Fatalf("tool messages = %d, want 1", len(msgs))
	}
	if got, want := msgs[0].Content, "write allowed"; got != want {
		t.Fatalf("tool result = %q, want %q", got, want)
	}
}
