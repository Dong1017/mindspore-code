package app

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/mindspore-lab/mindspore-cli/agent/loop"
	"github.com/mindspore-lab/mindspore-cli/integrations/llm"
	"github.com/mindspore-lab/mindspore-cli/tools"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

func TestResearchDisabledContinuationIntent(t *testing.T) {
	for _, input := range []string{"continue", "go on", "proceed", "keep going", "go ahead", "please summarize", "final answer"} {
		if !isResearchDisabledContinuationIntent(input) {
			t.Fatalf("isResearchDisabledContinuationIntent(%q) = false, want true", input)
		}
	}
	for _, input := range []string{"inspect more files", "fix the bug", "run tests"} {
		if isResearchDisabledContinuationIntent(input) {
			t.Fatalf("isResearchDisabledContinuationIntent(%q) = true, want false", input)
		}
	}
}

func TestRunTaskDisablesResearchToolsForPostLimitContinuation(t *testing.T) {
	provider := &captureTaskProvider{}
	app := &Application{
		Engine: loop.NewEngine(loop.EngineConfig{
			MaxIterations: 1,
			ContextWindow: 4096,
		}, provider, tools.NewRegistry()),
		EventCh:  make(chan model.Event, 32),
		llmReady: true,
	}

	app.runTask("inspect")
	if !app.prevTaskHitIterLimit {
		t.Fatal("prevTaskHitIterLimit = false, want true after max-iteration failure")
	}

	provider.responses = []llm.CompletionResponse{{Content: "summary", FinishReason: llm.FinishStop}}
	app.runTask("continue")
	if len(provider.requests) != 2 {
		t.Fatalf("provider requests = %d, want 2", len(provider.requests))
	}
	if !requestHasResearchDisabledGuidance(provider.requests[1]) {
		t.Fatalf("continuation request messages = %#v, want research-disabled guidance", provider.requests[1].Messages)
	}
	if app.prevTaskHitIterLimit {
		t.Fatal("prevTaskHitIterLimit = true, want false after successful continuation")
	}
}

func TestRunTaskDoesNotDisableResearchToolsForNonContinuationAfterLimit(t *testing.T) {
	provider := &captureTaskProvider{}
	app := &Application{
		Engine: loop.NewEngine(loop.EngineConfig{
			MaxIterations: 1,
			ContextWindow: 4096,
		}, provider, tools.NewRegistry()),
		EventCh:  make(chan model.Event, 32),
		llmReady: true,
	}

	app.runTask("inspect")
	if !app.prevTaskHitIterLimit {
		t.Fatal("prevTaskHitIterLimit = false, want true after max-iteration failure")
	}

	provider.responses = []llm.CompletionResponse{{Content: "normal", FinishReason: llm.FinishStop}}
	app.runTask("inspect more files")
	if len(provider.requests) != 2 {
		t.Fatalf("provider requests = %d, want 2", len(provider.requests))
	}
	if requestHasResearchDisabledGuidance(provider.requests[1]) {
		t.Fatalf("non-continuation request messages = %#v, want no research-disabled guidance", provider.requests[1].Messages)
	}
	if app.prevTaskHitIterLimit {
		t.Fatal("prevTaskHitIterLimit = true, want false after successful normal task")
	}
}

func requestHasResearchDisabledGuidance(req *llm.CompletionRequest) bool {
	if req == nil {
		return false
	}
	for _, msg := range req.Messages {
		if msg.Role == "system" && strings.Contains(msg.Content, "Research tools are disabled") {
			return true
		}
	}
	return false
}

type captureTaskProvider struct {
	requests  []*llm.CompletionRequest
	responses []llm.CompletionResponse
}

func (p *captureTaskProvider) Name() string {
	return "capture-task"
}

func (p *captureTaskProvider) Complete(context.Context, *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return nil, io.EOF
}

func (p *captureTaskProvider) CompleteStream(ctx context.Context, req *llm.CompletionRequest) (llm.StreamIterator, error) {
	copied := *req
	copied.Messages = append([]llm.Message(nil), req.Messages...)
	copied.Tools = append([]llm.Tool(nil), req.Tools...)
	p.requests = append(p.requests, &copied)

	if len(p.responses) == 0 {
		return &captureTaskIterator{chunks: []llm.StreamChunk{{
			ToolCalls: []llm.ToolCall{{
				ID:   "call-missing-read",
				Type: "function",
				Function: llm.ToolCallFunc{
					Name:      "read",
					Arguments: json.RawMessage(`{"path":"README.md"}`),
				},
			}},
			FinishReason: llm.FinishToolCalls,
		}}}, nil
	}

	resp := p.responses[0]
	p.responses = p.responses[1:]
	return &captureTaskIterator{chunks: []llm.StreamChunk{{
		Content:      resp.Content,
		ToolCalls:    append([]llm.ToolCall(nil), resp.ToolCalls...),
		FinishReason: resp.FinishReason,
	}}}, nil
}

func (p *captureTaskProvider) SupportsTools() bool {
	return true
}

func (p *captureTaskProvider) AvailableModels() []llm.ModelInfo {
	return nil
}

type captureTaskIterator struct {
	chunks []llm.StreamChunk
	index  int
}

func (it *captureTaskIterator) Next() (*llm.StreamChunk, error) {
	if it.index >= len(it.chunks) {
		return nil, io.EOF
	}
	chunk := it.chunks[it.index]
	it.index++
	return &chunk, nil
}

func (it *captureTaskIterator) Close() error {
	return nil
}
