package panels

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderRewindSummaryInputShowsCursorForPlaceholder(t *testing.T) {
	got := renderRewindSummaryInput("", 0, 40)
	if !strings.Contains(got, "add context (optional)") {
		t.Fatalf("summary input = %q, want placeholder", got)
	}
	if lipgloss.Width(got) != lipgloss.Width(" add context (optional)") {
		t.Fatalf("summary input width = %d, want placeholder plus cursor", lipgloss.Width(got))
	}
}

func TestRenderRewindSummaryInputShowsCursorAtEnd(t *testing.T) {
	got := renderRewindSummaryInput("keep decisions", len([]rune("keep decisions")), 40)
	if !strings.Contains(got, "keep") || !strings.Contains(got, "decisions") {
		t.Fatalf("summary input = %q, want typed context", got)
	}
	if lipgloss.Width(got) != lipgloss.Width("keep decisions ") {
		t.Fatalf("summary input width = %d, want typed context plus cursor", lipgloss.Width(got))
	}
}
