package loop

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mindspore-lab/mindspore-cli/integrations/llm"
	"github.com/mindspore-lab/mindspore-cli/tools"
)

func TestFiniteMaxIterationsDoesNotPreemptivelyDisableResearchTools(t *testing.T) {
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
		MaxIterations: 2,
		ContextWindow: 4096,
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
	if !contains(secondTools, "read") || !contains(secondTools, "write") {
		t.Fatalf("second request tools = %v, want read and write retained before explicit gating", secondTools)
	}

	for _, msg := range provider.requests[1].Messages {
		if msg.Role == "system" && strings.Contains(msg.Content, "Research tools are disabled") {
			t.Fatalf("second request message = %#v, want no research-disabled guidance", msg)
		}
	}
	for _, msg := range engine.ctxManager.GetMessages() {
		if strings.Contains(msg.Content, "Research tools are disabled") {
			t.Fatalf("research-disabled guidance was persisted in context: %#v", msg)
		}
	}
}

func TestUnlimitedIterationsLeaveResearchToolsEnabled(t *testing.T) {
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
		MaxIterations: 0,
		ContextWindow: 4096,
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

func TestDisableResearchToolsFiltersByFunctionName(t *testing.T) {
	provider := &captureProvider{}
	registry := tools.NewRegistry()
	registry.MustRegister(stubTool{name: "read", content: "file contents"})
	registry.MustRegister(stubTool{name: "write", content: "wrote"})
	registry.MustRegister(stubTool{name: "plain", content: "plain"})

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
	if contains(tools, "plain") {
		t.Fatalf("provider tools = %v, want unclassified tool filtered", tools)
	}
	if !contains(tools, "write") {
		t.Fatalf("provider tools = %v, want write retained", tools)
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
