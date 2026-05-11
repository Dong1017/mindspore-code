package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/card"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
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
	if len(args) >= 2 && args[0] == "pack" && args[1] == "sync" {
		if len(args) > 3 {
			a.EventCh <- model.Event{Type: model.AgentReply, Message: "Usage: /factory pack sync [source-path]"}
			return
		}
		source := ""
		if len(args) == 3 {
			source = args[2]
		} else {
			source = a.factoryPackSource()
		}
		a.cmdFactoryPackSync(source)
		return
	}
	a.EventCh <- model.Event{Type: model.AgentReply, Message: "Unsupported /factory command. Supported: /factory card create --from-last-run; /factory card submit <card-path>; /factory pack sync [source-path]"}
}

func (a *Application) cmdFactoryPackSync(source string) {
	if strings.TrimSpace(source) == "" {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: "Factory pack source is not configured. Configure factory.pack_source or pass a local source path."}
		return
	}
	result, err := pack.Sync(pack.SyncConfig{SourcePath: source})
	if err != nil {
		message := fmt.Sprintf("sync factory pack failed: %v", err)
		if dest, destErr := pack.DefaultPackPath(); destErr == nil {
			if _, statErr := os.Stat(dest); statErr == nil {
				message += "\nExisting local factory pack was preserved."
			} else if os.IsNotExist(statErr) {
				message += "\nNo local factory pack was installed."
			}
		}
		a.EventCh <- model.Event{Type: model.AgentReply, Message: message}
		return
	}
	a.EventCh <- model.Event{Type: model.AgentReply, Message: renderFactoryPackSyncResult(result)}
}

func (a *Application) factoryPackSource() string {
	return ""
}

func renderFactoryPackSyncResult(result *pack.SyncResult) string {
	return fmt.Sprintf("synced factory pack:\nsource: %s\ndestination: %s\npack_name: %s\npack_version: %s\nschema_version: %s\ncard_schema_version: %s\ncompiled_case_count: %d\nchecksum: %s",
		result.SourcePath,
		result.DestPath,
		result.PackName,
		result.PackVersion,
		result.SchemaVersion,
		result.CardSchemaVersion,
		result.CompiledCaseCount,
		result.Checksum,
	)
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
