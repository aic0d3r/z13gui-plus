package gui

import (
	"testing"

	"github.com/dahui/z13ctl/api"
)

func TestMatchingFanPreset(t *testing.T) {
	if got := matchingFanPreset(&api.FanCurveState{Mode: 2}); got != "auto" {
		t.Fatalf("automatic matchingFanPreset() = %q, want auto", got)
	}

	curve := &api.FanCurveState{
		Mode: 1,
		Points: []api.FanCurvePoint{
			{Temp: 30, PWM: 13}, {Temp: 45, PWM: 25},
			{Temp: 55, PWM: 51}, {Temp: 65, PWM: 89},
			{Temp: 75, PWM: 140}, {Temp: 82, PWM: 191},
			{Temp: 88, PWM: 229}, {Temp: 95, PWM: 255},
		},
	}
	if got := matchingFanPreset(curve); got != "balanced" {
		t.Fatalf("matchingFanPreset() = %q, want balanced", got)
	}

	curve.Points[0].PWM++
	if got := matchingFanPreset(curve); got != "" {
		t.Fatalf("custom curve matched preset %q", got)
	}
}

func TestFanStatus(t *testing.T) {
	tests := []struct {
		state       *api.State
		wantActive  string
		wantSummary string
		wantLocked  bool
	}{
		{nil, "", "—", false},
		{&api.State{}, "auto", "AUTO", false},
		{&api.State{FanCurveActive: true, FanCurve: &api.FanCurveState{Mode: 1}}, "", "CUSTOM", false},
		{&api.State{FanSafetyActive: true}, "", "SAFETY LOCK", true},
	}
	for _, tt := range tests {
		active, summary, _, locked := fanStatus(tt.state)
		if active != tt.wantActive || summary != tt.wantSummary || locked != tt.wantLocked {
			t.Errorf("fanStatus(%v) = %q, %q, %t", tt.state, active, summary, locked)
		}
	}
}

func TestFanPWMAtTemp(t *testing.T) {
	points := [8]api.FanCurvePoint{
		{Temp: 40, PWM: 0}, {Temp: 50, PWM: 100},
		{Temp: 60, PWM: 100}, {Temp: 70, PWM: 100},
		{Temp: 80, PWM: 100}, {Temp: 90, PWM: 100},
		{Temp: 100, PWM: 100}, {Temp: 110, PWM: 100},
	}
	for temp, want := range map[int]int{35: 0, 40: 0, 45: 50, 50: 100, 105: 100} {
		if got := fanPWMAtTemp(points, temp); got != want {
			t.Errorf("fanPWMAtTemp(%d) = %d, want %d", temp, got, want)
		}
	}
}

func TestTuningStatus(t *testing.T) {
	if got := tuningStatus(&api.State{}); got != "CPU — · TDP FIRMWARE · UV STOCK" {
		t.Fatalf("default tuningStatus() = %q", got)
	}
	state := &api.State{
		CPUPower:        &api.CPUPowerState{EPP: "balance_performance"},
		TDPActive:       true,
		TDP:             &api.TDPState{PL1SPL: 55},
		UndervoltActive: true,
		Undervolt:       &api.UndervoltState{CPUCO: -20},
	}
	if got := tuningStatus(state); got != "CPU RESPONSIVE · TDP 55 W · UV -20" {
		t.Fatalf("tuningStatus() = %q", got)
	}
}

func TestBatteryStrategySummary(t *testing.T) {
	tests := map[int]string{
		0:   "—",
		60:  "60%",
		75:  "75%",
		80:  "80%",
		100: "100%",
	}
	for limit, want := range tests {
		if got := batteryStrategySummary(limit); got != want {
			t.Errorf("batteryStrategySummary(%d) = %q, want %q", limit, got, want)
		}
	}
}

func TestThemeButtonLabel(t *testing.T) {
	if got := themeButtonLabel("rog-dark", "", false); got != "ROG Dark" {
		t.Fatalf("themeButtonLabel() = %q, want ROG Dark", got)
	}
	if got := themeButtonLabel("", "", true); got != "Custom Theme" {
		t.Fatalf("custom themeButtonLabel() = %q, want Custom Theme", got)
	}
}

func TestFormatBatteryDecimal(t *testing.T) {
	for input, want := range map[string]string{
		"12.3":   "12.30",
		"0":      "0.00",
		"17.567": "17.57",
		"—":      "—",
	} {
		if got := formatBatteryDecimal(input); got != want {
			t.Errorf("formatBatteryDecimal(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestBrightnessLabel(t *testing.T) {
	for level, want := range []string{"Low", "Medium", "High"} {
		level++
		if got := brightnessLabel(level); got != want {
			t.Errorf("brightnessLabel(%d) = %q, want %q", level, got, want)
		}
	}
	if got := brightnessLabel(0); got != "Low" {
		t.Errorf("brightnessLabel clamps low value: %q", got)
	}
	if got := brightnessLabel(99); got != "High" {
		t.Errorf("brightnessLabel clamps high value: %q", got)
	}
}

func TestEPPSupported(t *testing.T) {
	// amd-pstate-epp with governor=performance advertises only "performance".
	choices := []string{"performance"}
	for _, value := range []string{"balance_performance", "balance_power", "power"} {
		if eppSupported(choices, value) {
			t.Errorf("eppSupported(%v, %q) = true, want false", choices, value)
		}
	}
	if !eppSupported(choices, "performance") {
		t.Fatal("performance must stay available")
	}
	full := []string{"performance", "balance_performance", "balance_power", "power"}
	if !eppSupported(full, "power") {
		t.Fatal("all advertised choices must be available")
	}
	if !eppSupported(nil, "balance_power") {
		t.Fatal("an empty choice list must not hide anything")
	}
}

func TestKernelEPP(t *testing.T) {
	for in, want := range map[string]string{
		"dynamic(profile:performance)": "dynamic",
		"dynamic(profile:balanced)":    "dynamic",
		"performance":                  "performance",
		"balance_power":                "balance_power",
	} {
		if got := kernelEPP(in); got != want {
			t.Errorf("kernelEPP(%q) = %q, want %q", in, got, want)
		}
	}
	if got := tuningStatus(&api.State{CPUPower: &api.CPUPowerState{EPP: "dynamic(profile:performance)"}}); got != "CPU DYNAMIC · TDP FIRMWARE · UV STOCK" {
		t.Fatalf("tuningStatus(dynamic) = %q", got)
	}
}

func TestRefreshNoteText(t *testing.T) {
	cases := []struct {
		name  string
		state *api.State
		rate  int
		want  string
	}{
		{"built-in only", &api.State{}, 180, "Current rate: 180 Hz"},
		{"second screen", &api.State{ExternalDisplay: true}, 60, "Built-in panel: 60 Hz. Other displays aren't affected."},
		{"docked, panel off", &api.State{PanelOff: true, ExternalDisplay: true}, 0, "Built-in panel is off. These buttons don't change other displays."},
		{"unreadable", &api.State{}, 0, "The current rate can't be read, so neither option is marked."},
		{"unreadable with external", &api.State{ExternalDisplay: true}, 0, "The current rate can't be read, so neither option is marked."},
		{"no state yet", nil, 0, "The current rate can't be read, so neither option is marked."},
	}
	for _, tc := range cases {
		if got := refreshNoteText(tc.state, tc.rate); got != tc.want {
			t.Errorf("%s: refreshNoteText() = %q, want %q", tc.name, got, tc.want)
		}
	}
}
