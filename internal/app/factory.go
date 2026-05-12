package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/card"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/compiler"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
	factoryruntime "github.com/mindspore-lab/mindspore-cli/internal/factory/runtime"
	"github.com/mindspore-lab/mindspore-cli/ui/model"
)

func (a *Application) cmdFactory(input string) {
	args, err := parseFactoryArgs(input)
	if err != nil {
		a.replyFactory(fmt.Sprintf("parse /factory command failed: %v", err))
		return
	}
	if len(args) == 0 {
		a.replyFactory(renderFactoryHelp())
		return
	}
	switch args[0] {
	case "card":
		a.cmdFactoryCard(args[1:])
	case "pack":
		a.cmdFactoryPack(args[1:])
	default:
		a.replyFactory(renderFactoryHelp())
	}
}

func (a *Application) cmdFactoryCard(args []string) {
	if len(args) == 0 {
		a.replyFactory(renderFactoryCardHelp())
		return
	}
	switch args[0] {
	case "create":
		a.cmdFactoryCardCreate(args[1:])
	case "submit":
		a.cmdFactoryCardSubmit(args[1:])
	case "review":
		a.cmdFactoryCardReview(args[1:])
	default:
		a.replyFactory(renderFactoryCardHelp())
	}
}

func (a *Application) cmdFactoryPack(args []string) {
	if len(args) == 0 {
		a.replyFactory(renderFactoryPackHelp())
		return
	}
	switch args[0] {
	case "build":
		a.cmdFactoryPackBuild(args[1:])
	case "sync":
		a.cmdFactoryPackSync(args[1:])
	case "match-debug":
		a.cmdFactoryPackMatchDebug(args[1:])
	default:
		a.replyFactory(renderFactoryPackHelp())
	}
}

func (a *Application) replyFactory(message string) {
	a.EventCh <- model.Event{Type: model.AgentReply, Message: message}
}

func renderFactoryHelp() string {
	return "Factory commands:\n\nCard workflow:\n  /factory card create\n  /factory card submit <card-path>\n  /factory card review <card-id>\n  /factory card review <card-id> --approve --confidence observed --rationale \"<manual rationale>\"\n\nPack workflow:\n  /factory pack build <cards-dir> <output-pack>\n  /factory pack sync [source-path]\n  /factory pack match-debug \"<diagnose text>\""
}

func renderFactoryCardHelp() string {
	return "Factory card commands:\n  /factory card create\n  /factory card submit <card-path>\n  /factory card review <card-id>\n  /factory card review <card-id> --approve --confidence observed --rationale \"<manual rationale>\""
}

func renderFactoryPackHelp() string {
	return "Factory pack commands:\n  /factory pack build <cards-dir> <output-pack>\n  /factory pack sync [source-path]\n  /factory pack match-debug \"<diagnose text>\""
}

func (a *Application) cmdFactoryCardCreate(args []string) {
	if len(args) == 0 || (len(args) == 1 && args[0] == "--from-last-run") {
		a.cmdFactoryCardCreateFromLastRun()
		return
	}
	a.replyFactory("Usage: /factory card create")
}

func (a *Application) cmdFactoryCardSubmit(args []string) {
	if len(args) != 1 {
		a.replyFactory("Usage: /factory card submit <card-path>")
		return
	}
	bundle, err := card.SubmitDraftCard(args[0], card.SubmitOptions{})
	if err != nil {
		a.replyFactory(fmt.Sprintf("submit draft card failed: %v", err))
		return
	}
	a.replyFactory(fmt.Sprintf("created local review item: %s\nfiles: %s\nnext: /factory card review %s", bundle.CardID, filepath.ToSlash(bundle.Path)+"/", bundle.CardID))
}

