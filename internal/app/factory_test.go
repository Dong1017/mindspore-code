package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	factoryruntime "github.com/mindspore-lab/mindspore-cli/internal/factory/runtime"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

func TestCmdFactoryUnsupportedCommand(t *testing.T) {
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("pack build")
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "Unsupported /factory command") {
		t.Fatalf("Message = %q, want unsupported command", ev.Message)
	}
}

func TestCmdFactoryCardCreateUsesLatestDiagnoseOnly(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	app := &Application{
		EventCh:       make(chan model.Event, 4),
		latestRunKind: "diagnose",
		latestDiagnoseSummary: &factoryruntime.DiagnoseRunSummary{
			Topic:       "accuracy loss during training",
			KeyEvidence: []string{"accuracy", "loss"},
		},
	}
	app.cmdFactory("card create --from-last-run")
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "created draft card:") {
		t.Fatalf("Message = %q, want created draft", ev.Message)
	}
	assertDraftContains(t, dir, "accuracy")
}

func TestCmdFactoryCardCreateUsesLatestFixOnly(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	app := &Application{
		EventCh:       make(chan model.Event, 4),
		latestRunKind: "fix",
		latestFixSummary: &factoryruntime.FixRunSummary{
			Topic:              "ImportError torch_npu missing",
			UserProblemSummary: "ImportError torch_npu missing",
			PlannedFixSummary:  "source CANN env",
			KeyEvidence:        []string{"ImportError", "torch_npu"},
		},
	}
	app.cmdFactory("card create --from-last-run")
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "created draft card:") {
		t.Fatalf("Message = %q, want created draft", ev.Message)
	}
	assertDraftContains(t, dir, "source CANN env")
}

func TestCmdFactoryCardCreateMergesSameTopic(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	app := &Application{
		EventCh:       make(chan model.Event, 4),
		latestRunKind: "fix",
		latestDiagnoseSummary: &factoryruntime.DiagnoseRunSummary{
			Topic:       "ImportError torch_npu missing",
			KeyEvidence: []string{"diagnose-evidence"},
		},
		latestFixSummary: &factoryruntime.FixRunSummary{
			Topic:              "ImportError torch_npu missing",
			UserProblemSummary: "ImportError torch_npu missing",
			PlannedFixSummary:  "fix-evidence",
			KeyEvidence:        []string{"fix-evidence"},
		},
	}
	app.cmdFactory("card create --from-last-run")
	ev := <-app.EventCh
	if strings.Contains(ev.Message, "topics differ") {
		t.Fatalf("Message = %q, want no warning", ev.Message)
	}
	assertDraftContains(t, dir, "diagnose-evidence")
	assertDraftContains(t, dir, "fix-evidence")
}

func TestCmdFactoryCardCreateUsesLatestDiagnoseWhenTopicsDiffer(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	app := &Application{
		EventCh:       make(chan model.Event, 4),
		latestRunKind: "diagnose",
		latestDiagnoseSummary: &factoryruntime.DiagnoseRunSummary{
			Topic:       "accuracy loss during training",
			KeyEvidence: []string{"accuracy-latest"},
		},
		latestFixSummary: &factoryruntime.FixRunSummary{
			Topic:              "ImportError torch_npu missing",
			UserProblemSummary: "ImportError torch_npu missing",
			PlannedFixSummary:  "fix-old",
			KeyEvidence:        []string{"fix-old"},
		},
	}
	app.cmdFactory("card create --from-last-run")
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "topics differ") {
		t.Fatalf("Message = %q, want topic warning", ev.Message)
	}
	assertDraftContains(t, dir, "accuracy-latest")
	assertDraftNotContains(t, dir, "fix-old")
}

func TestCmdFactoryCardCreateUsesLatestFixWhenTopicsDiffer(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	app := &Application{
		EventCh:       make(chan model.Event, 4),
		latestRunKind: "fix",
		latestDiagnoseSummary: &factoryruntime.DiagnoseRunSummary{
			Topic:       "accuracy loss during training",
			KeyEvidence: []string{"diagnose-old"},
		},
		latestFixSummary: &factoryruntime.FixRunSummary{
			Topic:              "ImportError torch_npu missing",
			UserProblemSummary: "ImportError torch_npu missing",
			PlannedFixSummary:  "fix-latest",
			KeyEvidence:        []string{"fix-latest"},
		},
	}
	app.cmdFactory("card create --from-last-run")
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "topics differ") {
		t.Fatalf("Message = %q, want topic warning", ev.Message)
	}
	assertDraftContains(t, dir, "fix-latest")
	assertDraftNotContains(t, dir, "diagnose-old")
}
func TestCmdFactoryCardSubmitCreatesBundle(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	app := &Application{
		EventCh: make(chan model.Event, 4),
		latestDiagnoseSummary: &factoryruntime.DiagnoseRunSummary{
			Topic:              "ImportError torch_npu missing on Ascend",
			UserProblemSummary: "ImportError torch_npu missing on Ascend",
			KeyEvidence:        []string{"ImportError", "torch_npu", "ascend"},
		},
	}
	app.cmdFactory("card create --from-last-run")
	<-app.EventCh
	matches, err := filepath.Glob(filepath.Join(dir, "factory", "cards", "drafts", "*.yaml"))
	if err != nil {
		t.Fatalf("glob draft cards: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("draft count = %d, want 1", len(matches))
	}

	app.cmdFactory("card submit " + matches[0])
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "created review bundle:") {
		t.Fatalf("Message = %q, want created review bundle", ev.Message)
	}
	bundleMatches, err := filepath.Glob(filepath.Join(dir, "factory", "submissions", "*", "validation.json"))
	if err != nil {
		t.Fatalf("glob validation: %v", err)
	}
	if len(bundleMatches) != 1 {
		t.Fatalf("validation count = %d, want 1", len(bundleMatches))
	}
}

