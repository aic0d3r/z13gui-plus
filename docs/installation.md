# Installation

## Prerequisites

- Linux kernel (x86_64)
- Wayland compositor with layer-shell support, or gamescope (Steam Gaming Mode)
- [z13ctl-plus](https://github.com/aic0d3r/z13ctl-plus) installed with the compatible `z13ctl` daemon running

### Runtime dependencies

Z13GUI+ links dynamically against GTK 4 and gtk4-layer-shell. The AUR package
pulls these automatically; for other install methods, install them via your
system package manager first.

| Dependency | Arch | Debian / Ubuntu | Fedora |
|---|---|---|---|
| GTK 4 | `gtk4` | `libgtk-4-1` | `gtk4` |
| gtk4-layer-shell | `gtk4-layer-shell` | `libgtk4-layer-shell0` | `gtk4-layer-shell` |

---

## Install

=== "Release binary"

    Install the [runtime dependencies](#runtime-dependencies) for your distro,
    then download the latest `linux_amd64` archive from the
    [Releases](https://github.com/aic0d3r/z13gui-plus/releases) page:

    ```sh
    tar xzf z13gui-plus_*_linux_amd64.tar.gz
    sudo install -Dm755 z13gui /usr/local/bin/z13gui
    sudo install -Dm644 contrib/z13gui.desktop \
        /usr/local/share/applications/z13gui.desktop
    ```

    Install the systemd user service:

    ```sh
    install -Dm644 contrib/z13gui.service \
        ~/.config/systemd/user/z13gui.service
    systemctl --user daemon-reload
    systemctl --user enable --now z13gui
    ```

=== "Arch Linux (AUR)"

    Install the [z13gui-plus-bin](https://aur.archlinux.org/packages/z13gui-plus-bin)
    package with your preferred AUR helper:

    ```sh
    yay -S z13gui-plus-bin
    ```

    The package installs the binary, systemd service, udev rules, and desktop
    entry. Services are enabled automatically for all users on next login.

    Alternatively, download the `.pkg.tar.zst` package directly from the
    [Releases](https://github.com/aic0d3r/z13gui-plus/releases) page and install with
    pacman:

    ```sh
    sudo pacman -U z13gui-plus-*.pkg.tar.zst
    ```

=== "Debian / Ubuntu"

    Download the `.deb` package from the
    [Releases](https://github.com/aic0d3r/z13gui-plus/releases) page, then install:

    ```sh
    sudo apt install ./z13gui-plus_*.deb
    ```

    The package installs the binary, systemd service, udev rules, and desktop
    entry. After installing, enable the service:

    ```sh
    systemctl --user enable --now z13gui
    ```

=== "Fedora / RHEL"

    Download the `.rpm` package from the
    [Releases](https://github.com/aic0d3r/z13gui-plus/releases) page, then install:

    ```sh
    sudo dnf install ./z13gui-plus_*.rpm
    ```

    The package installs the binary, systemd service, udev rules, and desktop
    entry. After installing, enable the service:

    ```sh
    systemctl --user enable --now z13gui
    ```

=== "From source"

    Requires Go 1.25+, CGO enabled, and GTK4 development libraries.

    **Arch Linux:**

    ```sh
    sudo pacman -S gtk4 gtk4-layer-shell
    ```

    **Debian/Ubuntu:**

    ```sh
    sudo apt-get install -y libgtk-4-dev libgtk4-layer-shell-dev
    ```

    **Fedora:**

    ```sh
    sudo dnf install gtk4-devel gtk4-layer-shell-devel
    ```

    Then clone and build:

    ```sh
    git clone https://github.com/aic0d3r/z13gui-plus
    cd z13gui-plus
    make build
    sudo make install
    make install-service
    ```

---

## Gamepad input blocking (capabilities)

In Steam Gaming Mode (gamescope), Z13GUI+ suppresses controller input while the
drawer is open so button presses navigate the overlay instead of the game.

Z13GUI+ supports two blocking methods and selects the best one automatically:

| Method | Requires | Behaviour | Side effects |
|--------|----------|-----------|--------------|
| **BPF blocker** (preferred) | `CAP_BPF` + `CAP_PERFMON` on the binary | Blocks PS / Nintendo controller reads at the kernel level via a BPF LSM hook | None — Steam and the game keep running normally |
| **SIGSTOP fallback** | No extra capabilities | Pauses the Steam process with SIGSTOP / SIGCONT | Game also pauses; PipeWire frame delivery stops |

The AUR, `.deb`, and `.rpm` packages grant the required capabilities
automatically during installation. If you installed from source or from the
release binary, grant them manually:

```sh
sudo setcap cap_bpf,cap_perfmon+ep /usr/local/bin/z13gui
```

??? note "What are these capabilities and are they safe?"

    ### Short version

    These two capabilities let Z13GUI+ load a tiny kernel filter that tells
    the system "when Steam tries to read a PS or Nintendo controller, return
    a temporary 'try again later' error instead." That's all it does. It
    cannot access your files, network, or any other part of the system. The
    filter is automatically removed when Z13GUI+ exits.

    ### Technical details

    **CAP_BPF** allows loading BPF programs into the kernel. Z13GUI+ uses
    this to attach a single LSM (`lsm/file_permission`) hook that
    intercepts `read()` calls on hidraw character devices
    (`/dev/hidraw*`). The hook checks whether the calling PID is in a
    small allow-list map and the target device is a hidraw device
    (major 244). If both conditions match, it returns `-EAGAIN`; otherwise
    it returns `0` (allow).

    **CAP_PERFMON** is required by the kernel to attach BPF LSM programs.
    Z13GUI+ does not use performance monitoring — this capability is a
    kernel-imposed prerequisite for LSM attachment.

    **What the BPF program can do:**

    - Return `-EAGAIN` for `read()` calls on hidraw devices by specific PIDs
    - Nothing else — the program is verified by the kernel's BPF verifier
      before loading and cannot be modified at runtime

    **What the BPF program cannot do:**

    - Access files, network, or memory outside its own BPF maps
    - Survive a process exit — all BPF resources are released when Z13GUI+
      stops, crashes, or is killed
    - Affect any process not explicitly added to its PID map
    - Block any operation other than `read()` on hidraw devices

    **Compared to running as root:** file capabilities grant only the two
    listed privileges to the `z13gui` binary. The process runs as your normal
    user with no other elevated access. This is strictly safer than running
    with `sudo` or as root.

---

## Verify the installation

```sh
z13gui --version
```

Then press the Armoury Crate button on your Z13. The drawer should slide in
from the right edge of the screen.

---

## Uninstall

Stop and remove the service:

```sh
make uninstall-service
```

Or manually:

```sh
systemctl --user disable --now z13gui
rm -f ~/.config/systemd/user/z13gui.service
systemctl --user daemon-reload
```

Remove the binary:

```sh
sudo rm /usr/local/bin/z13gui
```

---

## Troubleshooting

**Drawer doesn't appear**

Make sure the z13ctl daemon is running:

```sh
systemctl --user status z13ctl.service
```

**Service fails to start**

Check the journal:

```sh
journalctl --user -u z13gui -n 50
```

Run with debug logging to see GTK and initialization output:

```sh
z13gui --debug
```

**CachyOS: NPU shows ACTIVE but no watts or utilization**

Current CachyOS kernels include an amdxdna driver with NPU power and
utilization sensors. An older `amdxdna-dkms` module can override it with a
driver that supports inference but not those sensors. Check the loaded module:

```sh
modinfo -n amdxdna
```

If the path contains `updates/dkms`, follow the
[z13ctl-plus NPU telemetry guidance](https://github.com/aic0d3r/z13ctl-plus/blob/main/docs/installation.md#npu-telemetry-on-cachyos)
to test the native driver with a documented rollback. Do not remove DKMS on an
older kernel unless that kernel already provides its own amdxdna module.

**Gamescope: controller input not suppressed while drawer is open**

Grant BPF capabilities so Z13GUI+ can block controller input at the kernel level:

```sh
sudo setcap cap_bpf,cap_perfmon+ep /usr/local/bin/z13gui
```

Without capabilities, Z13GUI+ falls back to freezing Steam (SIGSTOP), which
also pauses running games.

**Gamescope: drawer doesn't show**

Verify `GAMESCOPE_WAYLAND_DISPLAY` is set and the socket exists:

```sh
echo $GAMESCOPE_WAYLAND_DISPLAY
ls "$XDG_RUNTIME_DIR/$GAMESCOPE_WAYLAND_DISPLAY"
```

If the socket is missing (stale environment from a previous Gaming Mode session),
Z13GUI+ automatically falls back to Wayland layer-shell mode.
