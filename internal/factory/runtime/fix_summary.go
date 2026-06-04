package runtime

import "strings"

type FixRunSummary struct {
	Command            string
	Topic              string
	UserProblemSummary string
	PlannedFixSummary  string
	Verification       []string
	KeyEvidence        []string
	Privacy            SummaryPrivacy
}

type LastRunSummary struct {
	Diagnose *DiagnoseRunSummary
	Fix      *FixRunSummary
	Warning  string
}

type FixRunSummaryInput struct {
	Topic              string
	UserProblemSummary string
	PlannedFixSummary  string
	KeyEvidence        []string
}

func BuildFixRunSummary(input FixRunSummaryInput) FixRunSummary {
	summary := FixRunSummary{
		Command:            "fix",
		Topic:              firstNonEmpty(input.Topic, input.UserProblemSummary),
		UserProblemSummary: input.UserProblemSummary,
		PlannedFixSummary:  input.PlannedFixSummary,
		KeyEvidence:        input.KeyEvidence,
		Privacy: SummaryPrivacy{
			RawLogsIncluded:      false,
			SensitiveEnvIncluded: false,
		},
	}
	return BoundFixRunSummary(summary)
}

func BoundFixRunSummary(summary FixRunSummary) FixRunSummary {
	summary.Topic = trimApproxTokens(summary.Topic, 80)
	summary.UserProblemSummary = trimApproxTokens(summary.UserProblemSummary, 240)
	summary.PlannedFixSummary = trimApproxTokens(summary.PlannedFixSummary, 240)
	summary.KeyEvidence = limitStrings(summary.KeyEvidence, 8)
	summary.Verification = limitStrings(summary.Verification, 4)
	return summary
}

func SelectLastRunSummary(diagnose *DiagnoseRunSummary, fix *FixRunSummary, latestRunKind ...string) LastRunSummary {
	if diagnose == nil && fix == nil {
		return LastRunSummary{}
	}
	if diagnose == nil {
		return LastRunSummary{Fix: fix}
	}
	if fix == nil {
		return LastRunSummary{Diagnose: diagnose}
	}
	if sameSummaryTopic(diagnose.Topic, fix.Topic) {
		return LastRunSummary{Diagnose: diagnose, Fix: fix}
	}
	warning := "latest diagnose and fix topics differ; using latest run summary only"
	if len(latestRunKind) > 0 && latestRunKind[0] == "diagnose" {
		return LastRunSummary{Diagnose: diagnose, Warning: warning}
	}
	return LastRunSummary{Fix: fix, Warning: warning}
}

func sameSummaryTopic(left, right string) bool {
	left = normalizeSummaryTopic(left)
	right = normalizeSummaryTopic(right)
	if left == "" || right == "" {
		return false
	}
	return left == right || strings.Contains(left, right) || strings.Contains(right, left)
}

func normalizeSummaryTopic(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}
