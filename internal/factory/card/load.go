package card

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func LoadFile(path string) (*KnownIssueCard, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read card: %w", err)
	}
	var card KnownIssueCard
	if err := yaml.Unmarshal(data, &card); err != nil {
		return nil, fmt.Errorf("parse card yaml: %w", err)
	}
	return &card, nil
}
