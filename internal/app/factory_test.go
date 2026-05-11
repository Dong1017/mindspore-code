package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/card"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/compiler"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
	factoryruntime "github.com/mindspore-lab/mindspore-cli/internal/factory/runtime"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
	"gopkg.in/yaml.v3"
)

func TestCmdFactoryHelpRoutes(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"top", "", "Factory commands:"},
		{"card", "card", "Factory card commands:"},
		{"pack", "pack", "Factory pack commands:"},
		{"unknown top", "unknown", "Factory commands:"},
		{"unknown card", "card publish", "Factory card commands:"},
		{"unknown pack", "pack publish", "Factory pack commands:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := &Application{EventCh: make(chan model.Event, 4)}
			app.cmdFactory(tc.input)
			ev := <-app.EventCh
			if !strings.Contains(ev.Message, tc.want) {
				t.Fatalf("Message = %q, want %q", ev.Message, tc.want)
			}
		})
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
	if !strings.Contains(ev.Message, "created local review item:") {
		t.Fatalf("Message = %q, want local review item", ev.Message)
	}
	if !strings.Contains(ev.Message, "next: /factory card review ") {
		t.Fatalf("Message = %q, want review next step", ev.Message)
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

func TestCmdFactoryCardCreatePreferredCommandWritesDraft(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	app := &Application{
		EventCh: make(chan model.Event, 4),
		latestDiagnoseSummary: &factoryruntime.DiagnoseRunSummary{
			Topic:       "ImportError torch_npu missing on Ascend",
			KeyEvidence: []string{"ImportError", "torch_npu", "ascend"},
		},
	}
	app.cmdFactory("card create")
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "created draft card:") {
		t.Fatalf("Message = %q, want created draft", ev.Message)
	}
	assertDraftContains(t, dir, "torch_npu")
}

func TestCmdFactoryCardCreateBadArgsReturnsUsage(t *testing.T) {
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("card create --from-file card.yaml")
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "Usage: /factory card create") {
		t.Fatalf("Message = %q, want usage", ev.Message)
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

func TestFactoryCardReviewRendersLocalReviewItem(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	cardID := createReviewBundleFromLastRun(t)
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("card review " + cardID)
	ev := <-app.EventCh
	for _, want := range []string{"factory card review:", "case.problem_type:", "validation:", "Manual review required"} {
		if !strings.Contains(ev.Message, want) {
			t.Fatalf("Message = %q, want %q", ev.Message, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "factory", "cards", cardID+".yaml")); !os.IsNotExist(err) {
		t.Fatalf("approved card stat err = %v, want not exist", err)
	}
}

func TestFactoryCardReviewMissingSubmissionReturnsClearError(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("card review missing-card")
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "review submission does not exist") {
		t.Fatalf("Message = %q, want missing submission", ev.Message)
	}
}

func TestFactoryCardReviewApproveWritesApprovedCard(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	cardID := createPackReadyReviewBundle(t)
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("card review " + cardID + " --approve --confidence observed --rationale \"manual review passed\"")
	ev := <-app.EventCh
	for _, want := range []string{"approved local factory card: " + cardID, "governance: lifecycle=stable review_status=approved confidence=observed", "pack build still required: /factory pack build factory/cards <output-pack>"} {
		if !strings.Contains(ev.Message, want) {
			t.Fatalf("Message = %q, want %q", ev.Message, want)
		}
	}
	approved, err := card.LoadFile(filepath.Join(dir, "factory", "cards", cardID+".yaml"))
	if err != nil {
		t.Fatalf("LoadFile(approved) error = %v", err)
	}
	if approved.Governance.Lifecycle != card.LifecycleStable || approved.Governance.ReviewStatus != card.ReviewApproved || approved.Governance.Confidence != card.ConfidenceObserved {
		t.Fatalf("Governance = %+v, want stable approved observed", approved.Governance)
	}
	if approved.Governance.Rationale != "manual review passed" || strings.TrimSpace(approved.Governance.UpdatedAt) == "" {
		t.Fatalf("Governance = %+v, want rationale and updated_at", approved.Governance)
	}
	if _, err := os.Stat(filepath.Join(dir, "factory", "factory-core.pack")); !os.IsNotExist(err) {
		t.Fatalf("pack stat err = %v, want not exist", err)
	}
}

func TestFactoryCardReviewApproveRejectsInvalidArgs(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	cardID := createPackReadyReviewBundle(t)
	for _, input := range []string{
		"card review " + cardID + " --approve --confidence observed",
		"card review " + cardID + " --approve --confidence verified --rationale manual",
		"card review " + cardID + " --approve --confidence observed --rationale \"\"",
	} {
		app := &Application{EventCh: make(chan model.Event, 4)}
		app.cmdFactory(input)
		ev := <-app.EventCh
		if !strings.Contains(ev.Message, "Usage:") && !strings.Contains(ev.Message, "approve review card failed") {
			t.Fatalf("input %q Message = %q, want approval failure", input, ev.Message)
		}
	}
}

func TestFactoryPackBuildExplicitOutputWorks(t *testing.T) {
	sourceDir, err := filepath.Abs(filepath.FromSlash("../factory/compiler/testdata/cards"))
	if err != nil {
		t.Fatalf("resolve source cards: %v", err)
	}
	dir := t.TempDir()
	withWorkingDir(t, dir)
	output := filepath.Join(dir, "built.pack")
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("pack build " + sourceDir + " " + output)
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "built factory pack:") {
		t.Fatalf("Message = %q, want build summary", ev.Message)
	}
	for _, want := range []string{
		"cards_dir: " + sourceDir,
		"output: " + output,
		"pack_name: factory-core",
		"schema_version: 1",
		"card_schema_version: known_issue/v0.5",
		"source_case_count: 6",
		"compiled_case_count: 3",
		"checksum: sha256:",
	} {
		if !strings.Contains(ev.Message, want) {
			t.Fatalf("Message = %q, want substring %q", ev.Message, want)
		}
	}
	if _, err := pack.Load(output); err != nil {
		t.Fatalf("Load(built) error = %v", err)
	}
}

