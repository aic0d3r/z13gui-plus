package gui

import (
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

func TestCompactPresetSummary(t *testing.T) {
	preset := api.Preset{
		Profile:     "balanced",
		TDP:         &api.TDPState{PL1SPL: 45, PL2SPPT: 55, FPPT: 65},
		Undervolt:   &api.UndervoltState{CPUCO: -15},
		RefreshRate: 180,
	}
	if got, want := compactPresetSummary(preset), "Balanced · 45 W · UV -15 · 180 Hz"; got != want {
		t.Fatalf("compactPresetSummary() = %q, want %q", got, want)
	}
}

func TestAutoPresetName(t *testing.T) {
	state := &api.State{Profile: "balanced", TDPActive: true, TDP: &api.TDPState{PL1SPL: 70}}
	if got := autoPresetName(state); got != "Balanced 70 W" {
		t.Fatalf("autoPresetName() = %q", got)
	}
	state.Presets = map[string]api.Preset{"Balanced 70 W": {}, "Balanced 70 W 2": {}}
	if got := autoPresetName(state); got != "Balanced 70 W 3" {
		t.Fatalf("autoPresetName() with collisions = %q", got)
	}
	state.TDPActive = false
	if got := autoPresetName(state); got != "Balanced" {
		t.Fatalf("autoPresetName() without TDP override = %q", got)
	}
}

func TestPresetUsageSeparatesActiveAndAssigned(t *testing.T) {
	state := &api.State{
		ActivePreset: "Daily",
		PowerPolicy:  &api.PowerPolicy{ACPreset: "Daily", BatteryPreset: "Mobile"},
	}
	if got := presetUsage("Daily", state); got != "Active now · Plugged in" {
		t.Fatalf("presetUsage() = %q", got)
	}
	if !presetAssigned("Daily", state) || presetAssigned("Manual", state) {
		t.Fatal("presetAssigned() did not reflect policy assignments")
	}
}

func TestAutomationCurrent(t *testing.T) {
	state := &api.State{
		PowerSource:  "ac",
		ActivePreset: "70W",
		PowerPolicy:  &api.PowerPolicy{Enabled: true, ACPreset: "70W", BatteryPreset: "On Battery"},
	}
	if name, manual := automationCurrent(state); name != "70W" || manual {
		t.Fatalf("applied preset = %q manual=%v", name, manual)
	}
	state.ActivePreset = ""
	if name, manual := automationCurrent(state); name != "70W" || !manual {
		t.Fatalf("after manual change = %q manual=%v", name, manual)
	}
	state.PowerPolicy.Enabled = false
	if _, manual := automationCurrent(state); manual {
		t.Fatal("manual reported while automation is off")
	}
	if got := assignedForSource(state); got != "70W" {
		t.Fatalf("assignedForSource() = %q", got)
	}
}

func TestPresetHighTDP(t *testing.T) {
	if presetHighTDP(api.Preset{TDP: &api.TDPState{PL1SPL: 75}}) {
		t.Fatal("75 W flagged as high TDP")
	}
	p := api.Preset{Profile: "balanced", TDP: &api.TDPState{PL1SPL: 80}}
	if !presetHighTDP(p) {
		t.Fatal("80 W not flagged")
	}
	if got := compactPresetSummary(p); got != "Balanced · 80 W · safety fans" {
		t.Fatalf("compactPresetSummary() = %q", got)
	}
}
