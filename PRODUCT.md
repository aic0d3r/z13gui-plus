# Product

<!-- impeccable:product-schema 1 -->

## Platform

linux-gtk

<!-- Native Linux desktop app (GTK4 + layer-shell, plus a gamescope X11
overlay). Not one of web/ios/android/adaptive: web-only tooling (live mode,
generate, the HTML detector) does not apply. -->

## Users

The owner of a 2025 ASUS ROG Flow Z13 (GZ302, Strix Halo) running Linux: an
enthusiast who moves between three postures on the same device during a day.

- **Desk**: KDE Plasma (or another layer-shell Wayland desktop), mouse and
  keyboard, often with an external monitor to the right.
- **Tablet**: detached keyboard, touch only.
- **Steam Gaming Mode** (gamescope): gamepad-driven, full-screen game behind
  the overlay.

The job is a quick hardware change or glance — switch profile or preset, fan
mode, battery cap, refresh rate, lighting — without leaving what they were
doing. Deep tuning (TDP, fan curve, undervolt, preset editing) is an occasional
secondary job.

## Product Purpose

A slide-in control drawer, summoned by the Armoury Crate button, that is the
touch/gamepad face of the `z13ctl-plus` daemon. Success: the user opens it,
reads current state at a glance, changes what they came for in one or two taps,
and it gets out of the way. Deep controls exist but are never in the path of the
quick change.

## Positioning

The only Z13 control surface that works identically across a Wayland desktop,
touch-only tablet use, and Steam Gaming Mode, driven by the same hardware
button, with whole-system power automation (named presets assigned to
plugged-in and battery) on top of independent profile/TDP/fan/undervolt
overrides.

## Operating Context

- Opened and dismissed many times a day; Escape, focus loss, or the button
  closes it. Interaction sessions are seconds long.
- The daemon is the source of truth; the drawer re-reads state on show and
  polls telemetry while visible.
- Companion stack: `z13ctl-plus` (daemon/CLI, owns hardware) and
  `z13-tablet-kit` (posture and touch). The CLI offers everything the drawer
  does plus scripting.
- Fork of `dahui/z13gui`; installs alongside upstream under its own names.

## Capabilities and Constraints

- Profiles (quiet/balanced/performance), fan mode (Auto/Balanced/Turbo, plus
  custom curve), power automation with named presets and an inline preset
  editor, CPU min frequency/EPP/boost, TDP, undervolt, battery charge limit,
  refresh rate and panel overdrive, RGB keyboard/lightbar, tablet settings,
  live telemetry Overview.
- **Input parity**: every control must be reachable by touch, gamepad D-pad
  focus, and pointer; touch targets at least 40 px.
- **No popovers, dropdowns, or separate windows**: gamescope does not composite
  them. Use stack view switching and inline buttons.
- **Fixed 320 px drawer** on the right edge; height is content-driven and
  scrolls. Gamescope scales it by output resolution.
- **User themes must keep working**: 15 built-in themes with accent variants and
  custom `theme.toml`; colors come only from `@define-color` tokens in
  `theme-default.css`. Structure lives in `layout.css`.
- Known GTK limits recorded in AGENTS.md (no box-shadow on animated containers,
  no scale marks, no Revealer slide, no `hexpand` in CSS).
- Fan mode is never part of a preset; presets above 75 W sustained TDP engage
  safety fan cooling.

## Brand Commitments

- Name: **Z13GUI+**; command `z13gui-plus`. Never present it as upstream
  `z13gui`.
- Copy speaks plainly about hardware effects (what a change does, what it locks)
  rather than marketing.

## Evidence on Hand

- Screenshot: `assets/z13gui-plus.avif`.
- Docs site sources in `docs/` (getting started, configuration, theming).
- No testimonials, user counts, or benchmarks exist; do not invent them.

## Product Principles

1. **Seconds, then gone.** Optimize the open → glance → change → close loop;
   anything slower must earn its place.
2. **State before controls.** Show what is true now (and why, e.g. a safety
   lock or manual override) before asking the user to act.
3. **One way, every input.** A control that only works with a mouse is broken.
4. **Depth on request.** Advanced tuning stays one level away, never in the
   quick path, and never hidden when it is the cause of current state.
5. **Honest about hardware.** Name side effects (fan locks, applies now) at the
   moment of the change.

## Accessibility & Inclusion

- Large touch targets and full gamepad focus navigation (also serves motor and
  couch/handheld use).
- Theme contrast must hold across all built-in light and dark themes.
- GTK accessibility bridge is disabled (`GTK_A11Y=none`) to avoid AT-SPI
  timeouts under systemd; screen-reader support is currently not provided —
  treat as an open decision, not a goal already met.
