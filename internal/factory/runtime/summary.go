package runtime

type DiagnoseRunSummary struct {
	Command            string
	Topic              string
	UserProblemSummary string
	KeyEvidence        []string
	FactoryHintsUsed   []FactoryHintSummary
	Privacy            SummaryPrivacy
}

type FactoryHintSummary struct {
	CaseID     string
	Title      string
	WhyMatched []string
}

type SummaryPrivacy struct {
	RawLogsIncluded      bool
	SensitiveEnvIncluded bool
}

func BoundDiagnoseRunSummary(summary DiagnoseRunSummary) DiagnoseRunSummary {
	summary.KeyEvidence = limitStrings(summary.KeyEvidence, 8)
	if len(summary.FactoryHintsUsed) > 3 {
		summary.FactoryHintsUsed = summary.FactoryHintsUsed[:3]
	}
	for i := range summary.FactoryHintsUsed {
		summary.FactoryHintsUsed[i].WhyMatched = limitStrings(summary.FactoryHintsUsed[i].WhyMatched, 3)
	}
	summary.Topic = trimApproxTokens(summary.Topic, 80)
	summary.UserProblemSummary = trimApproxTokens(summary.UserProblemSummary, 240)
	return summary
}

func limitStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}
