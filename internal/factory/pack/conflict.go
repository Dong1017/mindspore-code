package pack

import "strings"

func hasHardConflict(c matchCase, fp Fingerprint) bool {
	if explicitNegativeConflict(c, fp.Signals.NegativeSignals) {
		return true
	}
	if conflictingKnown(c.Stage, fp.Stage) && !hasPositiveTextMatch(c, fp) {
		return true
	}
	if conflictingKnown(c.ProblemType, fp.ProblemType) && !hasPositiveTextMatch(c, fp) {
		return true
	}
	return false
}

func hasWeakConflict(c matchCase, fp Fingerprint) bool {
	searchText := fingerprintSearchText(fp)
	for _, pattern := range c.NegativePatterns {
		if containsAny(searchText, pattern) {
			return true
		}
	}
	return false
}

func explicitNegativeConflict(c matchCase, signals []string) bool {
	for _, pattern := range c.NegativePatterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		for _, signal := range signals {
			signal = strings.ToLower(strings.TrimSpace(signal))
			if signal != "" && strings.Contains(signal, pattern) {
				return true
			}
		}
	}
	return false
}

func hasPositiveTextMatch(c matchCase, fp Fingerprint) bool {
	searchText := fingerprintSearchText(fp)
	for _, keyword := range c.Keywords {
		if containsAny(searchText, keyword) {
			return true
		}
	}
	if firstRegexHit(c.RegexPatterns, searchText) != "" {
		return true
	}
	if firstContainsHit(c.StackPatterns, stackSearchText(fp)) != "" {
		return true
	}
	return false
}

func conflictingKnown(left, right string) bool {
	left = normalizeUnknown(left)
	right = normalizeUnknown(right)
	return left != "unknown" && right != "unknown" && left != right
}
