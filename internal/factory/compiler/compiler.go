package compiler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/card"
)

type sourceCard struct {
	path string
	data []byte
	card *card.KnownIssueCard
}

func CompilePack(cardsDir string, outputPath string) (*BuildSummary, error) {
	if strings.TrimSpace(cardsDir) == "" {
		return nil, fmt.Errorf("cardsDir is required")
	}
	if strings.TrimSpace(outputPath) == "" {
		return nil, fmt.Errorf("outputPath is required")
	}

	sources, err := readSourceCards(cardsDir)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("no source cards found in %s", cardsDir)
	}

	summary := newBuildSummary(outputPath)
	summary.SourceCaseCount = len(sources)
	compiled := make([]*card.KnownIssueCard, 0, len(sources))

	for _, source := range sources {
		if err := card.ValidateDraft(source.card); err != nil {
			summary.InvalidCount++
			return nil, fmt.Errorf("validate %s: %w", source.path, err)
		}

		switch source.card.Lifecycle.State {
		case card.LifecycleDraft:
			summary.DraftExcluded++
			continue
		case card.LifecycleDeprecated:
			summary.DeprecatedExcluded++
			continue
		case card.LifecycleArchived:
			summary.ArchivedExcluded++
			continue
		}

		if err := card.ValidatePackEligible(source.card); err != nil {
			summary.InvalidCount++
			return nil, fmt.Errorf("pack eligibility %s: %w", source.path, err)
		}
		compiled = append(compiled, source.card)
	}

	if len(compiled) == 0 {
		return nil, fmt.Errorf("no eligible stable cards found")
	}

	summary.CompiledCaseCount = len(compiled)
	sourceHash := sourceContentHash(sources)
	manifest := buildManifest(summary, sourceHash)

	if err := writeSQLitePack(outputPath, compiled, manifest); err != nil {
		return nil, err
	}
	checksum, err := updateSQLitePackChecksum(outputPath)
	if err != nil {
		return nil, err
	}
	summary.Checksum = checksum
	return summary, nil
}

func readSourceCards(cardsDir string) ([]sourceCard, error) {
	info, err := os.Stat(cardsDir)
	if err != nil {
		return nil, fmt.Errorf("read cards dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("cardsDir is not a directory: %s", cardsDir)
	}

	var paths []string
	if err := filepath.WalkDir(cardsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yaml" || ext == ".yml" {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk cards dir: %w", err)
	}
	sort.Strings(paths)

	sources := make([]sourceCard, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read card %s: %w", path, err)
		}
		loaded, err := card.LoadFile(path)
		if err != nil {
			return nil, fmt.Errorf("load card %s: %w", path, err)
		}
		sources = append(sources, sourceCard{path: path, data: normalizeNewlines(data), card: loaded})
	}
	return sources, nil
}

func sourceContentHash(sources []sourceCard) string {
	h := sha256.New()
	for _, source := range sources {
		h.Write([]byte(filepath.ToSlash(source.path)))
		h.Write([]byte{0})
		h.Write(source.data)
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func normalizeNewlines(data []byte) []byte {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return []byte(text)
}
