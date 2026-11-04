package gui

import (
	"testing"
	"time"
)

// A telemetry sync resetting the slider between the user's change and the
// timer firing must not change what is sent; only the last change is sent.
func TestDebounceValueSendsCapturedLastValue(t *testing.T) {
	var timer *time.Timer
	sent := make(chan int, 4)
	slider := 1000
	debounceValue(&timer, 20*time.Millisecond, slider, func(v int) { sent <- v })
	slider = 3000
	debounceValue(&timer, 20*time.Millisecond, slider, func(v int) { sent <- v })
	slider = 400 // sync writes the daemon's old value back into the widget

	select {
	case v := <-sent:
		if v != 3000 {
			t.Fatalf("sent %d, want 3000", v)
		}
	case <-time.After(time.Second):
		t.Fatal("nothing sent")
	}
	select {
	case v := <-sent:
		t.Fatalf("extra send %d: earlier timer not stopped", v)
	case <-time.After(50 * time.Millisecond):
	}
}
