package gui

import "testing"

func TestHSLRoundTrip(t *testing.T) {
	for _, hex := range []string{"FF0000", "00FF00", "0000FF", "FFFFFF", "000000", "808080", "FF6600", "123456", "FFCC00"} {
		h, s, l := hexToHSL(hex)
		if got := hslToHex(h, s, l); got != hex {
			t.Errorf("%s -> HSL(%.1f,%.1f,%.1f) -> %s", hex, h, s, l, got)
		}
	}
}

func TestHexToHSLPrimaries(t *testing.T) {
	for hex, want := range map[string][3]float64{
		"FF0000": {0, 100, 50},
		"00FF00": {120, 100, 50},
		"0000FF": {240, 100, 50},
		"FFFFFF": {0, 0, 100},
	} {
		h, s, l := hexToHSL(hex)
		if int(h+0.5) != int(want[0]) || int(s+0.5) != int(want[1]) || int(l+0.5) != int(want[2]) {
			t.Errorf("%s: got %.1f %.1f %.1f, want %v", hex, h, s, l, want)
		}
	}
}
