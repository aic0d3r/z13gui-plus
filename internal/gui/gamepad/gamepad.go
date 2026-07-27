// Package gamepad reads Linux evdev gamepad events and dispatches normalized
// actions to the GUI. It scans /dev/input/event* for gamepad devices, reads
// events in background goroutines, and translates them into Action values.
// Only dispatches when the overlay is visible so games keep their input.
//
// Device classification:
//   - gamepad: full controllers (Xbox, PS, Switch, virtual Steam devices) →
//     read events + EVIOCGRAB to suppress background game input
//   - grab-only: Steam virtual gamepads → EVIOCGRAB only, events discarded
//   - ignored: accelerometers/gyro (INPUT_PROP_ACCELEROMETER), keyboards,
//     mice, and other non-gamepad devices
//
// Permissions: on modern systemd (Arch, Fedora, Ubuntu 22.04+), the uaccess
// udev tag in 70-uaccess.rules grants the active session user ACL access to
// joystick devices automatically — no input group membership needed. If device
// access fails, the device is silently skipped (graceful degradation).
package gamepad

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	evdev "github.com/holoplot/go-evdev"
)

// Action represents a normalized gamepad input dispatched to the GUI.
type Action int

// Gamepad actions dispatched to the GUI handler.
const (
	ActionUp     Action = iota // D-pad up
	ActionDown                 // D-pad down
	ActionLeft                 // D-pad left
	ActionRight                // D-pad right
	ActionAccept               // A / BTN_SOUTH — activate focused widget
	ActionBack                 // B / BTN_EAST — dismiss / go back
	ActionBumpL                // Left shoulder (LB / BTN_TL)
	ActionBumpR                // Right shoulder (RB / BTN_TR)
)

// Handler is called on the GTK main thread for each gamepad action.
type Handler func(Action)

// deviceClass categorizes an evdev device for input handling.
type deviceClass int

const (
	deviceIgnore   deviceClass = iota // not gamepad-related; skip
	deviceGamepad                     // full gamepad: read events + EVIOCGRAB
	deviceGrabOnly                    // Steam virtual gamepad: EVIOCGRAB only
)

// gamepadButtons are evdev button codes that identify a device as a gamepad.
// Covers Xbox, PlayStation, Nintendo Switch, and Steam virtual controllers.
var gamepadButtons = []evdev.EvCode{
	evdev.BTN_SOUTH,  // A / Cross
	evdev.BTN_EAST,   // B / Circle
	evdev.BTN_NORTH,  // Y / Triangle
	evdev.BTN_WEST,   // X / Square
	evdev.BTN_TL,     // Left bumper
	evdev.BTN_TR,     // Right bumper
	evdev.BTN_TL2,    // Left trigger (digital)
	evdev.BTN_TR2,    // Right trigger (digital)
	evdev.BTN_SELECT, // Select / Share
	evdev.BTN_START,  // Start / Options
	evdev.BTN_MODE,   // PS / Xbox / Home button
	evdev.BTN_THUMBL, // L3 (left stick click)
	evdev.BTN_THUMBR, // R3 (right stick click)
}

// Reader manages gamepad device discovery and event reading.
type Reader struct {
	handler   Handler
	isVisible func() bool
	dispatch  func(func()) // wraps glib.IdleAdd; injected to avoid glib import

	mu      sync.Mutex
	devices map[string]*evdev.InputDevice
	grabbed bool // true while overlay is visible (exclusive grab)
}

// New creates a Reader. handler is called (via dispatch) for each action.
// isVisible gates dispatch so events are ignored when the overlay is hidden.
// dispatch must schedule f on the GTK main thread (typically glib.IdleAdd wrapper).
func New(handler Handler, isVisible func() bool, dispatch func(func())) *Reader {
	return &Reader{
		handler:   handler,
		isVisible: isVisible,
		dispatch:  dispatch,
		devices:   make(map[string]*evdev.InputDevice),
	}
}

// Run scans for gamepad devices and reads events for the process lifetime.
func (r *Reader) Run() {
	r.scan()
	for range time.Tick(5 * time.Second) {
		r.scan()
	}
}

// GrabAll acquires exclusive access (EVIOCGRAB) on all tracked devices
// so events are not delivered to other readers (e.g. the background game).
// New devices discovered while grabbed are auto-grabbed in tryOpen.
func (r *Reader) GrabAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.grabbed = true
	for path, dev := range r.devices {
		if err := dev.Grab(); err != nil {
			slog.Warn("gamepad: grab failed", "path", path, "err", err)
		} else {
			slog.Info("gamepad: grabbed", "path", path)
		}
	}
}