func (a *Application) cmdFactoryCardReview(args []string) {
	if len(args) == 1 {
		view, err := card.RenderReviewItem(args[0], card.ReviewOptions{})
		if err != nil {
			a.replyFactory(fmt.Sprintf("review card failed: %v", err))
			return
		}
		a.replyFactory(view)
		return
	}
	if len(args) == 6 && args[1] == "--approve" && args[2] == "--confidence" && args[4] == "--rationale" {
		result, err := card.ApproveReviewItem(args[0], card.ApprovalOptions{Confidence: args[3], Rationale: args[5]})
		if err != nil {
			a.replyFactory(fmt.Sprintf("approve review card failed: %v", err))
			return
		}
		a.replyFactory(fmt.Sprintf("approved local factory card: %s\nfile: %s\ngovernance: lifecycle=stable review_status=approved confidence=observed\npack build still required: /factory pack build factory/cards <output-pack>", result.CardID, result.Path))
		return
	}
	a.replyFactory("Usage: /factory card review <card-id> [--approve --confidence observed --rationale \"<manual rationale>\"]")
}

func parseFactoryArgs(input string) ([]string, error) {
	var args []string
	var current strings.Builder
	inQuote := false
	for i := 0; i < len(input); i++ {
		ch := input[i]
		switch ch {
		case ' ', '\t', '\n', '\r':
			if inQuote {
				current.WriteByte(ch)
			} else if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		case '"':
			inQuote = !inQuote
		default:
			current.WriteByte(ch)
		}
	}
	if inQuote {
		return nil, fmt.Errorf("unterminated quoted argument")
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args, nil
}

func (a *Application) cmdFactoryPackBuild(args []string) {
	if len(args) != 2 {
		a.replyFactory("Usage: /factory pack build <cards-dir> <output-pack>")
		return
	}
	cardsDir, outputPack := args[0], args[1]
	result, err := compiler.CompilePack(cardsDir, outputPack)
	if err != nil {
		a.replyFactory(fmt.Sprintf("build factory pack failed: %v", err))
		return
	}
	a.replyFactory(renderFactoryPackBuildResult(cardsDir, result))
}

func renderFactoryPackBuildResult(cardsDir string, result *compiler.BuildSummary) string {
	return fmt.Sprintf("built factory pack:\ncards_dir: %s\noutput: %s\npack_name: %s\nschema_version: %s\ncard_schema_version: %s\nsource_case_count: %d\ncompiled_case_count: %d\nchecksum: %s",
		cardsDir,
		result.OutputPath,
		result.PackName,
		result.SchemaVersion,
		pack.CardSchemaVersion,
		result.SourceCaseCount,
		result.CompiledCaseCount,
		result.Checksum,
	)
}

func (a *Application) cmdFactoryPackSync(args []string) {
	if len(args) > 1 {
		a.replyFactory("Usage: /factory pack sync [source-path]")
		return
	}
	source := ""
	if len(args) == 1 {
		source = args[0]
	} else {
		source = a.factoryPackSource()
	}
	if strings.TrimSpace(source) == "" {
		a.replyFactory("Factory pack source is not configured. Configure factory.pack_source or pass a local source path.")
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
		a.replyFactory(message)
		return
	}
	a.replyFactory(renderFactoryPackSyncResult(result))
}

func (a *Application) cmdFactoryPackMatchDebug(args []string) {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		a.replyFactory("Usage: /factory pack match-debug \"<diagnose text>\"")
		return
	}
	loadedPack, err := pack.LoadDefault(pack.LoadConfig{})
	if err != nil {
		a.replyFactory(renderFactoryPackMatchDebugResult(factoryPackMatchDebugResult{
			PackLoadStatus: "failed",
			FallbackReason: fmt.Sprintf("pack load failed: %v", err),
		}))
		return
	}
	matches, err := loadedPack.MatchCases(factoryMatchDebugContext(args[0]).ToFingerprint(), pack.MatchOptions{})
	if err != nil {
		a.replyFactory(renderFactoryPackMatchDebugResult(factoryPackMatchDebugResult{
			PackLoadStatus:  "loaded",
			ManifestSummary: factoryPackManifestSummary(loadedPack.Manifest),
			FallbackReason:  fmt.Sprintf("match failed: %v", err),
		}))
		return
	}
	result := factoryPackMatchDebugResult{
		PackLoadStatus:   "loaded",
		ManifestSummary:  factoryPackManifestSummary(loadedPack.Manifest),
		CandidateCount:   len(matches),
		EmittedHintCount: len(matches),
	}
	if result.EmittedHintCount > pack.MaxHintCases {
		result.EmittedHintCount = pack.MaxHintCases
	}
	if len(matches) == 0 {
		result.FallbackReason = "no match"
	} else {
		result.Match = &matches[0]
	}
	a.replyFactory(renderFactoryPackMatchDebugResult(result))
}

