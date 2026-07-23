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

func TestTuningStatus(t *testing.T) {
	if got := tuningStatus(&api.State{}); got != "TDP FIRMWARE · UV STOCK" {
		t.Fatalf("default tuningStatus() = %q", got)
	}
	state := &api.State{
		TDPActive: true, TDP: &api.TDPState{PL1SPL: 55},
		UndervoltActive: true, Undervolt: &api.UndervoltState{CPUCO: -20},
	}
	if got := tuningStatus(state); got != "TDP 55 W · UV -20" {
		t.Fatalf("tuningStatus() = %q", got)
	}
}

func TestBatteryStrategySummary(t *testing.T) {
	tests := map[int]string{
		0:   "—",
		60:  "MAX LIFE · 60%",
		75:  "CUSTOM · 75%",
		80:  "BALANCED · 80%",
		100: "STANDARD · 100%",
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