// UngrabAll releases exclusive access on all tracked devices,
// allowing the background game to receive events again.
func (r *Reader) UngrabAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.grabbed = false
	for path, dev := range r.devices {
		if err := dev.Ungrab(); err != nil {
			slog.Warn("gamepad: ungrab failed", "path", path, "err", err)
		} else {
			slog.Info("gamepad: ungrabbed", "path", path)
		}
	}
}

// scan enumerates /dev/input/event* and starts readers for new devices.
func (r *Reader) scan() {
	paths, err := evdev.ListDevicePaths()
	if err != nil {
		slog.Debug("gamepad: scan failed", "err", err)
		return
	}
	for _, p := range paths {
		r.mu.Lock()
		_, registered := r.devices[p.Path]
		r.mu.Unlock()
		if registered {
			continue
		}
		r.tryOpen(p.Path)
	}
}

// tryOpen opens a device, classifies it, and starts the appropriate handler.
func (r *Reader) tryOpen(path string) {
	dev, err := evdev.OpenWithFlags(path, os.O_RDONLY)
	if err != nil {
		return
	}

	class := classifyDevice(dev)
	if class == deviceIgnore {
		_ = dev.Close()
		return
	}

	name, _ := dev.Name()
	id, _ := dev.InputID()
	attrs := []any{
		"path", path,
		"name", name,
		"id", fmt.Sprintf("%04x:%04x", id.Vendor, id.Product),
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.devices[path] = dev
	if r.grabbed {
		if err := dev.Grab(); err != nil {
			slog.Warn("gamepad: grab failed", append(attrs, "err", err)...)
		}
	}

	switch class {
	case deviceGamepad:
		go r.readLoop(path, dev)
		slog.Info("gamepad: found", append(attrs, "class", "gamepad")...)
	case deviceGrabOnly:
		go r.holdLoop(path, dev)
		slog.Info("gamepad: found", append(attrs, "class", "grab-only")...)
	}
}

// classifyDevice determines how to handle an evdev device.
func classifyDevice(dev *evdev.InputDevice) deviceClass {
	name, _ := dev.Name()
	id, _ := dev.InputID()
	return classifyCapabilities(
		name,
		dev.Properties(),
		dev.CapableEvents(evdev.EV_KEY),
		dev.CapableEvents(evdev.EV_ABS),
		id,
	)
}

func classifyCapabilities(name string, properties []evdev.EvProp, keys, abs []evdev.EvCode, id evdev.InputID) deviceClass {
	// Skip accelerometers/gyro (PS motion sensors). High-frequency events,
	// not routable to game input — grabbing is wasteful.
	if slices.Contains(properties, evdev.INPUT_PROP_ACCELEROMETER) {
		return deviceIgnore
	}

	// Identify touchpad-like nodes before deciding whether a controller's main
	// event node or its separate touchpad node is being inspected.
	lowerName := strings.ToLower(name)
	isTouchpad := strings.Contains(lowerName, "touchpad") || strings.Contains(lowerName, "trackpad")
	isTouchpad = isTouchpad || slices.ContainsFunc(properties, func(p evdev.EvProp) bool {
		return p == evdev.INPUT_PROP_BUTTONPAD || p == evdev.INPUT_PROP_SEMI_MT
	})
	if slices.Contains(properties, evdev.INPUT_PROP_POINTER) && slices.Contains(abs, evdev.ABS_MT_POSITION_X) {
		isTouchpad = true
	}

	// Check for gamepad button capabilities (Xbox, PS, Switch, virtual).
	hasGamepadBtn := slices.ContainsFunc(keys, func(k evdev.EvCode) bool {
		return slices.Contains(gamepadButtons, k)
	})
	if hasGamepadBtn {
		if isTouchpad && id.Vendor != 0x054C && id.Vendor != 0x28DE {
			return deviceIgnore
		}
		// Steam virtual gamepad (VID 28de, PID 11ff) — grab to block game's
		// evdev reader, but don't read events (we read the physical device).
		if id.Vendor == 0x28DE && id.Product == 0x11FF {
			return deviceGrabOnly
		}
		return deviceGamepad
	}
	if isTouchpad {
		// Keep controller touchpads suppressed while the overlay is open, but
		// never open/grab desktop or folio touchpads.
		if id.Vendor == 0x054C || id.Vendor == 0x28DE {
			return deviceGrabOnly
		}
		return deviceIgnore
	}

	return deviceIgnore
}

// repeat timing constants.
const (
	repeatInitial  = 400 * time.Millisecond
	repeatInterval = 120 * time.Millisecond
)

// readLoop reads events from a gamepad device. Runs until the device
// disconnects or the reader is stopped.
func (r *Reader) readLoop(path string, dev *evdev.InputDevice) {
	defer func() {
		r.mu.Lock()
		if r.grabbed {
			_ = dev.Ungrab()
		}
		delete(r.devices, path)
		r.mu.Unlock()
		_ = dev.Close()
		slog.Info("gamepad: disconnected", "path", path)
	}()

	var repeatMu sync.Mutex
	var repeatTimer *time.Timer

	stopRepeat := func() {
		repeatMu.Lock()
		if repeatTimer != nil {
			repeatTimer.Stop()
			repeatTimer = nil
		}
		repeatMu.Unlock()
	}
	defer stopRepeat()

	startRepeat := func(a Action) {
		repeatMu.Lock()
		if repeatTimer != nil {
			repeatTimer.Stop()
		}
		var tick func()
		tick = func() {
			r.emit(a)
			repeatMu.Lock()
			if repeatTimer != nil {
				repeatTimer = time.AfterFunc(repeatInterval, tick)
			}
			repeatMu.Unlock()
		}
		repeatTimer = time.AfterFunc(repeatInitial, tick)
		repeatMu.Unlock()
	}

	for {
		ev, err := dev.ReadOne()
		if err != nil {
			return // device disconnected
		}

		switch ev.Type {
		case evdev.EV_KEY:
			switch ev.Value {
			case 1: // key down
				if a, ok := buttonToAction(ev.Code); ok {
					r.emit(a)
					if isDirectional(a) {
						startRepeat(a)
					}
				}
			case 0: // key up
				if a, ok := buttonToAction(ev.Code); ok {
					if isDirectional(a) {
						stopRepeat()
					}
				}
			}

		case evdev.EV_ABS:
			switch ev.Code {
			case evdev.ABS_HAT0Y:
				switch {
				case ev.Value < 0:
					r.emit(ActionUp)
					startRepeat(ActionUp)
				case ev.Value > 0:
					r.emit(ActionDown)
					startRepeat(ActionDown)
				default:
					stopRepeat()
				}
			case evdev.ABS_HAT0X:
				switch {
				case ev.Value < 0:
					r.emit(ActionLeft)
					startRepeat(ActionLeft)
				case ev.Value > 0:
					r.emit(ActionRight)
					startRepeat(ActionRight)
				default:
					stopRepeat()
				}
			}
		}
	}
}

// holdLoop holds a grab-only device open (e.g. PS touchpad). Reads and
// discards events to detect disconnect for cleanup.
func (r *Reader) holdLoop(path string, dev *evdev.InputDevice) {
	defer func() {
		r.mu.Lock()
		if r.grabbed {
			_ = dev.Ungrab()
		}
		delete(r.devices, path)
		r.mu.Unlock()
		_ = dev.Close()
		slog.Info("gamepad: grab-only disconnected", "path", path)
	}()

	for {
		_, err := dev.ReadOne()
		if err != nil {
			return // device disconnected
		}
	}
}

// emit dispatches an action to the GUI thread if the overlay is visible.
func (r *Reader) emit(a Action) {
	if !r.isVisible() {
		return
	}
	r.dispatch(func() { r.handler(a) })
}

// buttonToAction maps evdev button codes to actions.
func buttonToAction(code evdev.EvCode) (Action, bool) {
	switch code {
	case evdev.BTN_SOUTH:
		return ActionAccept, true
	case evdev.BTN_EAST:
		return ActionBack, true
	case evdev.BTN_TL:
		return ActionBumpL, true
	case evdev.BTN_TR:
		return ActionBumpR, true
	case evdev.BTN_DPAD_UP:
		return ActionUp, true
	case evdev.BTN_DPAD_DOWN:
		return ActionDown, true
	case evdev.BTN_DPAD_LEFT:
		return ActionLeft, true
	case evdev.BTN_DPAD_RIGHT:
		return ActionRight, true
	default:
		return 0, false
	}
}

// isDirectional returns true for actions that should auto-repeat when held.
func isDirectional(a Action) bool {
	return a == ActionUp || a == ActionDown || a == ActionLeft || a == ActionRight
}
