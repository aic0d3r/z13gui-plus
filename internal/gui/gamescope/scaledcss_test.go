package gamescope

import (
	"strings"
	"testing"
)

// TestScaledCSS guards the 90+-verb fmt string: a verb/arg count mismatch
// panics at runtime, which no compile step catches.
func TestScaledCSS(t *testing.T) {
	b := &Backend{scale: 1.5}
	css := b.scaledCSS()
	for _, want := range []string{".section-collapse ", ".section-collapse-label", ".section-collapse-summary", ".section-collapse-action", ".disclosure-icon"} {
		if !strings.Contains(css, want) {
			t.Errorf("scaledCSS() missing %q", want)
		}
	}
	if !strings.Contains(css, "font-size: 20px") { // 13px label * 1.5
		t.Error("scaledCSS() label font not scaled")
	}
}

func TestScaledCSSNoVerbMismatch(t *testing.T) {
	css := (&Backend{scale: 1.5}).scaledCSS()
	if strings.Contains(css, "%!") {
		t.Fatal("scaledCSS() has a fmt verb/arg mismatch")
	}
	if !strings.Contains(css, "battery-time-metric .battery-metric-value { font-size: 27px") { // 18 * 1.5
		t.Error("battery time metric not scaled")
	}
}

func TestInitialScale(t *testing.T) {
	for env, want := range map[string]float64{"": minScale, "junk": minScale, "-2": minScale, "0.5": minScale, "2": 2, "9": maxScale} {
		t.Setenv("Z13GUI_PLUS_SCALE", env)
		if got := initialScale(); got != want {
			t.Errorf("initialScale(%q) = %v, want %v", env, got, want)
		}
	}
}
