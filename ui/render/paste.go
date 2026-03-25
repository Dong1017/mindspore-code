package render

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	largePasteMinLines = 6
	largePasteMinRunes = 280
)

// SummarizeLargePaste returns a compact display label for oversized pasted
// content.
func SummarizeLargePaste(content string) (string, bool) {
	trimmed := strings.TrimRight(content, "\n")
	if trimmed == "" {
		return "", false
	}

	lines := strings.Count(trimmed, "\n") + 1
	runes := utf8.RuneCountInString(trimmed)
	if lines < largePasteMinLines && runes < largePasteMinRunes {
		return "", false
	}

	return fmt.Sprintf("[pasted content: %d lines, %d chars]", lines, runes), true
}
