package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/card"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/compiler"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/remote"
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
	case "status":
		a.cmdFactoryStatus(args[1:])
	case "card":
		a.cmdFactoryCard(args[1:])
	case "pack":
		a.cmdFactoryPack(args[1:])
	default:
		a.replyFactory(renderFactoryHelp())
	}
}

func renderFactoryStatusHelp() string {
	return "Usage: /factory status"
}

func (a *Application) cmdFactoryStatus(args []string) {
	if len(args) != 0 {
		a.replyFactory(renderFactoryStatusHelp())
		return
	}
	a.replyFactory(a.renderFactoryStatus())
}

func (a *Application) renderFactoryStatus() string {
	var b strings.Builder
	b.WriteString("factory status:")

	localPath, localPack := renderFactoryLocalPackStatus(&b)
	config := resolveFactoryServerConfig()
	serverConfigured := config.Configured()
	fmt.Fprintf(&b, "\nserver_configured: %t", serverConfigured)
	fmt.Fprintf(&b, "\nconfig_source: %s", factoryConfigSourceLabel(config.Source))
	if serverConfigured {
		renderFactoryServerStatus(&b, config, localPack)
	}
	renderFactoryCardCounts(&b)
	if localPath == "" {
		fmt.Fprintf(&b, "\nlocal_pack_path: unavailable")
	}
	return b.String()
}

func renderFactoryLocalPackStatus(b *strings.Builder) (string, *pack.Pack) {
	localPath, err := pack.DefaultPackPath()
	if err != nil {
		fmt.Fprintf(b, "\nlocal_pack_installed: false\nlocal_pack_path: unavailable\nlocal_pack_reason: %s", boundedFactoryReason(err))
		return "", nil
	}
	fmt.Fprintf(b, "\nlocal_pack_path: %s", localPath)
	loaded, err := pack.Load(localPath)
	if err != nil {
		fmt.Fprintf(b, "\nlocal_pack_installed: false")
		if !os.IsNotExist(err) {
			fmt.Fprintf(b, "\nlocal_pack_reason: %s", boundedFactoryReason(err))
		}
		return localPath, nil
	}
	manifest := loaded.Manifest
	fmt.Fprintf(b, "\nlocal_pack_installed: true")
	fmt.Fprintf(b, "\nlocal_pack_name: %s", manifest.PackName)
	fmt.Fprintf(b, "\nlocal_pack_version: %s", manifest.PackVersion)
	fmt.Fprintf(b, "\nlocal_schema_version: %s", manifest.SchemaVersion)
	fmt.Fprintf(b, "\nlocal_card_schema_version: %s", manifest.CardSchemaVersion)
	fmt.Fprintf(b, "\nlocal_compiled_case_count: %d", manifest.CompiledCaseCount)
	fmt.Fprintf(b, "\nlocal_checksum: %s", manifest.Checksum)
	return localPath, loaded
}

func renderFactoryServerStatus(b *strings.Builder, config factoryServerConfig, localPack *pack.Pack) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	metadata, err := (&remote.Client{BaseURL: config.ServerURL, Token: config.Token}).GetLatestPack(ctx)
	if err != nil {
		fmt.Fprintf(b, "\nserver_reachable: false\nserver_reason: %s", boundedFactoryReason(err))
		return
	}
	fmt.Fprintf(b, "\nserver_reachable: true")
	fmt.Fprintf(b, "\nserver_pack_id: %d", metadata.ID)
	fmt.Fprintf(b, "\nserver_pack_name: %s", metadata.PackName)
	fmt.Fprintf(b, "\nserver_pack_version: %s", metadata.PackVersion)
	fmt.Fprintf(b, "\nserver_schema_version: %s", metadata.SchemaVersion)
	fmt.Fprintf(b, "\nserver_card_schema_version: %s", metadata.CardSchemaVersion)
	fmt.Fprintf(b, "\nserver_compiled_case_count: %d", metadata.CompiledCaseCount)
	fmt.Fprintf(b, "\nserver_checksum: %s", metadata.Checksum)
	if localPack != nil && strings.TrimSpace(localPack.Manifest.Checksum) != "" && strings.TrimSpace(metadata.Checksum) != "" {
		fmt.Fprintf(b, "\nlocal_matches_server_latest: %t", localPack.Manifest.Checksum == metadata.Checksum)
	}
}

