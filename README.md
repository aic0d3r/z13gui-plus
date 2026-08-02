# Z13GUI+

GTK4 overlay drawer for [z13ctl-plus](https://github.com/aic0d3r/z13ctl-plus) on Wayland.

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

![Z13GUI+](assets/z13gui-plus.avif)

Z13GUI+ is an independently named fork of
[dahui/z13gui](https://github.com/dahui/z13gui) for
[z13ctl-plus](https://github.com/aic0d3r/z13ctl-plus). It preserves intentional
API and UI compatibility, but v2.0.0 uses its own command, service,
configuration, runtime, desktop, and application-ID namespaces. The installed
command is `z13gui-plus`; no `z13gui` alias is provided.

The `z13gui-plus` package can coexist with upstream. Packages do not provide,
conflict with, or replace upstream, and installation leaves the GUI service
disabled. Enable only the GUI that matches the controller daemon you selected.

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
tar xzf z13gui-plus_*_linux_amd64.tar.gz
sudo install -Dm755 z13gui-plus /usr/local/bin/z13gui-plus
```

See the [Installation guide](https://aic0d3r.github.io/z13gui-plus/installation/) for
systemd service setup, source builds, and uninstall instructions.

## Quick Start

Press the **Armoury Crate button** on your Z13 to open the drawer. The Overview
shows live system state; Power contains automation, CPU policy, battery strategy,
and advanced tuning. Hardware-control changes are sent to the `z13ctl-plus`
daemon.

## Upgrading from v1

After installing v2.0.0, copy the complete legacy configuration only if the new
Plus configuration directory does not exist:

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
- [Contributing](https://aic0d3r.github.io/z13gui-plus/contributing/)

## License

[Apache 2.0](LICENSE)
