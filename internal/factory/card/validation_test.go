package card

import (
	"strings"
	"testing"
)

func TestValidateDraft(t *testing.T) {
	card := loadTestCard(t, "valid_draft.yaml")
	if err := ValidateDraft(card); err != nil {
		t.Fatalf("ValidateDraft() error = %v", err)
	}
}

func TestValidateStable(t *testing.T) {
	card := loadTestCard(t, "valid_stable.yaml")
	if err := ValidateStable(card); err != nil {
		t.Fatalf("ValidateStable() error = %v", err)
	}
}

func TestValidatePackEligible(t *testing.T) {
	card := loadTestCard(t, "valid_stable.yaml")
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
		fixture string
		wantErr string
	}{
		{name: "problem type", fixture: "invalid_problem_type.yaml", wantErr: "invalid problem.problem_type"},
		{name: "stage", fixture: "invalid_stage.yaml", wantErr: "invalid problem.stage"},
		{name: "framework", fixture: "invalid_framework.yaml", wantErr: "invalid environment.frameworks.name"},
		{name: "regex", fixture: "invalid_regex.yaml", wantErr: "invalid match.regex"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := loadTestCard(t, tt.fixture)
			err := ValidateDraft(card)
			assertErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestPackEligibilityFailures(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		wantErr string
	}{
		{name: "stable without approval", fixture: "stable_without_approval.yaml", wantErr: "stable card requires review.status approved"},
		{name: "stable without match signal", fixture: "stable_without_match_signal.yaml", wantErr: "pack eligibility requires at least one match signal"},
		{name: "draft excluded", fixture: "valid_draft.yaml", wantErr: "lifecycle.state must be stable"},
		{name: "deprecated excluded", fixture: "deprecated.yaml", wantErr: "lifecycle.state must be stable"},
		{name: "archived excluded", fixture: "archived.yaml", wantErr: "lifecycle.state must be stable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := loadTestCard(t, tt.fixture)
			err := ValidatePackEligible(card)
			assertErrorContains(t, err, tt.wantErr)
			if IsPackEligible(card) {
				t.Fatalf("IsPackEligible() = true")
			}
		})
	}
}

func TestValidatePrivacy(t *testing.T) {
	card := loadTestCard(t, "valid_draft.yaml")
	card.Diagnosis.Explanation = "The command included access_token=secret-value."
	err := ValidatePrivacy(card)
	assertErrorContains(t, err, "privacy validation failed")
}

func loadTestCard(t *testing.T, name string) *KnownIssueCard {
	t.Helper()
	card, err := LoadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("LoadFile(%q) error = %v", name, err)
	}
	return card
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
