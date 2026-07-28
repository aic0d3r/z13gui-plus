package gui

import (
	"strings"
	"testing"

	"github.com/dahui/z13ctl/api"
)

func TestPresetStateChanged(t *testing.T) {
	policy := &api.PowerPolicy{Enabled: true, ACPreset: "AC", BatteryPreset: "Battery"}
	base := &api.State{ActivePreset: "AC", PowerSource: "ac", PowerPolicy: policy, Presets: map[string]api.Preset{"AC": {Profile: "performance"}}}
	same := &api.State{ActivePreset: "AC", PowerSource: "ac", PowerPolicy: &api.PowerPolicy{Enabled: true, ACPreset: "AC", BatteryPreset: "Battery"}, Presets: map[string]api.Preset{"AC": {Profile: "performance"}}}
	if presetStateChanged(base, same) {
		t.Fatal("presetStateChanged() = true for equivalent state")
	}
	same.ActivePreset = "Battery"
	if !presetStateChanged(base, same) {
		t.Fatal("presetStateChanged() = false after active preset changed")
	}
}

func TestPowerPolicyAssignments(t *testing.T) {
	ac, battery := powerPolicyAssignments(nil)
	if ac != "Not assigned" || battery != "Not assigned" {
		t.Fatalf("nil policy assignments = %q, %q", ac, battery)
	}

	state := &api.State{PowerPolicy: &api.PowerPolicy{
		Enabled: true, ACPreset: "Plugged In", BatteryPreset: "On Battery",
	}}
	ac, battery = powerPolicyAssignments(state)
	if ac != "Plugged In" || battery != "On Battery" {
		t.Fatalf("policy assignments = %q, %q", ac, battery)
	}
}

func TestPresetSummary(t *testing.T) {
	overdrive := 1
	preset := api.Preset{
		Profile:        "balanced",
		TDP:            &api.TDPState{PL1SPL: 45, PL2SPPT: 55, FPPT: 65},
		Undervolt:      &api.UndervoltState{CPUCO: -15},
		CPUPower:       &api.CPUPowerPreset{MinFrequencyKHz: 625000, EPP: "balance_power", Boost: true},
		RefreshRate:    180,
		PanelOverdrive: &overdrive,
	}
	want := "Profile: Balanced\nFan: Auto\nTDP: 45/55/65 W\nUndervolt: -15\nCPU: 625 MHz min · Efficient CPU · Boost on\nRefresh: 180 Hz\nOverdrive: On"
	if got := presetSummary(preset); got != want {
		t.Fatalf("presetSummary() = %q, want %q", got, want)
	}
	if got, want := compactPresetSummary(preset), "Balanced · Auto · 45/55/65 W"; got != want {
		t.Fatalf("compactPresetSummary() = %q, want %q", got, want)
	}
}

func TestCurrentSettingsSummaryUsesOverrideFlags(t *testing.T) {
	state := &api.State{
		Profile: "quiet",
		TDP:     &api.TDPState{PL1SPL: 55, PL2SPPT: 65, FPPT: 75},
	}
	if got := currentSettingsSummary(state); !strings.Contains(got, "TDP: Default") {
		t.Fatalf("inactive TDP included in current summary: %q", got)
	}
	state.TDPActive = true
	if got := currentSettingsSummary(state); !strings.Contains(got, "TDP: 55/65/75 W") {
		t.Fatalf("active TDP missing from current summary: %q", got)
	}
}

func TestPresetUsageSeparatesActiveAndAssigned(t *testing.T) {
	state := &api.State{
		ActivePreset: "Daily",
		PowerPolicy:  &api.PowerPolicy{ACPreset: "Daily", BatteryPreset: "Mobile"},
	}
	if got := presetUsage("Daily", state); got != "ACTIVE NOW · PLUGGED IN" {
		t.Fatalf("presetUsage() = %q", got)
	}
	if !presetAssigned("Daily", state) || presetAssigned("Manual", state) {
		t.Fatal("presetAssigned() did not reflect policy assignments")
	}
}

func TestEPPDisplayName(t *testing.T) {
	tests := map[string]string{
		"balance_performance": "Responsive CPU",
		"balance_power":       "Efficient CPU",
		"performance":         "Maximum performance",
		"power":               "Maximum efficiency",
	}
	for value, want := range tests {
		if got := eppDisplayName(value); got != want {
			t.Errorf("eppDisplayName(%q) = %q, want %q", value, got, want)
		}
	}
}
