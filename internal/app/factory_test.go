package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitcode.com/mindspore/mscli/internal/factory/compiler"
	"gitcode.com/mindspore/mscli/internal/factory/pack"
	factoryruntime "gitcode.com/mindspore/mscli/internal/factory/runtime"
	"gitcode.com/mindspore/mscli/ui/model"
)

func TestCmdFactoryHelpRoutes(t *testing.T) {
	cases := []struct {
		name         string
		input        string
		want         string
		wantRawANSI  bool
		placeholders []string
	}{
		{"top", "", "Factory commands:", true, []string{"{card-path}", "{card-id}", "{cards-dir}", "{output-pack}", "{source-path}", "{diagnose text}", "/factory status"}},
		{"status", "status unexpected", "Usage: /factory status", false, nil},
		{"card", "card", "Factory card commands:", true, []string{"{card-path}", "{card-id}"}},
		{"pack", "pack", "Factory pack commands:", true, []string{"{cards-dir}", "{output-pack}", "{source-path}", "{diagnose text}"}},
		{"unknown top", "unknown", "Factory commands:", true, []string{"{card-path}", "{card-id}", "{cards-dir}", "{output-pack}", "{source-path}", "{diagnose text}"}},
		{"unknown card", "card publish", "Factory card commands:", true, []string{"{card-path}", "{card-id}"}},
		{"unknown pack", "pack unknown", "Factory pack commands:", true, []string{"{cards-dir}", "{output-pack}", "{source-path}", "{diagnose text}"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := &Application{EventCh: make(chan model.Event, 4)}
			app.cmdFactory(tc.input)
			ev := <-app.EventCh
			if ev.RawANSI != tc.wantRawANSI {
				t.Fatalf("RawANSI = %t, want %t", ev.RawANSI, tc.wantRawANSI)
			}
			assertContainsAll(t, ev.Message, append([]string{tc.want}, tc.placeholders...)...)
			if strings.Contains(ev.Message, "<card-path>") || strings.Contains(ev.Message, "<card-id>") || strings.Contains(ev.Message, "<cards-dir>") || strings.Contains(ev.Message, "<output-pack>") || strings.Contains(ev.Message, "<diagnose text>") {
				t.Fatalf("Message = %q, should not contain angle placeholder", ev.Message)
			}
		})
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
	assertContainsAll(t, ev.Message, "next:", "/factory card submit ")
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

func TestCmdFactoryCardSubmitReviewSmoke(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	cardID := createReviewBundleFromLastRun(t)
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("card review " + cardID)
	ev := <-app.EventCh
	assertContainsAll(t, ev.Message, "factory card review:", "Manual review required", "/factory card review "+cardID+" --approve")
	if _, err := os.Stat(filepath.Join(dir, "factory", "cards", cardID+".yaml")); !os.IsNotExist(err) {
		t.Fatalf("approved card stat err = %v, want not exist", err)
	}
}

func TestFactoryPackBuildAndSyncSmoke(t *testing.T) {
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
	assertContainsAll(t, ev.Message, "built factory pack:", "output: "+output, "/factory pack sync "+output)
	if _, err := pack.Load(output); err != nil {
		t.Fatalf("Load(built) error = %v", err)
	}

	app.cmdFactory("pack sync " + output)
	ev = <-app.EventCh
	assertContainsAll(t, ev.Message, "synced factory pack:", "/factory status")
	assertFileExists(t, filepath.Join(dir, "home", ".mscli", "factory", "factory-core.pack"))
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

func TestFactoryPackSyncRequiresLocalSource(t *testing.T) {
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("pack sync")
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "Usage: /factory pack sync {source-path}") {
		t.Fatalf("Message = %q, want local source usage", ev.Message)
	}
}

func TestFactoryStatusMissingLocalPackAndNoServer(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	withHomeDir(t, filepath.Join(dir, "home"))

	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("status")
	ev := <-app.EventCh
	assertContainsAll(t, ev.Message,
		"factory status:",
		"local_pack_installed: false",
		"local_pack_reason: not installed",
		"draft_cards: 0",
		"review_items: 0",
		"approved_cards: 0",
	)
}

func TestFactoryPackMatchDebugShowsMatch(t *testing.T) {
	source := compileAppTestPack(t)
	dir := t.TempDir()
	withHomeDir(t, filepath.Join(dir, "home"))
	if _, err := pack.Sync(pack.SyncConfig{SourcePath: source}); err != nil {
		t.Fatalf("Sync(source) error = %v", err)
	}
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("pack match-debug \"ImportError: torch_npu failed because CANN runtime dependency is missing\"")
	ev := <-app.EventCh
	assertContainsAll(t, ev.Message, "factory pack match-debug:", "matched_case_id: stable-ascend-import", "why_matched:")
	for _, forbidden := range []string{"schema_version:", "CREATE TABLE", "INSERT INTO", "ImportError: torch_npu failed because CANN runtime dependency is missing"} {
		if strings.Contains(ev.Message, forbidden) {
			t.Fatalf("Message = %q, should not contain %q", ev.Message, forbidden)
		}
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

func assertContainsAll(t *testing.T, message string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(message, want) {
			t.Fatalf("Message = %q, want %q", message, want)
		}
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

func compileAppTestPack(t *testing.T) string {
	t.Helper()
	sourceDir, err := filepath.Abs(filepath.FromSlash("../factory/compiler/testdata/cards"))
	if err != nil {
		t.Fatalf("resolve source cards: %v", err)
	}
	path := filepath.Join(t.TempDir(), "factory-core.pack")
	if _, err := compiler.CompilePack(sourceDir, path); err != nil {
		t.Fatalf("CompilePack() error = %v", err)
	}
	return path
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
}
