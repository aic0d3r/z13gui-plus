# Z13GUI+

GTK4 overlay drawer for [z13ctl-plus](https://github.com/aic0d3r/z13ctl-plus) on Wayland.

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

![Z13GUI+](assets/screen3.png)

Z13GUI+ is a compatibility-preserving distribution of
[dahui/z13gui](https://github.com/dahui/z13gui). It keeps the `z13gui` command,
`z13gui.service`, desktop and udev filenames, configuration and runtime paths, D-Bus
application ID, Go module, daemon API, and hardware/socket protocols unchanged.

The drawer is triggered by the Armoury Crate button (KEY_PROG3). It renders as a
Wayland layer-shell overlay (KDE Plasma, Hyprland, Sway) or a gamescope X11 overlay
in Steam Gaming Mode. All hardware communication goes through the `z13ctl` daemon.

Building on the upstream project, this fork adds AC/battery power automation,
expanded live telemetry, CPU minimum-frequency/EPP/boost controls, redesigned
Overview and Power interfaces, and gamescope-aware controller handling.

## Install

```sh
# Arch Linux (AUR)
yay -S z13gui-plus-bin

# Debian / Ubuntu
sudo apt install ./z13gui-plus_*.deb

# Fedora / RHEL
sudo dnf install ./z13gui-plus_*.rpm

# Manual (from release tarball)
tar xzf z13gui-plus_*_linux_amd64.tar.gz
sudo install -Dm755 z13gui /usr/local/bin/z13gui
```

See the [Installation guide](https://aic0d3r.github.io/z13gui-plus/installation/) for
systemd service setup, source builds, and uninstall instructions.

## Quick Start

Press the **Armoury Crate button** on your Z13 to open the drawer. Press it again,
click outside, or press Escape to close it.

The drawer provides live system telemetry, profile and automated power-source
switching, CPU and TDP tuning, fan curves, undervolting, battery charge limits, RGB
lighting, panel controls, and boot sound. Changes are sent to the `z13ctl` daemon.

## Documentation

Full documentation at **<https://aic0d3r.github.io/z13gui-plus>**

- [Installation](https://aic0d3r.github.io/z13gui-plus/installation/)
- [Quick Start](https://aic0d3r.github.io/z13gui-plus/getting-started/)
- [Configuration](https://aic0d3r.github.io/z13gui-plus/configuration/)
- [Theming](https://aic0d3r.github.io/z13gui-plus/theming/)
- [Contributing](https://aic0d3r.github.io/z13gui-plus/contributing/)

## License

[Apache 2.0](LICENSE)
