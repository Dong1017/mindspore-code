package card

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	factoryruntime "github.com/mindspore-lab/mindspore-cli/internal/factory/runtime"
	"gopkg.in/yaml.v3"
)

func TestSubmitDraftCardCreatesReviewBundle(t *testing.T) {
	cardPath := writeTestDraftCard(t, t.TempDir(), "ImportError torch_npu missing")
	bundle, err := SubmitDraftCard(cardPath, SubmitOptions{OutputRoot: filepath.Join(t.TempDir(), "submissions")})
	if err != nil {
		t.Fatalf("SubmitDraftCard() error = %v", err)
	}
	assertFileExists(t, bundle.CardPath)
	assertFileExists(t, bundle.SummaryPath)
	assertFileExists(t, bundle.ValidationPath)
	validation := readValidationResult(t, bundle.ValidationPath)
	if !validation.Schema.OK || !validation.Privacy.OK || !validation.Draft.OK {
		t.Fatalf("validation = %#v, want schema/privacy/draft ok", validation)
	}
	if validation.PackReadiness.OK || !validation.NotReadyForPack {
		t.Fatalf("pack readiness = %#v not_ready=%v, want non-blocking not ready", validation.PackReadiness, validation.NotReadyForPack)
	}
}

func TestSubmitDraftCardSanitizesCardYAML(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	cardPath := writeTestDraftCardWithOptions(t, dir, "ImportError at "+home, DraftOptions{HomeDir: home})
	bundle, err := SubmitDraftCard(cardPath, SubmitOptions{OutputRoot: filepath.Join(t.TempDir(), "submissions"), HomeDir: home})
	if err != nil {
		t.Fatalf("SubmitDraftCard() error = %v", err)
	}
	data, err := os.ReadFile(bundle.CardPath)
	if err != nil {
		t.Fatalf("read card.yaml: %v", err)
	}
	if strings.Contains(string(data), home) || !strings.Contains(string(data), "<HOME>") {
		t.Fatalf("card.yaml = %q, want sanitized home", string(data))
	}
}

func TestSubmitDraftCardPrivacyFailureWritesNoBundle(t *testing.T) {
	card := validDraftCard(t, "ImportError")
	card.Match.Keywords = append(card.Match.Keywords, "access_token=secret")
	path := writeCardYAML(t, t.TempDir(), card)
	out := filepath.Join(t.TempDir(), "submissions")
	_, err := SubmitDraftCard(path, SubmitOptions{OutputRoot: out})
	if err == nil || !strings.Contains(err.Error(), "privacy validation failed") {
		t.Fatalf("error = %v, want privacy validation failure", err)
	}
	assertNoBundleDirs(t, out)
}

func TestSubmitDraftCardInvalidDraftWritesNoBundle(t *testing.T) {
	card := validDraftCard(t, "ImportError")
	card.Problem.Symptoms = nil
	path := writeCardYAML(t, t.TempDir(), card)
	out := filepath.Join(t.TempDir(), "submissions")
	_, err := SubmitDraftCard(path, SubmitOptions{OutputRoot: out})
	if err == nil || !strings.Contains(err.Error(), "draft validation failed") {
		t.Fatalf("error = %v, want draft validation failure", err)
	}
	assertNoBundleDirs(t, out)
}

func TestSubmitDraftCardMissingPathReturnsClearError(t *testing.T) {
	_, err := SubmitDraftCard(filepath.Join(t.TempDir(), "missing.yaml"), SubmitOptions{OutputRoot: filepath.Join(t.TempDir(), "submissions")})
	if err == nil || !strings.Contains(err.Error(), "card path does not exist") {
		t.Fatalf("error = %v, want missing path", err)
	}
}

func TestSubmitDraftCardDoesNotOverwriteExistingBundle(t *testing.T) {
	cardPath := writeTestDraftCard(t, t.TempDir(), "ImportError torch_npu missing")
	out := filepath.Join(t.TempDir(), "submissions")
	if _, err := SubmitDraftCard(cardPath, SubmitOptions{OutputRoot: out}); err != nil {
		t.Fatalf("first SubmitDraftCard() error = %v", err)
	}
	_, err := SubmitDraftCard(cardPath, SubmitOptions{OutputRoot: out})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second error = %v, want already exists", err)
	}
}

func validDraftCard(t *testing.T, topic string) *KnownIssueCard {
	t.Helper()
	card, err := NewDraftFromRunSummary(factoryruntime.LastRunSummary{
		Diagnose: &factoryruntime.DiagnoseRunSummary{Topic: topic, UserProblemSummary: topic, KeyEvidence: []string{topic}},
	}, DraftOptions{})
	if err != nil {
		t.Fatalf("NewDraftFromRunSummary() error = %v", err)
	}
	return card
}

func writeTestDraftCard(t *testing.T, dir string, topic string) string {
	t.Helper()
	return writeTestDraftCardWithOptions(t, dir, topic, DraftOptions{})
}

func writeTestDraftCardWithOptions(t *testing.T, dir string, topic string, opts DraftOptions) string {
	t.Helper()
	card, err := NewDraftFromRunSummary(factoryruntime.LastRunSummary{
		Diagnose: &factoryruntime.DiagnoseRunSummary{Topic: topic, UserProblemSummary: topic, KeyEvidence: []string{topic}},
	}, opts)
	if err != nil {
		t.Fatalf("NewDraftFromRunSummary() error = %v", err)
	}
	return writeCardYAML(t, dir, card)
}

func writeCardYAML(t *testing.T, dir string, card *KnownIssueCard) string {
	t.Helper()
	path := filepath.Join(dir, card.ID+".yaml")
	data, err := yaml.Marshal(card)
	if err != nil {
		t.Fatalf("marshal card: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write card yaml: %v", err)
	}
	return path
}

func readValidationResult(t *testing.T, path string) ValidationResult {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read validation: %v", err)
	}
	var result ValidationResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal validation: %v", err)
	}
	return result
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func assertNoBundleDirs(t *testing.T, out string) {
	t.Helper()
	entries, err := os.ReadDir(out)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatalf("read output root: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("bundle dirs = %d, want 0", len(entries))
	}
}
