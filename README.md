<div align="center">

<img src="assets/logo.svg" width="440" alt="Z13GUI+">

</div>

# Z13GUI+ — ROG Flow Z13 control panel for Linux

<div align="center">

[![Release](https://img.shields.io/github/v/release/aic0d3r/z13gui-plus)](https://github.com/aic0d3r/z13gui-plus/releases)
[![AUR](https://img.shields.io/aur/version/z13gui-plus-bin)](https://aur.archlinux.org/packages/z13gui-plus-bin)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

[Install](#install) •
[Darker screen](#darker-screen) •
[Quick Start](#quick-start) •
[Docs](#documentation)

</div>

A touch- and gamepad-friendly drawer for the **2025 ASUS ROG Flow Z13** (Strix Halo),
opened with the Armoury Crate button. Works on KDE Plasma, other layer-shell Wayland
desktops, and Steam Gaming Mode.

- **Power** — profiles, AC/battery presets, TDP, fan curves, undervolt
- **Darker screen** — [z13-backlight-quirk](https://github.com/aic0d3r/z13-backlight-quirk)
  lowers the Z13's minimum brightness; built-in **Dim** covers the rest
- **RGB** — keyboard and lightbar
- **Display & battery** — 60/180 Hz, overdrive, charge limit, live telemetry

Part of a three-repo Z13 stack:

| Repo | Role |
|------|------|
| [`z13ctl-plus`](https://github.com/aic0d3r/z13ctl-plus) | Daemon + CLI that owns the hardware (**required**) |
| **Z13GUI+** (this repo) | Touch drawer on top of it |
| [`z13-tablet-kit`](https://github.com/aic0d3r/z13-tablet-kit) | Dock/folio/tablet posture, touch scrolling, context menus (optional) |

Other Strix Halo machines are experimental; GNOME is an untested beta. Fork of
[`dahui/z13gui`](https://github.com/dahui/z13gui) that installs alongside upstream.
[More](https://aic0d3r.github.io/z13gui-plus/overview/).

<p align="center">
  <img src="assets/z13gui-plus.avif" width="380" alt="Z13GUI+ control drawer for the ASUS ROG Flow Z13 on Linux">
</p>

## Install

```sh
# Arch Linux (AUR)
yay -S z13gui-plus-bin

# Debian / Ubuntu
sudo apt install ./z13gui-plus_*.deb

# Fedora / RHEL
sudo dnf install ./z13gui-plus_*.rpm

# Manual (from release tarball)
wget https://github.com/aic0d3r/z13gui-plus/releases/download/v2.8.0/z13gui-plus_2.8.0_linux_amd64.tar.gz
tar xzf z13gui-plus_2.8.0_linux_amd64.tar.gz
sudo install -Dm755 z13gui-plus /usr/local/bin/z13gui-plus
```

See the [Installation guide](https://aic0d3r.github.io/z13gui-plus/installation/) for
dependencies, gamepad input blocking, source builds, and uninstall. Packages leave
the service disabled.

## Darker screen

The Z13's panel is still bright at 0%. Install
[z13-backlight-quirk](https://github.com/aic0d3r/z13-backlight-quirk), a DKMS
kernel quirk that lowers the floor, then reboot:

```sh
yay -S z13-backlight-quirk-dkms
```

Without it, **Settings → Display → Dim** darkens the screen in software (KDE).

## Quick Start

```sh
systemctl --user enable --now z13ctl-plus.socket z13ctl-plus.service z13gui-plus.service
```

Press the **Armoury Crate button** (or **Meta+Z** on KDE) to open the drawer.

## Documentation

Full docs: **<https://aic0d3r.github.io/z13gui-plus>** —
[Overview & compatibility](https://aic0d3r.github.io/z13gui-plus/overview/) ·
[Installation](https://aic0d3r.github.io/z13gui-plus/installation/) ·
[Quick Start](https://aic0d3r.github.io/z13gui-plus/getting-started/) ·
[Configuration](https://aic0d3r.github.io/z13gui-plus/configuration/) ·
[Theming](https://aic0d3r.github.io/z13gui-plus/theming/) ·
[Changelog](https://aic0d3r.github.io/z13gui-plus/changelog/)

## License

[Apache 2.0](LICENSE) · If it helps, [star it](https://github.com/aic0d3r/z13gui-plus) or [sponsor me](https://github.com/sponsors/aic0d3r).
