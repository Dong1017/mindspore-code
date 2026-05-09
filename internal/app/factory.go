package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/card"
	factoryruntime "github.com/mindspore-lab/mindspore-cli/internal/factory/runtime"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

func (a *Application) cmdFactory(input string) {
	args := strings.Fields(input)
	if len(args) == 3 && args[0] == "card" && args[1] == "create" && args[2] == "--from-last-run" {
		a.cmdFactoryCardCreateFromLastRun()
		return
	}
	if len(args) >= 2 && args[0] == "card" && args[1] == "submit" {
		if len(args) != 3 {
			a.EventCh <- model.Event{Type: model.AgentReply, Message: "Usage: /factory card submit <card-path>"}
			return
		}
		a.cmdFactoryCardSubmit(args[2])
		return
	}
	a.EventCh <- model.Event{Type: model.AgentReply, Message: "Unsupported /factory command. Supported: /factory card create --from-last-run; /factory card submit <card-path>"}
}

func (a *Application) cmdFactoryCardSubmit(cardPath string) {
	bundle, err := card.SubmitDraftCard(cardPath, card.SubmitOptions{})
	if err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: fmt.Sprintf("submit draft card failed: %v", err)}
		return
	}
	a.EventCh <- model.Event{Type: model.AgentReply, Message: fmt.Sprintf("created review bundle: %s", bundle.Path)}
}

func (a *Application) cmdFactoryCardCreateFromLastRun() {
	selected := factoryruntime.SelectLastRunSummary(a.latestDiagnoseSummary, a.latestFixSummary, a.latestRunKind)
	if selected.Diagnose == nil && selected.Fix == nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: "No latest /diagnose or /fix run summary is available."}
		return
	}
	draft, err := card.NewDraftFromRunSummary(selected, card.DraftOptions{Reporter: a.issueUser})
	if err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: fmt.Sprintf("create draft card failed: %v", err)}
		return
	}
	path, err := card.WriteDraftYAML(draft, filepath.FromSlash(card.DefaultDraftCardsDir))
	if err != nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: fmt.Sprintf("create draft card failed: %v", err)}
		return
	}
	message := fmt.Sprintf("created draft card: %s", path)
	if selected.Warning != "" {
		message = selected.Warning + "\n" + message
	}
	a.EventCh <- model.Event{Type: model.AgentReply, Message: message}
}
