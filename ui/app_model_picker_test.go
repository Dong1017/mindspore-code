package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"gitcode.com/mindspore/mscli/ui/model"
)

func TestSetupPopupOpenAndNavigate(t *testing.T) {
	userCh := make(chan string, 1)
	app := New(nil, userCh, "test", ".", "", "demo-model", 4096)
	app.bootActive = false
	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app = next.(App)

	next, _ = app.handleEvent(model.Event{
		Type: model.ModelSetupOpen,
		SetupPopup: &model.SetupPopup{
			Screen:    model.SetupScreenEnvInfo,
			CanEscape: true,
		},
	})
	app = next.(App)

	if app.setupPopup == nil {
		t.Fatal("expected setup popup to be open")
	}

	view := app.View()
	if !strings.Contains(view, "MSCLI_API_KEY") {
		t.Fatalf("expected env info screen in view, got:\n%s", view)
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	app = next.(App)
	if app.setupPopup != nil {
		t.Fatal("expected setup popup to close on esc")
	}
}

func TestSetupPopupNoEscapeOnFirstBoot(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "", 4096)
	app.bootActive = false

	next, _ := app.handleEvent(model.Event{
		Type: model.ModelSetupOpen,
		SetupPopup: &model.SetupPopup{
			Screen:    model.SetupScreenEnvInfo,
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

func TestInlineModeSetupPopupUsesTemporaryFullscreenView(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false
	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app = next.(App)

	next, cmd := app.handleEvent(model.Event{
		Type: model.ModelSetupOpen,
		SetupPopup: &model.SetupPopup{
			Screen:    model.SetupScreenEnvInfo,
			CanEscape: true,
		},
	})
	app = next.(App)

	if cmd == nil {
		t.Fatal("expected inline mode setup popup to request temporary alt-screen")
	}
	if !app.modalAltScreen {
		t.Fatal("expected inline mode setup popup to mark temporary alt-screen active")
	}
	if view := app.View(); !strings.Contains(view, "MSCLI_API_KEY") {
		t.Fatalf("expected inline setup popup to be visible, got:\n%s", view)
	}

	next, cmd = app.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	app = next.(App)

	if cmd == nil {
		t.Fatal("expected closing inline setup popup to request alt-screen exit")
	}
	if app.modalAltScreen {
		t.Fatal("expected temporary alt-screen flag to clear after popup close")
	}
}

func TestModelSetupPopupSuppressesThinkingIndicatorWithoutClearingState(t *testing.T) {
	app := New(nil, nil, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app = next.(App)

	next, _ = app.handleEvent(model.Event{Type: model.AgentThinking})
	app = next.(App)
	if view := app.View(); !strings.Contains(view, "Working...") {
		t.Fatalf("expected thinking indicator before popup, got:\n%s", view)
	}

	next, _ = app.handleEvent(model.Event{
		Type: model.ModelSetupOpen,
		SetupPopup: &model.SetupPopup{
			Screen:    model.SetupScreenEnvInfo,
			CanEscape: true,
		},
	})
	app = next.(App)

	if !app.state.IsThinking {
		t.Fatal("expected popup open to preserve underlying thinking state")
	}
	view := app.View()
	if !strings.Contains(view, "MSCLI_API_KEY") {
		t.Fatalf("expected model setup popup in view, got:\n%s", view)
	}
	if strings.Contains(view, "Working...") {
		t.Fatalf("expected popup view to suppress background thinking indicator, got:\n%s", view)
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
	app = next.(App)
	if view := app.View(); !strings.Contains(view, "Working...") {
		t.Fatalf("expected thinking indicator to return after popup close, got:\n%s", view)
	}
}
