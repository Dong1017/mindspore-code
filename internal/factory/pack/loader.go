package pack

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Pack struct {
	Path     string
	Manifest Manifest
}

func LoadDefault(config LoadConfig) (*Pack, error) {
	path, err := resolveDefaultPath(config)
	if err != nil {
		return nil, err
	}
	return Load(path)
}

func Load(path string) (*Pack, error) {
	manifest, err := Inspect(path)
	if err != nil {
		return nil, err
	}
	if err := validateRuntimeChecksum(path, manifest.Checksum); err != nil {
		return nil, err
	}
	return &Pack{Path: path, Manifest: *manifest}, nil
}

func validateRuntimeChecksum(path string, expected string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open pack for checksum: %w", err)
	}
	defer db.Close()
	actual, err := ComputeRuntimeChecksum(db)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("pack checksum mismatch")
	}
	return nil
}

func requirePath(path string) error {
	if path == "" {
		return fmt.Errorf("pack path is required")
	}
	return nil
}
