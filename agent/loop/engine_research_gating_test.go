package loop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mindspore-lab/mindspore-cli/integrations/llm"
	"github.com/mindspore-lab/mindspore-cli/permission"
	"github.com/mindspore-lab/mindspore-cli/tools"
)

func TestResearchBudgetDisablesResearchToolsOnNextRequest(t *testing.T) {
	args, err := json.Marshal(map[string]string{"path": "README.md"})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	provider := &scriptedStreamProvider{
		responses: []*llm.CompletionResponse{
			{
				ToolCalls: []llm.ToolCall{{
					ID:   "call-read-1",
					Type: "function",
					Function: llm.ToolCallFunc{
						Name:      "read",
						Arguments: args,
					},
				}},
				FinishReason: llm.FinishToolCalls,
			},
			{
				Content:      "done",
				FinishReason: llm.FinishStop,
			},
		},
	}

	registry := tools.NewRegistry()
	registry.MustRegister(stubTool{name: "read", content: "file contents"})
	registry.MustRegister(stubTool{name: "write", content: "wrote"})

	engine := NewEngine(EngineConfig{
		MaxIterations:        2,
		MaxResearchToolCalls: 1,
		ContextWindow:        4096,
	}, provider, registry)

	_, err = engine.RunWithContext(context.Background(), Task{Description: "review"})
	if err != nil {
		t.Fatalf("RunWithContext error = %v", err)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("provider requests = %d, want 2", len(provider.requests))
	}

	firstTools := toolNames(provider.requests[0].Tools)
	if !contains(firstTools, "read") || !contains(firstTools, "write") {
		t.Fatalf("first request tools = %v, want read and write", firstTools)
	}
	secondTools := toolNames(provider.requests[1].Tools)
	if contains(secondTools, "read") {
		t.Fatalf("second request tools = %v, want read filtered", secondTools)
	}
	if !contains(secondTools, "write") {
		t.Fatalf("second request tools = %v, want write retained", secondTools)
	}

	foundGuidance := false
	for _, msg := range provider.requests[1].Messages {
		if msg.Role == "system" && strings.Contains(msg.Content, "Research tools are disabled") {
			foundGuidance = true
			break
		}
	}
	if !foundGuidance {
		t.Fatalf("second request messages = %#v, want research-disabled guidance", provider.requests[1].Messages)
	}
	for _, msg := range engine.ctxManager.GetMessages() {
		if strings.Contains(msg.Content, "Research tools are disabled") {
			t.Fatalf("research-disabled guidance was persisted in context: %#v", msg)
		}
	}
}

func TestResearchBudgetZeroLeavesResearchToolsEnabled(t *testing.T) {
	provider := &scriptedStreamProvider{
		responses: []*llm.CompletionResponse{
			{
				ToolCalls: []llm.ToolCall{{
					ID:   "call-read-1",
					Type: "function",
					Function: llm.ToolCallFunc{
						Name:      "read",
						Arguments: json.RawMessage(`{"path":"README.md"}`),
					},
				}},
				FinishReason: llm.FinishToolCalls,
			},
			{
				Content:      "done",
				FinishReason: llm.FinishStop,
			},
		},
	}

	registry := tools.NewRegistry()
	registry.MustRegister(stubTool{name: "read", content: "file contents"})
	registry.MustRegister(stubTool{name: "write", content: "wrote"})

	engine := NewEngine(EngineConfig{
		MaxIterations:        2,
		MaxResearchToolCalls: 0,
		ContextWindow:        4096,
	}, provider, registry)

	_, err := engine.RunWithContext(context.Background(), Task{Description: "review"})
	if err != nil {
		t.Fatalf("RunWithContext error = %v", err)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("provider requests = %d, want 2", len(provider.requests))
	}
	secondTools := toolNames(provider.requests[1].Tools)
	if !contains(secondTools, "read") || !contains(secondTools, "write") {
		t.Fatalf("second request tools = %v, want read and write retained", secondTools)
	}
	for _, msg := range provider.requests[1].Messages {
		if strings.Contains(msg.Content, "Research tools are disabled") {
			t.Fatalf("second request message = %#v, want no research-disabled guidance", msg)
		}
	}
}

