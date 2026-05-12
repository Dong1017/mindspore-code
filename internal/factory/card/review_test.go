package card

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestRenderReviewItemReadsBundle(t *testing.T) {
	bundle := writeReviewTestBundle(t, validReviewTestCard())
	view, err := RenderReviewItem(bundle.CardID, ReviewOptions{SubmissionsRoot: filepath.Dir(bundle.Path)})
	if err != nil {
		t.Fatalf("RenderReviewItem() error = %v", err)
	}
	for _, want := range []string{"factory card review:", "case.problem_type: failure", "governance.lifecycle: draft", "not_ready_for_pack: true", "Manual review required"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view = %q, want %q", view, want)
		}
	}
}

func TestRenderReviewItemMissingFilesReturnClearErrors(t *testing.T) {
	bundle := writeReviewTestBundle(t, validReviewTestCard())
	root := filepath.Dir(bundle.Path)
	cases := []struct {
		name string
		path string
		want string
	}{
		{"card", bundle.CardPath, "missing card.yaml"},
		{"summary", bundle.SummaryPath, "missing summary.md"},
		{"validation", bundle.ValidationPath, "missing validation.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			copyBundle := writeReviewTestBundle(t, validReviewTestCard())
			copyRoot := filepath.Dir(copyBundle.Path)
			removePath := filepath.Join(copyRoot, bundle.CardID, filepath.Base(tc.path))
			if err := os.Remove(removePath); err != nil {
				t.Fatalf("remove %s: %v", removePath, err)
			}
			_, err := RenderReviewItem(bundle.CardID, ReviewOptions{SubmissionsRoot: copyRoot})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	_ = root
}

func TestRenderReviewItemMissingSubmissionReturnsClearError(t *testing.T) {
	_, err := RenderReviewItem("missing-card", ReviewOptions{SubmissionsRoot: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "review submission does not exist") {
		t.Fatalf("error = %v, want missing submission", err)
	}
}

func TestRenderReviewItemMalformedValidationReturnsClearError(t *testing.T) {
	bundle := writeReviewTestBundle(t, validReviewTestCard())
	if err := os.WriteFile(bundle.ValidationPath, []byte("{"), 0o600); err != nil {
		t.Fatalf("write malformed validation: %v", err)
	}
	_, err := RenderReviewItem(bundle.CardID, ReviewOptions{SubmissionsRoot: filepath.Dir(bundle.Path)})
	if err == nil || !strings.Contains(err.Error(), "parse review validation.json") {
		t.Fatalf("error = %v, want malformed validation", err)
	}
}

func TestRenderReviewItemIsBoundedAndReadOnly(t *testing.T) {
	bundle := writeReviewTestBundle(t, validReviewTestCard())
	if err := os.WriteFile(bundle.SummaryPath, []byte(strings.Repeat("summary line with many words\n", 200)), 0o600); err != nil {
		t.Fatalf("write long summary: %v", err)
	}
	beforeCard := readFileString(t, bundle.CardPath)
	beforeSummary := readFileString(t, bundle.SummaryPath)
	beforeValidation := readFileString(t, bundle.ValidationPath)
	view, err := RenderReviewItem(bundle.CardID, ReviewOptions{SubmissionsRoot: filepath.Dir(bundle.Path)})
	if err != nil {
		t.Fatalf("RenderReviewItem() error = %v", err)
	}
	if got := len(strings.Split(view, "\n")); got > 45 {
		t.Fatalf("review lines = %d, want bounded output", got)
	}
	if readFileString(t, bundle.CardPath) != beforeCard || readFileString(t, bundle.SummaryPath) != beforeSummary || readFileString(t, bundle.ValidationPath) != beforeValidation {
		t.Fatalf("review modified bundle files")
	}
	approvedPath := filepath.Join(filepath.Dir(filepath.Dir(bundle.Path)), "cards", bundle.CardID+".yaml")
	if _, err := os.Stat(approvedPath); !os.IsNotExist(err) {
		t.Fatalf("approved card stat err = %v, want not exist", err)
	}
}

func TestApproveReviewItemWritesApprovedCard(t *testing.T) {
	bundle := writeReviewTestBundle(t, packReadyReviewTestCard())
	cardsRoot := filepath.Join(t.TempDir(), "cards")
	result, err := ApproveReviewItem(bundle.CardID, ApprovalOptions{SubmissionsRoot: filepath.Dir(bundle.Path), CardsRoot: cardsRoot, Confidence: ConfidenceObserved, Rationale: "manual review passed", Now: time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("ApproveReviewItem() error = %v", err)
	}
	if result.Path != filepath.Join(cardsRoot, bundle.CardID+".yaml") {
		t.Fatalf("Path = %q, want approved path", result.Path)
	}
	approved, err := LoadFile(result.Path)
	if err != nil {
		t.Fatalf("LoadFile(approved) error = %v", err)
	}
	if approved.Governance.Lifecycle != LifecycleStable || approved.Governance.ReviewStatus != ReviewApproved || approved.Governance.Confidence != ConfidenceObserved {
		t.Fatalf("Governance = %+v, want stable approved observed", approved.Governance)
	}
	if approved.Governance.Rationale != "manual review passed" || approved.Governance.UpdatedAt != "2026-05-11T12:00:00Z" {
		t.Fatalf("Governance = %+v, want rationale and updated_at", approved.Governance)
	}
}

func TestApproveReviewItemRejectsInvalidInputs(t *testing.T) {
	bundle := writeReviewTestBundle(t, packReadyReviewTestCard())
	root := filepath.Dir(bundle.Path)
	for _, tc := range []struct {
		name       string
		confidence string
		rationale  string
		want       string
	}{
		{"missing rationale", ConfidenceObserved, "", "non-empty --rationale"},
		{"verified", ConfidenceVerified, "manual", "--confidence observed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ApproveReviewItem(bundle.CardID, ApprovalOptions{SubmissionsRoot: root, CardsRoot: t.TempDir(), Confidence: tc.confidence, Rationale: tc.rationale})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestApproveReviewItemRequiresPackReadiness(t *testing.T) {
	card := packReadyReviewTestCard()
	card.Match.Keywords = nil
	bundle := writeReviewTestBundle(t, card)
	_, err := ApproveReviewItem(bundle.CardID, ApprovalOptions{SubmissionsRoot: filepath.Dir(bundle.Path), CardsRoot: t.TempDir(), Confidence: ConfidenceObserved, Rationale: "manual"})
	if err == nil || !strings.Contains(err.Error(), "pack readiness validation failed") {
		t.Fatalf("error = %v, want pack readiness failure", err)
	}
}

func TestApproveReviewItemDoesNotOverwriteOrModifyBundle(t *testing.T) {
	bundle := writeReviewTestBundle(t, packReadyReviewTestCard())
	cardsRoot := t.TempDir()
	approvedPath := filepath.Join(cardsRoot, bundle.CardID+".yaml")
	if err := os.WriteFile(approvedPath, []byte("existing"), 0o600); err != nil {
		t.Fatalf("write existing approved: %v", err)
	}
	beforeCard := readFileString(t, bundle.CardPath)
	beforeSummary := readFileString(t, bundle.SummaryPath)
	beforeValidation := readFileString(t, bundle.ValidationPath)
	_, err := ApproveReviewItem(bundle.CardID, ApprovalOptions{SubmissionsRoot: filepath.Dir(bundle.Path), CardsRoot: cardsRoot, Confidence: ConfidenceObserved, Rationale: "manual"})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v, want already exists", err)
	}
	if readFileString(t, approvedPath) != "existing" {
		t.Fatalf("approved file was overwritten")
	}
	if readFileString(t, bundle.CardPath) != beforeCard || readFileString(t, bundle.SummaryPath) != beforeSummary || readFileString(t, bundle.ValidationPath) != beforeValidation {
		t.Fatalf("approval modified bundle files")
	}
}

func TestReviewItemRejectsMismatchedCardID(t *testing.T) {
	bundle := writeReviewTestBundle(t, packReadyReviewTestCard())
	mismatched := packReadyReviewTestCard()
	mismatched.ID = "other-review-card"
	if err := writeReviewCardFile(t, bundle.CardPath, mismatched); err != nil {
		t.Fatalf("write mismatched card: %v", err)
	}
	_, err := RenderReviewItem(bundle.CardID, ReviewOptions{SubmissionsRoot: filepath.Dir(bundle.Path)})
	if err == nil || !strings.Contains(err.Error(), "review card id mismatch: bundle review-card card other-review-card") {
		t.Fatalf("error = %v, want id mismatch", err)
	}
}

func TestApproveReviewItemRejectsMismatchedCardIDAndWritesNothing(t *testing.T) {
	bundle := writeReviewTestBundle(t, packReadyReviewTestCard())
	mismatched := packReadyReviewTestCard()
	mismatched.ID = "other-review-card"
	if err := writeReviewCardFile(t, bundle.CardPath, mismatched); err != nil {
		t.Fatalf("write mismatched card: %v", err)
	}
	cardsRoot := t.TempDir()
	_, err := ApproveReviewItem(bundle.CardID, ApprovalOptions{SubmissionsRoot: filepath.Dir(bundle.Path), CardsRoot: cardsRoot, Confidence: ConfidenceObserved, Rationale: "manual"})
	if err == nil || !strings.Contains(err.Error(), "review card id mismatch: bundle review-card card other-review-card") {
		t.Fatalf("error = %v, want id mismatch", err)
	}
	for _, path := range []string{filepath.Join(cardsRoot, bundle.CardID+".yaml"), filepath.Join(cardsRoot, mismatched.ID+".yaml")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("stat %s err = %v, want not exist", path, err)
		}
	}
}

func TestApproveReviewItemRejectsWeakPackReadyFields(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*KnownIssueCard)
		wantErr string
	}{
		{"unknown problem type", func(card *KnownIssueCard) { card.Case.ProblemType = ProblemTypeUnknown }, "unknown case.problem_type"},
		{"unknown stage", func(card *KnownIssueCard) { card.Case.Stage = StageUnknown }, "unknown case.stage"},
		{"unknown domain", func(card *KnownIssueCard) { card.Case.Domain = DomainUnknown }, "unknown case.domain"},
		{"unknown hardware", func(card *KnownIssueCard) { card.Case.Hardware = HardwareUnknown }, "unknown case.hardware"},
		{"placeholder diagnosis", func(card *KnownIssueCard) {
			card.Guidance.Diagnosis = "Draft generated from the latest bounded run summary; review and complete before promotion"
		}, "placeholder guidance.diagnosis"},
		{"placeholder verification", func(card *KnownIssueCard) {
			card.Guidance.Verification = "not verified; reviewer must add validation steps"
		}, "placeholder guidance.verification"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			card := packReadyReviewTestCard()
			tc.mutate(card)
			bundle := writeReviewTestBundle(t, card)
			_, err := ApproveReviewItem(bundle.CardID, ApprovalOptions{SubmissionsRoot: filepath.Dir(bundle.Path), CardsRoot: t.TempDir(), Confidence: ConfidenceObserved, Rationale: "manual"})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func writeReviewTestBundle(t *testing.T, card *KnownIssueCard) *ReviewBundle {
	t.Helper()
	bundle, err := SubmitDraftCard(writeReviewTestDraft(t, card), SubmitOptions{OutputRoot: filepath.Join(t.TempDir(), "submissions")})
	if err != nil {
		t.Fatalf("SubmitDraftCard() error = %v", err)
	}
	return bundle
}

func writeReviewTestDraft(t *testing.T, card *KnownIssueCard) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), card.ID+".yaml")
	if _, err := WriteDraftYAML(card, filepath.Dir(path)); err != nil {
		t.Fatalf("WriteDraftYAML() error = %v", err)
	}
	return path
}

func validReviewTestCard() *KnownIssueCard {
	card := packReadyReviewTestCard()
	card.Governance.Lifecycle = LifecycleDraft
	card.Governance.ReviewStatus = ReviewPending
	card.Governance.Confidence = ConfidenceBootstrap
	card.Governance.Rationale = ""
	card.Governance.UpdatedAt = ""
	return card
}

func packReadyReviewTestCard() *KnownIssueCard {
	return &KnownIssueCard{
		SchemaVersion: SchemaVersionKnownIssueV05,
		Kind:          KindKnownIssue,
		ID:            "review-card",
		Title:         "torch_npu import fails when CANN is missing",
		Tags:          []string{"ascend"},
		Case: Case{
			ProblemType: ProblemTypeFailure,
			Stage:       StageImport,
			Domain:      DomainTorchNPU,
			Hardware:    HardwareAscend,
		},
		Match: Match{Keywords: []string{"torch_npu"}},
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
			Lifecycle:    LifecycleStable,
			ReviewStatus: ReviewApproved,
			Confidence:   ConfidenceObserved,
			Rationale:    "manual",
		},
	}
}

func writeReviewCardFile(t *testing.T, path string, card *KnownIssueCard) error {
	t.Helper()
	data, err := yaml.Marshal(card)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
