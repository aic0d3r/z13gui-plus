package cssscale

import "testing"

func TestPx(t *testing.T) {
	tests := []struct {
		in   string
		s    float64
		want string
	}{
		{".a { min-height: 40px; padding: 4px 8px; }", 1.5, ".a { min-height: 60px; padding: 6px 12px; }"},
		{".a { letter-spacing: 0.5px; margin: -2px; }", 1.5, ".a { letter-spacing: 0.75px; margin: -3px; }"},
		{".a { border: 1px solid red; }", 1.25, ".a { border: 1.25px solid red; }"},
		{".a { font-size: 13px; width: 50%; line-height: 1.2em; }", 2, ".a { font-size: 26px; width: 50%; line-height: 1.2em; }"},
		{".a { letter-spacing: 0; }", 1.5, ".a { letter-spacing: 0; }"},
		{".a { font-size: 11px; }", 1.333, ".a { font-size: 14.66px; }"},
	}
	for _, tt := range tests {
		if got := Px(tt.in, tt.s); got != tt.want {
			t.Errorf("Px(%q, %v) = %q, want %q", tt.in, tt.s, got, tt.want)
		}
	}
}
