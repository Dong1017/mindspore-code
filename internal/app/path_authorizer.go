package app

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/mindspore-lab/mindspore-cli/agent/loop"
	"github.com/mindspore-lab/mindspore-cli/internal/pathpolicy"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

type pathAuthorizationDecision struct {
	decision loop.PathAuthorizationDecision
}

type pendingPathAuthorizationRequest struct {
	wait   chan pathAuthorizationDecision
	denial *pathpolicy.PathDenial
}

type PathAuthorizer struct {
	mu       sync.Mutex
	pending  *pendingPathAuthorizationRequest
	eventCh  chan<- model.Event
	policy   *pathpolicy.PathPolicy
	saveRead func([]string) error
}

func NewPathAuthorizer(eventCh chan<- model.Event, policy *pathpolicy.PathPolicy, saveRead func([]string) error) *PathAuthorizer {
	return &PathAuthorizer{eventCh: eventCh, policy: policy, saveRead: saveRead}
}

func (p *PathAuthorizer) RequestPathAuthorization(ctx context.Context, denial *pathpolicy.PathDenial) (loop.PathAuthorizationDecision, error) {
	if p == nil || denial == nil || denial.Kind != string(pathpolicy.DenialKindExternalRead) {
		return loop.PathAuthorizationDecision{Scope: loop.PathAuthorizationDeny}, nil
	}
	root := strings.TrimSpace(denial.SuggestedRoot)
	if root == "" {
		return loop.PathAuthorizationDecision{Scope: loop.PathAuthorizationDeny}, nil
	}

	p.mu.Lock()
	if p.pending != nil {
		p.mu.Unlock()
		return loop.PathAuthorizationDecision{}, fmt.Errorf("path authorization request already pending")
	}
	req := &pendingPathAuthorizationRequest{wait: make(chan pathAuthorizationDecision, 1), denial: denial}
	p.pending = req
	p.mu.Unlock()

	p.eventCh <- model.Event{Type: model.PermissionPrompt, Message: pathAuthorizationPromptMessage(denial), Permission: pathAuthorizationPromptData(denial)}

	select {
	case <-ctx.Done():
		p.clearPending(req)
		return loop.PathAuthorizationDecision{}, ctx.Err()
	case decision := <-req.wait:
		return decision.decision, nil
	}
}

func (p *PathAuthorizer) HandleInput(input string) bool {
	input = strings.ToLower(strings.TrimSpace(input))
	p.mu.Lock()
	req := p.pending
	p.mu.Unlock()
	if req == nil {
		return false
	}
	root := strings.TrimSpace(req.denial.SuggestedRoot)
	resolve := func(decision loop.PathAuthorizationDecision) bool {
		p.clearPending(req)
		if decision.Scope == loop.PathAuthorizationSession && p.policy != nil {
			p.policy.AddSessionReadRoot(decision.Root)
		}
		if decision.Scope == loop.PathAuthorizationPersistent {
			if p.policy != nil {
				p.policy.AddSessionReadRoot(decision.Root)
			}
			if p.saveRead != nil {
				roots := []string{decision.Root}
				if p.policy != nil && p.policy.ReadRoots != nil {
					seen := map[string]bool{}
					roots = roots[:0]
					for _, entry := range p.policy.ReadRoots.Snapshot() {
						if entry.Source == pathpolicy.RootSourceConfig || entry.Source == pathpolicy.RootSourceSession {
							key := strings.ToLower(entry.Path)
							if !seen[key] {
								seen[key] = true
								roots = append(roots, entry.Path)
							}
						}
					}
				}
				_ = p.saveRead(roots)
			}
		}
		req.wait <- pathAuthorizationDecision{decision: decision}
		return true
	}
	switch input {
	case "1", "once", "y", "yes":
		return resolve(loop.PathAuthorizationDecision{Scope: loop.PathAuthorizationOnce, Root: root, Mode: "read"})
	case "2", "session", "allow_session":
		return resolve(loop.PathAuthorizationDecision{Scope: loop.PathAuthorizationSession, Root: root, Mode: "read"})
	case "3", "always", "persistent":
		return resolve(loop.PathAuthorizationDecision{Scope: loop.PathAuthorizationPersistent, Root: root, Mode: "read"})
	case "4", "n", "no", "deny", "esc", "escape":
		return resolve(loop.PathAuthorizationDecision{Scope: loop.PathAuthorizationDeny, Root: root, Mode: "read"})
	default:
		p.eventCh <- model.Event{Type: model.PermissionPrompt, Message: "Please choose 1, 2, 3, or 4.", Permission: pathAuthorizationPromptData(req.denial)}
		return true
	}
}

func (p *PathAuthorizer) clearPending(req *pendingPathAuthorizationRequest) {
	p.mu.Lock()
	if p.pending == req {
		p.pending = nil
	}
	p.mu.Unlock()
}

func pathAuthorizationPromptMessage(denial *pathpolicy.PathDenial) string {
	return fmt.Sprintf("Read wants to access a file outside the current workspace.\n\nCurrent workspace:\n  %s\n\nRequested file:\n  %s\n\nSuggested external read root:\n  %s\n\nAllow read-only access?\n  1. Allow once\n  2. Allow for this session\n  3. Always allow\n  4. Deny\n\nEsc to cancel", denial.WorkDir, denial.InputPath, denial.SuggestedRoot)
}

func pathAuthorizationPromptData(denial *pathpolicy.PathDenial) *model.PermissionPromptData {
	return &model.PermissionPromptData{
		Title:   "External read access",
		Message: fmt.Sprintf("Read wants to access a file outside the current workspace.\n\nCurrent workspace:\n  %s\n\nRequested file:\n  %s\n\nSuggested external read root:\n  %s", denial.WorkDir, denial.InputPath, denial.SuggestedRoot),
		Options: []model.PermissionOption{
			{Input: "1", Label: "1. Allow once"},
			{Input: "2", Label: "2. Allow for this session"},
			{Input: "3", Label: "3. Always allow"},
			{Input: "4", Label: "4. Deny"},
		},
		DefaultIndex: 0,
	}
}
