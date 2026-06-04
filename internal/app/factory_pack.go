package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/compiler"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/remote"
)

func runFactoryPackBuild(args []string, opts factoryCommandOptions) (string, error) {
	if len(args) != 2 {
		message := factoryUsageError(opts.Surface, "pack build")
		if opts.Surface == factorySurfaceCLI {
			return "", fmt.Errorf("%s", message)
		}
		return message, nil
	}
	cardsDir, outputPack := args[0], args[1]
	result, err := compiler.CompilePack(cardsDir, outputPack)
	if err != nil {
		return "", fmt.Errorf("build factory pack failed: %w", err)
	}
	next := factoryCommand(opts.Surface, "pack publish "+result.OutputPath)
	return renderFactoryPackBuildResult(cardsDir, result) + "\nnext:\n  " + next, nil
}

func runFactoryPackPublish(args []string, opts factoryCommandOptions) (string, error) {
	if len(args) != 1 {
		message := factoryUsageError(opts.Surface, "pack publish")
		if opts.Surface == factorySurfaceCLI {
			return "", fmt.Errorf("%s", message)
		}
		return message, nil
	}
	config := resolveFactoryServerConfig()
	if !config.Configured() {
		return "", fmt.Errorf("Factory pack server is not configured. Pass a local source path, set MSCLI_FACTORY_SERVER_URL/MSCLI_FACTORY_TOKEN, or login/create ~/.mscli/credentials.json.")
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
	next := factoryCommand(opts.Surface, "pack sync")
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
			return "", fmt.Errorf("%s", message)
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
	return "", fmt.Errorf("Factory pack source is not configured. Pass a local source path, set MSCLI_FACTORY_SERVER_URL/MSCLI_FACTORY_TOKEN, or login/create ~/.mscli/credentials.json.")
}

func syncFactoryPackFromLocalSource(source string, opts factoryCommandOptions) (string, error) {
	result, err := pack.Sync(pack.SyncConfig{SourcePath: source})
	if err != nil {
		return "", fmt.Errorf("%s", factoryPackSyncFailureMessage(err))
	}
	next := factoryCommand(opts.Surface, "status")
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
		return "", fmt.Errorf("%s", factoryPackSyncFailureMessage(err))
	}
	next := factoryCommand(opts.Surface, "status")
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
