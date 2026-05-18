package app

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	case "publish":
		return runFactoryPackPublish(args[1:], opts)
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

func renderFactoryStatus() string {
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

func (a *Application) replyFactory(message string) {
	a.EventCh <- model.Event{Type: model.AgentReply, Message: message, RawANSI: strings.Contains(message, "\n")}
}

func renderFactoryHelpForSurface(surface factorySurface) string {
	if surface == factorySurfaceCLI {
		return "Factory commands:\n\nCard workflow:\n  mscli factory card submit {card-path}\n  mscli factory card review {card-id}\n  mscli factory card review {card-id} --approve --confidence observed --rationale \"{manual rationale}\"\n\nPack workflow:\n  mscli factory pack build {cards-dir} {output-pack}\n  mscli factory pack publish {pack-path}\n  mscli factory pack sync [{source-path}]\n  mscli factory pack match-debug \"{diagnose text}\"\n\nStatus:\n  mscli factory status"
	}
	return renderFactoryHelp()
}

func renderFactoryCardHelpForSurface(surface factorySurface) string {
	if surface == factorySurfaceCLI {
		return "Factory card commands:\n  mscli factory card submit {card-path}\n  mscli factory card review {card-id}\n  mscli factory card review {card-id} --approve --confidence observed --rationale \"{manual rationale}\""
	}
	return renderFactoryCardHelp()
}

func renderFactoryPackHelpForSurface(surface factorySurface) string {
	if surface == factorySurfaceCLI {
		return "Factory pack commands:\n  mscli factory pack build {cards-dir} {output-pack}\n  mscli factory pack publish {pack-path}\n  mscli factory pack sync [{source-path}]\n  mscli factory pack match-debug \"{diagnose text}\""
	}
	return renderFactoryPackHelp()
}

func factoryUsageError(surface factorySurface, command string) string {
	switch command {
	case "status":
		if surface == factorySurfaceCLI {
			return "Usage: mscli factory status"
		}
		return "Usage: /factory status"
	case "card submit":
		if surface == factorySurfaceCLI {
			return "Usage: mscli factory card submit {card-path}"
		}
		return "Usage: /factory card submit {card-path}"
	case "card review":
		if surface == factorySurfaceCLI {
			return "Usage: mscli factory card review {card-id} [--approve --confidence observed --rationale \"{manual rationale}\"]"
		}
		return "Usage: /factory card review {card-id} [--approve --confidence observed --rationale \"{manual rationale}\"]"
	case "pack build":
		if surface == factorySurfaceCLI {
			return "Usage: mscli factory pack build {cards-dir} {output-pack}"
		}
		return "Usage: /factory pack build {cards-dir} {output-pack}"
	case "pack publish":
		if surface == factorySurfaceCLI {
			return "Usage: mscli factory pack publish {pack-path}"
		}
		return "Usage: /factory pack publish {pack-path}"
	case "pack sync":
		if surface == factorySurfaceCLI {
			return "Usage: mscli factory pack sync [{source-path}]"
		}
		return "Usage: /factory pack sync [{source-path}]"
	case "pack match-debug":
		if surface == factorySurfaceCLI {
			return "Usage: mscli factory pack match-debug \"{diagnose text}\""
		}
		return "Usage: /factory pack match-debug \"{diagnose text}\""
	default:
		return renderFactoryHelpForSurface(surface)
	}
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

func runFactoryCardSubmit(args []string, opts factoryCommandOptions) (string, error) {
	if len(args) != 1 {
		message := factoryUsageError(opts.Surface, "card submit")
		if opts.Surface == factorySurfaceCLI {
			return "", errors.New(message)
		}
		return message, nil
	}
	bundle, err := card.SubmitDraftCard(args[0], card.SubmitOptions{})
	if err != nil {
		return "", fmt.Errorf("submit draft card failed: %w", err)
	}
	next := "/factory card review " + bundle.CardID
	if opts.Surface == factorySurfaceCLI {
		next = "mscli factory card review " + bundle.CardID
	}
	return fmt.Sprintf("created local review item: %s\nfiles: %s\nnext:\n  %s", bundle.CardID, filepath.ToSlash(bundle.Path)+"/", next), nil
}

func runFactoryCardReview(args []string, opts factoryCommandOptions) (string, error) {
	if len(args) == 1 {
		view, err := card.RenderReviewItem(args[0], card.ReviewOptions{})
		if err != nil {
			return "", fmt.Errorf("review card failed: %w", err)
		}
		next := fmt.Sprintf("/factory card review %s --approve --confidence observed --rationale \"{manual rationale}\"", args[0])
		if opts.Surface == factorySurfaceCLI {
			next = fmt.Sprintf("mscli factory card review %s --approve --confidence observed --rationale \"{manual rationale}\"", args[0])
		}
		return view + "\nnext:\n  " + next, nil
	}
	if len(args) == 6 && args[1] == "--approve" && args[2] == "--confidence" && args[4] == "--rationale" {
		result, err := card.ApproveReviewItem(args[0], card.ApprovalOptions{Confidence: args[3], Rationale: args[5]})
		if err != nil {
			return "", fmt.Errorf("approve review card failed: %w", err)
		}
		next := "/factory pack build factory/cards {output-pack}"
		if opts.Surface == factorySurfaceCLI {
			next = "mscli factory pack build factory/cards {output-pack}"
		}
		return fmt.Sprintf("approved local factory card: %s\nfile: %s\ngovernance: lifecycle=stable review_status=approved confidence=observed\npack build still required: %s\nnext:\n  %s", result.CardID, result.Path, next, next), nil
	}
	message := factoryUsageError(opts.Surface, "card review")
	if opts.Surface == factorySurfaceCLI {
		return "", errors.New(message)
	}
	return message, nil
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

func runFactoryPackBuild(args []string, opts factoryCommandOptions) (string, error) {
	if len(args) != 2 {
		message := factoryUsageError(opts.Surface, "pack build")
		if opts.Surface == factorySurfaceCLI {
			return "", errors.New(message)
		}
		return message, nil
	}
	cardsDir, outputPack := args[0], args[1]
	result, err := compiler.CompilePack(cardsDir, outputPack)
	if err != nil {
		return "", fmt.Errorf("build factory pack failed: %w", err)
	}
	next := "/factory pack publish " + result.OutputPath
	if opts.Surface == factorySurfaceCLI {
		next = "mscli factory pack publish " + result.OutputPath
	}
	return renderFactoryPackBuildResult(cardsDir, result) + "\nnext:\n  " + next, nil
}

func runFactoryPackPublish(args []string, opts factoryCommandOptions) (string, error) {
	if len(args) != 1 {
		message := factoryUsageError(opts.Surface, "pack publish")
		if opts.Surface == factorySurfaceCLI {
			return "", errors.New(message)
		}
		return message, nil
	}
	config := resolveFactoryServerConfig()
	if !config.Configured() {
		return "", errors.New("Factory pack server is not configured. Pass a local source path, set MSCLI_FACTORY_SERVER_URL/MSCLI_FACTORY_TOKEN, or login/create ~/.mscli/credentials.json.")
	}
	packPath := args[0]
	if _, err := pack.Load(packPath); err != nil {
		return "", fmt.Errorf("publish factory pack failed: validate pack: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	metadata, err := (&remote.Client{BaseURL: config.ServerURL, Token: config.Token}).PublishPack(ctx, packPath)
	if err != nil {
		return "", fmt.Errorf("publish factory pack failed: %w", err)
	}
	next := "/factory pack sync"
	if opts.Surface == factorySurfaceCLI {
		next = "mscli factory pack sync"
	}
	return renderFactoryPackPublishResult(packPath, config.ServerURL, metadata) + "\nnext:\n  " + next, nil
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

func runFactoryPackSync(args []string, opts factoryCommandOptions) (string, error) {
	if len(args) > 1 {
		message := factoryUsageError(opts.Surface, "pack sync")
		if opts.Surface == factorySurfaceCLI {
			return "", errors.New(message)
		}
		return message, nil
	}
	if len(args) == 1 {
		return syncFactoryPackFromLocalSource(args[0], opts)
	}
	config := resolveFactoryServerConfig()
	if config.Configured() {
		return syncFactoryPackFromRemote(config, opts)
	}
	return "", errors.New("Factory pack source is not configured. Pass a local source path, set MSCLI_FACTORY_SERVER_URL/MSCLI_FACTORY_TOKEN, or login/create ~/.mscli/credentials.json.")
}

func syncFactoryPackFromLocalSource(source string, opts factoryCommandOptions) (string, error) {
	result, err := pack.Sync(pack.SyncConfig{SourcePath: source})
	if err != nil {
		return "", errors.New(factoryPackSyncFailureMessage(err))
	}
	next := "/factory status"
	if opts.Surface == factorySurfaceCLI {
		next = "mscli factory status"
	}
	return renderFactoryPackSyncResult(result) + "\nnext:\n  " + next, nil
}

func syncFactoryPackFromRemote(config factoryServerConfig, opts factoryCommandOptions) (string, error) {
	tmp, err := os.CreateTemp("", "factory-remote-*.pack")
	if err != nil {
		return "", fmt.Errorf("sync factory pack failed: create temp pack: %w", err)
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	metadata, err := (&remote.Client{BaseURL: config.ServerURL, Token: config.Token}).DownloadLatestPack(ctx, tmpPath)
	if err != nil {
		return "", fmt.Errorf("sync factory pack failed: %w", err)
	}
	result, err := pack.Sync(pack.SyncConfig{SourcePath: tmpPath})
	if err != nil {
		return "", errors.New(factoryPackSyncFailureMessage(err))
	}
	next := "/factory status"
	if opts.Surface == factorySurfaceCLI {
		next = "mscli factory status"
	}
	return renderFactoryPackRemoteSyncResult(metadata, result) + "\nnext:\n  " + next, nil
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

func runFactoryPackMatchDebug(args []string, opts factoryCommandOptions) (string, error) {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		message := factoryUsageError(opts.Surface, "pack match-debug")
		if opts.Surface == factorySurfaceCLI {
			return "", errors.New(message)
		}
		return message, nil
	}
	loadedPack, err := pack.LoadDefault(pack.LoadConfig{})
	if err != nil {
		return renderFactoryPackMatchDebugResult(factoryPackMatchDebugResult{
			PackLoadStatus: "failed",
			FallbackReason: fmt.Sprintf("pack load failed: %v", err),
		}), nil
	}
	matches, err := loadedPack.MatchCases(factoryMatchDebugContext(args[0]).ToFingerprint(), pack.MatchOptions{})
	if err != nil {
		return renderFactoryPackMatchDebugResult(factoryPackMatchDebugResult{
			PackLoadStatus:  "loaded",
			ManifestSummary: factoryPackManifestSummary(loadedPack.Manifest),
			FallbackReason:  fmt.Sprintf("match failed: %v", err),
		}), nil
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
	return renderFactoryPackMatchDebugResult(result), nil
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
