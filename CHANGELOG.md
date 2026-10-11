# Changelog

All notable changes to Z13GUI+, the Linux control panel for the ASUS ROG Flow Z13.
Versions follow [Semantic Versioning](https://semver.org/). Releases and packages
are on the [GitHub releases page](https://github.com/aic0d3r/z13gui-plus/releases)
and in the AUR as `z13gui-plus-bin`.

## [2.1.2] - 2026-10-11

### Fixed

- KDE Plasma: the hidden drawer no longer blocks mouse clicks on a monitor placed
  to the right of the Z13 screen. The invisible parked drawer is now click-through
  while hidden ([#1](https://github.com/aic0d3r/z13gui-plus/issues/1), thanks
  @Sarpar12).

## [2.1.1] - 2026-10-11

### Fixed

- Gaming Mode: closing and quickly reopening the drawer no longer lets controller
  input reach Steam and the game while the drawer is open.
- RGB lighting changes run in the background (no UI freeze), revert when the
  daemon rejects them, and can no longer be undone by a stale state refresh.
- The battery badge shows "—" instead of "MAX LIFE · 0%" when the charge limit is
  unknown.

### Changed

- The release pipeline runs the full test suite before building packages.

## [2.1.0] - 2026-10-11

### Added

- Tablet integration with [z13-tablet-kit](https://github.com/aic0d3r/z13-tablet-kit):
  a live posture and health chip (hidden until the kit reports a heartbeat) with
  toggles for desktop touch policy, folio touchpad opt-in, and scroll
  sensitivity/speed.
- Two-finger-hold context menu toggle.

### Fixed

- AUR package provides and conflicts with the tarball package name.

## [2.0.2] - 2026-08-03

### Changed

- Detached lighting devices show as off and their toggle is disabled, while the
  saved state is kept for when they are reattached. Requires z13ctl-plus v2.0.1.

## [2.0.1] - 2026-08-03

### Fixed

- The desktop entry sets `StartupNotify=false`, so Plasma no longer shows a
  startup spinner that swallowed launcher clicks. Rapid launcher clicks now toggle
  the drawer reliably.

## [2.0.0] - 2026-08-02

Z13GUI+ gets a complete, independent application identity and targets the
`z13ctl-plus` controller only.

### Changed (breaking)

- The command is `z13gui-plus`; no `z13gui` alias is installed.
- The user unit is `z13gui-plus.service`.
- The application ID and desktop file are `io.github.aic0d3r.z13gui_plus`.
- Configuration moves to `$XDG_CONFIG_HOME/z13gui-plus`; runtime files move to
  `$XDG_RUNTIME_DIR/z13gui-plus`.
- Environment overrides are `Z13GUI_PLUS_SCALE` and `Z13GUI_PLUS_NO_GAMEPAD`.
- Packages coexist with upstream, depend on `z13ctl-plus`, and leave the GUI
  service disabled after installation.

### Added

- `z13gui-plus --migrate-config` copies the v1 configuration into the new
  location without overwriting or deleting anything.

## [1.3.0] - 2026-07-28

First release under the `z13gui-plus` distribution name.

### Added

- Overview tab with live CPU, GPU, VRAM, NPU, battery, thermal, and fan telemetry.
- CPU power controls (minimum frequency, EPP, boost), named presets, and AC/battery
  power automation.
- Tarball, Arch, Debian, RPM, and AUR packages.

### Changed

- Expanded RGB, fan, TDP, refresh-rate, and battery controls with safer input
  handling.
- Refined touch-first navigation, tuning feedback, color controls, and gamepad and
  touchscreen handling in desktop and Gaming Mode.

## 1.0.0 – 1.2.7 (upstream)

Released as [dahui/z13gui](https://github.com/dahui/z13gui) before this fork. See
the [upstream releases](https://github.com/dahui/z13gui/releases) for details.

[2.1.2]: https://github.com/aic0d3r/z13gui-plus/releases/tag/v2.1.2
[2.1.1]: https://github.com/aic0d3r/z13gui-plus/releases/tag/v2.1.1
[2.1.0]: https://github.com/aic0d3r/z13gui-plus/releases/tag/v2.1.0
[2.0.2]: https://github.com/aic0d3r/z13gui-plus/releases/tag/v2.0.2
[2.0.1]: https://github.com/aic0d3r/z13gui-plus/releases/tag/v2.0.1
[2.0.0]: https://github.com/aic0d3r/z13gui-plus/releases/tag/v2.0.0
[1.3.0]: https://github.com/aic0d3r/z13gui-plus/releases/tag/v1.3.0
