package gui

import (
	"testing"

	"github.com/dahui/z13ctl/api"
)

func TestHiddenCards(t *testing.T) {
	got := hiddenCards(" overview.fans, tab.rgb ,,")
	if len(got) != 2 || !got["overview.fans"] || !got["tab.rgb"] {
		t.Fatalf("hiddenCards = %v", got)
	}
	if len(hiddenCards("")) != 0 {
		t.Fatal("empty list must hide nothing")
	}
}

func TestCardUnavailable(t *testing.T) {
	w := &Window{}
	if w.cardUnavailable("tab.rgb") || w.cardUnavailable("overview.battery") {
		t.Fatal("unknown state must count as present")
	}
	w.state = &api.State{DevicePresence: map[string]bool{"keyboard": false, "lightbar": false}}
	for _, id := range []string{"tab.rgb", "overview.battery", "power.battery", "power.boost"} {
		if !w.cardUnavailable(id) {
			t.Errorf("%s should be unavailable on a machine without that hardware", id)
		}
	}
	w.state = &api.State{
		DevicePresence: map[string]bool{"keyboard": true},
		BatteryDetail:  &api.BatteryState{},
		CPUPower:       &api.CPUPowerState{},
	}
	for _, id := range []string{"tab.rgb", "overview.battery", "power.battery", "power.boost"} {
		if w.cardUnavailable(id) {
			t.Errorf("%s should be available", id)
		}
	}
}

func TestCardUnavailableFromCapabilities(t *testing.T) {
	w := &Window{state: &api.State{
		BatteryDetail: &api.BatteryState{},
		CPUPower:      &api.CPUPowerState{},
		Capabilities:  &api.Capabilities{},
	}}
	// A machine with none of the ASUS interfaces: everything ASUS-specific goes.
	for _, id := range []string{"power.profile", "power.fan", "power.battery", "set.refresh", "set.overdrive", "set.bootsound", "set.display"} {
		if !w.cardUnavailable(id) {
			t.Errorf("%s should be hidden when its capability is false", id)
		}
	}
	// CPU policy still works without ASUS interfaces, so tuning stays.
	if w.cardUnavailable("power.tuning") {
		t.Error("power.tuning must stay while cpu_power is readable")
	}
	w.state.CPUPower = nil
	if !w.cardUnavailable("power.tuning") {
		t.Error("power.tuning should hide when nothing can be tuned")
	}
	// Display card survives if any one control exists.
	w.state.Capabilities = &api.Capabilities{BootSound: true}
	if w.cardUnavailable("set.display") || w.cardUnavailable("set.bootsound") || !w.cardUnavailable("set.refresh") {
		t.Error("display card must follow its remaining controls")
	}
	// Older daemon: no capabilities means everything is present.
	w.state.Capabilities = nil
	if w.cardUnavailable("power.fan") || w.cardUnavailable("set.display") {
		t.Error("missing capabilities must read as all present")
	}
}

func TestLayoutCardIDsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range layoutCards {
		if seen[c.id] {
			t.Errorf("duplicate layout card %q", c.id)
		}
		seen[c.id] = true
	}
}
