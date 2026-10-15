---
name: Compatibility Report
about: Report how Z13GUI+ behaves on a machine other than the ROG Flow Z13
labels: compatibility
---

**Machine**
- Vendor and model (`cat /sys/class/dmi/id/sys_vendor /sys/class/dmi/id/product_name`):
- CPU (for example Ryzen AI Max+ 395):
- Laptop, mini PC, or desktop:
- Battery present:

**Software**
- Linux distribution and kernel (`uname -r`):
- Desktop or compositor:
- Z13GUI+ version (`z13gui-plus --version`):
- z13ctl-plus version (`z13ctl-plus --version`):

**What the daemon reports**

Paste the `capabilities` object from the daemon (needs `socat` and `jq`):

`echo '{"cmd":"get-state"}' | socat - UNIX-CONNECT:$XDG_RUNTIME_DIR/z13ctl-plus/z13ctl-plus.sock | jq '.state.capabilities'`

```
```

**What works**

Mark each as working, hidden (not shown), or broken:

- Overview telemetry (temperatures, power, fans, memory, clocks):
- Profile switching:
- Fan modes and fan curve:
- TDP and undervolt:
- CPU boost, EPP, minimum frequency:
- Battery card and charge limit:
- RGB tab:
- Refresh rate, panel overdrive, boot sound:
- Power presets (plugged in / on battery):

**Anything unexpected**

Controls that were shown but did nothing, errors in the status line, or logs
(`journalctl --user -u z13gui-plus.service`, `journalctl --user -u z13ctl-plus.service`).
