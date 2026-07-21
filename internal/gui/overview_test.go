package gui

import "testing"

func TestUnifiedMemory(t *testing.T) {
	used, total := unifiedMemory(8192, 124000, 1536, 4096)
	if used != 9728 || total != 128*1024 {
		t.Fatalf("unifiedMemory() = %d / %d MiB", used, total)
	}
	if got := formatMemory(used, total); got != "9.5 / 128 GB  7%" {
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
		name  string
		util  int
		power float64
		want  string
	}{
		{"unavailable", 100, 0, "POWER N/A  ·  100% raw util"},
		{"low power", 100, 0.42, "LOW POWER  ·  0.42 W  ·  100% raw util"},
		{"active", 96, 1.85, "ACTIVE  ·  1.85 W  ·  96% raw util"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatNPU(tt.util, tt.power); got != tt.want {
				t.Fatalf("formatNPU() = %q, want %q", got, tt.want)
			}
		})
	}
}
