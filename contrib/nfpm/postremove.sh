#!/bin/sh
udevadm control --reload-rules || true
udevadm trigger --subsystem-match=input || true
