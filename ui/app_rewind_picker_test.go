package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

func TestRewindPickerOpenConfirmConversationRestore(t *testing.T) {
	userCh := make(chan string, 1)
	app := New(nil, userCh, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app = next.(App)

	next, cmd := app.handleEvent(model.Event{
		Type: model.RewindPickerOpen,
		RewindPicker: &model.RewindPicker{
			Items: []model.RewindCheckpointItem{
				{
					MessageID:      "msg_000003",
					Timestamp:      time.Date(2026, time.April, 8, 12, 0, 0, 0, time.UTC),
					Preview:        "revert the last change",
					LastUserInput:  "stabilize formatting",
					TurnCount:      4,
					HasCodeRestore: false,
				},
			},
		},
	})
	app = next.(App)

	if cmd == nil {
		t.Fatal("expected rewind picker to request alt-screen")
	}
	if !app.modalAltScreen {
		t.Fatal("expected rewind picker to enable alt-screen")
	}
	if view := app.View(); !strings.Contains(view, "Rewind Session") || !strings.Contains(view, "revert the last change") || !strings.Contains(view, "stabilize formatting") || !strings.Contains(view, "4 turns before here") {
		t.Fatalf("expected rewind picker view, got:\n%s", view)
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	app = next.(App)
	if app.rewindPicker == nil || !app.rewindPicker.Confirming {
		t.Fatal("expected rewind picker to enter confirmation mode")
	}

	next, cmd = app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	app = next.(App)
	if cmd == nil {
		t.Fatal("expected rewind picker confirm to exit alt-screen")
	}
	if app.rewindPicker != nil {
		t.Fatal("expected rewind picker to close after confirmation")
	}

	select {
	case got := <-userCh:
		if got != "/__rewind msg_000003 conversation" {
			t.Fatalf("selection command = %q, want %q", got, "/__rewind msg_000003 conversation")
		}
	default:
		t.Fatal("expected rewind picker to submit conversation rewind command")
	}
}

func TestRewindPickerConfirmCodeRestore(t *testing.T) {
	userCh := make(chan string, 1)
	app := New(nil, userCh, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app = next.(App)

	next, _ = app.handleEvent(model.Event{
		Type: model.RewindPickerOpen,
		RewindPicker: &model.RewindPicker{
			Items: []model.RewindCheckpointItem{
				{
					MessageID:      "msg_000004",
					Timestamp:      time.Date(2026, time.April, 8, 13, 0, 0, 0, time.UTC),
					Preview:        "restore workspace",
					LastUserInput:  "seed workspace state",
					TurnCount:      2,
					HasCodeRestore: true,
				},
			},
		},
	})
	app = next.(App)

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	app = next.(App)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	app = next.(App)
	if app.rewindPicker == nil || app.rewindPicker.ConfirmMode() != model.RewindRestoreCodeConversation {
		t.Fatal("expected rewind picker to switch to code restore option")
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	app = next.(App)

	select {
	case got := <-userCh:
		if got != "/__rewind msg_000004 code" {
			t.Fatalf("selection command = %q, want %q", got, "/__rewind msg_000004 code")
		}
	default:
		t.Fatal("expected rewind picker to submit code restore command")
	}
}

func TestRewindPickerConfirmSummarizeWithContext(t *testing.T) {
	userCh := make(chan string, 1)
	app := New(nil, userCh, "test", ".", "", "demo-model", 4096)
	app.bootActive = false

	next, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app = next.(App)

	next, _ = app.handleEvent(model.Event{
		Type: model.RewindPickerOpen,
		RewindPicker: &model.RewindPicker{
			Items: []model.RewindCheckpointItem{
				{
					MessageID:      "msg_000005",
					Timestamp:      time.Date(2026, time.April, 8, 14, 0, 0, 0, time.UTC),
					Preview:        "summarize this branch",
					LastUserInput:  "previous work",
					TurnCount:      3,
					HasCodeRestore: false,
				},
			},
		},
	})
	app = next.(App)

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	app = next.(App)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	app = next.(App)
	if app.rewindPicker == nil || app.rewindPicker.ConfirmMode() != model.RewindRestoreSummarize {
		t.Fatal("expected rewind picker to switch to summarize option")
	}

	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	app = next.(App)
	if app.rewindPicker == nil || !app.rewindPicker.CapturingSummary {
		t.Fatal("expected rewind picker to capture optional summary context")
	}
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("keep decisions")})
	app = next.(App)
	next, _ = app.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	app = next.(App)

	select {
	case got := <-userCh:
		if got != "/__rewind msg_000005 summarize keep decisions" {
			t.Fatalf("selection command = %q, want %q", got, "/__rewind msg_000005 summarize keep decisions")
		}
	default:
		t.Fatal("expected rewind picker to submit summarize rewind command")
	}
}
