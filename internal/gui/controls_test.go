package gui

import (
	"testing"

	"github.com/dahui/z13ctl/api"
)

func TestMatchingFanPreset(t *testing.T) {
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

func TestThemeButtonLabel(t *testing.T) {
	if got := themeButtonLabel("rog-dark", "", false); got != "ROG Dark" {
		t.Fatalf("themeButtonLabel() = %q, want ROG Dark", got)
	}
	if got := themeButtonLabel("", "", true); got != "Custom Theme" {
		t.Fatalf("custom themeButtonLabel() = %q, want Custom Theme", got)
	}
}
