package panels

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"gitcode.com/mindspore/mscli/ui/model"
)

func RenderRewindPicker(picker *model.RewindPicker, width, height int) string {
	if picker == nil {
		return ""
	}

	boxWidth := width - 4
	if boxWidth < 60 {
		boxWidth = width
	}
	boxHeight := height - 2
	if boxHeight < 14 {
		boxHeight = height
	}
	contentWidth := maxInt(24, boxWidth-6)
	contentHeight := maxInt(8, boxHeight-4)

	lines := []string{
		sessionPickerTitleStyle.Width(contentWidth).Render("Rewind Session"),
		sessionPickerMetaStyle.Render("Restore to before a previous user message"),
		"",
	}

	bodyHeight := contentHeight - len(lines) - 2
	if bodyHeight < 4 {
		bodyHeight = 4
	}
	if picker.Confirming {
		lines = append(lines, renderRewindConfirmBody(picker, contentWidth, bodyHeight)...)
		lines = append(lines, "")
		if picker.CapturingSummary {
			lines = append(lines, sessionPickerHintStyle.Render("enter summarize · esc restore options"))
		} else {
			lines = append(lines, sessionPickerHintStyle.Render("↑/↓ choose restore mode · enter confirm · esc back"))
		}
	} else {
		lines = append(lines, renderRewindListBody(picker, contentWidth, bodyHeight)...)
		lines = append(lines, "")
		lines = append(lines, sessionPickerHintStyle.Render("↑/↓ select checkpoint · enter continue · esc cancel"))
	}

	box := sessionPickerBorderStyle.
		Width(boxWidth).
		Height(boxHeight).
		Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

func renderRewindListBody(picker *model.RewindPicker, width, height int) []string {
	if len(picker.Items) == 0 {
		msg := picker.EmptyMessage
		if strings.TrimSpace(msg) == "" {
			msg = "No checkpoints available for this session."
		}
		return []string{sessionPickerEmptyStyle.Width(width).Render(msg)}
	}

	const linesPerItem = 5
	visibleCount := height / linesPerItem
	if visibleCount < 1 {
		visibleCount = 1
	}
	if visibleCount > len(picker.Items) {
		visibleCount = len(picker.Items)
	}

	start := picker.Selected - visibleCount/2
	if start < 0 {
		start = 0
	}
	if maxStart := len(picker.Items) - visibleCount; start > maxStart {
		start = maxStart
	}
	end := start + visibleCount

	lines := make([]string, 0, visibleCount*linesPerItem+2)
	if start > 0 {
		lines = append(lines, sessionPickerMetaStyle.Render(fmt.Sprintf("%d older checkpoint(s) above", start)))
	} else {
		lines = append(lines, "")
	}
	for idx := start; idx < end; idx++ {
		lines = append(lines, renderRewindListItem(picker.Items[idx], idx == picker.Selected, width-2)...)
	}
	if end < len(picker.Items) {
		lines = append(lines, sessionPickerMetaStyle.Render(fmt.Sprintf("%d older checkpoint(s) below", len(picker.Items)-end)))
	}
	return lines
}

func renderRewindListItem(item model.RewindCheckpointItem, selected bool, width int) []string {
	marker := "  "
	titleStyle := sessionPickerNormalStyle
	metaStyle := sessionPickerMetaStyle
	previewStyle := sessionPickerPreviewStyle
	if selected {
		marker = "> "
		titleStyle = sessionPickerSelectedStyle
		metaStyle = sessionPickerSelectedMetaStyle
		previewStyle = sessionPickerSelectedStyle
	}

	restoreMode := "conversation only"
	if item.HasCodeRestore {
		restoreMode = "code + conversation available"
	}
	return []string{
		marker + titleStyle.Render(item.Timestamp.Format("2006-01-02 15:04:05")),
		"  " + previewStyle.Render(truncateSessionPickerText(item.Preview, width)),
		"  " + previewStyle.Render("Keep last: "+truncateSessionPickerText(sessionPickerLastUserText(item.LastUserInput), maxInt(0, width-13))),
		"  " + metaStyle.Render(sessionPickerTurnCountText(item.TurnCount)+" before here · "+restoreMode),
		"",
	}
}

func renderRewindConfirmBody(picker *model.RewindPicker, width, _ int) []string {
	item := picker.SelectedItem()
	if item == nil {
		return []string{sessionPickerEmptyStyle.Width(width).Render("No checkpoint selected.")}
	}

	selectedConversation := picker.ConfirmMode() == model.RewindRestoreConversation
	selectedCode := picker.ConfirmMode() == model.RewindRestoreCodeConversation
	selectedSummary := picker.ConfirmMode() == model.RewindRestoreSummarize

	conversationStyle := sessionPickerNormalStyle
	codeStyle := sessionPickerNormalStyle
	summaryStyle := sessionPickerNormalStyle
	if selectedConversation {
		conversationStyle = sessionPickerSelectedStyle
	}
	if selectedCode {
		codeStyle = sessionPickerSelectedStyle
	}
	if selectedSummary {
		summaryStyle = sessionPickerSelectedStyle
	}

	lines := []string{
		sessionPickerNormalStyle.Render("Restore to before this message:"),
		sessionPickerPreviewStyle.Render(truncateSessionPickerText(item.Preview, width)),
		sessionPickerMetaStyle.Render(item.Timestamp.Format("2006-01-02 15:04:05")),
		sessionPickerMetaStyle.Render(sessionPickerTurnCountText(item.TurnCount) + " will remain"),
		sessionPickerMetaStyle.Render("Last kept prompt: " + truncateSessionPickerText(sessionPickerLastUserText(item.LastUserInput), maxInt(0, width-18))),
		"",
		(optionMarker(selectedConversation) + conversationStyle.Render("restore conversation")),
	}
	if item.HasCodeRestore {
		lines = append(lines, optionMarker(selectedCode)+codeStyle.Render("restore code + conversation"))
	} else {
		lines = append(lines, "  "+sessionPickerMetaStyle.Render("restore code + conversation (no tracked write/edit changes)"))
	}
	lines = append(lines, optionMarker(selectedSummary)+summaryStyle.Render("summarize from here"))
	if picker.CapturingSummary {
		lines = append(lines, "  "+renderRewindSummaryInput(picker.SummaryInput, picker.SummaryCursor, width-2))
	}
	lines = append(lines, "")
	lines = append(lines, sessionPickerMetaStyle.Render("Warning: shell/manual edits are not restored. Only write/edit tool changes are tracked."))
	return lines
}

func renderRewindSummaryInput(input string, cursor, width int) string {
	if width < 1 {
		width = 1
	}
	if input == "" {
		placeholder := truncateSessionPickerText("add context (optional)", maxInt(0, width-1))
		return tokenCursorStyle.Render(" ") + sessionPickerMetaStyle.Render(placeholder)
	}

	runes := []rune(input)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}

	maxContent := width
	if cursor == len(runes) {
		maxContent = width - 1
	}
	if maxContent < 0 {
		maxContent = 0
	}

	start := 0
	if maxContent > 0 && len(runes) > maxContent {
		if cursor == len(runes) {
			start = len(runes) - maxContent
		} else if cursor >= maxContent {
			start = cursor - maxContent + 1
		}
		if maxStart := len(runes) - maxContent; start > maxStart {
			start = maxStart
		}
	}
	if start < 0 {
		start = 0
	}

	end := len(runes)
	if maxContent == 0 {
		end = start
	} else if end > start+maxContent {
		end = start + maxContent
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		ch := rewindSummaryVisibleRune(runes[i])
		if i == cursor {
			b.WriteString(tokenCursorStyle.Render(ch))
		} else {
			b.WriteString(sessionPickerPreviewStyle.Render(ch))
		}
	}
	if cursor == len(runes) {
		b.WriteString(tokenCursorStyle.Render(" "))
	}
	return b.String()
}

func rewindSummaryVisibleRune(ch rune) string {
	if ch == ' ' {
		return "\u00a0"
	}
	return string(ch)
}

func optionMarker(selected bool) string {
	if selected {
		return "> "
	}
	return "  "
}
