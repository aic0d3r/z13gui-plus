#!/bin/sh
udevadm control --reload-rules
udevadm trigger --subsystem-match=input
gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor 2>/dev/null || true
# Restart only already-running instances so an upgrade doesn't keep the old binary.
for u in $(loginctl list-users --no-legend 2>/dev/null | awk '{print $1}'); do
  systemctl --user --machine="$u@.host" try-restart z13gui-plus.service 2>/dev/null || true
done
echo "z13gui-plus user service installed disabled; enable it after selecting z13ctl-plus:"
echo "  systemctl --user enable --now z13gui-plus.service"
