package gamescope

import "testing"

func TestInitialScale(t *testing.T) {
	for env, want := range map[string]float64{"": minScale, "junk": minScale, "-2": minScale, "0.5": minScale, "2": 2, "9": maxScale} {
		t.Setenv("Z13GUI_PLUS_SCALE", env)
		if got := initialScale(); got != want {
			t.Errorf("initialScale(%q) = %v, want %v", env, got, want)
		}
	}
}
