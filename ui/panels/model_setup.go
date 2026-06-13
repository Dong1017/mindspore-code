package panels

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"gitcode.com/mindspore/mscli/ui/model"
)

// Style vars are populated by InitStyles() in styles.go.
var (
	setupTitleStyle    lipgloss.Style
	setupNormalStyle   lipgloss.Style
	setupSelectedStyle lipgloss.Style
	setupDisabledStyle lipgloss.Style
	setupHintStyle     lipgloss.Style
	setupErrorStyle    lipgloss.Style
	setupLabelStyle    lipgloss.Style
	setupBadgeStyle    lipgloss.Style
	setupBorderStyle   lipgloss.Style
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
	modeModeOwn = "own"
)

func renderModeSelect(popup *model.SetupPopup) string {
	modes := []struct {
		label string
		mode  string
	}{
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
			marker = "❯ "
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
	maxW := len("Model Presets")
	for _, opt := range popup.PresetOptions {
		if w := 2 + len(opt.Label) + 12; w > maxW {
			maxW = w
		}
	}

	var lines []string
	lines = append(lines, setupTitleStyle.Width(maxW).Render("Model Presets"))
	lines = append(lines, "")
	for i, opt := range popup.PresetOptions {
		marker := "  "
		style := setupNormalStyle
		if opt.Disabled {
			style = setupDisabledStyle
		}
		if i == popup.PresetSelected {
			marker = "❯ "
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

// Style vars populated by InitStyles() in styles.go.
var (
	tokenCursorStyle lipgloss.Style
	tokenTextStyle   lipgloss.Style
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
	if popup.Configured {
		lines = append(lines, setupLabelStyle.Render("Current model environment config:"))
		lines = append(lines, "")
		lines = append(lines, setupNormalStyle.Render("  MSCLI_PROVIDER="+popup.Provider))
		lines = append(lines, setupNormalStyle.Render("  MSCLI_BASE_URL="+popup.BaseURL))
		lines = append(lines, setupNormalStyle.Render("  MSCLI_MODEL="+popup.ModelName))
		return setupBorderStyle.Render(strings.Join(lines, "\n"))
	}

	lines = append(lines, setupLabelStyle.Render("Model environment config was not found. Set it before starting mscli:"))
	lines = append(lines, "")
	lines = append(lines, setupNormalStyle.Render("  export MSCLI_PROVIDER=anthropic"))
	lines = append(lines, setupNormalStyle.Render("  export MSCLI_BASE_URL=https://api.deepseek.com/anthropic"))
	lines = append(lines, setupNormalStyle.Render("  export MSCLI_API_KEY=<your DeepSeek API key>"))
	lines = append(lines, setupNormalStyle.Render("  export MSCLI_MODEL=deepseek-v4-pro"))
	lines = append(lines, "")
	lines = append(lines, setupHintStyle.Render("Then restart mscli."))

	return setupBorderStyle.Render(strings.Join(lines, "\n"))
}