func renderFactoryCardCounts(b *strings.Builder) {
	fmt.Fprintf(b, "\ndraft_cards: %d", countFilesWithSuffix(filepath.FromSlash(card.DefaultDraftCardsDir), ".yaml"))
	fmt.Fprintf(b, "\nreview_items: %d", countDirs(filepath.FromSlash(card.DefaultSubmissionsDir)))
	fmt.Fprintf(b, "\napproved_cards: %d", countFilesWithSuffix(filepath.FromSlash(card.DefaultApprovedCardsDir), ".yaml"))
}

func factoryConfigSourceLabel(source string) string {
	if strings.TrimSpace(source) == "" {
		return "none"
	}
	return source
}

func boundedFactoryReason(err error) string {
	message := strings.TrimSpace(err.Error())
	if len(message) > 180 {
		return message[:180] + "..."
	}
	return message
}

func countFilesWithSuffix(root string, suffix string) int {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), suffix) {
			count++
		}
	}
	return count
}

func countDirs(root string) int {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}
	return count
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
	case "publish":
		a.cmdFactoryPackPublish(args[1:])
	case "sync":
		a.cmdFactoryPackSync(args[1:])
	case "match-debug":
		a.cmdFactoryPackMatchDebug(args[1:])
	default:
		a.replyFactory(renderFactoryPackHelp())
	}
}

func (a *Application) replyFactory(message string) {
	a.EventCh <- model.Event{Type: model.AgentReply, Message: message, RawANSI: strings.Contains(message, "\n")}
}

func renderFactoryHelp() string {
	return "Factory commands:\n\nCard workflow:\n  /factory card create\n  /factory card submit {card-path}\n  /factory card review {card-id}\n  /factory card review {card-id} --approve --confidence observed --rationale \"{manual rationale}\"\n\nPack workflow:\n  /factory pack build {cards-dir} {output-pack}\n  /factory pack publish {pack-path}\n  /factory pack sync [{source-path}]\n  /factory pack match-debug \"{diagnose text}\"\n\nStatus:\n  /factory status"
}

func renderFactoryCardHelp() string {
	return "Factory card commands:\n  /factory card create\n  /factory card submit {card-path}\n  /factory card review {card-id}\n  /factory card review {card-id} --approve --confidence observed --rationale \"{manual rationale}\""
}

func renderFactoryPackHelp() string {
	return "Factory pack commands:\n  /factory pack build {cards-dir} {output-pack}\n  /factory pack publish {pack-path}\n  /factory pack sync [{source-path}]\n  /factory pack match-debug \"{diagnose text}\""
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
		a.replyFactory("Usage: /factory card submit {card-path}")
		return
	}
	bundle, err := card.SubmitDraftCard(args[0], card.SubmitOptions{})
	if err != nil {
		a.replyFactory(fmt.Sprintf("submit draft card failed: %v", err))
		return
	}
	a.replyFactory(fmt.Sprintf("created local review item: %s\nfiles: %s\nnext:\n  /factory card review %s", bundle.CardID, filepath.ToSlash(bundle.Path)+"/", bundle.CardID))
}

