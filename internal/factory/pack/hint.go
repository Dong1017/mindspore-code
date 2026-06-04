package pack

import (
	"fmt"
	"strings"
)

func RenderFactoryHintBlock(matches []CaseMatch) (string, error) {
	if len(matches) == 0 {
		return "", nil
	}
	limit := len(matches)
	if limit > MaxHintCases {
		limit = MaxHintCases
	}

	var b strings.Builder
	b.WriteString("[Factory Diagnostic Hints]\n")
	b.WriteString("Source: factory-core.pack\n")
	b.WriteString("Policy: These are prior diagnostic hints, not confirmed conclusions.\n")
	b.WriteString("Do not treat them as final diagnosis without checking current evidence.\n\n")
	for i := 0; i < limit; i++ {
		candidate := renderHintCase(i+1, matches[i])
		if approxTokens(b.String()+candidate+"\n[/Factory Diagnostic Hints]") > MaxHintBlockTokens {
			break
		}
		b.WriteString(candidate)
		b.WriteString("\n")
	}
	b.WriteString("[/Factory Diagnostic Hints]")
	return b.String(), nil
}

func renderHintCase(index int, match CaseMatch) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Hint %d:\n", index)
	fmt.Fprintf(&b, "- case_id: %s\n", safeLine(match.CaseID))
	fmt.Fprintf(&b, "- title: %s\n", safeLineBudget(match.Title, 40))
	fmt.Fprintf(&b, "- confidence_level: %s\n", safeLine(match.ConfidenceLevel))
	writeList(&b, "why_matched", match.WhyMatched)
	writeList(&b, "suggested_next_checks", match.SuggestedNextChecks)
	if strings.TrimSpace(match.SuggestedFixTemplate) != "" {
		fmt.Fprintf(&b, "- suggested_fix_template: %s\n", safeLineBudget(match.SuggestedFixTemplate, 50))
	}
	writeList(&b, "verification", match.Verification)
	writeList(&b, "missing_evidence", match.MissingEvidence)
	writeList(&b, "conflicting_signals", match.ConflictingSignals)
	return b.String()
}

func writeList(b *strings.Builder, label string, values []string) {
	cleaned := cleanLines(values)
	if len(cleaned) == 0 {
		return
	}
	fmt.Fprintf(b, "- %s:\n", label)
	for _, value := range cleaned {
		fmt.Fprintf(b, "  - %s\n", safeLineBudget(value, 40))
	}
}

func cleanLines(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = safeLine(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func safeLine(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.Join(strings.Fields(value), " ")
	value = redactRawMarkers(value)
	return value
}

func safeLineBudget(value string, maxTokens int) string {
	value = safeLine(value)
	fields := strings.Fields(value)
	if len(fields) <= maxTokens {
		return value
	}
	return strings.Join(fields[:maxTokens], " ")
}

func redactRawMarkers(value string) string {
	for _, marker := range []string{"kind: known_issue", "pattern_type", "CREATE TABLE", "INSERT INTO"} {
		value = strings.ReplaceAll(value, marker, "[redacted]")
	}
	return value
}

func approxTokens(value string) int {
	return len(strings.Fields(value))
}
