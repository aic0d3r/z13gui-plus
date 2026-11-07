package gui

// dim.go — Settings > Display "Dim": darkens the built-in panel below the
// backlight floor.
//
// The firmware clamps the amdgpu backlight to a minimum, and KWin's own
// brightness and dimming only drive that backlight when the panel has one
// (DrmOutput::updateBrightness), so neither can go lower. What KWin does apply
// to the pixels is the vcgt calibration curve of an output's ICC profile. Dim
// generates an sRGB profile whose curve peaks at the chosen level and sets it
// on the eDP output with kscreen-doctor; turning Dim off restores the colour
// profile settings that were active before.
//
// KDE only: the row stays hidden without kscreen-doctor, an eDP output, or
// ICC support, and in gamescope; users can also hide it in Settings > Layout
// (useful once a kernel backlight quirk already lowers the floor). The on/off state is never kept: profiles live
// in the runtime dir and startup undoes a Dim profile left by a previous
// session, so a too-dark screen is always fixed by logging in again. Only the
// level is persisted (config.toml dim_level).

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aic0d3r/z13gui-plus/internal/theme"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

const (
	dimFloor        = 20 // darkest allowed level; lower is easy to black out by accident
	dimCeil         = 90
	dimDefaultLevel = 50
	dimFilePrefix   = "dim-" // profile names: <runtime>/z13gui-plus/dim-<pct>.icc
)

var (
	ansiRe       = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	kscreenOutRe = regexp.MustCompile(`^Output:\s+\d+\s+(\S+)`)
)

// colorProfile is the SDR colour profile setting of one output.
type colorProfile struct {
	source string // sRGB, ICC or EDID
	icc    string // ICC path; "" = none
}

// internalOutput finds the first eDP output in `kscreen-doctor -o` text and
// its colour profile. ok is false when there is no eDP output or KWin does
// not report an ICC setting for it.
func internalOutput(text string) (name string, prof colorProfile, ok bool) {
	var hasICC bool
	for _, line := range strings.Split(ansiRe.ReplaceAllString(text, ""), "\n") {
		if m := kscreenOutRe.FindStringSubmatch(line); m != nil {
			if name != "" {
				break // past the eDP block
			}
			if strings.HasPrefix(m[1], "eDP") {
				name = m[1]
			}
			continue
		}
		if name == "" {
			continue
		}
		k, v, found := strings.Cut(strings.TrimSpace(line), ":")
		if !found {
			continue
		}
		v = strings.TrimSpace(v)
		switch k {
		case "ICC profile":
			hasICC = true
			if v != "none" {
				prof.icc = v
			}
		case "Color profile source":
			prof.source = v
		}
	}
	return name, prof, name != "" && hasICC && prof.source != ""
}

// isDimProfile reports whether an ICC path is one Dim generated.
func isDimProfile(path string) bool {
	return strings.HasPrefix(filepath.Base(path), dimFilePrefix) &&
		filepath.Base(filepath.Dir(path)) == "z13gui-plus"
}

func clampDimLevel(v int) int {
	if v == 0 {
		return dimDefaultLevel
	}
	return (min(max(v, dimFloor), dimCeil) + 2) / 5 * 5
}

type vcgtHeader struct {
	Type                    uint32
	Channels, Entries, Size uint16
}

// dimICC builds a minimal ICC v2 sRGB display profile (D50 primaries, gamma
// 2.2) whose vcgt curve scales every channel to scale (0–1).
func dimICC(scale float64) []byte {
	put := func(b *bytes.Buffer, v any) { _ = binary.Write(b, binary.BigEndian, v) }
	s15 := func(b *bytes.Buffer, v float64) { put(b, int32(math.Round(v*65536))) }
	tag := func(sig string, fill func(*bytes.Buffer)) []byte {
		var b bytes.Buffer
		b.WriteString(sig)
		b.Write(make([]byte, 4))
		fill(&b)
		return b.Bytes()
	}
	xyz := func(x, y, z float64) []byte {
		return tag("XYZ ", func(b *bytes.Buffer) { s15(b, x); s15(b, y); s15(b, z) })
	}
	gamma := tag("curv", func(b *bytes.Buffer) { put(b, uint32(1)); put(b, [2]uint16{563, 0}) }) // u8Fixed8 2.2 + pad
	desc := tag("desc", func(b *bytes.Buffer) {
		text := "Z13GUI+ Dim\x00"
		put(b, uint32(len(text)))
		b.WriteString(text)
		b.Write(make([]byte, 4+4+2+1+67)) // empty Unicode and ScriptCode parts
	})
	vcgt := tag("vcgt", func(b *bytes.Buffer) {
		const n = 256
		put(b, vcgtHeader{0, 3, n, 2})
		for range 3 {
			for i := range n {
				put(b, uint16(math.Round(float64(i)/(n-1)*scale*65535)))
			}
		}
	})
	tags := []struct {
		sig  string
		data []byte
	}{
		{"desc", desc}, {"cprt", tag("text", func(b *bytes.Buffer) { b.WriteString("none\x00") })},
		{"wtpt", xyz(0.9642, 1, 0.8249)},
		{"rXYZ", xyz(0.4361, 0.2225, 0.0139)}, {"gXYZ", xyz(0.3851, 0.7169, 0.0971)}, {"bXYZ", xyz(0.1431, 0.0606, 0.7141)},
		{"rTRC", gamma}, {"gTRC", gamma}, {"bTRC", gamma},
		{"vcgt", vcgt},
	}

	var table, data bytes.Buffer
	base := 128 + 4 + 12*len(tags)
	for _, t := range tags {
		for (base+data.Len())%4 != 0 {
			data.WriteByte(0)
		}
		table.WriteString(t.sig)
		put(&table, [2]uint32{uint32(base + data.Len()), uint32(len(t.data))})
		data.Write(t.data)
	}

	var out bytes.Buffer
	put(&out, uint32(base+data.Len()))
	out.WriteString("lcms")
	put(&out, uint32(0x02100000))
	out.WriteString("mntrRGB XYZ ")
	out.Write(make([]byte, 12)) // date
	out.WriteString("acspAPPL")
	out.Write(make([]byte, 4+4+4+8+4)) // flags, manufacturer, model, attributes, intent
	s15(&out, 0.9642)
	s15(&out, 1)
	s15(&out, 0.8249)
	out.Write(make([]byte, 4+16+28)) // creator, profile ID, reserved
	put(&out, uint32(len(tags)))
	out.Write(table.Bytes())
	out.Write(data.Bytes())
	return out.Bytes()
}

