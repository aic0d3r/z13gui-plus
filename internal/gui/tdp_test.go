package gui

import (
	"testing"

	"github.com/dahui/z13ctl/api"
)

func TestEnforceConstraintsEdges(t *testing.T) {
	for _, tc := range []struct {
		idx, temp int
	}{{0, 100}, {0, 200}, {7, 35}, {7, 0}, {3, 105}, {3, 35}} {
		fc := &fanCurveEditor{points: defaultFanCurve()}
		fc.points[tc.idx].Temp = tc.temp
		fc.enforceConstraints(tc.idx)
		for i := range fc.points {
			p := fc.points[i]
			if p.Temp < 35 || p.Temp > 105 {
				t.Errorf("drag %d→%d: point %d temp %d out of range", tc.idx, tc.temp, i, p.Temp)
			}
			if i > 0 && p.Temp <= fc.points[i-1].Temp {
				t.Errorf("drag %d→%d: temps not strictly increasing: %v", tc.idx, tc.temp, fc.points)
			}
		}
	}
}

func TestEnforceConstraintsKeepsUntouchedPoints(t *testing.T) {
	fc := &fanCurveEditor{points: defaultFanCurve()}
	fc.points[0].Temp = 30 // built-in presets start below the editor floor
	fc.points[5].PWM = 180
	fc.enforceConstraints(5)
	if fc.points[0].Temp != 30 {
		t.Fatalf("dragging point 5 moved point 0 to %d°C", fc.points[0].Temp)
	}
}

func TestTuningTargetResetDefaults(t *testing.T) {
	got := tuningTarget(&api.State{}) // daemon reset: TDP/FanCurve nil
	if got.tdp != [3]int{50, 50, 50} || got.fan != defaultFanCurve() || got.cpuCO != 0 {
		t.Fatalf("reset target = %+v", got)
	}
	s := &api.State{
		TDP:             &api.TDPState{PL1SPL: 60, PL2SPPT: 70, FPPT: 80},
		Undervolt:       &api.UndervoltState{CPUCO: -20},
		UndervoltActive: true,
	}
	got = tuningTarget(s)
	if got.tdp != [3]int{60, 70, 80} || got.cpuCO != -20 {
		t.Fatalf("target = %+v", got)
	}
	s.UndervoltActive = false
	if tuningTarget(s).cpuCO != 0 {
		t.Fatal("inactive undervolt should read as 0")
	}
}

func TestCurveMeetsHighTDPFloor(t *testing.T) {
	pts := defaultFanCurve()
	if curveMeetsHighTDPFloor(pts[:]) {
		t.Fatal("default curve has points below the floor")
	}
	for i := range pts {
		pts[i].PWM = highTDPMinPWM
	}
	if !curveMeetsHighTDPFloor(pts[:]) || curveMeetsHighTDPFloor(nil) {
		t.Fatal("floor check wrong at boundary or for empty curve")
	}
}
