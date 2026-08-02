#!/bin/sh
udevadm control --reload-rules
udevadm trigger
setcap cap_bpf,cap_perfmon+ep /usr/bin/z13gui-plus 2>/dev/null || true
echo "z13gui-plus user service installed disabled; enable it after selecting z13ctl-plus:"
echo "  systemctl --user enable --now z13gui-plus.service"
