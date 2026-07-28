# Z13GUI+

GTK4 overlay drawer for [z13ctl-plus](https://github.com/aic0d3r/z13ctl-plus) on Wayland.

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

![Z13GUI+](assets/z13gui-plus.avif)

Z13GUI+ is a compatibility-preserving distribution of
[dahui/z13gui](https://github.com/dahui/z13gui). It keeps the `z13gui` command,
service, configuration paths, Go module, daemon API, and hardware protocols
unchanged.

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

Press the **Armoury Crate button** on your Z13 to open the drawer. The Overview
shows live system state; Power contains automation, CPU policy, battery strategy,
and advanced tuning. Hardware-control changes are sent to the `z13ctl` daemon.

## Documentation

Full documentation at **<https://aic0d3r.github.io/z13gui-plus>**

- [Installation](https://aic0d3r.github.io/z13gui-plus/installation/)
- [Quick Start](https://aic0d3r.github.io/z13gui-plus/getting-started/)
- [Configuration](https://aic0d3r.github.io/z13gui-plus/configuration/)
- [Theming](https://aic0d3r.github.io/z13gui-plus/theming/)
- [Contributing](https://aic0d3r.github.io/z13gui-plus/contributing/)

## License

[Apache 2.0](LICENSE)