func TestCmdFactoryCardSubmitMissingPathReturnsUsage(t *testing.T) {
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("card submit")
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "Usage: /factory card submit <card-path>") {
		t.Fatalf("Message = %q, want usage", ev.Message)
	}
}

func TestCmdFactoryCardSubmitPrivacyFailureWritesNoBundle(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	app := &Application{
		EventCh: make(chan model.Event, 4),
		latestDiagnoseSummary: &factoryruntime.DiagnoseRunSummary{
			Topic:       "ImportError",
			KeyEvidence: []string{"ImportError"},
		},
	}
	app.cmdFactory("card create --from-last-run")
	<-app.EventCh
	matches, err := filepath.Glob(filepath.Join(dir, "factory", "cards", "drafts", "*.yaml"))
	if err != nil {
		t.Fatalf("glob draft cards: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("draft count = %d, want 1", len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read draft: %v", err)
	}
	data = []byte(strings.Replace(string(data), "    - ImportError\n", "    - password=secret\n", 1))
	if err := os.WriteFile(matches[0], data, 0o600); err != nil {
		t.Fatalf("write unsafe draft: %v", err)
	}

	app.cmdFactory("card submit " + matches[0])
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "privacy validation failed") {
		t.Fatalf("Message = %q, want privacy failure", ev.Message)
	}
	bundleMatches, err := filepath.Glob(filepath.Join(dir, "factory", "submissions", "*"))
	if err != nil {
		t.Fatalf("glob submissions: %v", err)
	}
	if len(bundleMatches) != 0 {
		t.Fatalf("bundle count = %d, want 0", len(bundleMatches))
	}
}

func TestCmdFactoryCardSubmitMissingFileReturnsClearError(t *testing.T) {
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("card submit missing.yaml")
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "card path does not exist") {
		t.Fatalf("Message = %q, want missing path", ev.Message)
	}
}

func TestCmdFactoryCardCreateNoSummaryWritesNothing(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("card create --from-last-run")
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "No latest /diagnose or /fix run summary") {
		t.Fatalf("Message = %q, want no summary", ev.Message)
	}
	if _, err := os.Stat(filepath.Join(dir, "factory")); !os.IsNotExist(err) {
		t.Fatalf("factory dir stat err = %v, want not exist", err)
	}
}

func TestCmdFactoryCardCreateFromLastRunWritesDraft(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	app := &Application{
		EventCh: make(chan model.Event, 4),
		latestDiagnoseSummary: &factoryruntime.DiagnoseRunSummary{
			Topic:              "ImportError torch_npu missing on Ascend",
			UserProblemSummary: "ImportError torch_npu missing on Ascend",
			KeyEvidence:        []string{"ImportError", "torch_npu", "ascend"},
		},
	}
	app.cmdFactory("card create --from-last-run")
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "created draft card:") {
		t.Fatalf("Message = %q, want created draft", ev.Message)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "factory", "cards", "drafts", "*.yaml"))
	if err != nil {
		t.Fatalf("glob draft cards: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("draft count = %d, want 1", len(matches))
	}
}

func TestCmdFactoryCardCreatePrivacyFailureWritesNothing(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	app := &Application{
		EventCh: make(chan model.Event, 4),
		latestDiagnoseSummary: &factoryruntime.DiagnoseRunSummary{
			Topic:       "ImportError",
			KeyEvidence: []string{"password=secret"},
		},
	}
	app.cmdFactory("card create --from-last-run")
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "privacy validation failed") {
		t.Fatalf("Message = %q, want privacy failure", ev.Message)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "factory", "cards", "drafts", "*.yaml"))
	if err != nil {
		t.Fatalf("glob draft cards: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("draft count = %d, want 0", len(matches))
	}
}

func TestStoreFixRunSummaryDoesNotWriteFiles(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.storeFixRunSummary("Load skill failure-agent in fix mode.\n\nUser problem: ImportError torch_npu missing", issueCommandTarget{Prompt: "ImportError torch_npu missing"})
	if app.latestFixSummary == nil {
		t.Fatal("latestFixSummary = nil, want summary")
	}
	if _, err := os.Stat(filepath.Join(dir, "factory")); !os.IsNotExist(err) {
		t.Fatalf("factory dir stat err = %v, want not exist", err)
	}
}

func assertDraftContains(t *testing.T, dir string, want string) {
	t.Helper()
	data := readOnlyDraft(t, dir)
	if !strings.Contains(data, want) {
		t.Fatalf("draft = %q, want substring %q", data, want)
	}
}

func assertDraftNotContains(t *testing.T, dir string, forbidden string) {
	t.Helper()
	data := readOnlyDraft(t, dir)
	if strings.Contains(data, forbidden) {
		t.Fatalf("draft = %q, want no substring %q", data, forbidden)
	}
}

func readOnlyDraft(t *testing.T, dir string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "factory", "cards", "drafts", "*.yaml"))
	if err != nil {
		t.Fatalf("glob draft cards: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("draft count = %d, want 1", len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read draft: %v", err)
	}
	return string(data)
}

func withWorkingDir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
}
