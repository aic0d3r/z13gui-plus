package gamepad

import (
	"testing"

	evdev "github.com/holoplot/go-evdev"
)

func TestStickZone(t *testing.T) {
	xbox := evdev.AbsInfo{Minimum: -32768, Maximum: 32767}
	unsigned := evdev.AbsInfo{Minimum: 0, Maximum: 255}
	tests := []struct {
		name string
		v    int32
		info evdev.AbsInfo
		prev int
		want int
	}{
		{"centre", 0, xbox, 0, 0},
		{"small drift", 8000, xbox, 0, 0},
		{"full left", -32768, xbox, 0, -1},
		{"past half right", 20000, xbox, 0, 1},
		{"held right below enter threshold", 12000, xbox, 1, 1},
		{"released right", 5000, xbox, 1, 0},
		{"flicked from right to left", -30000, xbox, 1, -1},
		{"unsigned centre", 128, unsigned, 0, 0},
		{"unsigned full up", 0, unsigned, 0, -1},
		{"unsigned full down", 255, unsigned, 0, 1},
		{"no range", 5, evdev.AbsInfo{}, 0, 0},
	}
	for _, tt := range tests {
		if got := stickZone(tt.v, tt.info, tt.prev); got != tt.want {
			t.Errorf("%s: stickZone(%d) = %d, want %d", tt.name, tt.v, got, tt.want)
		}
	}
}

func TestClassifyCapabilities(t *testing.T) {
	tests := []struct {
		name       string
		deviceName string
		properties []evdev.EvProp
		keys       []evdev.EvCode
		abs        []evdev.EvCode
		id         evdev.InputID
		want       deviceClass
	}{
		{
			name: "physical gamepad",
			keys: []evdev.EvCode{evdev.BTN_SOUTH, evdev.BTN_START},
			want: deviceGamepad,
		},
		{
			name: "Steam virtual gamepad",
			keys: []evdev.EvCode{evdev.BTN_SOUTH},
			id:   evdev.InputID{Vendor: 0x28DE, Product: 0x11FF},
			want: deviceGrabOnly,
		},
		{
			name:       "folio touchpad by properties",
			deviceName: "ASUE120D:00 04F3:31BC Mouse",
			properties: []evdev.EvProp{evdev.INPUT_PROP_POINTER, evdev.INPUT_PROP_BUTTONPAD},
			abs:        []evdev.EvCode{evdev.ABS_MT_POSITION_X},
			want:       deviceIgnore,
		},
		{
			name:       "trackpad by name",
			deviceName: "Example Trackpad",
			keys:       []evdev.EvCode{evdev.BTN_SOUTH},
			want:       deviceIgnore,
		},
		{
			name:       "controller touchpad",
			deviceName: "Wireless Controller Touchpad",
			properties: []evdev.EvProp{evdev.INPUT_PROP_POINTER},
			abs:        []evdev.EvCode{evdev.ABS_MT_POSITION_X},
			id:         evdev.InputID{Vendor: 0x054C, Product: 0x0CE6},
			want:       deviceGrabOnly,
		},
		{
			name:       "controller combined node",
			deviceName: "Wireless Controller Touchpad",
			properties: []evdev.EvProp{evdev.INPUT_PROP_POINTER},
			keys:       []evdev.EvCode{evdev.BTN_SOUTH},
			abs:        []evdev.EvCode{evdev.ABS_MT_POSITION_X},
			id:         evdev.InputID{Vendor: 0x054C, Product: 0x0CE6},
			want:       deviceGamepad,
		},
		{
			name:       "touchscreen",
			properties: []evdev.EvProp{evdev.INPUT_PROP_DIRECT},
			abs:        []evdev.EvCode{evdev.ABS_MT_POSITION_X},
			want:       deviceIgnore,
		},
		{
			name:       "accelerometer",
			properties: []evdev.EvProp{evdev.INPUT_PROP_ACCELEROMETER},
			keys:       []evdev.EvCode{evdev.BTN_SOUTH},
			want:       deviceIgnore,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyCapabilities(tt.deviceName, tt.properties, tt.keys, tt.abs, tt.id)
			if got != tt.want {
				t.Fatalf("classifyCapabilities() = %v, want %v", got, tt.want)
			}
		})
	}
}