func (a *Application) cmdFactoryCardReview(args []string) {
	if len(args) == 1 {
		view, err := card.RenderReviewItem(args[0], card.ReviewOptions{})
		if err != nil {
			a.replyFactory(fmt.Sprintf("review card failed: %v", err))
			return
		}
		view += fmt.Sprintf("\nnext:\n  /factory card review %s --approve --confidence observed --rationale \"{manual rationale}\"", args[0])
		a.replyFactory(view)
		return
	}
	if len(args) == 6 && args[1] == "--approve" && args[2] == "--confidence" && args[4] == "--rationale" {
		result, err := card.ApproveReviewItem(args[0], card.ApprovalOptions{Confidence: args[3], Rationale: args[5]})
		if err != nil {
			a.replyFactory(fmt.Sprintf("approve review card failed: %v", err))
			return
		}
		a.replyFactory(fmt.Sprintf("approved local factory card: %s\nfile: %s\ngovernance: lifecycle=stable review_status=approved confidence=observed\npack build still required: /factory pack build factory/cards {output-pack}\nnext:\n  /factory pack build factory/cards {output-pack}", result.CardID, result.Path))
		return
	}
	a.replyFactory("Usage: /factory card review {card-id} [--approve --confidence observed --rationale \"{manual rationale}\"]")
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
		a.replyFactory("Usage: /factory pack build {cards-dir} {output-pack}")
		return
	}
	cardsDir, outputPack := args[0], args[1]
	result, err := compiler.CompilePack(cardsDir, outputPack)
	if err != nil {
		a.replyFactory(fmt.Sprintf("build factory pack failed: %v", err))
		return
	}
	a.replyFactory(renderFactoryPackBuildResult(cardsDir, result) + "\nnext:\n  /factory pack publish " + result.OutputPath)
}

func (a *Application) cmdFactoryPackPublish(args []string) {
	if len(args) != 1 {
		a.replyFactory("Usage: /factory pack publish {pack-path}")
		return
	}
	config := resolveFactoryServerConfig()
	if !config.Configured() {
		a.replyFactory("Factory pack server is not configured. Pass a local source path, set MSCLI_FACTORY_SERVER_URL/MSCLI_FACTORY_TOKEN, or login/create ~/.mscli/credentials.json.")
		return
	}
	packPath := args[0]
	if _, err := pack.Load(packPath); err != nil {
		a.replyFactory(fmt.Sprintf("publish factory pack failed: validate pack: %v", err))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	metadata, err := (&remote.Client{BaseURL: config.ServerURL, Token: config.Token}).PublishPack(ctx, packPath)
	if err != nil {
		a.replyFactory(fmt.Sprintf("publish factory pack failed: %v", err))
		return
	}
	a.replyFactory(renderFactoryPackPublishResult(packPath, config.ServerURL, metadata) + "\nnext:\n  /factory pack sync")
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
		a.replyFactory("Usage: /factory pack sync [{source-path}]")
		return
	}
	if len(args) == 1 {
		a.syncFactoryPackFromLocalSource(args[0])
		return
	}
	config := resolveFactoryServerConfig()
	if config.Configured() {
		a.syncFactoryPackFromRemote(config)
		return
	}
	if source := a.factoryPackSource(); strings.TrimSpace(source) != "" {
		a.syncFactoryPackFromLocalSource(source)
		return
	}
	a.replyFactory("Factory pack source is not configured. Pass a local source path, set MSCLI_FACTORY_SERVER_URL/MSCLI_FACTORY_TOKEN, or login/create ~/.mscli/credentials.json.")
}

func (a *Application) syncFactoryPackFromLocalSource(source string) {
	result, err := pack.Sync(pack.SyncConfig{SourcePath: source})
	if err != nil {
		a.replyFactory(factoryPackSyncFailureMessage(err))
		return
	}
	a.replyFactory(renderFactoryPackSyncResult(result) + "\nnext:\n  /factory status")
}

