package gui

// versions.go — "ABOUT" card on the System tab: versions of the three Z13 repos.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// Version is the z13gui-plus build version; main sets it before New.
var Version = "dev"

// buildVersionSection shows the GUI, daemon, and tablet kit versions. The
// external lookups run off the main thread because they exec other programs.
func (w *Window) buildVersionSection() *gtk.Box {
	card, summary := statusCard("ABOUT")
	summary.SetVisible(false)
	guiRow, guiVal := metricRow("Z13GUI+")
	ctlRow, ctlVal := metricRow("Z13CTL+")
	kitRow, kitVal := metricRow("Tablet Kit")
	guiVal.SetText(Version)
	card.Append(guiRow)
	card.Append(ctlRow)
	card.Append(kitRow)

	go func() {
		ctl, kit := daemonVersion(), tabletKitVersion()
		glib.IdleAdd(func() {
			ctlVal.SetText(ctl)
			kitVal.SetText(kit)
		})
	}()
	return card
}

// runVersion runs a command with a short timeout and returns trimmed stdout.
func runVersion(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// daemonVersion parses "z13ctl-plus version X" from the installed daemon binary.
func daemonVersion() string {
	if f := strings.Fields(runVersion("z13ctl-plus", "--version")); len(f) > 0 {
		return f[len(f)-1]
	}
	if _, err := exec.LookPath("z13ctl-plus"); err == nil {
		return "unknown"
	}
	return "not installed"
}

// tabletKitVersion asks the package manager; the kit itself has no --version.
// A from-source install.sh setup has no version, so it reports "installed".
func tabletKitVersion() string {
	const pkg = "z13-tablet-kit"
	if f := strings.Fields(runVersion("pacman", "-Q", pkg)); len(f) == 2 {
		return f[1]
	}
	if v := runVersion("dpkg-query", "-W", "-f=${Version}", pkg); v != "" {
		return v
	}
	if v := runVersion("rpm", "-q", "--qf", "%{VERSION}-%{RELEASE}", pkg); v != "" && !strings.Contains(v, " ") {
		return v
	}
	if home, err := os.UserHomeDir(); err == nil {
		if _, err := os.Stat(filepath.Join(home, ".local", "bin", "z13-tablet-switch")); err == nil {
			return "installed"
		}
	}
	return "not installed"
}
