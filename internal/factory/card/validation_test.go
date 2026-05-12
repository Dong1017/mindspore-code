package card

import (
	"os"
	"path/filepath"
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

func TestValidateDraftPermitsPlaceholders(t *testing.T) {
	card := validDraftTestCard()
	card.Case.ProblemType = ProblemTypeUnknown
	card.Case.Stage = StageUnknown
	card.Case.Domain = DomainUnknown
	card.Case.Hardware = HardwareUnknown
	card.Guidance.Symptom = "Draft generated from the latest bounded run summary"
	card.Guidance.Diagnosis = "Draft generated from the latest bounded run summary; review and complete before promotion"
	card.Guidance.Verification = "not verified; reviewer must add validation steps"
	if err := ValidateDraft(card); err != nil {
		t.Fatalf("ValidateDraft() error = %v", err)
	}
}

func TestPackEligibilityFailures(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*KnownIssueCard)
		wantErr string
	}{
		{name: "unknown problem type", mutate: func(card *KnownIssueCard) { card.Case.ProblemType = ProblemTypeUnknown }, wantErr: "unknown case.problem_type"},
		{name: "unknown stage", mutate: func(card *KnownIssueCard) { card.Case.Stage = StageUnknown }, wantErr: "unknown case.stage"},
		{name: "unknown domain", mutate: func(card *KnownIssueCard) { card.Case.Domain = DomainUnknown }, wantErr: "unknown case.domain"},
		{name: "unknown hardware", mutate: func(card *KnownIssueCard) { card.Case.Hardware = HardwareUnknown }, wantErr: "unknown case.hardware"},
		{name: "placeholder symptom", mutate: func(card *KnownIssueCard) {
			card.Guidance.Symptom = "Draft generated from the latest bounded run summary"
		}, wantErr: "placeholder guidance.symptom"},
		{name: "placeholder diagnosis", mutate: func(card *KnownIssueCard) {
			card.Guidance.Diagnosis = "Draft generated from the latest bounded run summary; review and complete before promotion"
		}, wantErr: "placeholder guidance.diagnosis"},
		{name: "placeholder verification", mutate: func(card *KnownIssueCard) {
			card.Guidance.Verification = "not verified; reviewer must add validation steps"
		}, wantErr: "placeholder guidance.verification"},
		{name: "stable without references", mutate: func(card *KnownIssueCard) { card.Provenance.References = nil }, wantErr: "provenance.references"},
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

func TestLoadFileStructuredReferencesError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "card.yaml")
	data := `schema_version: known_issue/v0.5
kind: known_issue
id: validation-card
title: torch_npu import fails when CANN is missing
tags:
  - ascend
case:
  problem_type: failure
  stage: import
  domain: torch_npu
  hardware: ascend
match:
  keywords:
    - torch_npu
guidance:
  symptom: ImportError mentions torch_npu
  diagnosis: CANN runtime is not visible
  verification: Run python import smoke test
provenance:
  references:
    - kind: script
      path: repro.py
  expected_behavior:
    - torch_npu imports
governance:
  confidence: observed
  lifecycle: stable
  review_status: approved
  rationale: manual review passed
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write card: %v", err)
	}
	_, err := LoadFile(path)
	assertErrorContains(t, err, "provenance.references must be a list of strings, not objects")
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
