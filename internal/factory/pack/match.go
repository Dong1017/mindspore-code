package pack

import "fmt"

type MatchOptions struct {
	TopK               int
	MinimumScoreToEmit int
}

type CaseMatch struct {
	CaseID               string
	Title                string
	ConfidenceLevel      string
	Score                int
	WhyMatched           []string
	SuggestedNextChecks  []string
	SuggestedFixTemplate string
	Verification         []string
	MissingEvidence      []string
	ConflictingSignals   []string
}

func (p *Pack) MatchCases(fp Fingerprint, opts MatchOptions) ([]CaseMatch, error) {
	if p == nil {
		return nil, fmt.Errorf("pack is nil")
	}
	if err := requirePath(p.Path); err != nil {
		return nil, err
	}
	cases, err := readMatchCases(p.Path)
	if err != nil {
		return nil, err
	}
	topK := opts.TopK
	if topK <= 0 {
		topK = DefaultTopK
	}
	minimumScore := opts.MinimumScoreToEmit
	if minimumScore <= 0 {
		minimumScore = DefaultMinimumScoreToEmit
	}

	matches := make([]CaseMatch, 0, len(cases))
	for _, c := range cases {
		result := scoreCase(c, fp)
		if result.filtered || result.score < minimumScore {
			continue
		}
		matches = append(matches, CaseMatch{
			CaseID:               c.ID,
			Title:                c.Title,
			ConfidenceLevel:      c.ConfidenceLevel,
			Score:                result.score,
			WhyMatched:           result.whyMatched,
			SuggestedNextChecks:  c.Advice["next_checks"],
			SuggestedFixTemplate: firstAdvice(c.Advice, "fix_template", "fix_summary"),
			Verification:         append(c.Advice["verification"], c.Advice["expected_result"]...),
			MissingEvidence:      append(result.missingEvidence, c.Advice["missing_evidence"]...),
			ConflictingSignals:   append(result.conflictingSignals, c.Advice["conflicts"]...),
		})
	}
	sortMatches(matches)
	if len(matches) > topK {
		matches = matches[:topK]
	}
	return matches, nil
}

func firstAdvice(advice map[string][]string, keys ...string) string {
	for _, key := range keys {
		values := advice[key]
		if len(values) > 0 {
			return values[0]
		}
	}
	return ""
}
