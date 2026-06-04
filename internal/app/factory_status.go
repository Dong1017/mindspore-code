package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/card"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/remote"
)

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
	if _, err := os.Stat(localPath); err != nil {
		fmt.Fprintf(b, "\nlocal_pack_installed: false")
		if os.IsNotExist(err) {
			fmt.Fprintf(b, "\nlocal_pack_reason: not installed")
		} else {
			fmt.Fprintf(b, "\nlocal_pack_reason: %s", boundedFactoryReason(err))
		}
		return localPath, nil
	}
	loaded, err := pack.Load(localPath)
	if err != nil {
		fmt.Fprintf(b, "\nlocal_pack_installed: false")
		fmt.Fprintf(b, "\nlocal_pack_reason: %s", boundedFactoryReason(err))
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