func TestResearchToolClassification(t *testing.T) {
	for _, name := range []string{"read", "grep", "glob", "shell", "load_skill"} {
		if !isResearchTool(name) {
			t.Fatalf("isResearchTool(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"write", "edit"} {
		if isResearchTool(name) {
			t.Fatalf("isResearchTool(%q) = true, want false", name)
		}
	}
}

func TestDisableResearchToolsFiltersByFunctionName(t *testing.T) {
	provider := &captureProvider{}
	registry := tools.NewRegistry()
	registry.MustRegister(stubTool{name: "read", content: "file contents"})
	registry.MustRegister(stubTool{name: "write", content: "wrote"})

	engine := NewEngine(EngineConfig{MaxIterations: 1, ContextWindow: 4096}, provider, registry)
	_, err := engine.RunWithContext(context.Background(), Task{
		Description:          "summarize",
		DisableResearchTools: true,
	})
	if err != nil {
		t.Fatalf("RunWithContext error = %v", err)
	}
	if provider.lastReq == nil {
		t.Fatal("provider request = nil")
	}
	tools := toolNames(provider.lastReq.Tools)
	if contains(tools, "read") {
		t.Fatalf("provider tools = %v, want read filtered by function name", tools)
	}
	if !contains(tools, "write") {
		t.Fatalf("provider tools = %v, want write retained", tools)
	}
}

func TestResearchBudgetExceededByBatchDisablesResearchToolsOnNextRequest(t *testing.T) {
	var readCalls int
	var grepCalls int
	provider := &scriptedStreamProvider{
		responses: []*llm.CompletionResponse{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call-read-1",
						Type: "function",
						Function: llm.ToolCallFunc{
							Name:      "read",
							Arguments: json.RawMessage(`{"path":"README.md"}`),
						},
					},
					{
						ID:   "call-grep-1",
						Type: "function",
						Function: llm.ToolCallFunc{
							Name:      "grep",
							Arguments: json.RawMessage(`{"pattern":"hello"}`),
						},
					},
				},
				FinishReason: llm.FinishToolCalls,
			},
			{
				Content:      "done",
				FinishReason: llm.FinishStop,
			},
		},
	}

	registry := tools.NewRegistry()
	registry.MustRegister(countingStubTool{stubTool: stubTool{name: "read", content: "file contents"}, count: &readCalls})
	registry.MustRegister(countingStubTool{stubTool: stubTool{name: "grep", content: "matches"}, count: &grepCalls})
	registry.MustRegister(stubTool{name: "write", content: "wrote"})

	engine := NewEngine(EngineConfig{
		MaxIterations:        2,
		MaxResearchToolCalls: 1,
		ContextWindow:        4096,
	}, provider, registry)

	_, err := engine.RunWithContext(context.Background(), Task{Description: "review"})
	if err != nil {
		t.Fatalf("RunWithContext error = %v", err)
	}
	if readCalls != 1 || grepCalls != 1 {
		t.Fatalf("executed read=%d grep=%d, want both calls executed once", readCalls, grepCalls)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("provider requests = %d, want 2", len(provider.requests))
	}
	secondTools := toolNames(provider.requests[1].Tools)
	if contains(secondTools, "read") || contains(secondTools, "grep") {
		t.Fatalf("second request tools = %v, want research tools filtered after batch", secondTools)
	}
	if !contains(secondTools, "write") {
		t.Fatalf("second request tools = %v, want write retained", secondTools)
	}
}

