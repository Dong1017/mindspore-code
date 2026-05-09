package card

import (
	"fmt"
	"regexp"
	"strings"
)

func ValidateDraft(card *KnownIssueCard) error {
	if card == nil {
		return fmt.Errorf("card is nil")
	}
	if err := validateCommon(card); err != nil {
		return err
	}
	if err := ValidatePrivacy(card); err != nil {
		return err
	}
	return nil
}

func ValidateStable(card *KnownIssueCard) error {
	if err := ValidateDraft(card); err != nil {
		return err
	}
	if card.Lifecycle.State != LifecycleStable {
		return fmt.Errorf("lifecycle.state must be stable")
	}
	if card.Review.Status != ReviewApproved {
		return fmt.Errorf("stable card requires review.status approved")
	}
	if strings.TrimSpace(card.Diagnosis.RootCause) == "" {
		return fmt.Errorf("diagnosis.root_cause is required for stable card")
	}
	if strings.TrimSpace(card.Diagnosis.ScopeNote) == "" {
		return fmt.Errorf("diagnosis.scope_note is required for stable card")
	}
	if len(nonEmptyStrings(card.Diagnosis.SuggestedNextChecks)) == 0 {
		return fmt.Errorf("diagnosis.suggested_next_checks is required for stable card")
	}
	if len(nonEmptyStrings(card.Verification.Checks)) == 0 {
		return fmt.Errorf("verification.checks is required for stable card")
	}
	if strings.TrimSpace(card.Verification.ExpectedResult) == "" {
		return fmt.Errorf("verification.expected_result is required for stable card")
	}
	if len(nonEmptyStrings(card.Provenance.References)) == 0 {
		return fmt.Errorf("provenance.references is required for stable card")
	}
	if strings.TrimSpace(card.Confidence.Rationale) == "" {
		return fmt.Errorf("confidence.rationale is required for stable card")
	}
	return nil
}

func ValidatePackEligible(card *KnownIssueCard) error {
	if err := ValidateStable(card); err != nil {
		return err
	}
	if !hasMatchSignal(card.Match) {
		return fmt.Errorf("pack eligibility requires at least one match signal")
	}
	return nil
}

func IsPackEligible(card *KnownIssueCard) bool {
	return ValidatePackEligible(card) == nil
}

func ValidatePrivacy(card *KnownIssueCard) error {
	values := collectPrivacyText(card)
	for _, value := range values {
		lower := strings.ToLower(value)
		for _, forbidden := range forbiddenPrivacyTerms {
			if strings.Contains(lower, forbidden) {
				return fmt.Errorf("privacy validation failed: contains %s", forbidden)
			}
		}
	}
	return nil
}

var forbiddenPrivacyTerms = []string{
	"api_key",
	"apikey",
	"access_token",
	"password",
	"passwd",
	"private key",
	"begin rsa private key",
	"begin openSSH private key",
	"secret=",
	"token=",
}

