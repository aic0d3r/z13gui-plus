// Package gamescope implements the X11 overlay display backend for gamescope
// (Steam Gaming Mode). It sets X11 atoms on the window so gamescope composites
// it as an external overlay above the running game.
package gamescope //nolint:gocritic // cgo requires separate import blocks

/*
#cgo pkg-config: gtk4 x11
#cgo CFLAGS: -Wno-deprecated-declarations
#include <gdk/x11/gdkx.h>
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <stdint.h>
#include <stdlib.h>

// surface_get_xid returns the X11 Window ID from a realized GDK surface.
// gdk_x11_surface_get_xid is deprecated since GDK 4.18 but still works;
// gotk4's code generator skips it, so we call it via CGO.
static unsigned long surface_get_xid(void *gdk_surface) {
    return gdk_x11_surface_get_xid(GDK_SURFACE(gdk_surface));
}

// display_get_xdisplay returns the Xlib Display* from a GDK display.
static void *display_get_xdisplay(void *gdk_display) {
    return gdk_x11_display_get_xdisplay(GDK_DISPLAY(gdk_display));
}

// set_cardinal sets a 32-bit CARDINAL property on an X11 window.
static void set_cardinal(void *xdisplay, unsigned long xid,
                         const char *name, uint32_t value) {
    Display *dpy = (Display *)xdisplay;
    Atom atom = XInternAtom(dpy, name, False);
    XChangeProperty(dpy, xid, atom, XA_CARDINAL, 32,
                    PropModeReplace, (unsigned char *)&value, 1);
    XFlush(dpy);
}

// grab_keyboard grabs keyboard input to the specified window.
static int grab_keyboard(void *xdisplay, unsigned long xid) {
    return XGrabKeyboard((Display *)xdisplay, xid, True,
                         GrabModeAsync, GrabModeAsync, CurrentTime);
}

// ungrab_keyboard releases the keyboard grab.
static void ungrab_keyboard(void *xdisplay) {
    XUngrabKeyboard((Display *)xdisplay, CurrentTime);
    XFlush((Display *)xdisplay);
}

*/
import "C" //nolint:gocritic // cgo requires standalone import

import (
	"log/slog"
	"os"
	"strconv"
	"unsafe" //nolint:gocritic // used with cgo, requires separate import block

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/aic0d3r/z13gui-plus/internal/cssscale"
)

const (
	referenceWidth = 1707.0     // 2560 / 1.5; matches KDE 150% at Z13 native resolution
	minScale       = 1.0        // lower bound for UI scale
	maxScale       = 3.0        // upper bound for UI scale
	marginFraction = 20         // screen height / N for 5% top/bottom margins
	fullOpacity    = 0xFFFFFFFF // _NET_WM_WINDOW_OPACITY value for fully visible
)

// Backend manages the gamescope X11 overlay window.
type Backend struct {
	appWin      *gtk.ApplicationWindow
	gtkWin      *gtk.Window
	drawerWidth int
	layoutCSS   string // structural sheet scaled by scaledCSS

	xdisplay unsafe.Pointer
	xid      C.ulong
	ready    bool // true after realize extracts XID

	outputHeight int     // from realize; used in WrapContent for margins
	scale        float64 // monitor width / referenceWidth, clamped [minScale, maxScale]
	onDismiss    func()
}

// New creates a gamescope backend. drawerWidth is the drawer panel width in
// pixels; layoutCSS is the structural sheet to scale for the output.
func New(appWin *gtk.ApplicationWindow, drawerWidth int, layoutCSS string) *Backend {
	return &Backend{
		appWin:      appWin,
		gtkWin:      &appWin.Window,
		drawerWidth: drawerWidth,
		layoutCSS:   layoutCSS,
		scale:       initialScale(), // used as-is if realize can't find a monitor
	}
}

// initialScale returns the Z13GUI_PLUS_SCALE override clamped to
// [minScale, maxScale], or minScale when unset or invalid.
func initialScale() float64 {
	if v, err := strconv.ParseFloat(os.Getenv("Z13GUI_PLUS_SCALE"), 64); err == nil && v > 0 {
		return min(max(v, minScale), maxScale)
	}
	return minScale
}

