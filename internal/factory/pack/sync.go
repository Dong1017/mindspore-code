package pack

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type SyncConfig struct {
	SourcePath string
	DestPath   string
}

type SyncResult struct {
	SourcePath        string
	DestPath          string
	PackName          string
	PackVersion       string
	SchemaVersion     string
	CardSchemaVersion string
	CompiledCaseCount int
	Checksum          string
}

func Sync(config SyncConfig) (*SyncResult, error) {
	source, err := resolveSyncSource(config.SourcePath)
	if err != nil {
		return nil, err
	}
	dest, err := resolveSyncDest(config.DestPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return nil, fmt.Errorf("create factory pack dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".factory-core-*.pack")
	if err != nil {
		return nil, fmt.Errorf("create temp pack: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := copyFileTo(source, tmp); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("close temp pack: %w", err)
	}
	loaded, err := Load(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("validate source pack: %w", err)
	}
	if err := installValidatedPack(tmpPath, dest); err != nil {
		return nil, err
	}
	return &SyncResult{
		SourcePath:        source,
		DestPath:          dest,
		PackName:          loaded.Manifest.PackName,
		PackVersion:       loaded.Manifest.PackVersion,
		SchemaVersion:     loaded.Manifest.SchemaVersion,
		CardSchemaVersion: loaded.Manifest.CardSchemaVersion,
		CompiledCaseCount: loaded.Manifest.CompiledCaseCount,
		Checksum:          loaded.Manifest.Checksum,
	}, nil
}

func resolveSyncSource(source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", fmt.Errorf("factory pack source is required")
	}
	if strings.HasPrefix(strings.ToLower(source), "file://") {
		u, err := url.Parse(source)
		if err != nil {
			return "", fmt.Errorf("parse file source: %w", err)
		}
		if u.Scheme != "file" || u.Host != "" {
			return "", fmt.Errorf("unsupported factory pack source: %s", source)
		}
		source = filepath.FromSlash(u.Path)
	}
	path, err := filepath.Abs(source)
	if err != nil {
		return "", fmt.Errorf("resolve source pack: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("source pack does not exist: %s", source)
		}
		return "", fmt.Errorf("stat source pack: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("source pack is a directory: %s", source)
	}
	return path, nil
}

func resolveSyncDest(dest string) (string, error) {
	dest = strings.TrimSpace(dest)
	if dest != "" {
		path, err := filepath.Abs(dest)
		if err != nil {
			return "", fmt.Errorf("resolve destination pack: %w", err)
		}
		return path, nil
	}
	return DefaultPackPath()
}

func copyFileTo(source string, dest *os.File) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open source pack: %w", err)
	}
	defer in.Close()
	if _, err := io.Copy(dest, in); err != nil {
		return fmt.Errorf("copy source pack: %w", err)
	}
	return nil
}

func installValidatedPack(tmpPath, dest string) error {
	backup := dest + ".bak"
	_ = os.Remove(backup)
	hadExisting := false
	_, statErr := os.Stat(dest)
	if statErr == nil {
		hadExisting = true
		if err := os.Rename(dest, backup); err != nil {
			return fmt.Errorf("backup existing pack: %w", err)
		}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("stat existing pack: %w", statErr)
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		if hadExisting {
			_ = os.Rename(backup, dest)
		}
		return fmt.Errorf("install factory pack: %w", err)
	}
	if hadExisting {
		_ = os.Remove(backup)
	}
	return nil
}