func (a *Application) syncFactoryPackFromRemote(config factoryServerConfig) {
	tmp, err := os.CreateTemp("", "factory-remote-*.pack")
	if err != nil {
		a.replyFactory(fmt.Sprintf("sync factory pack failed: create temp pack: %v", err))
		return
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	metadata, err := (&remote.Client{BaseURL: config.ServerURL, Token: config.Token}).DownloadLatestPack(ctx, tmpPath)
	if err != nil {
		a.replyFactory(fmt.Sprintf("sync factory pack failed: %v", err))
		return
	}
	result, err := pack.Sync(pack.SyncConfig{SourcePath: tmpPath})
	if err != nil {
		a.replyFactory(factoryPackSyncFailureMessage(err))
		return
	}
	a.replyFactory(renderFactoryPackRemoteSyncResult(metadata, result) + "\nnext:\n  /factory status")
}

func factoryPackSyncFailureMessage(err error) string {
	message := fmt.Sprintf("sync factory pack failed: %v", err)
	if dest, destErr := pack.DefaultPackPath(); destErr == nil {
		if _, statErr := os.Stat(dest); statErr == nil {
			message += "\nExisting local factory pack was preserved."
		} else if os.IsNotExist(statErr) {
			message += "\nNo local factory pack was installed."
		}
	}
	return message
}

func renderFactoryPackPublishResult(source string, serverURL string, metadata *remote.PackMetadata) string {
	return fmt.Sprintf("published factory pack:\nsource: %s\nserver: %s\npack_id: %d\npack_name: %s\npack_version: %s\nschema_version: %s\ncard_schema_version: %s\ncompiled_case_count: %d\nchecksum: %s\npublisher: %s\ncreated_at: %s",
		source,
		serverURL,
		metadata.ID,
		metadata.PackName,
		metadata.PackVersion,
		metadata.SchemaVersion,
		metadata.CardSchemaVersion,
		metadata.CompiledCaseCount,
		metadata.Checksum,
		metadata.Publisher,
		metadata.CreatedAt,
	)
}

func renderFactoryPackRemoteSyncResult(metadata *remote.PackMetadata, result *pack.SyncResult) string {
	return fmt.Sprintf("synced factory pack from server:\nremote_id: %d\nsource: latest server pack\ndestination: %s\npack_name: %s\npack_version: %s\nschema_version: %s\ncard_schema_version: %s\ncompiled_case_count: %d\nchecksum: %s\npublisher: %s\ncreated_at: %s",
		metadata.ID,
		result.DestPath,
		result.PackName,
		result.PackVersion,
		result.SchemaVersion,
		result.CardSchemaVersion,
		result.CompiledCaseCount,
		result.Checksum,
		metadata.Publisher,
		metadata.CreatedAt,
	)
}

type factoryServerConfig struct {
	ServerURL string
	Token     string
	Source    string
}

func resolveFactoryServerConfig() factoryServerConfig {
	if config := factoryServerConfigFromEnv(); config.Configured() {
		return config
	}
	cred, err := loadCredentials()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return factoryServerConfig{}
		}
		return factoryServerConfig{}
	}
	config := factoryServerConfig{
		ServerURL: strings.TrimSpace(cred.ServerURL),
		Token:     strings.TrimSpace(cred.Token),
		Source:    "credentials.json",
	}
	if config.Configured() {
		return config
	}
	return factoryServerConfig{}
}

func factoryServerConfigFromEnv() factoryServerConfig {
	config := factoryServerConfig{
		ServerURL: strings.TrimSpace(os.Getenv("MSCLI_FACTORY_SERVER_URL")),
		Token:     strings.TrimSpace(os.Getenv("MSCLI_FACTORY_TOKEN")),
	}
	if config.Configured() {
		config.Source = "env"
	}
	return config
}

func (c factoryServerConfig) Configured() bool {
	return c.ServerURL != "" && c.Token != ""
}

func (a *Application) cmdFactoryPackMatchDebug(args []string) {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		a.replyFactory("Usage: /factory pack match-debug \"{diagnose text}\"")
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
	message := fmt.Sprintf("created draft card: %s\nnext:\n  /factory card submit %s", path, path)
	if selected.Warning != "" {
		message = selected.Warning + "\n" + message
	}
	a.replyFactory(message)
}
