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
