package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentctx "github.com/mindspore-lab/mindspore-cli/agent/context"
	"github.com/mindspore-lab/mindspore-cli/agent/session"
	"github.com/mindspore-lab/mindspore-cli/integrations/llm"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

type rewindFixture struct {
	session        *session.Session
	secondID       string
	thirdID        string
	trackedPath    string
	currentContext []llm.Message
}

func buildRewindFixture(t *testing.T, workDir string, mutateThirdTurn bool) rewindFixture {
	t.Helper()

	trackedPath := filepath.Join(workDir, "tracked.txt")
	if err := os.WriteFile(trackedPath, []byte("base"), 0o644); err != nil {
		t.Fatalf("write tracked seed: %v", err)
	}

	runtimeSession, err := session.Create(workDir, "system prompt")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := runtimeSession.Activate(); err != nil {
		t.Fatalf("activate session: %v", err)
	}

	if err := runtimeSession.AppendUserInput("first request"); err != nil {
		t.Fatalf("append first user input: %v", err)
	}
	if err := runtimeSession.AppendAssistant("first reply"); err != nil {
		t.Fatalf("append first assistant reply: %v", err)
	}
	if err := runtimeSession.SaveSnapshot("system prompt", []llm.Message{
		llm.NewUserMessage("first request"),
		llm.NewAssistantMessage("first reply"),
	}); err != nil {
		t.Fatalf("save first snapshot: %v", err)
	}

	if err := runtimeSession.AppendUserInput("second request"); err != nil {
		t.Fatalf("append second user input: %v", err)
	}
	if err := runtimeSession.RecordFileMutation("tracked.txt", trackedPath); err != nil {
		t.Fatalf("record second-turn file mutation: %v", err)
	}
	if err := os.WriteFile(trackedPath, []byte("second turn"), 0o644); err != nil {
		t.Fatalf("write second-turn content: %v", err)
	}
	if err := runtimeSession.AppendAssistant("second reply"); err != nil {
		t.Fatalf("append second assistant reply: %v", err)
	}
	if err := runtimeSession.SaveSnapshot("system prompt", []llm.Message{
		llm.NewUserMessage("first request"),
		llm.NewAssistantMessage("first reply"),
		llm.NewUserMessage("second request"),
		llm.NewAssistantMessage("second reply"),
	}); err != nil {
		t.Fatalf("save second snapshot: %v", err)
	}

	if err := runtimeSession.AppendUserInput("third request"); err != nil {
		t.Fatalf("append third user input: %v", err)
	}
	if mutateThirdTurn {
		if err := runtimeSession.RecordFileMutation("tracked.txt", trackedPath); err != nil {
			t.Fatalf("record third-turn file mutation: %v", err)
		}
		if err := os.WriteFile(trackedPath, []byte("broken"), 0o644); err != nil {
			t.Fatalf("write third-turn content: %v", err)
		}
	}

	checkpoints := runtimeSession.ListCheckpoints()
	if got, want := len(checkpoints), 3; got != want {
		t.Fatalf("checkpoint count = %d, want %d", got, want)
	}

	return rewindFixture{
		session:     runtimeSession,
		secondID:    checkpoints[1].MessageID,
		thirdID:     checkpoints[0].MessageID,
		trackedPath: trackedPath,
		currentContext: []llm.Message{
			llm.NewUserMessage("first request"),
			llm.NewAssistantMessage("first reply"),
			llm.NewUserMessage("second request"),
			llm.NewAssistantMessage("second reply"),
			llm.NewUserMessage("third request"),
		},
	}
}

func TestCmdRewindOpensCheckpointPicker(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	workDir := t.TempDir()
	fixture := buildRewindFixture(t, workDir, false)
	t.Cleanup(func() {
		_ = fixture.session.Close()
	})

	app := newModelCommandTestApp()
	app.WorkDir = workDir
	app.session = fixture.session

	app.cmdRewind(nil)

	ev := drainUntilEventType(t, app, model.RewindPickerOpen)
	if ev.RewindPicker == nil {
		t.Fatal("expected rewind picker payload")
	}
	if got, want := len(ev.RewindPicker.Items), 3; got != want {
		t.Fatalf("rewind picker item count = %d, want %d", got, want)
	}
	if got, want := ev.RewindPicker.Items[0].Preview, "third request"; got != want {
		t.Fatalf("latest rewind preview = %q, want %q", got, want)
	}
	if got, want := ev.RewindPicker.Items[0].LastUserInput, "second request"; got != want {
		t.Fatalf("latest rewind last user input = %q, want %q", got, want)
	}
	if got, want := ev.RewindPicker.Items[0].TurnCount, 2; got != want {
		t.Fatalf("latest rewind turn count = %d, want %d", got, want)
	}
}

