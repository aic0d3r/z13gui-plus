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

## What Plus Adds

- **Power automation** — save named configurations, assign separate plugged-in
  and battery presets, and apply them automatically when the power source changes
- **Independent CPU policy** — control minimum CPU frequency, AMD EPP, and boost
  without coupling them to GPU clocks
- **Expanded telemetry** — monitor CPU, GPU, NPU, memory, battery, power, clocks,
  temperatures, and both fans from a dedicated Overview
- **Battery insight** — see health, energy capacity, system draw, charge state,
  and direction-aware runtime estimates
- **Direct display control** — switch the internal panel between 60 Hz and 180 Hz
- **Safer tuning feedback** — identify unsaved overrides, high-TDP fan safety,
  the live fan-curve operating point, and reset all tuning overrides together
- **Faster fan control** — select Auto, Balanced, or Turbo without editing a curve

Hardware-control changes are sent through the compatible `z13ctl` daemon.

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
