package gamepad

import (
	"testing"

	evdev "github.com/holoplot/go-evdev"
)

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
