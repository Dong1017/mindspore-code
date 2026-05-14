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
		name         string
		input        string
		want         string
		placeholders []string
	}{
		{"top", "", "Factory commands:", []string{"{card-path}", "{card-id}", "{cards-dir}", "{output-pack}", "{diagnose text}"}},
		{"card", "card", "Factory card commands:", []string{"{card-path}", "{card-id}"}},
		{"pack", "pack", "Factory pack commands:", []string{"{cards-dir}", "{output-pack}", "{diagnose text}"}},
		{"unknown top", "unknown", "Factory commands:", []string{"{card-path}", "{card-id}", "{cards-dir}", "{output-pack}", "{diagnose text}"}},
		{"unknown card", "card publish", "Factory card commands:", []string{"{card-path}", "{card-id}"}},
		{"unknown pack", "pack publish", "Factory pack commands:", []string{"{cards-dir}", "{output-pack}", "{diagnose text}"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := &Application{EventCh: make(chan model.Event, 4)}
			app.cmdFactory(tc.input)
			ev := <-app.EventCh
			if !ev.RawANSI {
				t.Fatalf("RawANSI = false, want true for multiline factory help")
			}
			for _, want := range append([]string{tc.want}, tc.placeholders...) {
				if !strings.Contains(ev.Message, want) {
					t.Fatalf("Message = %q, want %q", ev.Message, want)
				}
			}
			if strings.Contains(ev.Message, "<card-path>") || strings.Contains(ev.Message, "<card-id>") || strings.Contains(ev.Message, "<cards-dir>") || strings.Contains(ev.Message, "<output-pack>") || strings.Contains(ev.Message, "<diagnose text>") {
				t.Fatalf("Message = %q, should not contain angle placeholder", ev.Message)
			}
		})
	}
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
	if !strings.Contains(ev.Message, "Usage: /factory card submit {card-path}") {
		t.Fatalf("Message = %q, want usage", ev.Message)
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

func TestFactoryCardReviewApproveWritesApprovedCard(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	cardID := createPackReadyReviewBundle(t)
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("card review " + cardID + " --approve --confidence observed --rationale \"manual review passed\"")
	ev := <-app.EventCh
	for _, want := range []string{"approved local factory card: " + cardID, "governance: lifecycle=stable review_status=approved confidence=observed", "pack build still required: /factory pack build factory/cards {output-pack}"} {
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

func TestFactoryPackBuildRoutes(t *testing.T) {
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
	if len(strings.Split(ev.Message, "\n")) < 2 {
		t.Fatalf("Message = %q, want multiline output", ev.Message)
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

func TestFactoryPackBuildBadArgsReturnUsage(t *testing.T) {
	for _, input := range []string{"pack build", "pack build cards", "pack build cards output extra"} {
		app := &Application{EventCh: make(chan model.Event, 4)}
		app.cmdFactory(input)
		ev := <-app.EventCh
		if !strings.Contains(ev.Message, "Usage: /factory pack build {cards-dir} {output-pack}") {
			t.Fatalf("input %q Message = %q, want usage", input, ev.Message)
		}
	}
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
	if len(strings.Split(ev.Message, "\n")) < 2 {
		t.Fatalf("Message = %q, want multiline output", ev.Message)
	}
	if !strings.Contains(ev.Message, "card_schema_version: known_issue/v0.5") {
		t.Fatalf("Message = %q, want card schema version", ev.Message)
	}
	assertFileExists(t, filepath.Join(dir, "home", ".mscli", "factory", "factory-core.pack"))
}

func TestFactoryPackSyncNoConfiguredSourceReturnsClearError(t *testing.T) {
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("pack sync")
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "Factory pack source is not configured") {
		t.Fatalf("Message = %q, want configured source error", ev.Message)
	}
}

func TestFactoryPackMatchDebugShowsMatch(t *testing.T) {
	sourceDir, err := filepath.Abs(filepath.FromSlash("../factory/compiler/testdata/cards"))
	if err != nil {
		t.Fatalf("resolve source cards: %v", err)
	}
	dir := t.TempDir()
	withHomeDir(t, filepath.Join(dir, "home"))
	if _, err := compiler.CompilePack(sourceDir, filepath.Join(dir, "home", ".mscli", "factory", "factory-core.pack")); err != nil {
		t.Fatalf("CompilePack() error = %v", err)
	}
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("pack match-debug \"ImportError: torch_npu failed because CANN runtime dependency is missing\"")
	ev := <-app.EventCh
	for _, want := range []string{
		"factory pack match-debug:",
		"pack_load_status: loaded",
		"manifest_summary: factory-core schema=1 card_schema=known_issue/v0.5 cases=3",
		"candidate_count:",
		"emitted_hint_count:",
		"matched_case_id: stable-ascend-import",
		"score:",
		"why_matched:",
	} {
		if !strings.Contains(ev.Message, want) {
			t.Fatalf("Message = %q, want %q", ev.Message, want)
		}
	}
	for _, forbidden := range []string{"schema_version:", "CREATE TABLE", "INSERT INTO", "ImportError: torch_npu failed because CANN runtime dependency is missing"} {
		if strings.Contains(ev.Message, forbidden) {
			t.Fatalf("Message = %q, should not contain %q", ev.Message, forbidden)
		}
	}
}

func TestFactoryPackMatchDebugNoMatch(t *testing.T) {
	sourceDir, err := filepath.Abs(filepath.FromSlash("../factory/compiler/testdata/cards"))
	if err != nil {
		t.Fatalf("resolve source cards: %v", err)
	}
	dir := t.TempDir()
	withHomeDir(t, filepath.Join(dir, "home"))
	if _, err := compiler.CompilePack(sourceDir, filepath.Join(dir, "home", ".mscli", "factory", "factory-core.pack")); err != nil {
		t.Fatalf("CompilePack() error = %v", err)
	}
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("pack match-debug \"validation labels are imbalanced and accuracy drifts slowly\"")
	ev := <-app.EventCh
	for _, want := range []string{"pack_load_status: loaded", "candidate_count: 0", "emitted_hint_count: 0", "fallback_reason: no match"} {
		if !strings.Contains(ev.Message, want) {
			t.Fatalf("Message = %q, want %q", ev.Message, want)
		}
	}
}

func TestFactoryPackMatchDebugMissingPack(t *testing.T) {
	dir := t.TempDir()
	withHomeDir(t, filepath.Join(dir, "home"))
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("pack match-debug \"ImportError torch_npu\"")
	ev := <-app.EventCh
	for _, want := range []string{"pack_load_status: failed", "candidate_count: 0", "emitted_hint_count: 0", "fallback_reason: pack load failed"} {
		if !strings.Contains(ev.Message, want) {
			t.Fatalf("Message = %q, want %q", ev.Message, want)
		}
	}
}
func TestFactoryPackMatchDebugBoundsLongInput(t *testing.T) {
	sourceDir, err := filepath.Abs(filepath.FromSlash("../factory/compiler/testdata/cards"))
	if err != nil {
		t.Fatalf("resolve source cards: %v", err)
	}
	dir := t.TempDir()
	withHomeDir(t, filepath.Join(dir, "home"))
	if _, err := compiler.CompilePack(sourceDir, filepath.Join(dir, "home", ".mscli", "factory", "factory-core.pack")); err != nil {
		t.Fatalf("CompilePack() error = %v", err)
	}
	longInput := strings.Repeat("torch_npu ", 200)
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("pack match-debug \"" + longInput + "\"")
	ev := <-app.EventCh
	if strings.Contains(ev.Message, longInput) {
		t.Fatalf("Message echoed full long input")
	}
	if got := len(strings.Split(ev.Message, "\n")); got > 40 {
		t.Fatalf("match-debug output lines = %d, want bounded", got)
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
	loaded.Guidance.Symptom = "ImportError mentions torch_npu on Ascend"
	loaded.Guidance.Diagnosis = "CANN runtime is not visible"
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

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
}
