# Changelog

All notable changes. Newest first. Bodies mirror the GitHub release notes;
release-notes.md (gitignored) is the scratch file for the next entry.

## v2.4.0 (2026-10-13)

Requires z13ctl-plus v2.3.0 or newer.

- Power Automation card gets one-tap preset chips: tap to apply, the active
  preset is highlighted, and presets above 75 W sustained TDP are flagged
  before the tap.
- A single Presets screen replaces the separate library and chooser: each
  preset has Apply / Edit and inline AC / Battery assignment toggles.
  "+ Save Current Settings" names the preset automatically.
- Presets are now editable: profile, TDP (default or PL1/PL2/PL3),
  undervolt, CPU minimum/EPP/boost, refresh rate and panel overdrive.
  Renaming keeps assignments. Leaving with unsaved changes asks first.
- When a manual change overrides the assigned preset, the card shows a
  MANUAL pill and a "Reapply <preset>" button; while automation is off, a
  hint names the preset that turning it on will apply.
- Preset status and partial-apply errors show on the main card and in the
  editor.
- Presets no longer show a fan-mode row (presets no longer carry fan mode).
- KDE Plasma: **Meta+Z** toggles the drawer (default global shortcut,
  rebindable under System Settings → Shortcuts → Z13GUI+). Takes effect
  after the next login.

## v2.3.0 (2026-10-12)

- Energy-preference picker now only offers the values the kernel accepts.
  amd-pstate-epp advertises the allowed EPP values per current cpufreq
  policy — under the performance policy that is `performance` alone. All
  four buttons used to render regardless; the daemon rejected the rest
  (`unsupported CPU EPP`) and, because the failure was only reported in the
  Presets view, the picker silently snapped back to Performance. Buttons now
  follow the daemon's `epp_choices` (the kernel's advertised list) and
  repopulate live whenever the policy changes.
- New **Dynamic** energy preference on Linux 7.3+ kernels: amd-pstate-epp's
  dynamic mode lets the kernel pick the EPP from the power source / platform
  profile natively. The button only appears when the kernel advertises it,
  so older kernels are unaffected.
- Fixed the active highlight in dynamic mode: the 7.3 kernel reads dynamic
  back as `dynamic(profile:…)`, which previously matched no button (nothing
  turned red) and leaked into the tuning summary and presets captured from
  live state. The suffix is now stripped at every consumption point.

## v2.2.1 (2026-10-12)

- System tab: new ABOUT card showing the installed versions of Z13GUI+,
  Z13CTL+, and z13-tablet-kit.
- Switches no longer freeze halfway when the periodic state refresh lands
  while you are toggling them.

## v2.2.0 (2026-10-12)

- Tablet Kit panel: new "Keep my display scale" toggle. When on, z13ctl-plus
  never changes the screen scale on posture changes. Requires z13ctl-plus
  v2.2.1. [z13-tablet-kit#1](https://github.com/aic0d3r/z13-tablet-kit/issues/1)
- KDE Plasma: the hidden drawer now really stops blocking clicks on a monitor to
  the right of the Z13 screen (the v2.1.2 fix was not enough on KWin). While hidden
  the drawer drops below all windows.
  [#1](https://github.com/aic0d3r/z13gui-plus/issues/1) (thanks @Sarpar12)
- With a monitor to the right, the drawer no longer alternates between fading and
  sliding across that monitor on every other open.

## v2.1.2 (2026-10-11)

- KDE Plasma: the hidden drawer no longer blocks mouse clicks on a monitor placed
  to the right of the Z13 screen. The invisible parked drawer is now click-through
  while hidden.
  [#1](https://github.com/aic0d3r/z13gui-plus/issues/1) (thanks @Sarpar12)

## v2.1.1 (2026-10-11)

- Gaming Mode: closing and quickly reopening the drawer no longer lets controller
  input reach Steam and the game while the drawer is open.
- RGB lighting changes run in the background (no UI freeze), revert when the daemon
  rejects them, and can no longer be undone by a stale state refresh.
- The battery badge shows "—" instead of "MAX LIFE · 0%" when the charge limit is
  unknown.
- The release pipeline runs the full test suite before building packages.

## v2.1.0 (2026-10-11)

- Tablet integration with
  [z13-tablet-kit](https://github.com/aic0d3r/z13-tablet-kit): a live posture and
  health chip (hidden until the kit reports a heartbeat) with toggles for desktop
  touch policy, folio touchpad opt-in, and scroll sensitivity/speed.
- Two-finger-hold context menu toggle.
- The AUR package provides and conflicts with the tarball package name.

## v2.0.2 (2026-08-03)

- Detached lighting devices show as off and their toggle is disabled, while the
  saved state is kept for when they are reattached. Requires z13ctl-plus v2.0.1.

## v2.0.1 (2026-08-03)

- The desktop entry sets `StartupNotify=false`, so Plasma no longer shows a startup
  spinner that swallowed launcher clicks. Rapid launcher clicks now toggle the
  drawer reliably.

## v2.0.0 (2026-08-02)

Independent application identity; targets the `z13ctl-plus` controller only.

- The command is `z13gui-plus`; no `z13gui` alias is installed.
- The user unit is `z13gui-plus.service`; the application ID and desktop file are
  `io.github.aic0d3r.z13gui_plus`.
- Configuration moves to `$XDG_CONFIG_HOME/z13gui-plus`; runtime files move to
  `$XDG_RUNTIME_DIR/z13gui-plus`.
- Environment overrides are `Z13GUI_PLUS_SCALE` and `Z13GUI_PLUS_NO_GAMEPAD`.
- Packages coexist with upstream, depend on `z13ctl-plus`, and leave the GUI
  service disabled after installation.
- `z13gui-plus --migrate-config` copies the v1 configuration into the new location
  without overwriting or deleting anything.

## v1.3.0 (2026-07-28)

First release under the `z13gui-plus` distribution name.

- Overview tab with live CPU, GPU, VRAM, NPU, battery, thermal, and fan telemetry.
- CPU power controls (minimum frequency, EPP, boost), named presets, and AC/battery
  power automation.
- Tarball, Arch, Debian, RPM, and AUR packages.
- Expanded RGB, fan, TDP, refresh-rate, and battery controls with safer input
  handling; refined touch-first navigation, tuning feedback, color controls, and
  gamepad and touchscreen handling in desktop and Gaming Mode.

## v1.0.0 – v1.2.7

Released as [dahui/z13gui](https://github.com/dahui/z13gui) before this fork. See
the [upstream releases](https://github.com/dahui/z13gui/releases).
