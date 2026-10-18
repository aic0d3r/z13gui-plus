package gui

import (
	"github.com/diamondburned/gotk4-layer-shell/pkg/gtk4layershell"
	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// Backend abstracts display-mode-specific window management.
// Implementations: layershell.Backend (compositors with zwlr_layer_shell_v1),
// overlay.Backend (fullscreen transparent window elsewhere, GNOME included)
// and gamescope.Backend (X11 overlay for Steam Gaming Mode).
type Backend interface {
	// Configure sets up the window for this display mode. Must be called
	// before the window is realized.
	// isVisible reports whether the drawer is currently on-screen.
	// onDismiss is called when the backend wants to hide the drawer
	// (focus loss in layer-shell, backdrop click in gamescope).
	Configure(isVisible func() bool, onDismiss func())

	// WrapContent optionally wraps the drawer widget for this display mode.
	// Layer-shell returns it as-is; gamescope wraps it in a fullscreen
	// container with a click-to-dismiss backdrop.
	WrapContent(drawer gtk.Widgetter) gtk.Widgetter

	// Show makes the drawer visible (animation, atom toggle, etc).
	Show()

	// Hide hides the drawer (animation, atom toggle, etc).
	Hide()
}

// layerShellUsable reports whether the compositor implements zwlr_layer_shell_v1.
func layerShellUsable() bool {
	return gtk4layershell.IsSupported()
}

// waylandDisplay reports whether GDK is running on Wayland. The overlay backend
// needs Wayland input/opaque regions; on X11 its monitor-sized window would
// swallow every click. Asks GDK rather than XDG_SESSION_TYPE, which a systemd
// user service may not inherit.
func waylandDisplay() bool {
	d := gdk.DisplayGetDefault()
	return d != nil && coreglib.BaseObject(d).TypeFromInstance().Name() == "GdkWaylandDisplay"
}
