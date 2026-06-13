package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gitcode.com/mindspore/mscli/agent/session"
	"gitcode.com/mindspore/mscli/integrations/llm"
	"gitcode.com/mindspore/mscli/ui/model"
)

func (a *Application) cmdRewind(args []string) {
	if len(args) != 0 {
		a.emitToolError("session", "usage: /rewind")
		return
	}
	a.openRewindPicker()
}

func (a *Application) cmdBranch(commandName string, args []string) {
	if len(args) != 0 {
		a.emitToolError("session", "usage: %s", commandName)
		return
	}
	a.applyBranch()
}

func (a *Application) openRewindPicker() {
	if a == nil || a.EventCh == nil {
		return
	}

	items := []model.RewindCheckpointItem{}
	if a.session != nil {
		for _, checkpoint := range a.session.ListCheckpoints() {
			items = append(items, model.RewindCheckpointItem{
				MessageID:      checkpoint.MessageID,
				Timestamp:      checkpoint.Timestamp,
				Preview:        checkpoint.Preview,
				LastUserInput:  checkpoint.LastUserInput,
				TurnCount:      checkpoint.TurnCount,
				HasCodeRestore: checkpoint.HasCodeRestore,
			})
		}
	}

	a.EventCh <- model.Event{
		Type: model.RewindPickerOpen,
		RewindPicker: &model.RewindPicker{
			Items:        items,
			EmptyMessage: "No checkpoints yet. Checkpoints are created automatically when you send a message.",
		},
	}
}

func (a *Application) cmdRewindApply(args []string) {
	if len(args) < 2 {
		a.emitToolError("session", "usage: /__rewind <message-id> <conversation|code|summarize>")
		return
	}

	messageID := strings.TrimSpace(args[0])
	if messageID == "" {
		a.emitToolError("session", "rewind checkpoint id cannot be empty")
		return
	}

	restoreCode := false
	switch strings.TrimSpace(args[1]) {
	case string(model.RewindRestoreConversation):
		if len(args) != 2 {
			a.emitToolError("session", "usage: /__rewind <message-id> conversation")
			return
		}
		restoreCode = false
	case string(model.RewindRestoreCodeConversation):
		if len(args) != 2 {
			a.emitToolError("session", "usage: /__rewind <message-id> code")
			return
		}
		restoreCode = true
	case string(model.RewindRestoreSummarize):
		a.applyRewindWithSummary(messageID, strings.Join(args[2:], " "))
		return
	default:
		a.emitToolError("session", "unknown rewind mode %q", args[1])
		return
	}

	a.applyRewind(messageID, restoreCode)
}

func (a *Application) preserveCurrentSessionForFork(oldSession *session.Session) bool {
	if oldSession == nil {
		return false
	}
	if err := oldSession.Activate(); err != nil {
		a.emitToolError("session", "Failed to preserve the current conversation: %v", err)
		return false
	}
	if err := a.persistSessionSnapshot(); err != nil {
		a.emitToolError("session", "Failed to preserve the current conversation: %v", err)
		return false
	}
	return true
}

func (a *Application) switchToForkedConversation(oldSession, forked *session.Session, message, oldSessionID, inputPrefill string) {
	if err := forked.Activate(); err != nil {
		a.emitToolError("session", "Failed to activate forked session: %v", err)
		return
	}

	systemPrompt, messages := forked.RestoreContext()
	replayBacklog := forked.ReplayEvents()
	if notice, ok := inlineResumeNoticeForSession(oldSessionID); ok {
		replayBacklog = append(replayBacklog, notice)
	}
	loaded := &loadedConversation{
		runtimeSession: forked,
		systemPrompt:   systemPrompt,
		messages:       messages,
		usageSnapshot:  forked.UsageSnapshot(),
		replayBacklog:  replayBacklog,
	}

	a.bindConversation(loaded, sessionSwitchOptions{})
	if oldSession != a.session {
		_ = oldSession.Close()
	}

	a.EventCh <- model.Event{
		Type:         model.ClearScreen,
		Message:      message,
		InputPrefill: inputPrefill,
	}
	a.startReplayHistory()
}