func validateCommon(card *KnownIssueCard) error {
	if strings.TrimSpace(card.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if card.Kind != KindKnownIssue {
		return fmt.Errorf("kind must be %s", KindKnownIssue)
	}
	if strings.TrimSpace(card.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if !validProblemType(card.Problem.ProblemType) {
		return fmt.Errorf("invalid problem.problem_type: %s", card.Problem.ProblemType)
	}
	if !validStage(card.Problem.Stage) {
		return fmt.Errorf("invalid problem.stage: %s", card.Problem.Stage)
	}
	if len(nonEmptyStrings(card.Problem.Symptoms)) == 0 {
		return fmt.Errorf("problem.symptoms is required")
	}
	if strings.TrimSpace(card.Diagnosis.Explanation) == "" {
		return fmt.Errorf("diagnosis.explanation is required")
	}
	if !validConfidenceLevel(card.Confidence.Level) {
		return fmt.Errorf("invalid confidence.level: %s", card.Confidence.Level)
	}
	if !validLifecycleState(card.Lifecycle.State) {
		return fmt.Errorf("invalid lifecycle.state: %s", card.Lifecycle.State)
	}
	if strings.TrimSpace(card.Lifecycle.Reason) == "" {
		return fmt.Errorf("lifecycle.reason is required")
	}
	if len(nonEmptyStrings(card.Tags)) == 0 {
		return fmt.Errorf("tags is required")
	}
	if card.Review.Status != "" && !validReviewStatus(card.Review.Status) {
		return fmt.Errorf("invalid review.status: %s", card.Review.Status)
	}
	for _, framework := range card.Environment.Frameworks {
		if strings.TrimSpace(framework.Name) == "" {
			continue
		}
		if !validFrameworkName(framework.Name) {
			return fmt.Errorf("invalid environment.frameworks.name: %s", framework.Name)
		}
	}
	if err := validateRegexList("match.regex", card.Match.Regex); err != nil {
		return err
	}
	if err := validateRegexList("match.negative_patterns", card.Match.NegativePatterns); err != nil {
		return err
	}
	return nil
}

func validProblemType(value string) bool {
	switch value {
	case ProblemTypeFailure, ProblemTypeAccuracy, ProblemTypePerformance:
		return true
	default:
		return false
	}
}

func validStage(value string) bool {
	switch value {
	case StageSetup, StageImport, StageTrain, StageEval, StageInfer, StageCompile, StageData, StageGraphOpt, StageExecution, StageUnknown:
		return true
	default:
		return false
	}
}

func validFrameworkName(value string) bool {
	switch value {
	case FrameworkTorch, FrameworkTorchNPU, FrameworkMindSpore, FrameworkUnknown:
		return true
	default:
		return false
	}
}

func validConfidenceLevel(value string) bool {
	switch value {
	case ConfidenceBootstrap, ConfidenceObserved, ConfidenceVerified:
		return true
	default:
		return false
	}
}

func validLifecycleState(value string) bool {
	switch value {
	case LifecycleDraft, LifecycleStable, LifecycleDeprecated, LifecycleArchived:
		return true
	default:
		return false
	}
}

func validReviewStatus(value string) bool {
	switch value {
	case ReviewPending, ReviewApproved, ReviewRejected:
		return true
	default:
		return false
	}
}

func validateRegexList(field string, patterns []string) error {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid %s: %w", field, err)
		}
	}
	return nil
}

func hasMatchSignal(match Match) bool {
	return len(nonEmptyStrings(match.Keywords)) > 0 || len(nonEmptyStrings(match.Regex)) > 0 || len(nonEmptyStrings(match.StackKeywords)) > 0
}

func nonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func collectPrivacyText(card *KnownIssueCard) []string {
	if card == nil {
		return nil
	}
	values := []string{
		card.ID,
		card.Title,
		card.Diagnosis.RootCause,
		card.Diagnosis.Explanation,
		card.Diagnosis.ScopeNote,
		card.Fix.Summary,
		card.Fix.Template,
		card.Fix.WhyItWorks,
		card.Verification.ExpectedResult,
		card.Provenance.Notes,
		card.Confidence.Rationale,
		card.Lifecycle.Reason,
		card.Review.ReviewerNotes,
	}
	values = append(values, card.Problem.Symptoms...)
	values = append(values, card.Match.Keywords...)
	values = append(values, card.Match.Regex...)
	values = append(values, card.Match.StackKeywords...)
	values = append(values, card.Match.NegativePatterns...)
	values = append(values, card.Diagnosis.SuggestedNextChecks...)
	values = append(values, card.Diagnosis.MissingEvidence...)
	values = append(values, card.Diagnosis.ConflictingSignals...)
	values = append(values, card.Fix.Steps...)
	values = append(values, card.Verification.Checks...)
	values = append(values, card.Verification.Commands...)
	values = append(values, card.Verification.RegressionTests...)
	values = append(values, card.Provenance.References...)
	values = append(values, card.Tags...)
	return values
}