type factoryPackMatchDebugResult struct {
	PackLoadStatus   string
	ManifestSummary  string
	CandidateCount   int
	EmittedHintCount int
	Match            *pack.CaseMatch
	FallbackReason   string
}

func renderFactoryPackMatchDebugResult(result factoryPackMatchDebugResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "factory pack match-debug:\npack_load_status: %s", result.PackLoadStatus)
	if result.ManifestSummary != "" {
		fmt.Fprintf(&b, "\nmanifest_summary: %s", result.ManifestSummary)
	}
	fmt.Fprintf(&b, "\ncandidate_count: %d\nemitted_hint_count: %d", result.CandidateCount, result.EmittedHintCount)
	if result.Match != nil {
		fmt.Fprintf(&b, "\nmatched_case_id: %s\nscore: %d", result.Match.CaseID, result.Match.Score)
		for _, reason := range result.Match.WhyMatched {
			reason = strings.TrimSpace(reason)
			if reason != "" {
				fmt.Fprintf(&b, "\nwhy_matched: %s", reason)
			}
		}
	}
	if result.FallbackReason != "" {
		fmt.Fprintf(&b, "\nfallback_reason: %s", result.FallbackReason)
	}
	return b.String()
}

func factoryPackManifestSummary(manifest pack.Manifest) string {
	return fmt.Sprintf("%s schema=%s card_schema=%s cases=%d", manifest.PackName, manifest.SchemaVersion, manifest.CardSchemaVersion, manifest.CompiledCaseCount)
}

func factoryMatchDebugContext(text string) pack.DiagnosticContext {
	text = strings.TrimSpace(text)
	return pack.DiagnosticContext{
		Command:   "/diagnose",
		UserInput: text,
		Signals: pack.DiagnosticSignals{
			MainError: text,
			Keywords:  factoryMatchDebugKeywords(text),
		},
	}
}

func factoryMatchDebugKeywords(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(r == '_' || r == '-' || r == '.' || r == '/' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z')
	})
	seen := map[string]bool{}
	keywords := make([]string, 0, 12)
	for _, field := range fields {
		field = strings.Trim(field, ".,;:()[]{}<>\"'")
		if len(field) < 3 || seen[field] {
			continue
		}
		seen[field] = true
		keywords = append(keywords, field)
		if len(keywords) == 12 {
			break
		}
	}
	return keywords
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

func (a *Application) cmdFactoryCardCreateFromLastRun() {
	selected := factoryruntime.SelectLastRunSummary(a.latestDiagnoseSummary, a.latestFixSummary, a.latestRunKind)
	if selected.Diagnose == nil && selected.Fix == nil {
		a.replyFactory("No latest /diagnose or /fix run summary is available.")
		return
	}
	draft, err := card.NewDraftFromRunSummary(selected, card.DraftOptions{Reporter: a.issueUser})
	if err != nil {
		a.replyFactory(fmt.Sprintf("create draft card failed: %v", err))
		return
	}
	path, err := card.WriteDraftYAML(draft, filepath.FromSlash(card.DefaultDraftCardsDir))
	if err != nil {
		a.replyFactory(fmt.Sprintf("create draft card failed: %v", err))
		return
	}
	message := fmt.Sprintf("created draft card: %s", path)
	if selected.Warning != "" {
		message = selected.Warning + "\n" + message
	}
	a.replyFactory(message)
}