func TestFactoryPackBuildMissingArgsReturnsUsage(t *testing.T) {
	for _, input := range []string{"pack build", "pack build cards"} {
		app := &Application{EventCh: make(chan model.Event, 4)}
		app.cmdFactory(input)
		ev := <-app.EventCh
		if !strings.Contains(ev.Message, "Usage: /factory pack build <cards-dir> <output-pack>") {
			t.Fatalf("input %q Message = %q, want usage", input, ev.Message)
		}
	}
}

func TestFactoryPackBuildTooManyArgsReturnsUsage(t *testing.T) {
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("pack build cards output extra")
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "Usage: /factory pack build <cards-dir> <output-pack>") {
		t.Fatalf("Message = %q, want usage", ev.Message)
	}
}

func TestFactoryPackBuildDoesNotInstallDefaultPack(t *testing.T) {
	sourceDir, err := filepath.Abs(filepath.FromSlash("../factory/compiler/testdata/cards"))
	if err != nil {
		t.Fatalf("resolve source cards: %v", err)
	}
	dir := t.TempDir()
	withWorkingDir(t, dir)
	withHomeDir(t, filepath.Join(dir, "home"))
	output := filepath.Join(dir, "built.pack")
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("pack build " + sourceDir + " " + output)
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "built factory pack:") {
		t.Fatalf("Message = %q, want build summary", ev.Message)
	}
	if _, err := os.Stat(filepath.Join(dir, "home", ".mscli", "factory", "factory-core.pack")); !os.IsNotExist(err) {
		t.Fatalf("default pack stat err = %v, want not exist", err)
	}
}

func TestFactoryPackBuildDoesNotModifyExistingDefaultPack(t *testing.T) {
	sourceDir, err := filepath.Abs(filepath.FromSlash("../factory/compiler/testdata/cards"))
	if err != nil {
		t.Fatalf("resolve source cards: %v", err)
	}
	dir := t.TempDir()
	withWorkingDir(t, dir)
	home := filepath.Join(dir, "home")
	withHomeDir(t, home)
	defaultPack := filepath.Join(home, ".mscli", "factory", "factory-core.pack")
	if err := os.MkdirAll(filepath.Dir(defaultPack), 0o700); err != nil {
		t.Fatalf("create default pack dir: %v", err)
	}
	if err := os.WriteFile(defaultPack, []byte("existing default"), 0o600); err != nil {
		t.Fatalf("write default pack: %v", err)
	}
	output := filepath.Join(dir, "built.pack")
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("pack build " + sourceDir + " " + output)
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "built factory pack:") {
		t.Fatalf("Message = %q, want build summary", ev.Message)
	}
	assertFileContent(t, defaultPack, "existing default")
}

