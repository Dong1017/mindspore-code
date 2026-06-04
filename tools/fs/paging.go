package fs

import (
	"fmt"
	"strings"
)

const defaultSearchResultLimit = 100

func normalizeSearchResultLimit(limit int) int {
	if limit <= 0 || limit > defaultSearchResultLimit {
		return defaultSearchResultLimit
	}
	return limit
}

func pagedSearchSummary(total, offset, returned int, noun string) string {
	if total <= 0 {
		return fmt.Sprintf("0 %s", noun)
	}
	if returned <= 0 {
		return fmt.Sprintf("showing 0 of %d %s", total, noun)
	}

	start := 1
	if offset > 1 {
		start = offset
	}
	end := start + returned - 1
	return fmt.Sprintf("showing %d-%d of %d %s", start, end, total, noun)
}

func buildSearchResultContent(summary string, lines []string) string {
	if len(lines) == 0 {
		return summary
	}
	return summary + "\n" + strings.Join(lines, "\n")
}

func sliceWithOffsetLimit[T any](items []T, offset, limit int) []T {
	start := 0
	if offset > 1 {
		start = offset - 1
	}
	if start >= len(items) {
		return nil
	}

	end := len(items)
	if limit > 0 && start+limit < end {
		end = start + limit
	}

	return items[start:end]
}
