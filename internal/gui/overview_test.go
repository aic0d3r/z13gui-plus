package gui

import "testing"

func TestUnifiedMemory(t *testing.T) {
	used, total := unifiedMemory(8192, 124000, 1536, 4096)
	if used != 9728 || total != 128*1024 {
		t.Fatalf("unifiedMemory() = %d / %d MiB", used, total)
	}
	if got := formatMemory(used, total); got != "9.5 / 128 GB" {
		t.Fatalf("formatMemory() = %q", got)
	}
}

func TestUnifiedMemoryRequiresSystemMemory(t *testing.T) {
	used, total := unifiedMemory(0, 0, 1536, 4096)
	if used != 0 || total != 0 {
		t.Fatalf("partial unified memory should be unavailable, got %d / %d MiB", used, total)
	}
}

func TestFormatNPU(t *testing.T) {
	tests := []struct {
		name      string
		available bool
		util      int
		power     float64
		clock     int
		want      string
	}{
		{"sensor unavailable", false, 0, 0, 0, "NO DATA"},
		{"clock fallback", true, 0, 0, 1267, "1.3 GHz"},
		{"power unavailable", true, 100, 0, 0, "100%"},
		{"active fallback", true, 0, 0, 0, "ACTIVE"},
		{"low power", true, 100, 0.42, 1267, "0.4 W · 100%"},
		{"active", true, 96, 1.85, 1267, "1.9 W · 96%"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatNPU(tt.available, tt.util, tt.power, tt.clock); got != tt.want {
				t.Fatalf("formatNPU() = %q; want %q", got, tt.want)
			}
		})
	}
}
