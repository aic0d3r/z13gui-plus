#!/bin/sh
udevadm control --reload-rules || true
udevadm trigger --subsystem-match=input || true
gtk-update-icon-cache -q -t -f /usr/share/icons/hicolor 2>/dev/null || true
