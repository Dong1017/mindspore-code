package app

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/card"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
	factoryruntime "github.com/mindspore-lab/mindspore-cli/internal/factory/runtime"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
	"gopkg.in/yaml.v3"
)

func TestFactoryCLIStatusPrintsStatus(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	withHomeDir(t, filepath.Join(dir, "home"))
	withFactoryServerEnv(t, "", "")
	withMissingFactoryCredentials(t)

	output, err := runFactoryCLITest("status")
	if err != nil {
		t.Fatalf("Run(factory status) error = %v", err)
	}
	assertContainsAll(t, output, "factory status:", "local_pack_installed: false", "local_pack_reason: not installed", "server_configured: false", "config_source: none")
}

func TestFactoryCLICardSubmitReviewAndApprove(t *testing.T) {
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
	app.cmdFactory("card create")
	<-app.EventCh
	drafts, err := filepath.Glob(filepath.Join(dir, "factory", "cards", "drafts", "*.yaml"))
	if err != nil {
		t.Fatalf("glob drafts: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("draft count = %d, want 1", len(drafts))
	}

	output, err := runFactoryCLITest("card", "submit", drafts[0])
	if err != nil {
		t.Fatalf("Run(factory card submit) error = %v", err)
	}
	assertContainsAll(t, output, "created local review item:", "next:", "mscli factory card review ")
	bundles, err := filepath.Glob(filepath.Join(dir, "factory", "submissions", "*"))
	if err != nil {
		t.Fatalf("glob bundles: %v", err)
	}
	if len(bundles) != 1 {
		t.Fatalf("bundle count = %d, want 1", len(bundles))
	}
	cardID := filepath.Base(bundles[0])

	output, err = runFactoryCLITest("card", "review", cardID)
	if err != nil {
		t.Fatalf("Run(factory card review) error = %v", err)
	}
	assertContainsAll(t, output, "factory card review:", "Manual review required", "next:", "mscli factory card review "+cardID+" --approve")

	makeReviewBundlePackReady(t, filepath.Join("factory", "submissions", cardID, "card.yaml"))
	output, err = runFactoryCLITest("card", "review", cardID, "--approve", "--confidence", "observed", "--rationale", "manual review passed")
	if err != nil {
		t.Fatalf("Run(factory card review --approve) error = %v", err)
	}
	assertContainsAll(t, output, "approved local factory card: "+cardID, "governance: lifecycle=stable review_status=approved confidence=observed", "next:", "mscli factory pack build factory/cards {output-pack}")
	if _, err := card.LoadFile(filepath.Join(dir, "factory", "cards", cardID+".yaml")); err != nil {
		t.Fatalf("LoadFile(approved) error = %v", err)
	}
}

func TestFactoryCLIPackBuildPublishAndSync(t *testing.T) {
	sourceDir, err := filepath.Abs(filepath.FromSlash("../factory/compiler/testdata/cards"))
	if err != nil {
		t.Fatalf("resolve source cards: %v", err)
	}
	dir := t.TempDir()
	withWorkingDir(t, dir)
	withHomeDir(t, filepath.Join(dir, "home"))
	outputPack := filepath.Join(dir, "built.pack")

	output, err := runFactoryCLITest("pack", "build", sourceDir, outputPack)
	if err != nil {
		t.Fatalf("Run(factory pack build) error = %v", err)
	}
	assertContainsAll(t, output, "built factory pack:", "output: "+outputPack, "next:", "mscli factory pack publish "+outputPack)
	if _, err := pack.Load(outputPack); err != nil {
		t.Fatalf("Load(built) error = %v", err)
	}

	var sawPublishAuth bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("Authorization = %q, want bearer", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/factory/packs":
			sawPublishAuth = true
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprint(w, `{"id":21,"pack_name":"factory-core","pack_version":"1","schema_version":"1","card_schema_version":"known_issue/v0.5","compiled_case_count":3,"checksum":"sha256:abc","publisher":"alice","created_at":"2026-05-14T00:00:00Z"}`)
		case "/factory/packs/latest":
			_, _ = fmt.Fprint(w, `{"id":22,"pack_name":"factory-core","pack_version":"1","schema_version":"1","card_schema_version":"known_issue/v0.5","compiled_case_count":3,"checksum":"sha256:abc","publisher":"alice","created_at":"2026-05-14T00:00:00Z"}`)
		case "/factory/packs/latest/download":
			data, err := os.ReadFile(outputPack)
			if err != nil {
				t.Fatalf("read output pack: %v", err)
			}
			_, _ = w.Write(data)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	withFactoryServerEnv(t, server.URL, "secret")

	output, err = runFactoryCLITest("pack", "publish", outputPack)
	if err != nil {
		t.Fatalf("Run(factory pack publish) error = %v", err)
	}
	assertContainsAll(t, output, "published factory pack:", "pack_id: 21", "next:", "mscli factory pack sync")
	if !sawPublishAuth {
		t.Fatal("publish did not reach server")
	}

	output, err = runFactoryCLITest("pack", "sync", outputPack)
	if err != nil {
		t.Fatalf("Run(factory pack sync source) error = %v", err)
	}
	assertContainsAll(t, output, "synced factory pack:", "next:", "mscli factory status")

	output, err = runFactoryCLITest("pack", "sync")
	if err != nil {
		t.Fatalf("Run(factory pack sync remote) error = %v", err)
	}
	assertContainsAll(t, output, "synced factory pack from server:", "remote_id: 22", "next:", "mscli factory status")
}

func TestFactoryCLIPackMatchDebug(t *testing.T) {
	source := compileAppTestPack(t)
	dir := t.TempDir()
	withHomeDir(t, filepath.Join(dir, "home"))
	if _, err := pack.Sync(pack.SyncConfig{SourcePath: source}); err != nil {
		t.Fatalf("Sync(source) error = %v", err)
	}

	output, err := runFactoryCLITest("pack", "match-debug", "ImportError: torch_npu failed because CANN runtime dependency is missing")
	if err != nil {
		t.Fatalf("Run(factory pack match-debug) error = %v", err)
	}
	assertContainsAll(t, output, "factory pack match-debug:", "pack_load_status: loaded", "matched_case_id: stable-ascend-import")
}

func TestFactoryCLIUnsupportedCommandsReturnError(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"unknown", []string{"unknown"}},
		{"card_create", []string{"card", "create"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runFactoryCLITest(tc.args...)
			if err == nil {
				t.Fatalf("Run(factory %v) error = nil, want error", tc.args)
			}
		})
	}
}

func runFactoryCLITest(args ...string) (string, error) {
	var buf bytes.Buffer
	previous := cliStdout
	cliStdout = &buf
	defer func() { cliStdout = previous }()
	err := Run(append([]string{"factory"}, args...))
	return buf.String(), err
}

func makeReviewBundlePackReady(t *testing.T, cardPath string) {
	t.Helper()
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
}
