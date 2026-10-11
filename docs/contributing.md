# Contributing

Contributions are welcome. Please open an issue before starting work on a
significant change so the approach can be discussed first.

---

## Repository structure

The root module is `github.com/aic0d3r/z13gui-plus`. Z13GUI+ intentionally
continues to import the controller API as `github.com/dahui/z13ctl/api`; the
`go.mod` replacement selects `github.com/aic0d3r/z13ctl-plus/api`, whose client
targets the Plus socket. Do not change the import to a fork-specific path or
remove the replacement.

| Package | Purpose |
|---------|---------|
| `internal/gui` | Main Window type, daemon subscription, gamepad navigation |
| `internal/gui/layershell` | Wayland layer-shell display backend |
| `internal/gui/gamescope` | X11 overlay backend for Steam Gaming Mode |
| `internal/gui/gamepad` | Linux evdev gamepad reader |
| `internal/gui/gamepad/hidblocker` | BPF LSM hidraw blocker (blocks PS/Nintendo controller reads) |
| `internal/gui/fonts` | Embedded Inter font registration |
| `internal/theme` | Color definitions, TOML parsing, CSS generation — pure Go |

---

## Development setup

```sh
git clone https://github.com/aic0d3r/z13gui-plus
cd z13gui-plus
```

**Build dependencies (Arch Linux):**

```sh
sudo pacman -S gtk4 gtk4-layer-shell
```

**Build dependencies (Debian/Ubuntu):**

```sh
sudo apt-get install -y libgtk-4-dev libgtk4-layer-shell-dev
```

**Build dependencies (Fedora):**

```sh
sudo dnf install gtk4-devel gtk4-layer-shell-devel
```

**BPF development (optional — only for modifying the hidraw blocker):**

Requires `clang`, `bpftool`, and kernel BTF support.

```sh
make vmlinux   # generate kernel BTF header
make generate  # compile BPF and generate Go bindings
```

To work against a local copy of the z13ctl-plus API module, create a `go.work`
file (it is gitignored; adjust the sibling directory name if needed):

```sh
go work init . ../z13ctl-plus/api
```

---

## Before submitting a pull request

```sh
make build     # compile (requires GTK4 headers)
make lint      # run golangci-lint
make test      # run unit tests (pure Go, no GTK4 required)
```

Tests live in the pure-Go packages (`internal/theme`, `internal/togglegate`) — no
hardware or GTK4 dependency. GUI packages are integration-tested manually against
hardware.

`make test` lists those packages explicitly instead of using `./...`, since
`internal/gui` requires CGO and GTK4 headers. If you add a new pure-Go package with
tests, add it to the `test` and `cover` targets in the Makefile or its tests will
never run.

Pull requests must pass `make build`, `make lint`, and `make test` without errors,
and should include tests for any changes to the pure-Go packages.

---

## Testing notes

- `internal/theme` — fully unit-testable; covers color parsing, CSS generation,
  config persistence, and all 78 built-in theme/accent combinations
- `internal/togglegate` — pure debounce helper for duplicate `gui-toggle` bursts
- `internal/gui` — requires GTK4; integration-tested manually against hardware
- Display backends (layershell, gamescope) — require a compositor or gamescope;
  no automated tests

---

## Release workflow (maintainers only)

```sh
git tag -a v2.0.0 -F release-notes.md
git push origin v2.0.0
```

GoReleaser handles binary builds, the `.pkg.tar.zst`, `.deb`, and `.rpm`
packages, AUR publishing, and GitHub Release creation automatically when
the tag is pushed. v2.0.0 is the independently named Plus namespace release;
release notes must not claim shared runtime-name compatibility with upstream.