// Configure sets up the fullscreen undecorated window and registers a realize
// handler that extracts the X11 window ID and sets the overlay atom.
// The window is kept mapped at all times; visibility is controlled by toggling
// _NET_WM_WINDOW_OPACITY (0 = hidden, 0xFFFFFFFF = visible). This avoids the
// Wayland surface lifecycle churn that SetVisible toggling causes in XWayland.
func (b *Backend) Configure(_ func() bool, onDismiss func()) {
	b.onDismiss = onDismiss
	b.appWin.SetDecorated(false)

	b.appWin.Connect("realize", func() {
		display := gdk.DisplayGetDefault()
		surfacer := b.appWin.Surface()
		if display == nil || surfacer == nil {
			slog.Warn("gamescope: could not get display or surface on realize")
			return
		}
		surface, ok := surfacer.(*gdk.Surface)
		if !ok || surface == nil {
			slog.Warn("gamescope: surface is not *gdk.Surface")
			return
		}
		b.xdisplay = C.display_get_xdisplay(unsafe.Pointer(display.Native())) //nolint:govet // GObject pointer is C-heap-allocated and pinned; uintptr→unsafe.Pointer is safe
		b.xid = C.surface_get_xid(unsafe.Pointer(surface.Native()))           //nolint:govet // GObject pointer is C-heap-allocated and pinned; uintptr→unsafe.Pointer is safe
		b.ready = true

		// Store output height for WrapContent (which runs after realize).
		// Compute a UI scale so the drawer occupies the same physical
		// screen fraction as KDE at 150% (~18.75% of screen width).
		// Z13GUI_PLUS_SCALE env var overrides auto-detection.
		if monitor := display.MonitorAtSurface(surface); monitor != nil {
			geo := monitor.Geometry()
			b.outputHeight = geo.Height()
			if os.Getenv("Z13GUI_PLUS_SCALE") == "" {
				b.scale = min(max(float64(geo.Width())/referenceWidth, minScale), maxScale)
			}
			b.appWin.SetDefaultSize(geo.Width(), geo.Height())
			slog.Info("gamescope: sized to monitor", "w", geo.Width(), "h", geo.Height(), "scale", b.scale)
		}

		b.setCardinal("STEAM_OVERLAY", 1)
		b.setCardinal("_NET_WM_WINDOW_OPACITY", 0) // start hidden
		slog.Info("gamescope: overlay atom set", "xid", uint64(b.xid))
	})

	// Escape key dismiss.
	key := gtk.NewEventControllerKey()
	key.ConnectKeyPressed(func(keyval, _ uint, _ gdk.ModifierType) bool {
		if keyval == 0xff1b { // GDK_KEY_Escape
			onDismiss()
			return true
		}
		return false
	})
	b.gtkWin.AddController(key)

	b.appWin.Fullscreen()
	b.appWin.SetVisible(true) // keep mapped always; hidden via opacity=0
	slog.Info("backend", "mode", "gamescope")
}

// WrapContent wraps the drawer in a fullscreen container with a click-to-dismiss
// backdrop on the left and the drawer right-aligned.
func (b *Backend) WrapContent(drawer gtk.Widgetter) gtk.Widgetter {
	wrapper := gtk.NewBox(gtk.OrientationHorizontal, 0)
	wrapper.AddCSSClass("gs-wrapper")

	// Transparent click-to-dismiss backdrop.
	backdrop := gtk.NewBox(gtk.OrientationVertical, 0)
	backdrop.SetHExpand(true)
	backdrop.AddCSSClass("gs-backdrop")
	click := gtk.NewGestureClick()
	click.ConnectReleased(func(_ int, _, _ float64) {
		slog.Debug("gamescope: backdrop clicked")
		if b.onDismiss != nil {
			b.onDismiss()
		}
	})
	backdrop.AddController(click)

	// Constrain drawer to scaled width.
	scaledWidth := int(float64(b.drawerWidth) * b.scale)
	if scaledWidth < b.drawerWidth {
		scaledWidth = b.drawerWidth
	}
	panel := gtk.NewBox(gtk.OrientationVertical, 0)
	panel.SetSizeRequest(scaledWidth, -1)
	panel.SetHExpand(false)

	// 5% top/bottom margins (matches layer-shell backend).
	if b.outputHeight > 0 {
		margin := b.outputHeight / marginFraction
		panel.SetMarginTop(margin)
		panel.SetMarginBottom(margin)
	}

	panel.Append(drawer)
	wrapper.Append(backdrop)
	wrapper.Append(panel)

	// Inject the scaled copy of layout.css (see scaledCSS).
	css := gtk.NewCSSProvider()
	css.LoadFromString(b.scaledCSS())
	gtk.StyleContextAddProviderForDisplay(
		gdk.DisplayGetDefault(), css,
		gtk.STYLE_PROVIDER_PRIORITY_APPLICATION+1,
	)

	return wrapper
}

// Show makes the overlay visible by setting full opacity and captures input
// via STEAM_INPUT_FOCUS atom and an X11 keyboard grab. No pointer grab is
// used — XGrabPointer's core event mask interferes with XI2 touch delivery,
// breaking button/radio-button touch activation. STEAM_INPUT_FOCUS handles
// pointer/touch routing natively in gamescope.
func (b *Backend) Show() {
	slog.Debug("gamescope: Show enter", "ready", b.ready)
	if b.ready {
		b.setCardinal("_NET_WM_WINDOW_OPACITY", fullOpacity)
		b.setCardinal("STEAM_INPUT_FOCUS", 1)
		kbResult := C.grab_keyboard(b.xdisplay, b.xid)
		slog.Debug("gamescope: overlay shown", "grabKB", int(kbResult))
	} else {
		slog.Warn("gamescope: Show() called but XID not ready")
	}
	b.appWin.Present()
}

// Hide releases input grabs and hides the overlay via zero opacity.
func (b *Backend) Hide() {
	slog.Debug("gamescope: Hide enter", "ready", b.ready)
	if b.ready {
		C.ungrab_keyboard(b.xdisplay)
		b.setCardinal("STEAM_INPUT_FOCUS", 0)
		b.setCardinal("_NET_WM_WINDOW_OPACITY", 0)
		slog.Debug("gamescope: overlay hidden")
	}
}

// setCardinal sets a 32-bit CARDINAL property on the overlay window.
func (b *Backend) setCardinal(name string, value uint32) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	C.set_cardinal(b.xdisplay, b.xid, cname, C.uint32_t(value))
}

// scaledCSS returns layout.css with every pixel length multiplied by the
// gamescope resolution factor, plus the base font the desktop gets from its
// GTK theme. Loaded at PRIORITY_APPLICATION+1 so it overrides layout.css and
// any GTK theme defaults that differ between desktop and gamescope.
func (b *Backend) scaledCSS() string {
	return cssscale.Px(".drawer { font-family: 'Inter', sans-serif; font-size: 14px; }\n"+b.layoutCSS, b.scale)
}
