# Z13GUI+

GTK4 overlay drawer for **z13ctl-plus** on Wayland — graphical controls for the
2025 ASUS ROG Flow Z13 on Linux.

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://github.com/aic0d3r/z13gui-plus/blob/main/LICENSE)

Z13GUI+ is an independently named fork of
[dahui/z13gui](https://github.com/dahui/z13gui) targeting
[z13ctl-plus](https://github.com/aic0d3r/z13ctl-plus). It preserves intentional
API and UI compatibility, not runtime-name compatibility. Starting with v2.0.0,
the command, service, configuration/runtime paths, desktop and udev filenames,
GApplication ID, and Go module all use the Plus namespace. The only installed
command is `z13gui-plus`.

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

Hardware-control changes are sent through the `z13ctl-plus` daemon.

---

## Requirements

- Wayland compositor with layer-shell support, or gamescope (Steam Gaming Mode)
- GTK 4 and gtk4-layer-shell libraries (see [Installation](installation.md#runtime-dependencies) for distro package names)
- [z13ctl-plus](https://github.com/aic0d3r/z13ctl-plus) daemon running

The Plus and upstream packages can coexist. Installation does not enable the
GUI service or manage either controller service; explicitly enable only the GUI
that matches your selected daemon.

---

## Next steps

- [**Installation**](installation.md) — download the binary or build from source
- [**Quick Start**](getting-started.md) — open the drawer and explore the controls
- [**Configuration**](configuration.md) — config file, environment variables
- [**Theming**](theming.md) — built-in themes and custom color definitions
