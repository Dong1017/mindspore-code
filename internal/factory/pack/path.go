package pack

import (
	"fmt"
	"os"
	"path/filepath"
)

type LoadConfig struct {
	PackPath string
}

func DefaultPackPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	if home == "" {
		return "", fmt.Errorf("home dir cannot be empty")
	}
	return filepath.Join(home, ".mscli", "factory", FileName), nil
}

func resolveDefaultPath(config LoadConfig) (string, error) {
	if config.PackPath != "" {
		return config.PackPath, nil
	}
	return DefaultPackPath()
}
