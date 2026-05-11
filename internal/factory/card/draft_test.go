package card

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	factoryruntime "github.com/mindspore-lab/mindspore-cli/internal/factory/runtime"
)

func TestNewDraftFromRunSummaryDefaultsToDraftPendingBootstrap(t *testing.T) {
	card, err := NewDraftFromRunSummary(factoryruntime.LastRunSummary{
		Diagnose: &factoryruntime.DiagnoseRunSummary{
			Topic:              "ImportError torch_npu missing on Ascend",
			UserProblemSummary: "ImportError torch_npu missing on Ascend",
			KeyEvidence:        []string{"ImportError", "torch_npu", "ascend"},
		},
	}, DraftOptions{})
	if err != nil {
		t.Fatalf("NewDraftFromRunSummary() error = %v", err)
	}
	if card.Governance.Lifecycle != LifecycleDraft {
		t.Fatalf("Governance.Lifecycle = %q, want draft", card.Governance.Lifecycle)
	}
	if card.Governance.ReviewStatus != ReviewPending {
		t.Fatalf("Governance.ReviewStatus = %q, want pending", card.Governance.ReviewStatus)
	}
	if card.Governance.Confidence != ConfidenceBootstrap {
		t.Fatalf("Governance.Confidence = %q, want bootstrap", card.Governance.Confidence)
	}
	if card.Case.ProblemType != ProblemTypeFailure {
		t.Fatalf("ProblemType = %q, want failure", card.Case.ProblemType)
	}
	if err := ValidateDraft(card); err != nil {
		t.Fatalf("ValidateDraft() error = %v", err)
	}
}

func TestNewDraftFromRunSummaryUnclearTypeUsesUnknownNeedsReview(t *testing.T) {
	card, err := NewDraftFromRunSummary(factoryruntime.LastRunSummary{
		Diagnose: &factoryruntime.DiagnoseRunSummary{Topic: "unexpected behavior", KeyEvidence: []string{"unexpected behavior"}},
	}, DraftOptions{})
	if err != nil {
		t.Fatalf("NewDraftFromRunSummary() error = %v", err)
	}
	if card.Case.ProblemType != ProblemTypeUnknown {
		t.Fatalf("ProblemType = %q, want unknown", card.Case.ProblemType)
	}
	if !strings.Contains(strings.Join(card.Guidance.DiagnosisDetails, " "), "set to unknown") {
		t.Fatalf("DiagnosisDetails = %#v, want unknown review warning", card.Guidance.DiagnosisDetails)
	}
}

func TestSanitizeDraftCardReplacesHomeAndRejectsSecrets(t *testing.T) {
	home := filepath.Join(string(os.PathSeparator), "home", "alice")
	card, err := NewDraftFromRunSummary(factoryruntime.LastRunSummary{
		Diagnose: &factoryruntime.DiagnoseRunSummary{Topic: home + " ImportError", KeyEvidence: []string{home + "/project"}},
	}, DraftOptions{HomeDir: home})
	if err != nil {
		t.Fatalf("NewDraftFromRunSummary() error = %v", err)
	}
	if strings.Contains(card.Title, home) || !strings.Contains(card.Title, "<HOME>") {
		t.Fatalf("Title = %q, want sanitized home", card.Title)
	}

	_, err = NewDraftFromRunSummary(factoryruntime.LastRunSummary{
		Diagnose: &factoryruntime.DiagnoseRunSummary{Topic: "ImportError", KeyEvidence: []string{"access_token=secret"}},
	}, DraftOptions{})
	if err == nil || !strings.Contains(err.Error(), "privacy validation failed") {
		t.Fatalf("error = %v, want privacy validation failure", err)
	}
}

func TestWriteDraftYAMLDoesNotOverwrite(t *testing.T) {
	card, err := NewDraftFromRunSummary(factoryruntime.LastRunSummary{
		Diagnose: &factoryruntime.DiagnoseRunSummary{Topic: "ImportError torch_npu missing", KeyEvidence: []string{"ImportError"}},
	}, DraftOptions{})
	if err != nil {
		t.Fatalf("NewDraftFromRunSummary() error = %v", err)
	}
	dir := t.TempDir()
	if _, err := WriteDraftYAML(card, dir); err != nil {
		t.Fatalf("first WriteDraftYAML() error = %v", err)
	}
	_, err = WriteDraftYAML(card, dir)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second error = %v, want already exists", err)
	}
}
