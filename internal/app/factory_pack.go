package app

import (
	"fmt"
	"os"

	"gitcode.com/mindspore/mscli/internal/factory/compiler"
	"gitcode.com/mindspore/mscli/internal/factory/pack"
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
	next := factoryCommand(opts.Surface, "pack sync "+result.OutputPath)
	return renderFactoryPackBuildResult(cardsDir, result) + "\nnext:\n  " + next, nil
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
	if len(args) != 1 {
		message := factoryUsageError(opts.Surface, "pack sync")
		if opts.Surface == factorySurfaceCLI {
			return "", fmt.Errorf("%s", message)
		}
		return message, nil
	}
	return syncFactoryPackFromLocalSource(args[0], opts)
}

func syncFactoryPackFromLocalSource(source string, opts factoryCommandOptions) (string, error) {
	result, err := pack.Sync(pack.SyncConfig{SourcePath: source})
	if err != nil {
		return "", fmt.Errorf("%s", factoryPackSyncFailureMessage(err))
	}
	next := factoryCommand(opts.Surface, "status")
	return renderFactoryPackSyncResult(result) + "\nnext:\n  " + next, nil
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
