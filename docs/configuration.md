# Configuration

## Config file

Z13GUI+ stores its configuration in
`$XDG_CONFIG_HOME/z13gui-plus/config.toml`, normally
`~/.config/z13gui-plus/config.toml`. This file is updated automatically when you
change themes using the in-app theme picker.

```toml
theme = "catppuccin-mocha"
accent = "sapphire"
```

| Key | Description |
|-----|-------------|
| `theme` | Built-in theme ID (see `z13gui-plus --list-themes` or [Theming](theming.md)) |
| `accent` | Accent color variant for themes that support it; `""` uses the theme default |

If no config file exists, Z13GUI+ defaults to the `rog-dark` theme.

You can edit this file by hand. Changes take effect the next time Z13GUI+ starts.

---

## Theme priority

See [Theme priority](theming.md#theme-priority) for the resolution order and
custom-theme behavior.

---

## Environment variables

| Variable | Description |
|----------|-------------|
| `Z13GUI_PLUS_SCALE` | Override CSS scale factor in gamescope mode (default: auto from output resolution) |
| `Z13GUI_PLUS_NO_GAMEPAD` | Set to `1` to disable gamepad input entirely |

---

## Command-line flags

| Flag | Description |
|------|-------------|
| `--debug`, `-d` | Enable debug logging (includes GTK messages) |
| `--version` | Print version and exit |
| `--print-theme` | Print the default theme.toml to stdout |
| `--list-themes` | List all built-in theme IDs and names |
| `--migrate-config` | Copy the complete legacy v1 config directory into the Plus namespace; fails if the Plus config already exists |

`--print-theme` is the recommended starting point for a custom theme:

```sh
mkdir -p ~/.config/z13gui-plus
z13gui-plus --print-theme > ~/.config/z13gui-plus/theme.toml
```

For v1 configuration, use the explicit
[migration command](installation.md#migrating-from-v1-to-v2). Do not manually
merge the old and new directories.

---

## Runtime files

Z13GUI+ keeps its owned runtime files under
`$XDG_RUNTIME_DIR/z13gui-plus`. If `XDG_RUNTIME_DIR` is unavailable, the
embedded-font cache falls back to `/tmp/z13gui-plus/fonts`, while Steam
frozen-process recovery follows the existing per-user convention at
`/run/user/$UID/z13gui-plus/frozen-pid`.