func dimRuntimeDir() string {
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		runtime = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	return filepath.Join(runtime, "z13gui-plus")
}

// dimRestorePath holds the profile setting to put back when Dim turns off. It
// lives in the config dir so it survives the reboot that clears the profiles.
func dimRestorePath() string { return filepath.Join(theme.ConfigDir(), "dim-restore") }

func saveDimRestore(p colorProfile) error {
	if err := os.MkdirAll(theme.ConfigDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dimRestorePath(), []byte(p.source+"\n"+p.icc+"\n"), 0o644)
}

// loadDimRestore falls back to Plasma's default (sRGB, no ICC) when the record
// is missing.
func loadDimRestore() colorProfile {
	data, err := os.ReadFile(dimRestorePath())
	if err != nil {
		return colorProfile{source: "sRGB"}
	}
	source, icc, _ := strings.Cut(strings.TrimRight(string(data), "\n"), "\n")
	if source == "" {
		source = "sRGB"
	}
	return colorProfile{source: source, icc: icc}
}

func runKscreen(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "kscreen-doctor", args...).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), err
}

// dimmer is owned by the worker goroutine started in startDimmer.
type dimmer struct {
	output string
	active bool // a Dim profile is set on the output
}

func (d *dimmer) setProfile(p colorProfile) error {
	_, err := runKscreen(
		fmt.Sprintf("output.%s.colorProfileSource.%s", d.output, p.source),
		fmt.Sprintf("output.%s.iccprofile.%s", d.output, p.icc),
	)
	return err
}

// restore puts back the pre-Dim profile setting and deletes the Dim profiles.
func (d *dimmer) restore() error {
	if err := d.setProfile(loadDimRestore()); err != nil {
		return err
	}
	d.active = false
	_ = os.Remove(dimRestorePath())
	old, _ := filepath.Glob(filepath.Join(dimRuntimeDir(), dimFilePrefix+"*.icc"))
	for _, f := range old {
		_ = os.Remove(f)
	}
	return nil
}

func (d *dimmer) apply(pct int) error {
	if pct >= 100 {
		if !d.active {
			return nil
		}
		return d.restore()
	}
	if !d.active {
		// Record what to restore, read fresh in case it changed since startup.
		out, err := runKscreen("-o")
		if err != nil {
			return err
		}
		_, prof, ok := internalOutput(out)
		if !ok {
			return errors.New("built-in output not found")
		}
		if !isDimProfile(prof.icc) {
			if err := saveDimRestore(prof); err != nil {
				return err
			}
		}
	}
	dir := dimRuntimeDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// One file per level: a new path makes KWin reload instead of reusing a cached profile.
	path := filepath.Join(dir, fmt.Sprintf("%s%d.icc", dimFilePrefix, pct))
	if err := os.WriteFile(path, dimICC(float64(pct)/100), 0o644); err != nil {
		return err
	}
	if err := d.setProfile(colorProfile{source: "ICC", icc: path}); err != nil {
		return err
	}
	d.active = true
	return nil
}

