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

func TestLayoutCardIDsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range layoutCards {
		if seen[c.id] {
			t.Errorf("duplicate layout card %q", c.id)
		}
		seen[c.id] = true
	}
}
