package app

import (
	"fmt"
	"path/filepath"

	"gitcode.com/mindspore/mscli/internal/factory/card"
	factoryruntime "gitcode.com/mindspore/mscli/internal/factory/runtime"
)

func (a *Application) cmdFactoryCardCreate(args []string) {
	if len(args) == 0 || (len(args) == 1 && args[0] == "--from-last-run") {
		a.cmdFactoryCardCreateFromLastRun()
		return
	}
	a.replyFactory("Usage: /factory card create")
}

func runFactoryCardSubmit(args []string, opts factoryCommandOptions) (string, error) {
	if len(args) != 1 {
		message := factoryUsageError(opts.Surface, "card submit")
		if opts.Surface == factorySurfaceCLI {
			return "", fmt.Errorf("%s", message)
		}
		return message, nil
	}
	bundle, err := card.SubmitDraftCard(args[0], card.SubmitOptions{})
	if err != nil {
		return "", fmt.Errorf("submit draft card failed: %w", err)
	}
	next := factoryCommand(opts.Surface, "card review "+bundle.CardID)
	return fmt.Sprintf("created local review item: %s\nfiles: %s\nnext:\n  %s", bundle.CardID, filepath.ToSlash(bundle.Path)+"/", next), nil
}

func runFactoryCardReview(args []string, opts factoryCommandOptions) (string, error) {
	if len(args) == 1 {
		view, err := card.RenderReviewItem(args[0], card.ReviewOptions{})
		if err != nil {
			return "", fmt.Errorf("review card failed: %w", err)
		}
		next := factoryCommand(opts.Surface, fmt.Sprintf("card review %s --approve --confidence observed --rationale \"{manual rationale}\"", args[0]))
		return view + "\nnext:\n  " + next, nil
	}
	if len(args) == 6 && args[1] == "--approve" && args[2] == "--confidence" && args[4] == "--rationale" {
		result, err := card.ApproveReviewItem(args[0], card.ApprovalOptions{Confidence: args[3], Rationale: args[5]})
		if err != nil {
			return "", fmt.Errorf("approve review card failed: %w", err)
		}
		next := factoryCommand(opts.Surface, "pack build factory/cards {output-pack}")
		return fmt.Sprintf("approved local factory card: %s\nfile: %s\ngovernance: lifecycle=stable review_status=approved confidence=observed\npack build still required: %s\nnext:\n  %s", result.CardID, result.Path, next, next), nil
	}
	message := factoryUsageError(opts.Surface, "card review")
	if opts.Surface == factorySurfaceCLI {
		return "", fmt.Errorf("%s", message)
	}
	return message, nil
}

func (a *Application) cmdFactoryCardCreateFromLastRun() {
	selected := factoryruntime.SelectLastRunSummary(a.latestDiagnoseSummary, a.latestFixSummary, a.latestRunKind)
	if selected.Diagnose == nil && selected.Fix == nil {
		a.replyFactory("No latest /diagnose or /fix run summary is available.")
		return
	}
	draft, err := card.NewDraftFromRunSummary(selected, card.DraftOptions{})
	if err != nil {
		a.replyFactory(fmt.Sprintf("create draft card failed: %v", err))
		return
	}
	path, err := card.WriteDraftYAML(draft, filepath.FromSlash(card.DefaultDraftCardsDir))
	if err != nil {
		a.replyFactory(fmt.Sprintf("create draft card failed: %v", err))
		return
	}
	message := fmt.Sprintf("created draft card: %s\nnext:\n  %s", path, factoryCommand(factorySurfaceTUI, "card submit "+path))
	if selected.Warning != "" {
		message = selected.Warning + "\n" + message
	}
	a.replyFactory(message)
}
