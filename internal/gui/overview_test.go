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
		name       string
		available  bool
		util       int
		power      float64
		wantState  string
		wantDetail string
	}{
		{"sensor unavailable", false, 0, 0, "UNAVAILABLE", "NO DATA"},
		{"idle", true, 0, 0, "IDLE", "0.0 W"},
		{"power unavailable", true, 100, 0, "LOW POWER", "POWER N/A"},
		{"low power", true, 100, 0.42, "LOW POWER", "0.4 W"},
		{"active", true, 96, 1.85, "ACTIVE", "1.9 W"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, detail := formatNPU(tt.available, tt.util, tt.power)
			if state != tt.wantState || detail != tt.wantDetail {
				t.Fatalf("formatNPU() = %q, %q; want %q, %q", state, detail, tt.wantState, tt.wantDetail)
			}
		})
	}
}