func TestFactoryPackSyncExplicitSourcePathWorks(t *testing.T) {
	sourceDir, err := filepath.Abs(filepath.FromSlash("../factory/compiler/testdata/cards"))
	if err != nil {
		t.Fatalf("resolve source cards: %v", err)
	}
	dir := t.TempDir()
	withWorkingDir(t, dir)
	withHomeDir(t, filepath.Join(dir, "home"))
	source := filepath.Join(dir, "source.pack")
	if _, err := compiler.CompilePack(sourceDir, source); err != nil {
		t.Fatalf("CompilePack() error = %v", err)
	}
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("pack sync " + source)
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "synced factory pack:") {
		t.Fatalf("Message = %q, want sync summary", ev.Message)
	}
	if !strings.Contains(ev.Message, "card_schema_version: known_issue/v0.5") {
		t.Fatalf("Message = %q, want card schema version", ev.Message)
	}
	assertFileExists(t, filepath.Join(dir, "home", ".mscli", "factory", "factory-core.pack"))
}

func TestFactoryPackSyncDoesNotChangeCardCreateOrSubmitBehavior(t *testing.T) {
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
	createEvent := <-app.EventCh
	if !strings.Contains(createEvent.Message, "created draft card:") {
		t.Fatalf("Message = %q, want created draft", createEvent.Message)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "factory", "cards", "drafts", "*.yaml"))
	if err != nil {
		t.Fatalf("glob draft cards: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("draft count = %d, want 1", len(matches))
	}
	app.cmdFactory("card submit " + matches[0])
	submitEvent := <-app.EventCh
	if !strings.Contains(submitEvent.Message, "created local review item:") {
		t.Fatalf("Message = %q, want local review item", submitEvent.Message)
	}
}

func TestFactoryPackSyncNoConfiguredSourceReturnsClearError(t *testing.T) {
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("pack sync")
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "Factory pack source is not configured") {
		t.Fatalf("Message = %q, want configured source error", ev.Message)
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

func withHomeDir(t *testing.T, dir string) {
	t.Helper()
	oldHome := os.Getenv("HOME")
	oldUserProfile := os.Getenv("USERPROFILE")
	if err := os.Setenv("HOME", dir); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	if err := os.Setenv("USERPROFILE", dir); err != nil {
		t.Fatalf("set USERPROFILE: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("USERPROFILE", oldUserProfile)
	})
}

func createReviewBundleFromLastRun(t *testing.T) string {
	t.Helper()
	app := &Application{
		EventCh: make(chan model.Event, 4),
		latestDiagnoseSummary: &factoryruntime.DiagnoseRunSummary{
			Topic:              "ImportError torch_npu missing on Ascend",
			UserProblemSummary: "ImportError torch_npu missing on Ascend",
			KeyEvidence:        []string{"ImportError", "torch_npu", "ascend"},
		},
	}
	app.cmdFactory("card create")
	<-app.EventCh
	matches, err := filepath.Glob(filepath.Join("factory", "cards", "drafts", "*.yaml"))
	if err != nil {
		t.Fatalf("glob draft cards: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("draft count = %d, want 1", len(matches))
	}
	app.cmdFactory("card submit " + matches[0])
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "created local review item:") {
		t.Fatalf("Message = %q, want local review item", ev.Message)
	}
	bundleMatches, err := filepath.Glob(filepath.Join("factory", "submissions", "*"))
	if err != nil {
		t.Fatalf("glob submissions: %v", err)
	}
	if len(bundleMatches) != 1 {
		t.Fatalf("bundle count = %d, want 1", len(bundleMatches))
	}
	return filepath.Base(bundleMatches[0])
}

func createPackReadyReviewBundle(t *testing.T) string {
	t.Helper()
	cardID := createReviewBundleFromLastRun(t)
	cardPath := filepath.Join("factory", "submissions", cardID, "card.yaml")
	loaded, err := card.LoadFile(cardPath)
	if err != nil {
		t.Fatalf("LoadFile(review card) error = %v", err)
	}
	loaded.Match.Keywords = []string{"torch_npu"}
	loaded.Guidance.Verification = "Run python import smoke test"
	loaded.Provenance.References = []string{"local review evidence"}
	loaded.Provenance.ExpectedBehavior = []string{"torch_npu imports"}
	data, err := yaml.Marshal(loaded)
	if err != nil {
		t.Fatalf("marshal review card: %v", err)
	}
	if err := os.WriteFile(cardPath, data, 0o600); err != nil {
		t.Fatalf("write pack ready review card: %v", err)
	}
	return cardID
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("content = %q, want %q", string(data), want)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
}
