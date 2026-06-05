package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/card"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
)

func renderFactoryStatus() string {
	var b strings.Builder
	b.WriteString("factory status:")

	localPath, _ := renderFactoryLocalPackStatus(&b)
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

func renderFactoryCardCounts(b *strings.Builder) {
	fmt.Fprintf(b, "\ndraft_cards: %d", countFilesWithSuffix(filepath.FromSlash(card.DefaultDraftCardsDir), ".yaml"))
	fmt.Fprintf(b, "\nreview_items: %d", countDirs(filepath.FromSlash(card.DefaultSubmissionsDir)))
	fmt.Fprintf(b, "\napproved_cards: %d", countFilesWithSuffix(filepath.FromSlash(card.DefaultApprovedCardsDir), ".yaml"))
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
