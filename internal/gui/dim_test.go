package gui

import (
	"bytes"
	"encoding/binary"
	"testing"
)

const kscreenSample = "\x1b[01;32mOutput: \x1b[0;0m1 HDMI-A-1 abc\n\tICC profile: /x/ext.icc\n\tColor profile source: ICC\n" +
	"\x1b[01;32mOutput: \x1b[0;0m2 eDP-1 1405-uuid\n\tenabled\n\x1b[01;33m\tICC profile: \x1b[0;0mnone\n\x1b[01;33m\tColor profile source: \x1b[0;0msRGB\n"

func TestInternalOutput(t *testing.T) {
	name, prof, ok := internalOutput(kscreenSample)
	if !ok || name != "eDP-1" || prof != (colorProfile{source: "sRGB"}) {
		t.Errorf("internalOutput = %q %+v %v", name, prof, ok)
	}
	if _, _, ok := internalOutput("Output: 1 DP-2 x\n\tICC profile: none\n\tColor profile source: sRGB\n"); ok {
		t.Error("no eDP output should not be supported")
	}
	if _, _, ok := internalOutput("Output: 1 eDP-1 x\n\tenabled\n"); ok {
		t.Error("eDP without ICC support should not be supported")
	}
}

func TestIsDimProfile(t *testing.T) {
	for path, want := range map[string]bool{
		"/run/user/1000/z13gui-plus/dim-40.icc": true,
		"/home/u/.local/share/icc/dim-40.icc":   false,
		"/run/user/1000/z13gui-plus/frozen-pid": false,
		"":                                      false,
	} {
		if got := isDimProfile(path); got != want {
			t.Errorf("isDimProfile(%q) = %v", path, got)
		}
	}
}

func TestDimICC(t *testing.T) {
	icc := dimICC(0.4)
	if got := binary.BigEndian.Uint32(icc); int(got) != len(icc) {
		t.Fatalf("header size %d, file %d", got, len(icc))
	}
	if string(icc[36:40]) != "acsp" {
		t.Fatalf("missing acsp signature")
	}
	i := bytes.Index(icc, []byte("vcgt\x00\x00\x00\x00"))
	if i < 0 {
		t.Fatal("no vcgt tag data")
	}
	// Last entry of the first channel is the curve's peak.
	peak := binary.BigEndian.Uint16(icc[i+18+255*2:])
	// 0.4 of full scale (65535), rounded.
	if peak != 26214 {
		t.Errorf("vcgt peak = %d, want 26214", peak)
	}
}

func TestClampDimLevel(t *testing.T) {
	for in, want := range map[int]int{0: dimDefaultLevel, 5: dimFloor, 35: 35, 34: 35, 100: dimCeil} {
		if got := clampDimLevel(in); got != want {
			t.Errorf("clampDimLevel(%d) = %d, want %d", in, got, want)
		}
	}
}
