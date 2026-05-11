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
	if card.Governance.Lifecycle != LifecycleStable {
		return fmt.Errorf("governance.lifecycle must be stable")
	}
	if card.Governance.ReviewStatus != ReviewApproved {
		return fmt.Errorf("stable card requires governance.review_status approved")
	}
	if strings.TrimSpace(card.Guidance.Verification) == "" {
		return fmt.Errorf("guidance.verification is required for stable card")
	}
	if len(nonEmptyStrings(card.Provenance.References)) == 0 {
		return fmt.Errorf("provenance.references is required for stable card")
	}
	if len(nonEmptyStrings(card.Provenance.ExpectedBehavior)) == 0 {
		return fmt.Errorf("provenance.expected_behavior is required for stable card")
	}
	if strings.TrimSpace(card.Governance.Rationale) == "" {
		return fmt.Errorf("governance.rationale is required for stable card")
	}
	return nil
}

func ValidatePackEligible(card *KnownIssueCard) error {
	if err := ValidateStable(card); err != nil {
		return err
	}
	if card.Governance.Confidence != ConfidenceObserved && card.Governance.Confidence != ConfidenceVerified {
		return fmt.Errorf("pack eligibility requires governance.confidence observed or verified")
	}
	if !hasMatchSignal(card.Match) {
		return fmt.Errorf("pack eligibility requires at least one match signal")
	}
	if strings.TrimSpace(card.Guidance.Symptom) == "" {
		return fmt.Errorf("pack eligibility requires guidance.symptom")
	}
	if strings.TrimSpace(card.Guidance.Diagnosis) == "" {
		return fmt.Errorf("pack eligibility requires guidance.diagnosis")
	}
	if strings.TrimSpace(card.Guidance.Verification) == "" {
		return fmt.Errorf("pack eligibility requires guidance.verification")
	}
	if len(nonEmptyStrings(card.Provenance.References)) == 0 {
		return fmt.Errorf("pack eligibility requires provenance.references")
	}
	if len(nonEmptyStrings(card.Provenance.ExpectedBehavior)) == 0 {
		return fmt.Errorf("pack eligibility requires provenance.expected_behavior")
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
	"begin openssh private key",
	"secret=",
	"token=",
}

func validateCommon(card *KnownIssueCard) error {
	if card.SchemaVersion != SchemaVersionKnownIssueV05 {
		return fmt.Errorf("schema_version must be %s", SchemaVersionKnownIssueV05)
	}
	if strings.TrimSpace(card.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if card.Kind != KindKnownIssue {
		return fmt.Errorf("kind must be %s", KindKnownIssue)
	}
	if strings.TrimSpace(card.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if len(nonEmptyStrings(card.Tags)) == 0 {
		return fmt.Errorf("tags is required")
	}
	if !validProblemType(card.Case.ProblemType) {
		return fmt.Errorf("invalid case.problem_type: %s", card.Case.ProblemType)
	}
	if !validStage(card.Case.Stage) {
		return fmt.Errorf("invalid case.stage: %s", card.Case.Stage)
	}
	if !validDomain(card.Case.Domain) {
		return fmt.Errorf("invalid case.domain: %s", card.Case.Domain)
	}
	if !validHardware(card.Case.Hardware) {
		return fmt.Errorf("invalid case.hardware: %s", card.Case.Hardware)
	}
	if card.Case.Severity != "" && !validSeverity(card.Case.Severity) {
		return fmt.Errorf("invalid case.severity: %s", card.Case.Severity)
	}
	if strings.TrimSpace(card.Guidance.Symptom) == "" {
		return fmt.Errorf("guidance.symptom is required")
	}
	if strings.TrimSpace(card.Guidance.Diagnosis) == "" {
		return fmt.Errorf("guidance.diagnosis is required")
	}
	if !validConfidenceLevel(card.Governance.Confidence) {
		return fmt.Errorf("invalid governance.confidence: %s", card.Governance.Confidence)
	}
	if !validLifecycleState(card.Governance.Lifecycle) {
		return fmt.Errorf("invalid governance.lifecycle: %s", card.Governance.Lifecycle)
	}
	if card.Governance.ReviewStatus != "" && !validReviewStatus(card.Governance.ReviewStatus) {
		return fmt.Errorf("invalid governance.review_status: %s", card.Governance.ReviewStatus)
	}
	for _, framework := range card.Case.Environment.Frameworks {
		if strings.TrimSpace(framework.Name) == "" {
			continue
		}
		if !validFrameworkName(framework.Name) {
			return fmt.Errorf("invalid case.environment.frameworks.name: %s", framework.Name)
		}
	}
	if err := validateRegexList("match.regex", card.Match.Regex); err != nil {
		return err
	}
	return nil
}

func validProblemType(value string) bool {
	switch value {
	case ProblemTypeFailure, ProblemTypeAccuracy, ProblemTypePerformance, ProblemTypeUnknown:
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

func validDomain(value string) bool {
	switch value {
	case DomainMindSpore, DomainTorch, DomainTorchNPU, DomainCANN, DomainUnknown:
		return true
	default:
		return false
	}
}

func validHardware(value string) bool {
	switch value {
	case HardwareAscend, HardwareGPU, HardwareCPU, HardwareUnknown:
		return true
	default:
		return false
	}
}

func validSeverity(value string) bool {
	switch value {
	case SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical, SeverityUnknown:
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
	case ReviewPending, ReviewApproved, ReviewRejected, ReviewUnknown:
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
	return len(nonEmptyStrings(match.Keywords)) > 0 || len(nonEmptyStrings(match.Regex)) > 0
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
		card.SchemaVersion,
		card.Kind,
		card.ID,
		card.Title,
		card.Case.ProblemType,
		card.Case.Stage,
		card.Case.Domain,
		card.Case.Hardware,
		card.Case.Severity,
		card.Case.Environment.Runtime.CANNVersion,
		card.Case.Environment.Runtime.PythonVersion,
		card.Case.Environment.Model.Pattern,
		card.Case.Environment.Model.ExecutionMode,
		card.Case.Environment.Model.Optimization,
		card.Case.Environment.Model.InputReuse,
		card.Case.Environment.Model.DType,
		card.Case.Environment.Model.InputShapes.OriginalReport,
		card.Case.Environment.Model.InputShapes.RegressionNote,
		card.Guidance.Symptom,
		card.Guidance.Diagnosis,
		card.Guidance.Fix,
		card.Guidance.Verification,
		card.Provenance.Notes,
		card.Governance.Confidence,
		card.Governance.Lifecycle,
		card.Governance.ReviewStatus,
		card.Governance.Rationale,
		card.Governance.UpdatedAt,
	}
	for _, framework := range card.Case.Environment.Frameworks {
		values = append(values, framework.Name, framework.Version, framework.Branch, framework.Commit)
	}
	values = append(values, card.Tags...)
	values = append(values, card.Case.Environment.Affected...)
	values = append(values, card.Case.Environment.FixedBy...)
	values = append(values, card.Match.Keywords...)
	values = append(values, card.Match.Regex...)
	values = append(values, card.Guidance.TriggerSignals...)
	values = append(values, card.Guidance.RepresentativeErrors...)
	values = append(values, card.Guidance.DiagnosisDetails...)
	values = append(values, card.Guidance.Actions...)
	values = append(values, card.Guidance.WhyItWorks...)
	values = append(values, card.Guidance.NonCauses...)
	values = append(values, card.Provenance.References...)
	values = append(values, card.Provenance.ExpectedBehavior...)
	values = append(values, card.Provenance.RegressionTests...)
	return values
}
