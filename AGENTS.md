# Z13GUI+ — Project Context for Claude

## What this project is

Z13GUI+ is an independently named fork of
[`dahui/z13gui`](https://github.com/dahui/z13gui), providing a GTK4 Wayland
layer-shell overlay drawer for controlling the 2025 ASUS ROG Flow Z13 through
the `z13ctl-plus` daemon. It preserves intentional API and UI compatibility,
not runtime-name compatibility. It slides in from the right edge of the screen
when the Armoury Crate button (KEY_PROG3) is pressed. The daemon broadcasts
`gui-toggle` events over a subscribe socket; this GUI listens for them.

It has two display backends:
- **Layer-shell** (KDE/Wayland): margin-based slide animation
- **Gamescope** (Steam Gaming Mode): X11 overlay via `STEAM_OVERLAY` atom

- Module: `github.com/aic0d3r/z13gui-plus`
- Binary: `z13gui-plus`

## v2 namespace invariants

- v2.0.0 is the independently named Plus release. The only installed command is
  `z13gui-plus`; never add a `z13gui` alias.
- The user unit is `z13gui-plus.service`.
- The GApplication ID is `io.github.aic0d3r.z13gui_plus` and the desktop file is
  `io.github.aic0d3r.z13gui_plus.desktop`.
- The gamepad udev file is `99-z13gui-plus-gamepad.rules`.
- Configuration lives at `$XDG_CONFIG_HOME/z13gui-plus`, normally
  `~/.config/z13gui-plus`.
- Owned runtime files live under `$XDG_RUNTIME_DIR/z13gui-plus`. Preserve the
  existing per-call fallback conventions: embedded fonts use
  `/tmp/z13gui-plus/fonts`, while frozen-PID recovery uses
  `/run/user/$UID/z13gui-plus/frozen-pid` when `XDG_RUNTIME_DIR` is unset.
- The only Z13GUI+-specific environment variables are `Z13GUI_PLUS_SCALE` and
  `Z13GUI_PLUS_NO_GAMEPAD`. Never restore the unprefixed v1 forms.
- The root module is `github.com/aic0d3r/z13gui-plus`.
- The controller API import intentionally remains `github.com/dahui/z13ctl/api`.
  The `go.mod` replacement selects `github.com/aic0d3r/z13ctl-plus/api` source,
  whose client targets only the Plus socket. Removing that replacement selects
  upstream source and the upstream socket.
- Plus packages can coexist with upstream and must not provide, conflict with,
  or replace it. `z13gui-plus-bin` depends on `z13ctl-plus-bin`; deb/rpm packages
  depend on `z13ctl-plus`.
- Package installation leaves `z13gui-plus.service` disabled and never manages
  controller services. Users explicitly enable only the GUI matching their
  selected daemon.
- `z13gui-plus --migrate-config` is the only supported v1 migration. It copies
  the entire legacy `$XDG_CONFIG_HOME/z13gui` directory only when the Plus
  directory is absent, leaves the source untouched, and does not change
  services or remove artifacts. Legacy paths are provenance-ambiguous; cleanup
  must be explicit and limited to files known to come from this fork.

## Companion projects

This repo is part of a three-repo tablet stack for the ASUS Z13:
- [`z13ctl-plus`](https://github.com/aic0d3r/z13ctl-plus) — the daemon this drawer
  talks to. Its API submodule retains the canonical path
  `github.com/dahui/z13ctl/api`; selected here through the fork replacement.
- [`z13-tablet-kit`](https://github.com/aic0d3r/z13-tablet-kit) — posture detection
  (dock/folio/tablet) and touch input. The tablet chip in this drawer stays hidden
  until the kit's `z13-tablet-switch` reports a heartbeat; toggles changed here are
  persisted by `z13ctl-plus` and applied live by the kit.

During local development, a `go.work` file in this repo (if present, gitignored)
provides the local override. In production `go.mod` redirects the canonical API
import to the tagged `github.com/aic0d3r/z13ctl-plus/api` module.

## Package layout

```
main.go                         GTK Application entry; ConnectActivate → gui.New(app)
                                Gamescope env detection + stale socket validation
Makefile                        build, install, lint, clean, snapshot, release
internal/gui/
  gui.go                        Window struct, backend selection, show/hide, subscribeLoop, theming
  backend.go                    Backend interface (Configure, WrapContent, Show, Hide)
  controls.go                   All GTK widget construction (drawer and views)
  tdp.go                        Custom profile view: TDP sliders, fan curve editor, undervolt, telemetry
  sync.go                       Daemon state sync and API send functions
  color.go                      colorInput struct, HSL conversion, color picker view logic
  focus.go                      2D grid gamepad focus navigation + modal slider editing
  log.go                        Split-level slog handler (app vs GTK noise filtering)
  layout.css                    Embedded structural CSS (touch targets, sizing) — PRIORITY_APPLICATION
  theme-default.css             Embedded theme template with @define-color placeholders — PRIORITY_USER
  theme-default.toml            Embedded default theme colors (rog-dark), used by --print-theme
internal/gui/fonts/
  font.go                       Embedded Inter font loading
internal/gui/layershell/
  layershell.go                 Layer-shell display backend (KDE/Wayland)
internal/gui/gamepad/
  gamepad.go                    evdev gamepad reader; device classification + EVIOCGRAB
  steam.go                      Steam PID discovery; drives the hidraw blocker
internal/gui/gamepad/hidblocker/
  hidblocker.go                 BPF LSM blocker: blocks hidraw reads for specific PIDs
  blocker.bpf.c                 BPF C program (SEC("lsm/file_permission"), returns -EAGAIN)
  gen.go                        bpf2go generate directive
  blocker_x86_bpfel.go          Generated Go bindings (committed)
  blocker_x86_bpfel.o           Generated BPF ELF object (committed)
  hidblocker_test.go            Tests (skip without root/BPF LSM)
  vmlinux.h                     Generated kernel BTF header (gitignored, machine-specific)
internal/gui/gamescope/
  gamescope.go                  Gamescope X11 overlay backend (Steam Gaming Mode)
internal/theme/
  theme.go                      Theme types, TOML parsing, CSS generation, config persistence
  builtins.go                   15 built-in themes (8 dark, 7 light) with accent variants
  theme_test.go                 Theme parsing and CSS generation tests
internal/togglegate/
  togglegate.go                 Pure debounce helper for duplicate gui-toggle bursts
  togglegate_test.go            Unit tests (pure Go, no GTK4)
contrib/
  z13gui-plus.service           systemd user service (EnvironmentFile for gamescope-session)
  io.github.aic0d3r.z13gui_plus.desktop
                                Desktop entry
  99-z13gui-plus-gamepad.rules  Gamepad access udev rules
```

## Key architectural decisions

- **Layer-shell** (KDE): `github.com/diamondburned/gotk4-layer-shell/pkg/gtk4layershell`
  (NOT `gtklayershell` which is GTK3). pkg-config name: `gtk4-layer-shell-0`.
- **Anchor**: right + top + bottom edges. Top/bottom margins set to 5% of screen height
  on realize. The surface is pinned to its monitor via `SetMonitor` (helps wlroots
  compositors clip overflow; KWin does NOT clip, see conditional fade below).
- **Keyboard mode**: `LayerShellKeyboardModeOnDemand` — gets focus when visible.
- **Animation**: layer-shell right-margin animation (`gtk4layershell.SetMargin`).
  `margin=0` → on-screen; `margin=-320` → off-screen to the right.
  Avoids GTK Revealer which causes pixman errors and smearing artifacts in Wayland.
- **Window visibility**: window is kept `SetVisible(true)` at all times after creation.
  It's "hidden" by setting margin = -(width-1) (off-screen) and opacity = 0, not by
  destroying/hiding the surface. This prevents the ghost-surface artifact that KDE Plasma
  shows when remapping a surface. The 1px margin keeps the surface in KWin's composited
  output for damage tracking; opacity 0 makes the sliver invisible to the user.
- **Width**: `SetSizeRequest(320, -1)`. Height is natural (content-driven, scrolled).
- **Show/hide animation**: smoothstep easing via `AddTickCallback` (VSync-synced),
  with a shared `animGen` generation counter so a show cancels an in-flight hide
  (and vice versa). Two paths, chosen per Show/Hide by `hasRightNeighbor()`:
  - **No monitor to the right** → slide the right margin (`slideMargin`). Show sets
    opacity=1 then slides in; Hide slides out then sets opacity=0 (hides the 1px
    sliver on the primary's own right edge).
  - **A monitor to the right** → fade in place (`fadeOpacity`) at margin 0, fully on
    the primary, then park the transparent surface off-screen. A rightward slide
    would otherwise bleed onto that monitor because KWin doesn't clip layer-surface
    overflow to the assigned output. `Backend.margin`/`Backend.opacity` track current
    state; use `setMargin`/`setOpacity` to keep them in sync.
- **Single instance / activate guard**:
  `gtk.NewApplication("io.github.aic0d3r.z13gui_plus", ...)`
  registers the app on the session bus, so launching the binary a second time does not
  start a second process — GApplication forwards `activate` to the running instance and
  the new process exits. `main.go` therefore holds the `*gui.Window` and only calls
  `gui.New` on first activation; re-activation calls `Toggle()` instead. Without that
  guard each re-activation builds an entire second drawer (its own layer surface,
  subscribe loop, gamepad reader, telemetry poller) overlapping the first, and both stay
  live and interactive. One click on
  `contrib/io.github.aic0d3r.z13gui_plus.desktop` while the user service is
  running is enough to trigger it. Diagnostic: two `drawer initialized` log
  lines under a single PID means the guard is missing or broken.
- **State source of truth**: daemon is the source of truth. On show, `api.SendGetState()`
  is called and `syncState()` updates widgets. Widget signals are suppressed during sync
  via `Window.syncing bool`.
- **Subscribe loop**: background goroutine, exponential backoff reconnect, dispatches
  `Toggle()` onto the GTK main thread via `glib.TimeoutAdd(0, ...)` followed by
  `MainContextDefault().Wakeup()` (the wakeup is required — the loop may deliver an
  event while the main context is blocked).
- **Toggle debounce**: on some firmware revisions a single Armoury Crate press reaches
  the GUI as two `gui-toggle` events in the same instant, which cancel each other out
  (an open drawer gets hide → show and appears stuck open). `subscribeLoop` gates events
  through `togglegate.Accept(last, now, daemonToggleDebounce)` (leading edge: keep the
  first event of a burst, drop the rest; the window does not extend on suppression).
  **The window is 50ms and must stay well under ~120ms.** It is a duplicate filter, not
  an animation rate limiter: measured human tapping bottoms out near 129ms between
  presses, so a larger window discards deliberate input — the original 250ms swallowed
  38% of presses in a 96-event sample from real use. If a future change wants to rate
  limit the 200ms slide animation, do it in `Toggle()`/the backend, not here.
  The debounce timestamp is a **local variable** in `subscribeLoop`, not a `Window`
  field: every other piece of `Window` state is main-thread-owned, so keeping this one
  out of the struct makes it unreachable from the main thread and removes any chance of
  an unsynchronized read. Do not promote it to a field — reading it from `show()`/
  `hide()` would be a data race, and `internal/gui` is not covered by `go test -race`.
- **CSS architecture**:
  - `layout.css` → `STYLE_PROVIDER_PRIORITY_APPLICATION` (structural, not overridable)
  - `theme-default.css` → `STYLE_PROVIDER_PRIORITY_USER` (colors, user-overridable)
  - No `hexpand: true` in CSS — use `widget.SetHExpand(true)` in Go instead.
  - No `box-shadow` on `.drawer` — it causes smearing outside the widget clip region
    during slide animations in Wayland Vulkan rendering.
  - No `AddMark()` on scales — scale marks inside an animated context cause GTK
    `GtkGizmo` allocation warnings and pixman errors.
  - CSS class hierarchy for text labels in custom view:
    - `.section-label` — section headers ("TDP", "UNDERVOLT", "FAN CURVE"): 11px, bold, letter-spaced, dim
    - `.scale-name` — slider name labels ("PL1 (SPL)", "CPU Curve Optimizer"): 10px, bold, no letter-spacing, dim
    - `.scale-value` — slider value readouts ("50 W", "CPU CO: -20"): 10px, normal weight, bright
- **Profile selector**: physical-profile buttons (`quiet`, `balanced`,
  `performance`), stored in `w.profileBtns map[string]*gtk.Button`. Fan, TDP,
  and undervolt are independent overrides; there is no selectable Custom
  profile. Not DropDown (popup broken in gamescope).
- **Focus-loss dismiss** (layer-shell): `EventControllerMotion` tracks `pointerInside`
  on the backend. On `notify::is-active` focus loss: if within 500ms of Show, ignored
  (compositor settle time for keyboard-mode transition). If pointer is inside, the drop
  is spurious (KDE Plasma briefly drops focus during keyboard-mode transitions) → ignored.
  If pointer is outside, user clicked elsewhere → dismiss after 200ms confirmation delay.
  Do NOT add a `focusedSinceShow` guard — it causes first-show dismiss regression on KDE
  where the compositor drops focus during keyboard-mode transition and never re-grants it.
  Escape key also dismisses in both backends.
- **GTK_A11Y=none**: set internally in `main.go` and
  `contrib/z13gui-plus.service`. Disables GTK4
  AT-SPI accessibility bridge, which sends D-Bus events on every widget state change.
  Under systemd (especially gamescope sessions), the AT-SPI bus may be unavailable,
  causing D-Bus timeouts that block GTK initialization.

## Gamescope backend (`internal/gui/gamescope/gamescope.go`)

The gamescope backend renders Z13GUI+ as an X11 overlay in Steam Gaming Mode.

- **Overlay type**: `STEAM_OVERLAY` atom (z-pos 3, interactive with input routing).
  NOT `GAMESCOPE_EXTERNAL_OVERLAY` (z-pos 2, display-only, no input).
- **Visibility**: opacity-based (`_NET_WM_WINDOW_OPACITY`). Window stays mapped always.
- **Input**: keyboard-only X11 grab (`XGrabKeyboard`) + `STEAM_INPUT_FOCUS` atom.
  `XGrabPointer` was removed because its core X11 event mask interferes with XI2
  touch delivery. STEAM_INPUT_FOCUS handles pointer/touch routing natively.
- **Scaling**: resolution-based CSS scaling (`outputWidth / 1707`). Reference
  1707 = 2560/1.5 (matches KDE 150% at Z13 native resolution).
  `Z13GUI_PLUS_SCALE` overrides.
  GDK_SCALE CANNOT be used — causes double scaling (GTK + gamescope scaler).
- **Layout**: fullscreen window → horizontal box (backdrop + right-aligned panel).
  Panel has 5% top/bottom margins, scaled drawer width.
- **Popups don't work**: GTK4 popovers/dropdowns create separate X11 windows that
  gamescope doesn't composite. Solved via view switching (see below).

### View switching

In both KDE and gamescope modes, `buildContent()` wraps content in a `gtk.Stack` with 4 pages:
- `"main"` — normal drawer (profiles, RGB, battery, etc.)
- `"custom"` — internal stack key for the Advanced Tuning view (TDP, fan curve,
  undervolt, telemetry); the user-facing profile is still physical
- `"theme"` — theme picker (radio buttons + accent dots)
- `"color"` — HSL color picker (H/S/L sliders + presets + preview)

Both backends use these stack pages; no popovers are constructed. `hide()` resets to "main".

### Service environment

`contrib/z13gui-plus.service` uses
`EnvironmentFile=-%t/gamescope-environment` (optional).
`main.go` validates the gamescope Wayland socket exists before selecting the backend
to handle stale environment files after session switching.

## API usage (`github.com/dahui/z13ctl/api`)

Functions used:
- `api.SendGetState() (bool, *api.State, error)` — fetch full daemon state on show
- `api.Subscribe([]string{"gui-toggle"}) (<-chan string, func(), error)` — event stream
- `api.SendApply(device, color1, color2, mode, speed string, brightness int) (bool, error)`
- `api.SendOff(device string) (bool, error)` — turn off lighting for a device
- `api.SendProfileSet(profile string) (bool, error)`
- `api.SendBatteryLimitSet(limit int) (bool, error)`
- `api.SendPanelOverdriveSet(value int) (bool, error)` — 0 or 1
- `api.SendBootSoundSet(value int) (bool, error)` — 0 or 1
- `api.SendTdpSet(watts, pl1, pl2, pl3 string, force bool) (bool, error)` — set TDP.
  **`watts` (the `set` field) is mandatory even in advanced mode.** The daemon's
  `handleTDP` does `strconv.Atoi(req.Set)` before it looks at `pl1`/`pl2`/`pl3` and
  rejects the request with `TDP value must be an integer` if it is empty. `watts` is
  then used only as the default for any PL field left blank, so advanced mode passes
  PL1 as the base value (`SendTdpSet(pl1, pl1, pl2, pl3, force)` in `sendTdp()`).
- `api.SendTdpReset() (bool, error)` — reset TDP to firmware defaults
- `api.SendFanCurveSet(curve string) (bool, error)` — set custom fan curve ("temp:pwm,..." format)
- `api.SendFanCurveReset() (bool, error)` — reset fan curves to auto
- `api.SendUndervoltSet(cpu string) (bool, error)` — set CPU Curve Optimizer offset
- `api.SendUndervoltReset() (bool, error)` — reset undervolt to stock (0)

Key types from `api`:
```go
type State struct {
    Lighting           LightingState
    Devices            map[string]LightingState  // keyed by "keyboard", "lightbar"
    Profile            string
    Battery            int
    BootSound          int  // 0 or 1
    PanelOverdrive     int  // 0 or 1
    TDP                *TDPState
    FanCurve           *FanCurveState
    Undervolt          *UndervoltState
    UndervoltAvailable bool  // true if ryzen_smu is loaded
    Temperature        int   // APU temp, degrees Celsius
    FanRPM             int   // fan1 speed in RPM
}
type LightingState struct {
    Enabled bool; Mode string; Color string; Color2 string
    Speed string; Brightness int
}
type TDPState struct {
    PL1SPL int; PL2SPPT int; FPPT int
}
type FanCurveState struct {
    Mode   int              // 0=auto, 1=custom
    Points []FanCurvePoint  // 8 points
}
type FanCurvePoint struct {
    Temp int; PWM int
}
type UndervoltState struct {
    CPUCO  int   // all-core CPU Curve Optimizer offset (0 to -40)
    Active bool  // true when CO is applied to hardware
}
```

## Daemon socket

Path: `$XDG_RUNTIME_DIR/z13ctl-plus/z13ctl-plus.sock`, falling back to
`/tmp/z13ctl-plus/z13ctl-plus.sock` when `XDG_RUNTIME_DIR` is unset.

Daemon must be running for any `api.*` calls to succeed. If the daemon is not running,
`api.Subscribe` returns `nil, nil, nil` and `SendGetState` returns `false, nil, nil`.
The subscribe loop handles this with backoff retry.

## Build

```sh
make build      # CGO_ENABLED=1 go build -o z13gui-plus .
sudo make install  # installs the binary and application launcher under /usr/local
make test       # unit tests for the pure-Go packages (no GTK4 headers needed)
make lint       # golangci-lint run ./...
make clean      # rm z13gui-plus
make snapshot   # goreleaser local build (no publish)
make release    # goreleaser build + publish
```

Requires at build time: `gtk4-layer-shell` C library (`pkg-config gtk4-layer-shell-0`).

`make test` enumerates the pure-Go packages explicitly (`internal/theme`,
`internal/togglegate`) rather than using `./...`, because `internal/gui` needs CGO and
GTK4 headers. **Add new pure-Go packages to the `test` and `cover` targets** — otherwise
their tests never run; there is no CI test job (the only workflow is release).

## Known GTK issues (do not re-introduce)

- **`hexpand: true` in CSS** — not a valid CSS property. Use `widget.SetHExpand(true)` in Go.
- **`scale.AddMark()`** — causes `GtkGizmo (slider) reported min width -2` warnings and
  pixman `Invalid rectangle` errors when the scale widget is in an animated context.
  Display-only values work fine with `SetDrawValue(true)`.
- **`gtk.Revealer` with `SlideLeft`** — causes smearing artifacts in Wayland Vulkan
  rendering because GTK's damage region doesn't properly clear the transparent areas left
  behind as the revealer collapses. Use layer-shell margin animation instead.
- **`SetSizeRequest` + Revealer** — keeping the window at fixed width while the Revealer
  collapses internally still leaves stale pixels; the compositor doesn't know the content
  region shrank.
- **`box-shadow` on animated containers** — shadow pixels extend outside the widget clip
  region and are not cleared each frame in Wayland Vulkan rendering, causing smearing.
- **GTK4 popovers in gamescope** — create separate override-redirect X11 windows that
  gamescope doesn't composite. Use `gtk.Stack` view switching instead (gamescope only).
- **GDK_SCALE in gamescope** — causes double scaling (GTK scales buffer, then gamescope
  scaler scales again). Use manual CSS scaling via `scaledCSS()` instead.
- **GtkDropDown in gamescope** — popup list is a separate X11 window. Use buttons or
  radio buttons instead. Profile selector uses `gtk.Button` with CSS `.active` class.
- **CheckButton/Switch touch in gamescope** — GTK4's CheckButton and Switch use an
  internal BUBBLE-phase GestureClick, which fails for touch input in gamescope/XWayland.
  Button widgets use CAPTURE phase and work fine. Workaround: `addTouchActivate()` in
  controls.go adds a touch-only (`SetTouchOnly(true)`) CAPTURE-phase GestureClick to each
  affected widget. Do not remove — without it, all CheckButtons and Switches are
  untappable via touchscreen in gamescope mode.

## Current status

Feature-complete for both KDE and gamescope modes:
- Margin-based slide animation (smoothstep, 200ms) — KDE
- Gamescope X11 overlay with opacity-based visibility + keyboard grab + STEAM_INPUT_FOCUS
- Touch activation workaround for gamescope (CAPTURE-phase GestureClick on CheckButton/Switch)
- Pointer-inside guard for KDE focus-loss handling (spurious drop vs genuine click-outside)
- 50ms duplicate filter on daemon `gui-toggle` events (`internal/togglegate`)
- Single-instance activate guard (re-activation toggles instead of building a second drawer)
- GTK_A11Y=none for systemd AT-SPI timeout prevention
- RGB lighting controls (mode, color presets + custom chooser/HSL picker, speed, brightness)
- Profile switching via buttons (quiet/balanced/performance/custom)
- Custom profile view with:
  - TDP control: basic (single watt slider) and advanced (PL1/PL2/PL3) modes
  - Fan curve editor: 8-point Cairo graph with drag interaction, 35–105°C range
  - Undervolt: independent CPU Curve Optimizer slider (inside advanced TDP box,
    hidden when `ryzen_smu` unavailable).
    iGPU CO is not supported on Strix Halo.
  - Telemetry: APU temp + fan RPM in header and custom view, polled every 1s
  - Separate save/reset buttons for TDP, fans, and undervolt
- Battery charge limit slider
- Panel overdrive and boot sound toggles (footer switches)
- 15 built-in themes with accent variants + custom theme.toml support
- Gamescope view switching: theme picker view + HSL color picker view
- Resolution-based CSS scaling for gamescope (`Z13GUI_PLUS_SCALE` override)
- Split-level logging (app=Info, GTK=Error; `-d` enables all Debug)
- Tablet integration: optional live posture/health chip (hidden until the kit's
  heartbeat; stale after 75 s) with toggles for desktop touch policy,
  two-finger-hold context menu, folio touchpad opt-in, and scroll sensitivity/speed
  1–5 — persisted via `z13ctl-plus`, applied live by the kit.
- goreleaser + GitHub Actions release pipeline (current release **v2.1.0**)
- systemd user service with optional gamescope-environment loading