func TestResearchDisabledGuidanceNotInheritedByLaterNormalTask(t *testing.T) {
	provider := &scriptedStreamProvider{
		responses: []*llm.CompletionResponse{
			{
				Content:      "summary",
				FinishReason: llm.FinishStop,
			},
			{
				Content:      "normal",
				FinishReason: llm.FinishStop,
			},
		},
	}

	registry := tools.NewRegistry()
	registry.MustRegister(stubTool{name: "read", content: "file contents"})
	registry.MustRegister(stubTool{name: "write", content: "wrote"})

	engine := NewEngine(EngineConfig{MaxIterations: 1, ContextWindow: 4096}, provider, registry)
	_, err := engine.RunWithContext(context.Background(), Task{
		Description:          "summarize",
		DisableResearchTools: true,
	})
	if err != nil {
		t.Fatalf("first RunWithContext error = %v", err)
	}
	_, err = engine.RunWithContext(context.Background(), Task{Description: "inspect more files"})
	if err != nil {
		t.Fatalf("second RunWithContext error = %v", err)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("provider requests = %d, want 2", len(provider.requests))
	}
	for _, msg := range provider.requests[1].Messages {
		if strings.Contains(msg.Content, "Research tools are disabled") {
			t.Fatalf("later normal request inherited research-disabled guidance: %#v", msg)
		}
	}
	secondTools := toolNames(provider.requests[1].Tools)
	if !contains(secondTools, "read") || !contains(secondTools, "write") {
		t.Fatalf("later normal request tools = %v, want read and write", secondTools)
	}
}

func TestDisableResearchToolsSendsNilToolsWhenNoToolsRemain(t *testing.T) {
	provider := &captureProvider{}
	registry := tools.NewRegistry()
	registry.MustRegister(stubTool{name: "read", content: "file contents"})

	engine := NewEngine(EngineConfig{MaxIterations: 1, ContextWindow: 4096}, provider, registry)
	_, err := engine.RunWithContext(context.Background(), Task{
		Description:          "summarize",
		DisableResearchTools: true,
	})
	if err != nil {
		t.Fatalf("RunWithContext error = %v", err)
	}
	if provider.lastReq == nil {
		t.Fatal("provider request = nil")
	}
	if provider.lastReq.Tools != nil {
		t.Fatalf("provider tools = %#v, want nil", provider.lastReq.Tools)
	}
}

func TestResearchToolCallCountsWhenExecutionFails(t *testing.T) {
	args, err := json.Marshal(map[string]string{"path": "README.md"})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	provider := &scriptedStreamProvider{
		responses: []*llm.CompletionResponse{
			{
				ToolCalls: []llm.ToolCall{{
					ID:   "call-load-skill-1",
					Type: "function",
					Function: llm.ToolCallFunc{
						Name:      "load_skill",
						Arguments: args,
					},
				}},
				FinishReason: llm.FinishToolCalls,
			},
			{
				Content:      "done",
				FinishReason: llm.FinishStop,
			},
		},
	}

	registry := tools.NewRegistry()
	registry.MustRegister(stubTool{name: "load_skill", content: "skill"})
	registry.MustRegister(stubTool{name: "write", content: "wrote"})

	engine := NewEngine(EngineConfig{
		MaxIterations:        2,
		MaxResearchToolCalls: 1,
		ContextWindow:        4096,
	}, provider, registry)
	engine.SetPermissionService(denyPermissionService{})

	_, err = engine.RunWithContext(context.Background(), Task{Description: "review"})
	if err != nil {
		t.Fatalf("RunWithContext error = %v", err)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("provider requests = %d, want 2", len(provider.requests))
	}
	secondTools := toolNames(provider.requests[1].Tools)
	if contains(secondTools, "load_skill") {
		t.Fatalf("second request tools = %v, want load_skill filtered after denied attempt", secondTools)
	}
}

