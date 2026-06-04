package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mindspore-lab/mindspore-cli/configs"
	"github.com/mindspore-lab/mindspore-cli/integrations/llm"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

func TestCmdModel_AlwaysShowsSetupPopup(t *testing.T) {
	app := newModelCommandTestApp()

	app.cmdModel(nil)

	ev := drainUntilEventType(t, app, model.ModelSetupOpen)
	if ev.SetupPopup == nil {
		t.Fatal("ModelSetupOpen popup = nil, want SetupPopup")
	}
	if !ev.SetupPopup.CanEscape {
		t.Fatal("expected CanEscape=true from /model command")
	}
	if len(ev.SetupPopup.PresetOptions) == 0 || ev.SetupPopup.PresetOptions[0].ID != "kimi-k2.5-free" {
		t.Fatalf("preset options = %#v, want kimi preset option", ev.SetupPopup.PresetOptions)
	}
}

func TestCmdModel_WithArgsStillShowsPopup(t *testing.T) {
	app := newModelCommandTestApp()

	app.cmdModel([]string{"deepseek"})

	ev := drainUntilEventType(t, app, model.ModelSetupOpen)
	if ev.SetupPopup == nil {
		t.Fatal("ModelSetupOpen popup = nil, want SetupPopup")
	}
}

func TestCmdModelSetup_RequiresLoginWhenNoCredentials(t *testing.T) {
	app := newModelCommandTestApp()
	t.Setenv("HOME", t.TempDir())

	app.cmdModelSetup([]string{"kimi-k2.5-free"})

	drainUntilEventType(t, app, model.AgentThinking)
	ev := drainUntilEventType(t, app, model.ModelSetupTokenError)
	if !strings.Contains(ev.Message, "not logged in") {
		t.Fatalf("error = %q, want login requirement", ev.Message)
	}
}

func TestCmdModelSetup_UseSavedCredentialsWithoutToken(t *testing.T) {
	app := newModelCommandTestApp()

	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/me":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"user": "alice", "role": "user"})
		case "/model-presets/kimi-k2.5-free/credential":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"api_key": "server-kimi-key"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv("HOME", t.TempDir())
	app.Config.Server.URL = srv.URL
	cred := credentials{
		ServerURL: srv.URL,
		Token:     "user-token",
		User:      "alice",
		Role:      "user",
	}
	if err := saveCredentials(&cred); err != nil {
		t.Fatalf("saveCredentials() error = %v", err)
	}

	var resolved llm.ResolvedConfig
	origBuildProvider := buildProvider
	buildProvider = func(cfg llm.ResolvedConfig) (llm.Provider, error) {
		resolved = cfg
		return &blockingStreamProvider{started: make(chan struct{})}, nil
	}
	defer func() { buildProvider = origBuildProvider }()

	// No token argument — should use saved credentials.
	app.cmdModelSetup([]string{"kimi-k2.5-free"})
	drainUntilEventType(t, app, model.AgentThinking)
	drainUntilEventType(t, app, model.ModelUpdate)
	drainUntilEventType(t, app, model.ModelSetupClose)

	if got, want := capturedAuth, "Bearer user-token"; got != want {
		t.Fatalf("credential request auth = %q, want %q", got, want)
	}
	if got, want := string(resolved.Kind), "anthropic"; got != want {
		t.Fatalf("resolved provider = %q, want %q", got, want)
	}
	if got, want := resolved.APIKey, "server-kimi-key"; got != want {
		t.Fatalf("resolved key = %q, want %q", got, want)
	}
}

func TestCmdModelSetup_WithTokenLoginAndApply(t *testing.T) {
	app := newModelCommandTestApp()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/me":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"user": "bob", "role": "admin"})
		case "/model-presets/kimi-k2.5-free/credential":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"api_key": "server-kimi-key"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv("HOME", t.TempDir())
	app.Config.Server.URL = srv.URL

	origBuildProvider := buildProvider
	buildProvider = func(cfg llm.ResolvedConfig) (llm.Provider, error) {
		return &blockingStreamProvider{started: make(chan struct{})}, nil
	}
	defer func() { buildProvider = origBuildProvider }()

	// Provide token explicitly — should login and apply.
	app.cmdModelSetup([]string{"kimi-k2.5-free", "new-token"})
	drainUntilEventType(t, app, model.AgentThinking)
	drainUntilEventType(t, app, model.ModelUpdate)
	drainUntilEventType(t, app, model.ModelSetupClose)

	ev := drainUntilEventType(t, app, model.AgentReply)
	if !strings.Contains(ev.Message, "bob") {
		t.Fatalf("reply = %q, want user name", ev.Message)
	}
}

func newModelCommandTestApp() *Application {
	cfg := configs.DefaultConfig()
	cfg.Model.Key = "test-key"
	cfg.Server.URL = "https://issues.example"
	return &Application{
		EventCh: make(chan model.Event, 16),
		Config:  cfg,
	}
}

func drainUntilEventType(t *testing.T, app *Application, target model.EventType) model.Event {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	for {
		select {
		case ev := <-app.EventCh:
			if ev.Type == target {
				return ev
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for event type %s", target)
		}
	}
}