func TestCmdRewindApplyForksConversationBeforeCheckpoint(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	workDir := t.TempDir()
	fixture := buildRewindFixture(t, workDir, false)
	t.Cleanup(func() {
		_ = fixture.session.Close()
	})

	ctxManager := agentctx.NewManager(agentctx.ManagerConfig{
		ContextWindow: 4096,
		ReserveTokens: 512,
	})
	ctxManager.SetSystemPrompt("system prompt")
	for _, msg := range fixture.currentContext {
		if err := ctxManager.AddMessage(msg); err != nil {
			t.Fatalf("add current context: %v", err)
		}
	}

	app := newModelCommandTestApp()
	app.WorkDir = workDir
	app.session = fixture.session
	app.ctxManager = ctxManager
	oldSessionID := fixture.session.ID()

	app.cmdRewindApply([]string{fixture.thirdID, string(model.RewindRestoreConversation)})

	ev := drainUntilClearScreen(t, app)
	if got, want := ev.Message, "Conversation rewound."; got != want {
		t.Fatalf("clear message = %q, want %q", got, want)
	}
	if got, want := ev.InputPrefill, "third request"; got != want {
		t.Fatalf("clear input prefill = %q, want %q", got, want)
	}
	if !strings.Contains(ev.Summary, oldSessionID) {
		t.Fatalf("clear summary = %q, want old session hint", ev.Summary)
	}
	if got := app.session.ID(); got == oldSessionID {
		t.Fatalf("forked session id = %q, want a new session id", got)
	}
	if got, want := len(app.ctxManager.GetNonSystemMessages()), 4; got != want {
		t.Fatalf("rewound context message count = %d, want %d", got, want)
	}

	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	var replayed []model.Event
	for len(replayed) < 4 {
		select {
		case next := <-app.EventCh:
			if next.Type == model.UserInput || next.Type == model.AgentReply {
				replayed = append(replayed, next)
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for replayed fork history, got %v", replayed)
		}
	}
	want := []struct {
		eventType model.EventType
		message   string
	}{
		{eventType: model.UserInput, message: "first request"},
		{eventType: model.AgentReply, message: "first reply"},
		{eventType: model.UserInput, message: "second request"},
		{eventType: model.AgentReply, message: "second reply"},
	}
	for i := range want {
		if replayed[i].Type != want[i].eventType {
			t.Fatalf("replayed[%d].Type = %q, want %q", i, replayed[i].Type, want[i].eventType)
		}
		if replayed[i].Message != want[i].message {
			t.Fatalf("replayed[%d].Message = %q, want %q", i, replayed[i].Message, want[i].message)
		}
	}
}

func TestCmdRewindApplyRestoresTrackedFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	workDir := t.TempDir()
	fixture := buildRewindFixture(t, workDir, true)
	t.Cleanup(func() {
		_ = fixture.session.Close()
	})

	ctxManager := agentctx.NewManager(agentctx.ManagerConfig{
		ContextWindow: 4096,
		ReserveTokens: 512,
	})
	ctxManager.SetSystemPrompt("system prompt")
	for _, msg := range fixture.currentContext {
		if err := ctxManager.AddMessage(msg); err != nil {
			t.Fatalf("add current context: %v", err)
		}
	}

	app := newModelCommandTestApp()
	app.WorkDir = workDir
	app.session = fixture.session
	app.ctxManager = ctxManager

	app.cmdRewindApply([]string{fixture.thirdID, string(model.RewindRestoreCodeConversation)})

	ev := drainUntilClearScreen(t, app)
	if got, want := ev.Message, "Code and conversation rewound."; got != want {
		t.Fatalf("clear message = %q, want %q", got, want)
	}
	if got, want := ev.InputPrefill, "third request"; got != want {
		t.Fatalf("clear input prefill = %q, want %q", got, want)
	}
	if got, err := os.ReadFile(fixture.trackedPath); err != nil {
		t.Fatalf("read restored tracked file: %v", err)
	} else if string(got) != "second turn" {
		t.Fatalf("tracked file after rewind = %q, want %q", string(got), "second turn")
	}
	if got, want := len(app.ctxManager.GetNonSystemMessages()), 4; got != want {
		t.Fatalf("rewound context message count = %d, want %d", got, want)
	}
}

func TestCmdBranchForksCurrentConversation(t *testing.T) {
	for _, command := range []string{"/branch", "/fork"} {
		t.Run(strings.TrimPrefix(command, "/"), func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())

			workDir := t.TempDir()
			fixture := buildRewindFixture(t, workDir, false)
			t.Cleanup(func() {
				_ = fixture.session.Close()
			})

			ctxManager := agentctx.NewManager(agentctx.ManagerConfig{
				ContextWindow: 4096,
				ReserveTokens: 512,
			})
			ctxManager.SetSystemPrompt("system prompt")
			for _, msg := range fixture.currentContext {
				if err := ctxManager.AddMessage(msg); err != nil {
					t.Fatalf("add current context: %v", err)
				}
			}

			app := newModelCommandTestApp()
			app.WorkDir = workDir
			app.session = fixture.session
			app.ctxManager = ctxManager
			oldSessionID := fixture.session.ID()

			app.handleCommand(command)
			t.Cleanup(func() {
				if app.session != nil {
					_ = app.session.Close()
				}
			})

			ev := drainUntilClearScreen(t, app)
			if got, want := ev.Message, "Conversation branched."; got != want {
				t.Fatalf("clear message = %q, want %q", got, want)
			}
			if ev.InputPrefill != "" {
				t.Fatalf("clear input prefill = %q, want empty", ev.InputPrefill)
			}
			if !strings.Contains(ev.Summary, oldSessionID) {
				t.Fatalf("clear summary = %q, want old session hint", ev.Summary)
			}
			if got := app.session.ID(); got == oldSessionID {
				t.Fatalf("forked session id = %q, want a new session id", got)
			}
			if got, want := len(app.ctxManager.GetNonSystemMessages()), len(fixture.currentContext); got != want {
				t.Fatalf("branched context message count = %d, want %d", got, want)
			}
		})
	}
}