func TestResearchToolCallCountsWhenToolExecutionErrors(t *testing.T) {
	provider := &scriptedStreamProvider{
		responses: []*llm.CompletionResponse{
			{
				ToolCalls: []llm.ToolCall{{
					ID:   "call-read-1",
					Type: "function",
					Function: llm.ToolCallFunc{
						Name:      "read",
						Arguments: json.RawMessage(`{"path":"README.md"}`),
					},
				}},
				FinishReason: llm.FinishToolCalls,
			},
			{
				Content:      "done",
				FinishReason: llm.FinishStop,
			},
		},
	}

	registry := tools.NewRegistry()
	registry.MustRegister(failingStubTool{stubTool: stubTool{name: "read"}})
	registry.MustRegister(stubTool{name: "write", content: "wrote"})

	engine := NewEngine(EngineConfig{
		MaxIterations:        2,
		MaxResearchToolCalls: 1,
		ContextWindow:        4096,
	}, provider, registry)

	_, err := engine.RunWithContext(context.Background(), Task{Description: "review"})
	if err != nil {
		t.Fatalf("RunWithContext error = %v", err)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("provider requests = %d, want 2", len(provider.requests))
	}
	secondTools := toolNames(provider.requests[1].Tools)
	if contains(secondTools, "read") {
		t.Fatalf("second request tools = %v, want read filtered after tool execution error", secondTools)
	}
	if !contains(secondTools, "write") {
		t.Fatalf("second request tools = %v, want write retained", secondTools)
	}
}

func TestMaxIterationsReturnsTypedError(t *testing.T) {
	provider := &scriptedStreamProvider{
		responses: []*llm.CompletionResponse{{
			ToolCalls: []llm.ToolCall{{
				ID:   "call-read-1",
				Type: "function",
				Function: llm.ToolCallFunc{
					Name:      "read",
					Arguments: json.RawMessage(`{"path":"README.md"}`),
				},
			}},
			FinishReason: llm.FinishToolCalls,
		}},
	}
	registry := tools.NewRegistry()
	registry.MustRegister(stubTool{name: "read", content: "file contents"})

	engine := NewEngine(EngineConfig{MaxIterations: 1, ContextWindow: 4096}, provider, registry)
	events, err := engine.RunWithContext(context.Background(), Task{Description: "read"})
	if !errors.Is(err, ErrMaxIterations) {
		t.Fatalf("RunWithContext error = %v, want ErrMaxIterations", err)
	}
	if len(events) == 0 || events[len(events)-1].Type != EventTaskFailed {
		t.Fatalf("last event = %#v, want TaskFailed", events)
	}
}

type failingStubTool struct {
	stubTool
}

func (t failingStubTool) Execute(context.Context, json.RawMessage) (*tools.Result, error) {
	return nil, fmt.Errorf("boom")
}

type countingStubTool struct {
	stubTool
	count *int
}

func (t countingStubTool) Execute(ctx context.Context, params json.RawMessage) (*tools.Result, error) {
	(*t.count)++
	return t.stubTool.Execute(ctx, params)
}

func toolNames(tools []llm.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Function.Name)
	}
	return names
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

type denyPermissionService struct{}

func (denyPermissionService) Request(context.Context, string, string, string) (bool, error) {
	return false, nil
}

func (denyPermissionService) Check(string, string) permission.PermissionLevel {
	return permission.PermissionDeny
}

func (denyPermissionService) CheckCommand(string) permission.PermissionLevel {
	return permission.PermissionDeny
}

func (denyPermissionService) CheckPath(string) permission.PermissionLevel {
	return permission.PermissionDeny
}

func (denyPermissionService) Grant(string, permission.PermissionLevel) {}

func (denyPermissionService) GrantCommand(string, permission.PermissionLevel) {}

func (denyPermissionService) GrantPath(string, permission.PermissionLevel) {}

func (denyPermissionService) Revoke(string) {}

func (denyPermissionService) RevokeCommand(string) {}

func (denyPermissionService) RevokePath(string) {}
