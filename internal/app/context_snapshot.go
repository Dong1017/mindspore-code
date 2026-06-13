package app

import (
	"strings"

	agentctx "gitcode.com/mindspore/mscli/agent/context"
	"gitcode.com/mindspore/mscli/agent/session"
	"gitcode.com/mindspore/mscli/integrations/llm"
)

func providerUsageSnapshotFromDetails(details agentctx.TokenUsageDetails) *session.UsageSnapshot {
	if details.Source != agentctx.TokenUsageSourceProvider || details.ProviderSnapshotTokens <= 0 {
		return nil
	}

	return &session.UsageSnapshot{
		Provider:   details.Provider,
		TokenScope: string(details.ProviderTokenScope),
		Tokens:     details.ProviderSnapshotTokens,
		LocalDelta: details.LocalDelta,
		Usage:      ptrProviderUsage(details.ProviderUsage),
	}
}

func restoreProviderUsageSnapshot(cm *agentctx.Manager, usage *session.UsageSnapshot) {
	if cm == nil || usage == nil || usage.Tokens <= 0 {
		return
	}

	scope := agentctx.ProviderTokenScope(strings.TrimSpace(usage.TokenScope))
	if scope != agentctx.ProviderTokenScopeTotal {
		scope = agentctx.ProviderTokenScopePrompt
	}

	cm.RestoreProviderUsageSnapshot(agentctx.ProviderUsageSnapshot{
		Provider:   usage.Provider,
		TokenScope: scope,
		Tokens:     usage.Tokens,
		LocalDelta: usage.LocalDelta,
		Usage:      derefProviderUsage(usage.Usage),
	})
}

func ptrProviderUsage(usage llm.Usage) *llm.Usage {
	if usage.IsZero() {
		return nil
	}
	copy := usage.Clone()
	return &copy
}

func derefProviderUsage(usage *llm.Usage) llm.Usage {
	if usage == nil {
		return llm.Usage{}
	}
	return usage.Clone()
}
