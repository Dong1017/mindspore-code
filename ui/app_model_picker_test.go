package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vigo999/mindspore-code/ui/model"
)

func TestSetupPopupOpenAndNavigate(t *testing.T) {
	userCh := make(chan string, 1)
	app := New(nil, userCh, "test", ".", "", "demo-model", 4096)
	app.bootActive = false
	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app = next.(App)

	// Open setup popup via event
	next, _ = app.handleEvent(model.Event{
		Type: model.ModelSetupOpen,
		SetupPopup: &model.SetupPopup{
			Screen: model.SetupScreenModeSelect,
			PresetOptions: []model.SelectionOption{
				{ID: "kimi-k2.5-free", Label: "kimi-k2.5 [free]"},
				{ID: "glm-4.7", Label: "glm-4.7 (coming soon)", Disabled: true},
			},
			CanEscape: true,
		},
	})
	app = next.(App)

	if app.setupPopup == nil {
		t.Fatal("expected setup popup to be open")
	}

	view := app.View()
	if !strings.Contains(view, "mscode-provided") {
		t.Fatalf("expected mode select screen in view, got:\n%s", view)
	}

	// Press enter to go to preset picker (mode 0 = mscode-provided)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	app = next.(App)
	if app.setupPopup.Screen != model.SetupScreenPresetPicker {
		t.Fatalf("expected preset picker screen, got %d", app.setupPopup.Screen)
	}

	// Press esc to go back to mode select
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	app = next.(App)
	if app.setupPopup.Screen != model.SetupScreenModeSelect {
		t.Fatalf("expected mode select screen, got %d", app.setupPopup.Screen)
	}

	// Navigate to "your own model" and press enter
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	app = next.(App)
	if app.setupPopup.ModeSelected != 1 {
		t.Fatalf("expected mode 1, got %d", app.setupPopup.ModeSelected)
	}
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	app = next.(App)
	if app.setupPopup.Screen != model.SetupScreenEnvInfo {
		t.Fatalf("expected env info screen, got %d", app.setupPopup.Screen)
	}

	// Press esc to go back
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	app = next.(App)
	if app.setupPopup.Screen != model.SetupScreenModeSelect {
		t.Fatalf("expected mode select screen after esc from env info, got %d", app.setupPopup.Screen)
	}

	// Press esc again to close (CanEscape=true)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	app = next.(App)
	if app.setupPopup != nil {
		t.Fatal("expected setup popup to close on esc from mode select")
	}
}

func TestSetupPopupNoEscapeOnFirstBoot(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "", 4096)
	app.bootActive = false

	next, _ := app.handleEvent(model.Event{
		Type: model.ModelSetupOpen,
		SetupPopup: &model.SetupPopup{
			Screen:    model.SetupScreenModeSelect,
			CanEscape: false,
		},
	})
	app = next.(App)

	// Esc should NOT close the popup
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	app = next.(App)
	if app.setupPopup == nil {
		t.Fatal("expected setup popup to stay open when CanEscape=false")
	}
}