func (a *Application) applyBranch() {
	if a == nil || a.session == nil {
		a.emitToolError("session", "no active session to branch")
		return
	}

	oldSession := a.session
	oldSessionID := strings.TrimSpace(oldSession.ID())
	if !a.preserveCurrentSessionForFork(oldSession) {
		return
	}

	a.interruptReplay()
	a.interruptActiveTasks()

	forked, err := oldSession.ForkCurrent()
	if err != nil {
		a.emitToolError("session", "Failed to fork session: %v", err)
		return
	}

	a.switchToForkedConversation(oldSession, forked, "Conversation branched.", oldSessionID, "")
}

func (a *Application) rewindSummaryContext() (context.Context, context.CancelFunc) {
	ctx := context.Background()
	if a != nil && a.Config != nil && a.Config.Model.TimeoutSec > 0 {
		return context.WithTimeout(ctx, time.Duration(a.Config.Model.TimeoutSec)*time.Second)
	}
	return ctx, func() {}
}

func (a *Application) summarizeRewindSegment(oldSession *session.Session, messageID, userContext string) (llm.Message, string, error) {
	if a == nil || a.ctxManager == nil {
		return llm.Message{}, "", fmt.Errorf("context manager is not available")
	}
	messages, err := oldSession.MessagesFromCheckpoint(messageID)
	if err != nil {
		return llm.Message{}, "", err
	}
	ctx, cancel := a.rewindSummaryContext()
	defer cancel()
	return a.ctxManager.SummarizeRewindSegmentWithContext(ctx, messages, userContext)
}

func appendRewindSummaryToFork(forked *session.Session, summaryMsg llm.Message) error {
	if forked == nil {
		return fmt.Errorf("session is nil")
	}
	systemPrompt, messages := forked.RestoreContext()
	messages = append(messages, summaryMsg)
	if err := forked.SaveSnapshot(systemPrompt, messages); err != nil {
		return err
	}
	return forked.AppendContextCompaction("manual", 0, 0, "Conversation summarized from rewind point.")
}

func (a *Application) applyRewindWithSummary(messageID, userContext string) {
	if a == nil || a.session == nil {
		a.emitToolError("session", "no active session to rewind")
		return
	}

	oldSession := a.session
	oldSessionID := strings.TrimSpace(oldSession.ID())
	selectedInput, err := oldSession.CheckpointUserInput(messageID)
	if err != nil {
		a.emitToolError("session", "Failed to load rewind prompt: %v", err)
		return
	}
	if !a.preserveCurrentSessionForFork(oldSession) {
		return
	}

	a.interruptReplay()
	a.interruptActiveTasks()
	a.EventCh <- model.Event{Type: model.ContextCompactStarted}

	summaryMsg, _, err := a.summarizeRewindSegment(oldSession, messageID, userContext)
	if err != nil {
		a.emitToolError("context", "Failed to summarize rewind segment: %v", err)
		return
	}

	forked, err := oldSession.ForkFromCheckpoint(messageID)
	if err != nil {
		a.emitToolError("session", "Failed to fork rewound session: %v", err)
		return
	}
	if err := appendRewindSummaryToFork(forked, summaryMsg); err != nil {
		a.emitToolError("session", "Failed to attach rewind summary: %v", err)
		return
	}

	a.switchToForkedConversation(oldSession, forked, "Conversation rewound and summarized.", oldSessionID, selectedInput)
}

func (a *Application) applyRewind(messageID string, restoreCode bool) {
	if a == nil || a.session == nil {
		a.emitToolError("session", "no active session to rewind")
		return
	}

	oldSession := a.session
	oldSessionID := strings.TrimSpace(oldSession.ID())
	selectedInput, err := oldSession.CheckpointUserInput(messageID)
	if err != nil {
		a.emitToolError("session", "Failed to load rewind prompt: %v", err)
		return
	}
	if !a.preserveCurrentSessionForFork(oldSession) {
		return
	}

	a.interruptReplay()
	a.interruptActiveTasks()

	if restoreCode {
		if err := oldSession.RestoreCheckpointFiles(messageID); err != nil {
			a.emitToolError("session", "Failed to restore checkpoint files: %v", err)
			return
		}
	}

	forked, err := oldSession.ForkFromCheckpoint(messageID)
	if err != nil {
		a.emitToolError("session", "Failed to fork rewound session: %v", err)
		return
	}

	message := "Conversation rewound."
	if restoreCode {
		message = "Code and conversation rewound."
	}
	a.switchToForkedConversation(oldSession, forked, message, oldSessionID, selectedInput)
}
