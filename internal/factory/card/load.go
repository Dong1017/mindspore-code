package card

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadFile(path string) (*KnownIssueCard, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read card: %w", err)
	}
	var card KnownIssueCard
	if err := yaml.Unmarshal(data, &card); err != nil {
		if isStructuredReferencesError(err) {
			return nil, fmt.Errorf("parse card yaml: provenance.references must be a list of strings, not objects")
		}
		return nil, fmt.Errorf("parse card yaml: %w", err)
	}
	return &card, nil
}

func isStructuredReferencesError(err error) bool {
	message := err.Error()
	return strings.Contains(message, "line") && strings.Contains(message, "cannot unmarshal !!map into string")
}
