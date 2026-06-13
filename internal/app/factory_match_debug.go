package app

import (
	"fmt"
	"strings"

	"gitcode.com/mindspore/mscli/internal/factory/pack"
)

func runFactoryPackMatchDebug(args []string, opts factoryCommandOptions) (string, error) {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		message := factoryUsageError(opts.Surface, "pack match-debug")
		if opts.Surface == factorySurfaceCLI {
			return "", fmt.Errorf("%s", message)
		}
		return message, nil
	}
	loadedPack, err := pack.LoadDefault(pack.LoadConfig{})
	if err != nil {
		return renderFactoryPackMatchDebugResult(factoryPackMatchDebugResult{
			PackLoadStatus: "failed",
			FallbackReason: fmt.Sprintf("pack load failed: %v", err),
		}), nil
	}
	matches, err := loadedPack.MatchCases(factoryMatchDebugContext(args[0]).ToFingerprint(), pack.MatchOptions{})
	if err != nil {
		return renderFactoryPackMatchDebugResult(factoryPackMatchDebugResult{
			PackLoadStatus:  "loaded",
			ManifestSummary: factoryPackManifestSummary(loadedPack.Manifest),
			FallbackReason:  fmt.Sprintf("match failed: %v", err),
		}), nil
	}
	result := factoryPackMatchDebugResult{
		PackLoadStatus:   "loaded",
		ManifestSummary:  factoryPackManifestSummary(loadedPack.Manifest),
		CandidateCount:   len(matches),
		EmittedHintCount: len(matches),
	}
	if result.EmittedHintCount > pack.MaxHintCases {
		result.EmittedHintCount = pack.MaxHintCases
	}
	if len(matches) == 0 {
		result.FallbackReason = "no match"
	} else {
		result.Match = &matches[0]
	}
	return renderFactoryPackMatchDebugResult(result), nil
}

type factoryPackMatchDebugResult struct {
	PackLoadStatus   string
	ManifestSummary  string
	CandidateCount   int
	EmittedHintCount int
	Match            *pack.CaseMatch
	FallbackReason   string
}

func renderFactoryPackMatchDebugResult(result factoryPackMatchDebugResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "factory pack match-debug:\npack_load_status: %s", result.PackLoadStatus)
	if result.ManifestSummary != "" {
		fmt.Fprintf(&b, "\nmanifest_summary: %s", result.ManifestSummary)
	}
	fmt.Fprintf(&b, "\ncandidate_count: %d\nemitted_hint_count: %d", result.CandidateCount, result.EmittedHintCount)
	if result.Match != nil {
		fmt.Fprintf(&b, "\nmatched_case_id: %s\nscore: %d", result.Match.CaseID, result.Match.Score)
		for _, reason := range result.Match.WhyMatched {
			reason = strings.TrimSpace(reason)
			if reason != "" {
				fmt.Fprintf(&b, "\nwhy_matched: %s", reason)
			}
		}
	}
	if result.FallbackReason != "" {
		fmt.Fprintf(&b, "\nfallback_reason: %s", result.FallbackReason)
	}
	return b.String()
}

func factoryPackManifestSummary(manifest pack.Manifest) string {
	return fmt.Sprintf("%s schema=%s card_schema=%s cases=%d", manifest.PackName, manifest.SchemaVersion, manifest.CardSchemaVersion, manifest.CompiledCaseCount)
}

func factoryMatchDebugContext(text string) pack.DiagnosticContext {
	text = strings.TrimSpace(text)
	return pack.DiagnosticContext{
		Command:   "/diagnose",
		UserInput: text,
		Signals: pack.DiagnosticSignals{
			MainError: text,
			Keywords:  factoryMatchDebugKeywords(text),
		},
	}
}

func factoryMatchDebugKeywords(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(r == '_' || r == '-' || r == '.' || r == '/' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z')
	})
	seen := map[string]bool{}
	keywords := make([]string, 0, 12)
	for _, field := range fields {
		field = strings.Trim(field, ".,;:()[]{}<>\"'")
		if len(field) < 3 || seen[field] {
			continue
		}
		seen[field] = true
		keywords = append(keywords, field)
		if len(keywords) == 12 {
			break
		}
	}
	return keywords
}
