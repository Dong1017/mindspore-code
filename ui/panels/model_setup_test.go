package panels

import (
	"strings"
	"testing"

	"gitcode.com/mindspore/mscli/ui/model"
)

func TestRenderSetupPopupModeSelect(t *testing.T) {
	popup := &model.SetupPopup{
		Screen:       model.SetupScreenModeSelect,
		ModeSelected: 0,
		CanEscape:    true,
	}
	result := RenderSetupPopup(popup)
	if !strings.Contains(result, "your own model") {
		t.Error("expected 'your own model' in output")
	}
}

func TestRenderSetupPopupEnvInfo(t *testing.T) {
	popup := &model.SetupPopup{
		Screen:    model.SetupScreenEnvInfo,
		CanEscape: true,
	}
	result := RenderSetupPopup(popup)
	if !strings.Contains(result, "MSCLI_PROVIDER") {
		t.Error("expected env var example in output")
	}
	if !strings.Contains(result, "MSCLI_API_KEY") {
		t.Error("expected MSCLI_API_KEY in output")
	}
}
