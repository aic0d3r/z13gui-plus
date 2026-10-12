# Z13GUI+ — profiles, TDP, fan curves, RGB & undervolt control for the ASUS ROG Flow Z13 on Linux

<div align="center">

[![Release](https://img.shields.io/github/v/release/aic0d3r/z13gui-plus)](https://github.com/aic0d3r/z13gui-plus/releases)
[![AUR](https://img.shields.io/aur/version/z13gui-plus-bin)](https://aur.archlinux.org/packages/z13gui-plus-bin)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

[What Plus Adds](#what-plus-adds) •
[Tablet stack](#tablet-stack) •
[Interface](#interface) •
[Install](#install) •
[Quick Start](#quick-start) •
[Upgrading from v1](#upgrading-from-v1) •
[Documentation](#documentation) •
[License](#license)

</div>

GTK4 control drawer for the **2025 ASUS ROG Flow Z13** (GZ302, Ryzen AI Max /
Strix Halo): power profiles and AC/battery automation, TDP and fan-curve tuning,
CPU undervolt, RGB keyboard and lightbar lighting, battery charge limit, panel
refresh rate and overdrive, and live telemetry — slid in from the screen edge with
the Armoury Crate button. Works on KDE Plasma and other layer-shell Wayland
desktops, and inside Steam Gaming Mode (gamescope), with touch and gamepad
navigation.

`Z13GUI+` is an independently named fork of
[`dahui/z13gui`](https://github.com/dahui/z13gui) with its own command, service,
configuration, runtime, desktop, and application-ID namespaces, so it installs
alongside upstream; enable only one GUI at a time. It talks only to
[`z13ctl-plus`](https://github.com/aic0d3r/z13ctl-plus) — install that daemon
first. The installed command is `z13gui-plus`; no `z13gui` alias is provided.

> [!TIP]
> **Need the CLI or scripting? Install
> [z13ctl-plus](https://github.com/aic0d3r/z13ctl-plus).** It owns the hardware,
> exposes everything this drawer shows plus presets and scripting, and is required
> by Z13GUI+. This repo is the touch interface on top of it.

![Z13GUI+ control drawer for the ASUS ROG Flow Z13 on Linux](assets/z13gui-plus.avif)

## What Plus Adds

- **Power automation** — save named configurations and assign separate presets
  for plugged-in and battery operation
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

## Tablet stack

`Z13GUI+` is the settings and status surface in a three-repo tablet stack for the
Z13. Install the other two repos alongside it:

- [z13-tablet-kit](https://github.com/aic0d3r/z13-tablet-kit) — detects
  dock/folio/tablet posture, toggles the touchscreen, and provides touch
  scrolling and two-finger context menus
- [z13ctl-plus](https://github.com/aic0d3r/z13ctl-plus) — owns internal-panel
  scale/rotation/refresh changes and persists tablet settings (desktop touch
  policy, two-finger hold, folio touchpad opt-in, scroll speed, keep display scale)
- **Z13GUI+** (this repo) — GTK4 overlay drawer exposing live posture/health and
  the tablet toggles

Flow: the kit detects posture and publishes it to the daemon, which applies the
display changes; this drawer shows that state and persists settings back through
the daemon; the daemon projects them to a runtime settings file the kit reads and
applies live — no restarts required. The tablet chip in this drawer stays hidden
until the kit's `z13-tablet-switch` reports a heartbeat.

## Interface

<table>
  <tr>
    <td align="center"><strong>Power</strong><br><img src="assets/z13gui-plus-power.avif" alt="Z13GUI+ Power tab"></td>
    <td align="center"><strong>Power Tuning</strong><br><img src="assets/z13gui-plus-power-tuning.avif" alt="Z13GUI+ Power Tuning view"></td>
  </tr>
  <tr>
    <td align="center"><strong>RGB</strong><br><img src="assets/z13gui-plus-rgb.avif" alt="Z13GUI+ RGB tab"></td>
    <td align="center"><strong>System</strong><br><img src="assets/z13gui-plus-system.avif" alt="Z13GUI+ System tab"></td>
  </tr>
</table>

## Install

```sh
# Arch Linux (AUR)
yay -S z13gui-plus-bin

# Debian / Ubuntu
sudo apt install ./z13gui-plus_*.deb

# Fedora / RHEL
sudo dnf install ./z13gui-plus_*.rpm

# Manual (from release tarball)
wget https://github.com/aic0d3r/z13gui-plus/releases/download/v2.2.0/z13gui-plus_2.2.0_linux_amd64.tar.gz
tar xzf z13gui-plus_2.2.0_linux_amd64.tar.gz
sudo install -Dm755 z13gui-plus /usr/local/bin/z13gui-plus

# With z13ctl-plus set up
systemctl --user enable --now z13gui-plus.service
```

> **AUR lag:** the `z13gui-plus-bin` AUR package trails the GitHub releases (currently
> 1.3.0 vs. **v2.2.0** — AUR updates are paused). It lacks the tablet integration. For
> the latest, install the **release tarball** from GitHub instead of AUR.

See the [Installation guide](https://aic0d3r.github.io/z13gui-plus/installation/) for
runtime dependencies, gamepad input blocking, source builds, and uninstall
instructions. Packages leave the GUI service disabled; enable only the GUI that
matches the controller daemon you selected.

## Quick Start

```sh
# Armoury Crate button on the Z13 opens and closes the drawer
systemctl --user enable --now z13ctl-plus.socket z13ctl-plus.service
systemctl --user enable --now z13gui-plus.service
```

The **Overview** shows live system state; **Power** contains automation, CPU policy,
battery strategy, and advanced tuning; **RGB** drives the keyboard and lightbar;
**System** covers refresh rate, overdrive, boot sound, and tablet options.
Hardware-control changes are sent to the `z13ctl-plus` daemon.

## Upgrading from v1

After installing v2.0.0 or later, copy the complete legacy configuration only if the
new Plus configuration directory does not exist:

```sh
systemctl --user disable --now z13gui.service
z13gui-plus --migrate-config
systemctl --user enable --now z13gui-plus.service
```

Migration copies `$XDG_CONFIG_HOME/z13gui` to
`$XDG_CONFIG_HOME/z13gui-plus`; it leaves the source untouched and does not
change services or remove artifacts. Run the disable command only when the old
service is known to be from this fork. Legacy names may belong to upstream, so
remove old files only after verifying their provenance. See the
[migration guide](https://aic0d3r.github.io/z13gui-plus/installation/#migrating-from-v1-to-v2).

## Documentation

Full documentation at **<https://aic0d3r.github.io/z13gui-plus>**

- [Installation](https://aic0d3r.github.io/z13gui-plus/installation/)
- [Quick Start](https://aic0d3r.github.io/z13gui-plus/getting-started/)
- [Configuration](https://aic0d3r.github.io/z13gui-plus/configuration/)
- [Theming](https://aic0d3r.github.io/z13gui-plus/theming/)
- [Changelog](https://aic0d3r.github.io/z13gui-plus/changelog/)
- [Contributing](https://aic0d3r.github.io/z13gui-plus/contributing/)

## License

[Apache 2.0](LICENSE)