// startDimmer detects support off the main thread, undoes a Dim profile left
// by an earlier session, and then applies requested levels in order. Only the
// newest pending level is kept, so a slider drag never queues a backlog.
func (w *Window) startDimmer() {
	if w.gamescope {
		return
	}
	w.dimReq = make(chan int, 1)
	w.dimDone = make(chan struct{}, 1)
	go func() {
		out, err := runKscreen("-o")
		name, prof, ok := internalOutput(out)
		if err != nil || !ok {
			slog.Info("dim unavailable", "err", err, "output", name)
			return
		}
		d := &dimmer{output: name, active: isDimProfile(prof.icc)}
		if d.active {
			if err := d.restore(); err != nil {
				slog.Warn("dim reset failed", "err", err)
				return
			}
			slog.Info("dim reset from previous session")
		}
		glib.TimeoutAdd(0, func() bool {
			w.dimSupported = true
			w.applyLayout()
			w.buildMainFocusList()
			return false
		})
		glib.MainContextDefault().Wakeup()
		for pct := range w.dimReq {
			if err := d.apply(pct); err != nil {
				slog.Warn("dim failed", "pct", pct, "err", err)
			}
			if pct >= 100 {
				select {
				case w.dimDone <- struct{}{}:
				default:
				}
			}
		}
	}()
}

// Shutdown undoes Dim and waits briefly for the restore. Safe from any
// goroutine; call it before exiting so a stopped service never leaves the
// screen on a Dim profile.
func (w *Window) Shutdown() {
	if w.dimReq == nil {
		return
	}
	select {
	case <-w.dimDone: // stale ack
	default:
	}
	w.requestDim(100)
	select {
	case <-w.dimDone:
	case <-time.After(4 * time.Second):
	}
}

// requestDim queues a dim level (100 = off). Main thread only.
func (w *Window) requestDim(pct int) {
	if w.dimReq == nil {
		return
	}
	select {
	case <-w.dimReq: // drop the stale pending level
	default:
	}
	w.dimReq <- pct
}

// buildDimRow is the Settings > Display block: the Dim switch and its level
// slider. Visibility is owned by applyLayout (card "set.dim").
func (w *Window) buildDimRow() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 2)
	// The caption lives directly under the toggle (buildToggleWithCaption pattern):
	// tooltip copy must be visible without a pointer on the touch surface.
	box.Append(w.buildToggleWithCaption("Dim", "Darkens the screen below the lowest brightness. Lower is darker. Turns off at login.", &w.dimSwitch, func(on bool) {
		if on {
			w.requestDim(int(w.dimScale.Value()))
		} else {
			w.requestDim(100)
		}
	}))
	// Hiding the row in Settings > Layout must not leave the screen dimmed with no
	// way to undo it. applyLayout runs with syncing set, so call requestDim directly.
	box.Connect("notify::visible", func() {
		if !box.Visible() && w.dimSwitch.Active() {
			w.dimSwitch.SetActive(false)
			w.requestDim(100)
		}
	})

	// One header line: name left, live value right — matches the tuning sliders.
	// GTK's drawn value is off: it floats above the slider's start and collides
	// with the section label at this width.
	name := gtk.NewLabel("DIM LEVEL")
	name.SetHAlign(gtk.AlignStart)
	name.SetHExpand(true)
	name.AddCSSClass("scale-name")
	w.dimValue = gtk.NewLabel(fmt.Sprintf("%d%%", clampDimLevel(theme.LoadAppConfig().DimLevel)))
	w.dimValue.SetHAlign(gtk.AlignEnd)
	w.dimValue.AddCSSClass("scale-value")
	header := gtk.NewBox(gtk.OrientationHorizontal, 4)
	header.SetMarginTop(6)
	header.Append(name)
	header.Append(w.dimValue)
	box.Append(header)

	sc := gtk.NewScaleWithRange(gtk.OrientationHorizontal, dimFloor, dimCeil, 5)
	sc.SetValue(float64(clampDimLevel(theme.LoadAppConfig().DimLevel)))
	sc.SetDigits(0)
	sc.SetRoundDigits(0)
	sc.SetDrawValue(false)
	sc.SetHExpand(true)
	sc.SetFocusable(true)
	sc.SetTooltipText("Screen brightness while Dim is on, 20% to 90%")
	name.SetMnemonicWidget(sc)
	saveGen := 0
	sc.ConnectValueChanged(func() {
		// Drags ignore the step increment; snap so the level is always a 5% step.
		v := (int(sc.Value()+2.5) / 5) * 5
		if float64(v) != sc.Value() {
			sc.SetValue(float64(v)) // re-enters with the snapped value
			return
		}
		w.dimValue.SetText(fmt.Sprintf("%d%%", v))
		if w.dimSwitch.Active() {
			w.requestDim(v)
		}
		// Save once the drag settles instead of rewriting config.toml per step.
		saveGen++
		gen := saveGen
		glib.TimeoutAdd(500, func() bool {
			if gen == saveGen {
				cfg := theme.LoadAppConfig()
				cfg.DimLevel = v
				theme.SaveAppConfig(cfg)
			}
			return false
		})
	})
	w.dimScale = sc
	box.Append(sc)

	w.dimRow = box
	return box
}
