package gui

import (
	"testing"

	"github.com/dahui/z13ctl/api"
)

func TestPresetStateChanged(t *testing.T) {
	policy := &api.PowerPolicy{Enabled: true, ACPreset: "AC", BatteryPreset: "Battery"}
	base := &api.State{ActivePreset: "AC", PowerSource: "ac", PowerPolicy: policy, Presets: map[string]api.Preset{"AC": {Profile: "custom"}}}
	same := &api.State{ActivePreset: "AC", PowerSource: "ac", PowerPolicy: &api.PowerPolicy{Enabled: true, ACPreset: "AC", BatteryPreset: "Battery"}, Presets: map[string]api.Preset{"AC": {Profile: "custom"}}}
	if presetStateChanged(base, same) {
		t.Fatal("presetStateChanged() = true for equivalent state")
	}
	same.ActivePreset = "Battery"
	if !presetStateChanged(base, same) {
		t.Fatal("presetStateChanged() = false after active preset changed")
	}
}
