package model

import "testing"

func TestSetupPopupPresetNavigatesAllItems(t *testing.T) {
	popup := &SetupPopup{
		Screen: SetupScreenPresetPicker,
		PresetOptions: []SelectionOption{
			{ID: "preset-a", Label: "preset-a"},
			{ID: "preset-b", Label: "preset-b"},
			{ID: "preset-c", Label: "preset-c", Disabled: true},
			{ID: "preset-d", Label: "preset-d", Disabled: true},
		},
		PresetSelected: 0,
	}

	// Cursor moves to disabled items (they are visually navigable)
	popup.MovePresetSelection(1)
	if popup.PresetSelected != 1 {
		t.Errorf("expected 1, got %d", popup.PresetSelected)
	}
	popup.MovePresetSelection(1)
	if popup.PresetSelected != 2 {
		t.Errorf("expected 2, got %d", popup.PresetSelected)
	}
	// Wraps around
	popup.MovePresetSelection(1)
	popup.MovePresetSelection(1)
	if popup.PresetSelected != 0 {
		t.Errorf("expected wrap to 0, got %d", popup.PresetSelected)
	}
}

func TestSetupPopupMoveMode(t *testing.T) {
	popup := &SetupPopup{
		Screen:       SetupScreenModeSelect,
		ModeSelected: 0,
	}
	popup.MoveModeSelection(1)
	if popup.ModeSelected != 0 {
		t.Errorf("expected 0, got %d", popup.ModeSelected)
	}
	popup.MoveModeSelection(1)
	if popup.ModeSelected != 0 {
		t.Errorf("expected 0, got %d", popup.ModeSelected)
	}
}
