#!/bin/sh
udevadm control --reload-rules
udevadm trigger --subsystem-match=input
gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor 2>/dev/null || true
echo "z13gui-plus user service installed disabled; enable it after selecting z13ctl-plus:"
echo "  systemctl --user enable --now z13gui-plus.service"
