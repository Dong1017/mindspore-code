package panels

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vigo999/mindspore-code/ui/model"
)

var (
	setupTitleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true).Align(lipgloss.Center)
	setupNormalStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	setupSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	setupDisabledStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	setupHintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	setupErrorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	setupLabelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	setupBadgeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	setupBorderStyle   = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("12")).
				Padding(0, 2)
)

// RenderSetupPopup renders the multi-step model setup popup.
func RenderSetupPopup(popup *model.SetupPopup) string {
	switch popup.Screen {
	case model.SetupScreenModeSelect:
		return renderModeSelect(popup)
	case model.SetupScreenPresetPicker:
		return renderPresetPicker(popup)
	case model.SetupScreenTokenInput:
		return renderTokenInput(popup)
	case model.SetupScreenEnvInfo:
		return renderEnvInfo(popup)
	default:
		return ""
	}
}

const (
	modeMSCODEProvided = "mscode-provided"
	modeModeOwn        = "own"
)

func renderModeSelect(popup *model.SetupPopup) string {
	modes := []struct {
		label string
		mode  string
	}{
		{"mscode-provided model", modeMSCODEProvided},
		{"your own model", modeModeOwn},
	}

	maxW := len("Model Setup")
	for _, m := range modes {
		if w := 2 + len(m.label) + 12; w > maxW {
			maxW = w
		}
	}

	var lines []string
	lines = append(lines, setupTitleStyle.Width(maxW).Render("Model Setup"))
	lines = append(lines, "")
	for i, m := range modes {
		marker := "  "
		style := setupNormalStyle
		if i == popup.ModeSelected {
			marker = "> "
			style = setupSelectedStyle
		}
		label := m.label
		if popup.CurrentMode == m.mode {
			label += setupBadgeStyle.Render("  (current)")
		}
		lines = append(lines, marker+style.Render(label))
	}
	lines = append(lines, "")
	hint := "↑/↓ select · enter confirm"
	if popup.CanEscape {
		hint += " · esc cancel"
	}
	lines = append(lines, setupHintStyle.Render(hint))

	return setupBorderStyle.Render(strings.Join(lines, "\n"))
}

func renderPresetPicker(popup *model.SetupPopup) string {
	maxW := len("mscode-provided")
	for _, opt := range popup.PresetOptions {
		if w := 2 + len(opt.Label) + 12; w > maxW {
			maxW = w
		}
	}

	var lines []string
	lines = append(lines, setupTitleStyle.Width(maxW).Render("mscode-provided"))
	lines = append(lines, "")
	for i, opt := range popup.PresetOptions {
		marker := "  "
		style := setupNormalStyle
		if opt.Disabled {
			style = setupDisabledStyle
		}
		if i == popup.PresetSelected {
			marker = "> "
			if !opt.Disabled {
				style = setupSelectedStyle
			}
		}
		label := opt.Label
		if opt.ID == popup.CurrentPreset {
			label += setupBadgeStyle.Render("  (current)")
		}
		lines = append(lines, marker+style.Render(label))
	}
	lines = append(lines, "")
	lines = append(lines, setupHintStyle.Render("↑/↓ select · enter · esc back"))

	return setupBorderStyle.Render(strings.Join(lines, "\n"))
}

func renderTokenInput(popup *model.SetupPopup) string {
	title := popup.SelectedPreset.Label
	if title == "" {
		title = "Enter Token"
	}

	var lines []string
	lines = append(lines, setupTitleStyle.Width(40).Render(title))
	lines = append(lines, "")
	lines = append(lines, setupLabelStyle.Render("Token: ")+renderTokenField(popup.TokenValue))
	if popup.TokenError != "" {
		lines = append(lines, "")
		lines = append(lines, setupErrorStyle.Render(popup.TokenError))
	}
	lines = append(lines, "")
	lines = append(lines, setupHintStyle.Render("enter apply · esc back"))

	return setupBorderStyle.Render(strings.Join(lines, "\n"))
}

var (
	tokenCursorStyle = lipgloss.NewStyle().Background(lipgloss.Color("252")).Foreground(lipgloss.Color("0"))
	tokenTextStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

func renderTokenField(token string) string {
	if len(token) == 0 {
		return tokenCursorStyle.Render(" ")
	}
	return tokenTextStyle.Render(maskToken(token)) + tokenCursorStyle.Render(" ")
}

func maskToken(token string) string {
	runes := []rune(token)
	n := len(runes)
	if n <= 8 {
		return token
	}
	return string(runes[:4]) + strings.Repeat("·", n-8) + string(runes[n-4:])
}

func renderEnvInfo(popup *model.SetupPopup) string {
	var lines []string
	lines = append(lines, setupTitleStyle.Width(50).Render("Your Own Model"))
	lines = append(lines, "")
	lines = append(lines, setupLabelStyle.Render("Set environment variables:"))
	lines = append(lines, "")
	lines = append(lines, setupNormalStyle.Render("  export MSCODE_PROVIDER=openai-completion"))
	lines = append(lines, setupNormalStyle.Render("  export MSCODE_BASE_URL=https://api.openai.com/v1"))
	lines = append(lines, setupNormalStyle.Render("  export MSCODE_API_KEY=sk-..."))
	lines = append(lines, setupNormalStyle.Render("  export MSCODE_MODEL=gpt-5.4"))
	lines = append(lines, "")
	lines = append(lines, setupHintStyle.Render("Then restart mscode."))
	lines = append(lines, "")
	lines = append(lines, setupHintStyle.Render("esc back"))

	return setupBorderStyle.Render(strings.Join(lines, "\n"))
}
