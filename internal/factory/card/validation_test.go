package card

import (
	"strings"
	"testing"
)

func TestValidateDraft(t *testing.T) {
	if err := ValidateDraft(validDraftTestCard()); err != nil {
		t.Fatalf("ValidateDraft() error = %v", err)
	}
}

func TestValidateStable(t *testing.T) {
	if err := ValidateStable(validStableTestCard()); err != nil {
		t.Fatalf("ValidateStable() error = %v", err)
	}
}

func TestValidatePackEligible(t *testing.T) {
	card := validStableTestCard()
	if err := ValidatePackEligible(card); err != nil {
		t.Fatalf("ValidatePackEligible() error = %v", err)
	}
	if !IsPackEligible(card) {
		t.Fatalf("IsPackEligible() = false")
	}
}

func TestInvalidDraftCards(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*KnownIssueCard)
		wantErr string
	}{
		{name: "problem type", mutate: func(card *KnownIssueCard) { card.Case.ProblemType = "bad" }, wantErr: "invalid case.problem_type"},
		{name: "stage", mutate: func(card *KnownIssueCard) { card.Case.Stage = "bad" }, wantErr: "invalid case.stage"},
		{name: "framework", mutate: func(card *KnownIssueCard) { card.Case.Environment.Frameworks = []Framework{{Name: "bad"}} }, wantErr: "invalid case.environment.frameworks.name"},
		{name: "regex", mutate: func(card *KnownIssueCard) { card.Match.Regex = []string{"["} }, wantErr: "invalid match.regex"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := validDraftTestCard()
			tt.mutate(card)
			assertErrorContains(t, ValidateDraft(card), tt.wantErr)
		})
	}
}

func TestPackEligibilityFailures(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*KnownIssueCard)
		wantErr string
	}{
		{name: "stable without approval", mutate: func(card *KnownIssueCard) { card.Governance.ReviewStatus = ReviewPending }, wantErr: "stable card requires governance.review_status approved"},
		{name: "stable without match signal", mutate: func(card *KnownIssueCard) { card.Match = Match{} }, wantErr: "pack eligibility requires at least one match signal"},
		{name: "draft excluded", mutate: func(card *KnownIssueCard) { card.Governance.Lifecycle = LifecycleDraft }, wantErr: "governance.lifecycle must be stable"},
		{name: "deprecated excluded", mutate: func(card *KnownIssueCard) { card.Governance.Lifecycle = LifecycleDeprecated }, wantErr: "governance.lifecycle must be stable"},
		{name: "archived excluded", mutate: func(card *KnownIssueCard) { card.Governance.Lifecycle = LifecycleArchived }, wantErr: "governance.lifecycle must be stable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := validStableTestCard()
			tt.mutate(card)
			assertErrorContains(t, ValidatePackEligible(card), tt.wantErr)
			if IsPackEligible(card) {
				t.Fatalf("IsPackEligible() = true")
			}
		})
	}
}

func TestValidatePrivacy(t *testing.T) {
	card := validDraftTestCard()
	card.Guidance.Diagnosis = "The command included access_token=secret-value."
	assertErrorContains(t, ValidatePrivacy(card), "privacy validation failed")
}

func validDraftTestCard() *KnownIssueCard {
	card := validStableTestCard()
	card.Governance = Governance{Confidence: ConfidenceBootstrap, Lifecycle: LifecycleDraft, ReviewStatus: ReviewPending}
	return card
}

func validStableTestCard() *KnownIssueCard {
	return &KnownIssueCard{
		SchemaVersion: SchemaVersionKnownIssueV05,
		Kind:          KindKnownIssue,
		ID:            "validation-card",
		Title:         "torch_npu import fails when CANN is missing",
		Tags:          []string{"ascend"},
		Case: Case{
			ProblemType: ProblemTypeFailure,
			Stage:       StageImport,
			Domain:      DomainTorchNPU,
			Hardware:    HardwareAscend,
			Severity:    SeverityHigh,
			Environment: Environment{Frameworks: []Framework{{Name: FrameworkTorchNPU}}},
		},
		Match: Match{Keywords: []string{"torch_npu"}, Regex: []string{"ImportError.*torch_npu"}},
		Guidance: Guidance{
			Symptom:      "ImportError mentions torch_npu",
			Diagnosis:    "CANN runtime is not visible",
			Verification: "Run python import smoke test",
		},
		Provenance: Provenance{
			References:       []string{"local evidence"},
			ExpectedBehavior: []string{"torch_npu imports"},
		},
		Governance: Governance{
			Confidence:   ConfidenceObserved,
			Lifecycle:    LifecycleStable,
			ReviewStatus: ReviewApproved,
			Rationale:    "manual review passed",
		},
	}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want substring %q", err.Error(), want)
	}
}
