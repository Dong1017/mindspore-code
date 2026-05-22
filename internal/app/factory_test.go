package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/compiler"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
	factoryruntime "github.com/mindspore-lab/mindspore-cli/internal/factory/runtime"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

func TestCmdFactoryHelpRoutes(t *testing.T) {
	cases := []struct {
		name         string
		input        string
		want         string
		wantRawANSI  bool
		placeholders []string
	}{
		{"top", "", "Factory commands:", true, []string{"{card-path}", "{card-id}", "{cards-dir}", "{output-pack}", "{pack-path}", "[{source-path}]", "{diagnose text}", "/factory status"}},
		{"status", "status unexpected", "Usage: /factory status", false, nil},
		{"card", "card", "Factory card commands:", true, []string{"{card-path}", "{card-id}"}},
		{"pack", "pack", "Factory pack commands:", true, []string{"{cards-dir}", "{output-pack}", "{pack-path}", "[{source-path}]", "{diagnose text}"}},
		{"unknown top", "unknown", "Factory commands:", true, []string{"{card-path}", "{card-id}", "{cards-dir}", "{output-pack}", "{pack-path}", "[{source-path}]", "{diagnose text}"}},
		{"unknown card", "card publish", "Factory card commands:", true, []string{"{card-path}", "{card-id}"}},
		{"unknown pack", "pack unknown", "Factory pack commands:", true, []string{"{cards-dir}", "{output-pack}", "{pack-path}", "[{source-path}]", "{diagnose text}"}},
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
	assertContainsAll(t, ev.Message, "built factory pack:", "output: "+output, "/factory pack publish "+output)
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

func TestFactoryPackPublishRequiresServerConfigAndValidPack(t *testing.T) {
	withFactoryServerEnv(t, "", "")
	withMissingFactoryCredentials(t)
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("pack publish missing.pack")
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "Factory pack server is not configured") {
		t.Fatalf("Message = %q, want server config error", ev.Message)
	}

	withFactoryServerEnv(t, "http://example.invalid", "secret")
	badPack := filepath.Join(t.TempDir(), "bad.pack")
	if err := os.WriteFile(badPack, []byte("not a pack"), 0o600); err != nil {
		t.Fatalf("write bad pack: %v", err)
	}
	app.cmdFactory("pack publish " + badPack)
	ev = <-app.EventCh
	if !strings.Contains(ev.Message, "publish factory pack failed: validate pack") {
		t.Fatalf("Message = %q, want local validation error", ev.Message)
	}
}

func TestFactoryPackSyncNoConfiguredSourceReturnsClearError(t *testing.T) {
	withFactoryServerEnv(t, "", "")
	withMissingFactoryCredentials(t)
	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("pack sync")
	ev := <-app.EventCh
	if !strings.Contains(ev.Message, "Factory pack source is not configured") {
		t.Fatalf("Message = %q, want configured source error", ev.Message)
	}
}

func TestFactoryStatusMissingLocalPackAndNoServer(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	withHomeDir(t, filepath.Join(dir, "home"))
	withFactoryServerEnv(t, "", "")
	withMissingFactoryCredentials(t)

	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("status")
	ev := <-app.EventCh
	assertContainsAll(t, ev.Message,
		"factory status:",
		"local_pack_installed: false",
		"local_pack_reason: not installed",
		"server_configured: false",
		"config_source: none",
		"draft_cards: 0",
		"review_items: 0",
		"approved_cards: 0",
	)
	if strings.Contains(ev.Message, "server_reachable:") {
		t.Fatalf("Message = %q, should not probe unconfigured server", ev.Message)
	}
}

func TestFactoryStatusShowsInstalledPackCountsAndServerLatest(t *testing.T) {
	source := compileAppTestPack(t)
	dir := t.TempDir()
	withWorkingDir(t, dir)
	withHomeDir(t, filepath.Join(dir, "home"))

	if _, err := pack.Sync(pack.SyncConfig{SourcePath: source}); err != nil {
		t.Fatalf("Sync(source) error = %v", err)
	}
	writeAppTestFile(t, filepath.Join(dir, "factory", "cards", "drafts", "draft.yaml"), "draft")
	writeAppTestFile(t, filepath.Join(dir, "factory", "cards", "approved.yaml"), "approved")
	writeAppTestFile(t, filepath.Join(dir, "factory", "submissions", "item-1", "validation.json"), "{}")

	installed, err := pack.Load(filepath.Join(dir, "home", ".mscli", "factory", "factory-core.pack"))
	if err != nil {
		t.Fatalf("Load(installed) error = %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/factory/packs/latest" || r.Method != http.MethodGet {
			t.Fatalf("request = %s %s, want GET /factory/packs/latest", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("Authorization = %q, want bearer", r.Header.Get("Authorization"))
		}
		_, _ = fmt.Fprintf(w, `{"id":13,"pack_name":"factory-core","pack_version":"1","schema_version":"1","card_schema_version":"known_issue/v0.5","compiled_case_count":3,"checksum":%q,"publisher":"alice","created_at":"2026-05-14T00:00:00Z"}`, installed.Manifest.Checksum)
	}))
	defer server.Close()
	withFactoryServerEnv(t, server.URL, "secret")

	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("status")
	ev := <-app.EventCh
	assertContainsAll(t, ev.Message,
		"local_pack_installed: true",
		"local_pack_name: factory-core",
		"local_compiled_case_count: 3",
		"local_checksum: sha256:",
		"server_configured: true",
		"config_source: env",
		"server_reachable: true",
		"server_pack_id: 13",
		"server_checksum: "+installed.Manifest.Checksum,
		"local_matches_server_latest: true",
		"draft_cards: 1",
		"review_items: 1",
		"approved_cards: 1",
	)
}

func TestFactoryStatusServerUnreachableDoesNotFailWholeCommand(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	withHomeDir(t, filepath.Join(dir, "home"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, strings.Repeat("server unavailable ", 40), http.StatusServiceUnavailable)
	}))
	serverURL := server.URL
	server.Close()
	withFactoryServerEnv(t, serverURL, "secret")

	app := &Application{EventCh: make(chan model.Event, 4)}
	app.cmdFactory("status")
	ev := <-app.EventCh
	assertContainsAll(t, ev.Message, "factory status:", "server_configured: true", "config_source: env", "server_reachable: false", "server_reason:", "draft_cards: 0")
	if len(ev.Message) > 1200 {
		t.Fatalf("status message length = %d, want bounded", len(ev.Message))
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
	assertContainsAll(t, ev.Message,
		"factory pack match-debug:",
		"pack_load_status: loaded",
		"manifest_summary: factory-core schema=1 card_schema=known_issue/v0.5 cases=3",
		"candidate_count:",
		"emitted_hint_count:",
		"matched_case_id: stable-ascend-import",
		"score:",
		"why_matched:",
	)
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

func withFactoryServerEnv(t *testing.T, serverURL, token string) {
	t.Helper()
	oldURL := os.Getenv("MSCLI_FACTORY_SERVER_URL")
	oldToken := os.Getenv("MSCLI_FACTORY_TOKEN")
	if err := os.Setenv("MSCLI_FACTORY_SERVER_URL", serverURL); err != nil {
		t.Fatalf("set MSCLI_FACTORY_SERVER_URL: %v", err)
	}
	if err := os.Setenv("MSCLI_FACTORY_TOKEN", token); err != nil {
		t.Fatalf("set MSCLI_FACTORY_TOKEN: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("MSCLI_FACTORY_SERVER_URL", oldURL)
		_ = os.Setenv("MSCLI_FACTORY_TOKEN", oldToken)
	})
}

func TestFactoryServerConfigResolution(t *testing.T) {
	t.Run("env config works", func(t *testing.T) {
		withFactoryServerEnv(t, "http://env", "env-token")
		credentialsPathOverride = filepath.Join(t.TempDir(), "credentials.json")
		t.Cleanup(func() { credentialsPathOverride = "" })

		config := resolveFactoryServerConfig()
		if config.ServerURL != "http://env" || config.Token != "env-token" || config.Source != "env" {
			t.Fatalf("config = %+v, want env config", config)
		}
	})

	t.Run("credentials config works", func(t *testing.T) {
		withFactoryServerEnv(t, "", "")
		writeFactoryCredentials(t, `{"server_url":"http://cred","token":"cred-token","user":"alice","role":"admin"}`)

		config := resolveFactoryServerConfig()
		if config.ServerURL != "http://cred" || config.Token != "cred-token" || config.Source != "credentials.json" {
			t.Fatalf("config = %+v, want credentials config", config)
		}
	})

	t.Run("env overrides credentials", func(t *testing.T) {
		withFactoryServerEnv(t, "http://env", "env-token")
		writeFactoryCredentials(t, `{"server_url":"http://cred","token":"cred-token"}`)

		config := resolveFactoryServerConfig()
		if config.ServerURL != "http://env" || config.Token != "env-token" || config.Source != "env" {
			t.Fatalf("config = %+v, want env override", config)
		}
	})

	t.Run("missing both is not configured", func(t *testing.T) {
		withFactoryServerEnv(t, "", "")
		credentialsPathOverride = filepath.Join(t.TempDir(), "missing.json")
		t.Cleanup(func() { credentialsPathOverride = "" })

		config := resolveFactoryServerConfig()
		if config.Configured() || config.Source != "" {
			t.Fatalf("config = %+v, want not configured", config)
		}
	})
}

func writeFactoryCredentials(t *testing.T, content string) {
	t.Helper()
	credentialsPathOverride = filepath.Join(t.TempDir(), "credentials.json")
	t.Cleanup(func() { credentialsPathOverride = "" })
	if err := os.WriteFile(credentialsPathOverride, []byte(content), 0o600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}
}

func withMissingFactoryCredentials(t *testing.T) {
	t.Helper()
	credentialsPathOverride = filepath.Join(t.TempDir(), "missing.json")
	t.Cleanup(func() { credentialsPathOverride = "" })
}

func writeAppTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
}
