# Z13GUI+

GTK4 overlay drawer for **z13ctl** on Wayland — graphical controls for the
2025 ASUS ROG Flow Z13 on Linux.

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://github.com/aic0d3r/z13gui-plus/blob/main/LICENSE)

Z13GUI+ is a compatibility-preserving distribution of
[dahui/z13gui](https://github.com/dahui/z13gui). Distribution packages and
release artifacts use `z13gui-plus`, while the command, service, desktop and udev
filenames, configuration/runtime paths, D-Bus ID, canonical Go module, daemon API,
and hardware/socket protocols remain compatible as `z13gui`.

---

## What Z13GUI+ does

- **Power automation** — named presets with automatic AC/battery switching
- **Profile switching** — quiet, balanced, and performance hardware profiles
- **CPU power control** — minimum frequency, AMD EPP, and CPU boost controls independent of GPU clocks
- **Custom TDP control** — configurable power limits (PL1/PL2/PL3) in the custom profile view, with basic and advanced modes
- **Fan curve editor** — per-profile fan response curve editing (custom profile, advanced mode)
- **Undervolt** — CPU Curve Optimizer offset (requires `ryzen_smu` kernel module; iGPU CO is not supported on Strix Halo)
- **Live telemetry overview** — CPU/GPU temperatures and load, clocks, APU/GPU/NPU power, memory, battery context, and fan data where available
- **Battery charge limit** — set the charge cap (40–100%) from the drawer
- **RGB lighting** — mode, color, speed, and brightness for the keyboard
  backlight and edge lightbar
- **System toggles** — panel overdrive and boot sound on/off
- **Theme picker** — 15 built-in themes with full custom theme support
- **Redesigned Overview and Power UI** — separates live status from tuning and automation controls
- **Gamescope controller handling** — full D-pad/button navigation with controller-input suppression while the drawer is open

All hardware communication goes through the `z13ctl` daemon. Z13GUI+ never
touches HID devices or sysfs directly.

---

## Display backends

Two backends are supported, selected automatically based on the session
environment:

- **Layer-shell** (KDE Plasma, Hyprland, Sway) — Wayland layer-shell overlay
  with margin-based slide animation and focus-loss dismiss
- **Gamescope** (Steam Gaming Mode) — X11 overlay via the `STEAM_OVERLAY` atom
  with opacity-based visibility and a click-to-dismiss backdrop

---

## Requirements

- Wayland compositor with layer-shell support, or gamescope (Steam Gaming Mode)
- GTK 4 and gtk4-layer-shell libraries (see [Installation](installation.md#runtime-dependencies) for distro package names)
- [z13ctl-plus](https://github.com/aic0d3r/z13ctl-plus) (`z13ctl`) daemon running

---

## Next steps

- [**Installation**](installation.md) — download the binary or build from source
- [**Quick Start**](getting-started.md) — open the drawer and explore the controls
- [**Configuration**](configuration.md) — config file, environment variables
- [**Theming**](theming.md) — built-in themes and custom color definitions
