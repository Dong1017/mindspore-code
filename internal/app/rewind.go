package app

import (
	"strings"

	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

func (a *Application) cmdRewind(args []string) {
	if len(args) != 0 {
		a.emitToolError("session", "usage: /rewind")
		return
	}
	a.openRewindPicker()
}

func (a *Application) cmdCheckpoint(args []string) {
	if len(args) != 0 {
		a.emitToolError("session", "usage: /checkpoint")
		return
	}
	a.openRewindPicker()
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
	if len(args) != 2 {
		a.emitToolError("session", "usage: /__rewind <message-id> <conversation|code>")
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
		restoreCode = false
	case string(model.RewindRestoreCodeConversation):
		restoreCode = true
	default:
		a.emitToolError("session", "unknown rewind mode %q", args[1])
		return
	}

	a.applyRewind(messageID, restoreCode)
}

func (a *Application) applyRewind(messageID string, restoreCode bool) {
	if a == nil || a.session == nil {
		a.emitToolError("session", "no active session to rewind")
		return
	}

	oldSession := a.session
	oldSessionID := strings.TrimSpace(oldSession.ID())
	if err := oldSession.Activate(); err != nil {
		a.emitToolError("session", "Failed to preserve the current conversation: %v", err)
		return
	}
	if err := a.persistSessionSnapshot(); err != nil {
		a.emitToolError("session", "Failed to preserve the current conversation: %v", err)
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
	if err := forked.Activate(); err != nil {
		a.emitToolError("session", "Failed to activate rewound session: %v", err)
		return
	}

	systemPrompt, messages := forked.RestoreContext()
	loaded := &loadedConversation{
		runtimeSession: forked,
		systemPrompt:   systemPrompt,
		messages:       messages,
		usageSnapshot:  forked.UsageSnapshot(),
		replayBacklog:  forked.ReplayEvents(),
	}

	a.bindConversation(loaded, sessionSwitchOptions{})
	if oldSession != a.session {
		_ = oldSession.Close()
	}

	message := "Conversation rewound."
	if restoreCode {
		message = "Code and conversation rewound."
	}
	a.EventCh <- model.Event{
		Type:    model.ClearScreen,
		Message: message,
		Summary: inlineResumeHintForSession(oldSessionID),
	}
	a.startReplayHistory()
}
