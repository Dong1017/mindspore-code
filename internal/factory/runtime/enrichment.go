package runtime

import (
	"fmt"
	"strings"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
)

type FactoryEnrichment struct {
	HintBlock string
	Matches   []pack.CaseMatch
	Trace     FactoryEnrichmentTrace
}

func BuildFactoryEnrichment(ctx pack.DiagnosticContext, cfg pack.LoadConfig) (FactoryEnrichment, error) {
	trace := FactoryEnrichmentTrace{}
	loadedPack, err := pack.LoadDefault(cfg)
	if err != nil {
		trace.PackLoadStatus = "failed"
		trace.FallbackReason = fmt.Sprintf("pack load failed: %v", err)
		return FactoryEnrichment{Trace: trace}, nil
	}
	trace.PackLoadStatus = "loaded"
	trace.ManifestSummary = fmt.Sprintf("%s schema=%s cases=%d", loadedPack.Manifest.PackName, loadedPack.Manifest.SchemaVersion, loadedPack.Manifest.CompiledCaseCount)

	matches, err := loadedPack.MatchCases(ctx.ToFingerprint(), pack.MatchOptions{})
	if err != nil {
		trace.FallbackReason = fmt.Sprintf("match failed: %v", err)
		return FactoryEnrichment{Trace: trace}, nil
	}
	trace.CandidateCount = len(matches)
	if len(matches) == 0 {
		trace.FallbackReason = "no match"
		return FactoryEnrichment{Trace: trace}, nil
	}

	hint, err := pack.RenderFactoryHintBlock(matches)
	if err != nil {
		trace.FallbackReason = fmt.Sprintf("hint render failed: %v", err)
		return FactoryEnrichment{Trace: trace}, nil
	}
	if strings.TrimSpace(hint) == "" {
		trace.FallbackReason = "empty hint"
		return FactoryEnrichment{Trace: trace}, nil
	}
	trace.EmittedHintCount = len(matches)
	if trace.EmittedHintCount > pack.MaxHintCases {
		trace.EmittedHintCount = pack.MaxHintCases
	}
	return FactoryEnrichment{HintBlock: hint, Matches: matches, Trace: trace}, nil
}

func BuildFactoryHintBlock(ctx pack.DiagnosticContext, cfg pack.LoadConfig) (string, FactoryEnrichmentTrace, error) {
	enrichment, err := BuildFactoryEnrichment(ctx, cfg)
	return enrichment.HintBlock, enrichment.Trace, err
}

func BuildDiagnoseRunSummary(ctx pack.DiagnosticContext, matches []pack.CaseMatch) DiagnoseRunSummary {
	summary := DiagnoseRunSummary{
		Command:            "diagnose",
		Topic:              firstNonEmpty(ctx.Signals.MainError, ctx.UserInput),
		UserProblemSummary: ctx.UserInput,
		KeyEvidence:        buildKeyEvidence(ctx),
		Privacy: SummaryPrivacy{
			RawLogsIncluded:      false,
			SensitiveEnvIncluded: false,
		},
	}
	for _, match := range matches {
		summary.FactoryHintsUsed = append(summary.FactoryHintsUsed, FactoryHintSummary{
			CaseID:     match.CaseID,
			Title:      match.Title,
			WhyMatched: match.WhyMatched,
		})
	}
	return BoundDiagnoseRunSummary(summary)
}

func buildKeyEvidence(ctx pack.DiagnosticContext) []string {
	values := []string{
		ctx.Signals.MainError,
		ctx.Problem.InferredType,
		ctx.Problem.InferredStage,
		ctx.Environment.Hardware.Accelerator,
	}
	values = append(values, ctx.Signals.Keywords...)
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return limitStrings(out, 8)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return trimApproxTokens(value, 80)
		}
	}
	return ""
}

func trimApproxTokens(value string, maxTokens int) string {
	fields := strings.Fields(value)
	if len(fields) <= maxTokens {
		return strings.TrimSpace(value)
	}
	return strings.Join(fields[:maxTokens], " ")
}
