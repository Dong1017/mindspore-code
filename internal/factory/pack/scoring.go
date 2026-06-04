package pack

import (
	"regexp"
	"sort"
	"strings"
)

const (
	scoreRegexHit           = 5
	scoreStackKeywordHit    = 4
	scoreMainErrorHit       = 4
	scoreKeywordHit         = 2
	scoreKeywordHitMax      = 6
	scoreFrameworkMatch     = 2
	scoreHardwareMatch      = 2
	scoreRuntimeMatch       = 2
	scoreStageMatch         = 1
	scoreProblemTypeMatch   = 1
	scoreWeakConflict       = -4
	scoreMissingMetadata    = -1
	scoreMissingMetadataMax = -3
	scoreBootstrapPenalty   = -2
)

type scoreResult struct {
	score              int
	whyMatched         []string
	missingEvidence    []string
	conflictingSignals []string
	filtered           bool
}

func scoreCase(c matchCase, fp Fingerprint) scoreResult {
	result := scoreResult{}
	if hasHardConflict(c, fp) {
		result.filtered = true
		return result
	}
	searchText := fingerprintSearchText(fp)
	mainErrorText := strings.ToLower(fp.Signals.MainError)

	if regex := firstRegexHit(c.RegexPatterns, searchText); regex != "" {
		result.score += scoreRegexHit
		result.whyMatched = append(result.whyMatched, "regex matched current diagnostic evidence")
	}
	if stack := firstContainsHit(c.StackPatterns, stackSearchText(fp)); stack != "" {
		result.score += scoreStackKeywordHit
		result.whyMatched = append(result.whyMatched, "stack keyword matched: "+stack)
	}
	if main := firstContainsHit(append(c.Keywords, c.RegexPatterns...), []string{mainErrorText}); main != "" && mainErrorText != "" {
		result.score += scoreMainErrorHit
		result.whyMatched = append(result.whyMatched, "main error matched: "+main)
	}

	keywordScore := 0
	for _, keyword := range c.Keywords {
		if containsAny(searchText, keyword) {
			keywordScore += scoreKeywordHit
			result.whyMatched = append(result.whyMatched, "keyword matched: "+keyword)
			if keywordScore >= scoreKeywordHitMax {
				keywordScore = scoreKeywordHitMax
				break
			}
		}
	}
	result.score += keywordScore

	if knownEqual(c.Stage, fp.Stage) {
		result.score += scoreStageMatch
		result.whyMatched = append(result.whyMatched, "stage matched: "+c.Stage)
	}
	if knownEqual(c.ProblemType, fp.ProblemType) {
		result.score += scoreProblemTypeMatch
		result.whyMatched = append(result.whyMatched, "problem type matched: "+c.ProblemType)
	}
	if tagFrameworkMatch(c, fp.Environment.Frameworks) {
		result.score += scoreFrameworkMatch
		result.whyMatched = append(result.whyMatched, "framework matched")
	}
	if tagHardwareMatch(c, fp.Environment.Hardware.Accelerator) {
		result.score += scoreHardwareMatch
		result.whyMatched = append(result.whyMatched, "hardware accelerator matched")
	}
	if runtimeMetadataMatch(c, fp) {
		result.score += scoreRuntimeMatch
		result.whyMatched = append(result.whyMatched, "runtime metadata matched")
	}
	if hasWeakConflict(c, fp) {
		result.score += scoreWeakConflict
		result.conflictingSignals = append(result.conflictingSignals, "negative or partial conflict signal matched")
	}
	missingPenalty, missing := missingMetadataPenalty(fp)
	result.score += missingPenalty
	result.missingEvidence = append(result.missingEvidence, missing...)
	if c.ConfidenceLevel == "bootstrap" {
		result.score += scoreBootstrapPenalty
	}
	return result
}

func firstRegexHit(patterns []string, values []string) string {
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		for _, value := range values {
			if re.MatchString(value) {
				return pattern
			}
		}
	}
	return ""
}

func firstContainsHit(patterns []string, values []string) string {
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		for _, value := range values {
			if strings.Contains(strings.ToLower(value), pattern) {
				return pattern
			}
		}
	}
	return ""
}

func containsAny(values []string, pattern string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	if pattern == "" {
		return false
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), pattern) {
			return true
		}
	}
	return false
}

func fingerprintSearchText(fp Fingerprint) []string {
	values := []string{fp.Signals.UserInput, fp.Signals.MainError, fp.Signals.TracebackTail, fp.Signals.LogTail}
	values = append(values, fp.Signals.Keywords...)
	values = append(values, fp.Signals.StackKeywords...)
	return lowerNonEmpty(values)
}

func stackSearchText(fp Fingerprint) []string {
	values := []string{fp.Signals.TracebackTail, fp.Signals.MainError}
	values = append(values, fp.Signals.StackKeywords...)
	return lowerNonEmpty(values)
}

func lowerNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func knownEqual(left, right string) bool {
	left = normalizeUnknown(left)
	right = normalizeUnknown(right)
	return left != "unknown" && right != "unknown" && left == right
}

func tagFrameworkMatch(c matchCase, frameworks []FingerprintFramework) bool {
	cardFrameworks := lowerSet(c.Metadata["framework.name"])
	for _, framework := range frameworks {
		if framework.Name == "unknown" {
			continue
		}
		if _, ok := cardFrameworks[framework.Name]; ok {
			return true
		}
	}
	return false
}

func tagHardwareMatch(c matchCase, accelerator string) bool {
	accelerator = normalizeUnknown(accelerator)
	if accelerator == "unknown" {
		return false
	}
	for _, value := range c.Metadata["hardware.accelerator"] {
		if strings.EqualFold(value, accelerator) {
			return true
		}
	}
	return false
}

func lowerSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			set[value] = struct{}{}
		}
	}
	return set
}

func runtimeMetadataMatch(c matchCase, fp Fingerprint) bool {
	if fp.Environment.Runtime.CANNVersion == "" {
		return false
	}
	for _, value := range c.Metadata["runtime.cann_version"] {
		if value == fp.Environment.Runtime.CANNVersion {
			return true
		}
	}
	return containsAny(c.Keywords, fp.Environment.Runtime.CANNVersion) || containsAny(c.Tags, fp.Environment.Runtime.CANNVersion)
}

func missingMetadataPenalty(fp Fingerprint) (int, []string) {
	var missing []string
	if fp.Signals.MainError == "" {
		missing = append(missing, "main_error is unknown")
	}
	if fp.Stage == "unknown" {
		missing = append(missing, "stage is unknown")
	}
	if fp.ProblemType == "unknown" {
		missing = append(missing, "problem_type is unknown")
	}
	if fp.Environment.Hardware.Accelerator == "unknown" {
		missing = append(missing, "hardware accelerator is unknown")
	}
	penalty := len(missing) * scoreMissingMetadata
	if penalty < scoreMissingMetadataMax {
		penalty = scoreMissingMetadataMax
	}
	return penalty, missing
}

func sortMatches(matches []CaseMatch) {
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].CaseID < matches[j].CaseID
	})
}
