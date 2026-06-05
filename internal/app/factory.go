package app

import (
	"fmt"
	"io"
	"strings"

	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

func runFactoryCLI(args []string, stdout io.Writer) error {
	message, err := runFactoryCommand(args, factoryCommandOptions{Surface: factorySurfaceCLI})
	if err != nil {
		return err
	}
	if message == "" {
		return nil
	}
	_, err = fmt.Fprintln(stdout, message)
	return err
}

type factorySurface string

const (
	factorySurfaceTUI factorySurface = "tui"
	factorySurfaceCLI factorySurface = "cli"
)

type factoryCommandOptions struct {
	Surface factorySurface
}

func runFactoryCommand(args []string, opts factoryCommandOptions) (string, error) {
	if len(args) == 0 {
		return renderFactoryHelpForSurface(opts.Surface), nil
	}
	switch args[0] {
	case "status":
		return runFactoryStatus(args[1:], opts)
	case "card":
		return runFactoryCard(args[1:], opts)
	case "pack":
		return runFactoryPack(args[1:], opts)
	default:
		if opts.Surface == factorySurfaceCLI {
			return "", fmt.Errorf("unsupported factory command: %s\n%s", args[0], renderFactoryHelpForSurface(opts.Surface))
		}
		return renderFactoryHelpForSurface(opts.Surface), nil
	}
}

func runFactoryStatus(args []string, opts factoryCommandOptions) (string, error) {
	if len(args) != 0 {
		return factoryUsageError(opts.Surface, "status"), nil
	}
	return renderFactoryStatus(), nil
}

func runFactoryCard(args []string, opts factoryCommandOptions) (string, error) {
	if len(args) == 0 {
		return renderFactoryCardHelpForSurface(opts.Surface), nil
	}
	switch args[0] {
	case "submit":
		return runFactoryCardSubmit(args[1:], opts)
	case "review":
		return runFactoryCardReview(args[1:], opts)
	case "create":
		if opts.Surface == factorySurfaceCLI {
			return "", fmt.Errorf("mscli factory card create is not supported in the non-interactive CLI because it depends on the current TUI session's latest /diagnose or /fix summary")
		}
		return "", fmt.Errorf("factory card create is handled by the interactive TUI")
	default:
		if opts.Surface == factorySurfaceCLI {
			return "", fmt.Errorf("unsupported factory card command: %s\n%s", args[0], renderFactoryCardHelpForSurface(opts.Surface))
		}
		return renderFactoryCardHelpForSurface(opts.Surface), nil
	}
}

func runFactoryPack(args []string, opts factoryCommandOptions) (string, error) {
	if len(args) == 0 {
		return renderFactoryPackHelpForSurface(opts.Surface), nil
	}
	switch args[0] {
	case "build":
		return runFactoryPackBuild(args[1:], opts)
	case "sync":
		return runFactoryPackSync(args[1:], opts)
	case "match-debug":
		return runFactoryPackMatchDebug(args[1:], opts)
	default:
		if opts.Surface == factorySurfaceCLI {
			return "", fmt.Errorf("unsupported factory pack command: %s\n%s", args[0], renderFactoryPackHelpForSurface(opts.Surface))
		}
		return renderFactoryPackHelpForSurface(opts.Surface), nil
	}
}

func (a *Application) cmdFactory(input string) {
	args, err := parseFactoryArgs(input)
	if err != nil {
		a.replyFactory(fmt.Sprintf("parse /factory command failed: %v", err))
		return
	}
	if len(args) >= 2 && args[0] == "card" && args[1] == "create" {
		a.cmdFactoryCardCreate(args[2:])
		return
	}
	message, err := runFactoryCommand(args, factoryCommandOptions{Surface: factorySurfaceTUI})
	if err != nil {
		a.replyFactory(err.Error())
		return
	}
	a.replyFactory(message)
}

func (a *Application) replyFactory(message string) {
	a.EventCh <- model.Event{Type: model.AgentReply, Message: message, RawANSI: strings.Contains(message, "\n")}
}
