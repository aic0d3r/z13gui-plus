package gui

// versions.go — "ABOUT" card on the System tab: versions of the three Z13 repos.

import (
	"context"
	"fmt"
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

const repoURL = "https://github.com/aic0d3r/z13gui-plus"

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

	// Opens the repo page; where no handler exists (Gaming Mode) it does nothing.
	star := gtk.NewButtonWithLabel("Enjoying Z13GUI+? Star it on GitHub.")
	star.ConnectClicked(func() { gtk.NewURILauncher(repoURL).Launch(context.Background(), nil, nil) })
	starBox := gtk.NewBox(gtk.OrientationVertical, 0)
	starBox.AddCSSClass("btn-group")
	starBox.Append(star)
	card.Append(starBox)
	w.starBtn = star

	go func() {
		ctl, kit := daemonVersion(), tabletKitVersion()
		glib.IdleAdd(func() {
			ctlVal.SetText(ctl)
			kitVal.SetText(kit)
		})
	}()
	return card
}

// runCmd runs a command with a timeout and returns its trimmed stdout. On
// failure the error carries the command's stderr.
func runCmd(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if ee, ok := err.(*exec.ExitError); ok {
		err = fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
	}
	return strings.TrimSpace(string(out)), err
}

// runVersion runs a version query, returning "" on any failure.
func runVersion(name string, args ...string) string {
	out, err := runCmd(2*time.Second, name, args...)
	if err != nil {
		return ""
	}
	return out
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
