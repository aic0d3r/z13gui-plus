package gui

// sync.go — daemon state synchronization and API communication.

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/dahui/z13ctl/api"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// Defaults used when widget state is unavailable (e.g. before first sync).
const (
	defaultColor1     = "FF0000"
	defaultColor2     = "000000"
	defaultMode       = "static"
	defaultSpeed      = "normal"
	defaultBrightness = 3
)

// modeVis defines which subsections are visible for a given lighting mode.
type modeVis struct{ color1, color2, speed, brightness bool }

// modeVisMap maps lighting mode names to their subsection visibility.
var modeVisMap = map[string]modeVis{
	"static":  {true, false, false, true},
	"breathe": {true, true, true, true},
	"cycle":   {false, false, true, true},
	"rainbow": {false, false, true, true},
	"strobe":  {true, false, true, true},
	"off":     {false, false, false, false},
}

// activeButton returns the key of the button with the .active CSS class,
// or the fallback value if none is found.
func activeButton(btns map[string]*gtk.Button, fallback string) string {
	for k, b := range btns {
		if b.HasCSSClass("active") {
			return k
		}
	}
	return fallback
}

// syncModeVis shows/hides color and speed sections based on the active mode.
// Safe to call at any time (including during sync).
func (w *Window) syncModeVis() {
	mode := activeButton(w.modeButtons, "static")
	v, ok := modeVisMap[mode]
	if !ok {
		v = modeVis{true, true, true, true}
	}
	if w.color1Box != nil {
		w.color1Box.SetVisible(v.color1)
	}
	if w.color2Box != nil {
		w.color2Box.SetVisible(v.color2)
	}
	if w.speedBox != nil {
		w.speedBox.SetVisible(v.speed)
	}
	if w.brightBox != nil {
		w.brightBox.SetVisible(v.brightness)
	}
}

// syncState updates all widgets from the current daemon state.
// Sets syncing=true to suppress signal handlers from firing sendApply.
func (w *Window) syncState() {
	if w.state == nil {
		return
	}
	w.syncing = true
	defer func() { w.syncing = false }()
	w.syncLightingSection()
	w.syncProfile()
	w.syncCPUPower()
	w.syncBattery()
	w.syncRefreshRate()
	w.syncOverdrive()
	w.syncBootSound()
	w.syncNPUPower()
	w.syncCustomView()
	// Battery hero card — also updates on sync (initial show) so the card
	// isn't stale before the first telemetry tick (1 s later).
	if w.state.BatteryDetail != nil {
		w.updateBatteryHero(w.state.BatteryDetail)
	}
	// Overview tab — populate from initial state so it isn't blank for 1 s.
	w.syncOverviewTelemetry()
}

func (w *Window) syncCPUPower() {
	if w.cpuPowerBox == nil {
		return
	}
	state := w.state.CPUPower
	if state == nil {
		w.cpuPowerBox.SetVisible(false)
		return
	}
	w.cpuPowerBox.SetVisible(true)
	w.cpuMinScale.SetRange(float64(state.MinLimitKHz)/1000, float64(state.MaxLimitKHz)/1000)
	w.cpuMinScale.SetValue(float64(state.MinFrequencyKHz) / 1000)
	setActiveButton(w.cpuEPPBtns, state.EPP)
	for value, btn := range w.cpuEPPBtns {
		available := false
		for _, choice := range state.EPPChoices {
			if choice == value {
				available = true
				break
			}
		}
		btn.SetSensitive(available)
	}
	w.cpuBoostSwitch.SetActive(state.Boost)
}

func (w *Window) initCPUMinDebounce(sc *gtk.Scale) {
	var debounce *time.Timer
	sc.ConnectValueChanged(func() {
		if w.syncing {
			return
		}
		if debounce != nil {
			debounce.Stop()
		}
		debounce = time.AfterFunc(200*time.Millisecond, func() {
			glib.IdleAdd(func() bool {
				khz := int(sc.Value()) * 1000
				if _, err := api.SendCPUMinFrequencySet(khz); err != nil {
					slog.Warn("CPU minimum frequency set failed", "khz", khz, "err", err)
				}
				return false
			})
		})
	})
}

func (w *Window) sendCPUEPPSet(value string) {
	if _, err := api.SendCPUEPPSet(value); err != nil {
		slog.Warn("CPU EPP set failed", "value", value, "err", err)
	}
}

func (w *Window) sendCPUBoostSet(enabled bool) {
	if _, err := api.SendCPUBoostSet(enabled); err != nil {
		slog.Warn("CPU boost set failed", "enabled", enabled, "err", err)
	}
}

// syncLightingSection updates mode, colors, speed, and brightness from the
// daemon state for the active device tab.
func (w *Window) syncLightingSection() {
	prev := w.syncing
	w.syncing = true
	defer func() { w.syncing = prev }()

	var ls api.LightingState
	if w.state != nil {
		if dev, ok := w.state.Devices[w.tab]; ok {
			ls = dev
		} else {
			ls = w.state.Lighting
		}
	}
	mode := ls.Mode
	if !ls.Enabled {
		mode = "off"
	}
	setActiveButton(w.modeButtons, mode)
	if w.color1 != nil && ls.Color != "" {
		w.color1.hex = strings.ToUpper(ls.Color)
	}
	if w.color2 != nil && ls.Color2 != "" {
		w.color2.hex = strings.ToUpper(ls.Color2)
	}
	w.updateSwatches()
	setActiveButton(w.speedBtns, ls.Speed)
	if w.brightScale != nil {
		w.brightScale.SetValue(float64(ls.Brightness))
	}
	w.syncModeVis()
}

// syncProfile highlights the profile button matching the daemon state.
func (w *Window) syncProfile() {
	if w.state == nil || w.state.Profile == "" {
		return
	}
	setActiveButton(w.profileBtns, w.state.Profile)
}

// syncBattery sets the battery limit scale to match the daemon state.
// Also highlights the matching preset button (if any).
func (w *Window) syncBattery() {
	if w.state == nil || w.battScale == nil {
		return
	}
	if w.state.Battery != 0 {
		w.battScale.SetValue(float64(w.state.Battery))
	}
	setActiveIntButton(w.battPresetBtns, w.state.Battery)
}

// syncRefreshRate highlights the refresh rate button matching the daemon state.
func (w *Window) syncRefreshRate() {
	if w.state == nil {
		return
	}
	setActiveIntButton(w.refreshBtns, w.state.RefreshRate)
}

// syncOverviewTelemetry populates Overview tab widgets from the live Telemetry
// sub-state. Called from syncState (initial) and from the 1 s telemetry poll.
// Gracefully no-ops on daemons that don't populate Telemetry (legacy); gauges
// simply show their last-known value and text rows keep their "—" placeholder.
func (w *Window) syncOverviewTelemetry() {
	t := w.state.Telemetry
	if t == nil {
		return
	}
	if w.cpuTempGauge != nil && t.CPUTemp > 0 {
		w.cpuTempGauge.SetValue(float64(t.CPUTemp))
	}
	if w.gpuTempGauge != nil && t.GPUTemp > 0 {
		w.gpuTempGauge.SetValue(float64(t.GPUTemp))
	}
	if w.cpuUtilGauge != nil {
		w.cpuUtilGauge.SetValue(float64(t.CPUUtil))
	}
	if w.gpuUtilGauge != nil {
		w.gpuUtilGauge.SetValue(float64(t.GPUUtil))
	}
	if w.overviewSpark != nil && t.CPUTemp > 0 {
		w.overviewSpark.Push(float64(t.CPUTemp))
	}
	if w.cpuFanLabel != nil && t.CPUFanRPM > 0 {
		w.cpuFanLabel.SetLabel(fmt.Sprintf("%d RPM", t.CPUFanRPM))
	}
	if w.gpuFanLabel != nil && t.GPUFanRPM > 0 {
		w.gpuFanLabel.SetLabel(fmt.Sprintf("%d RPM", t.GPUFanRPM))
	}
	if w.overviewCPUClock != nil {
		w.overviewCPUClock.SetLabel(formatGHz(t.CPUClockMHz))
	}
	if w.overviewGPUClock != nil {
		w.overviewGPUClock.SetLabel(formatGHz(t.GPUClockMHz))
	}
	if w.npuLabel != nil {
		w.npuLabel.SetLabel(formatNPU(t.NPUUtil, t.NPUMaxClockMHz, t.NPUPowerW))
	}
	if w.overviewMemClock != nil && t.MemClockMTs > 0 {
		w.overviewMemClock.SetLabel(fmt.Sprintf("%d MT/s", t.MemClockMTs))
	}
	if w.overviewVRAMLbl != nil {
		w.overviewVRAMLbl.SetLabel(formatVRAM(t.VRAMUsedMB, t.VRAMTotalMB))
	}
	if w.overviewVRAMBar != nil && t.VRAMTotalMB > 0 {
		w.overviewVRAMBar.SetFraction(float64(t.VRAMUsedMB) / float64(t.VRAMTotalMB))
	}
}

// updateBatteryHero updates the System tab battery card from live telemetry.
// Called from syncState (initial) and from the 1s telemetry poll.
func (w *Window) updateBatteryHero(b *api.BatteryState) {	if w.battCapacityGauge != nil {
		w.battCapacityGauge.SetValue(float64(b.CapacityPct))
	}
	if w.battStatusLabel != nil {
		w.battStatusLabel.SetLabel(b.Status)
	}
	if w.battHealthLabel != nil {
		// Health row includes current/last-full Wh and design Wh so the user
		// sees the underlying numbers, not just the rounded %.
		w.battHealthLabel.SetLabel(fmt.Sprintf("Health %d%%  ·  %d / %d Wh", b.HealthPct, b.WhCurrent, b.WhDesign))
	}
	if w.battPowerLabel != nil {
		// Charging shows "+W", discharging shows "W draw", idle shows voltage only.
		if b.Charging {
			w.battPowerLabel.SetLabel(fmt.Sprintf("+%s W  ·  %s V", b.PowerWatts, b.VoltageVolts))
		} else if b.PowerWatts != "0" {
			w.battPowerLabel.SetLabel(fmt.Sprintf("-%s W  ·  %s V", b.PowerWatts, b.VoltageVolts))
		} else {
			w.battPowerLabel.SetLabel(fmt.Sprintf("%s V", b.VoltageVolts))
		}
	}
	if w.battPill != nil {
		// Threshold preset chip: 100=Standard, 80=Balanced, <80=Max Life.
		w.battPill.RemoveCSSClass("success")
		w.battPill.RemoveCSSClass("warning")
		w.battPill.RemoveCSSClass("accent")
		var label, cls string
		switch {
		case b.ThresholdPct >= 100:
			label, cls = "STANDARD", "accent"
		case b.ThresholdPct >= 80:
			label, cls = fmt.Sprintf("BALANCED · %d%%", b.ThresholdPct), "warning"
		default:
			label, cls = fmt.Sprintf("MAX LIFE · %d%%", b.ThresholdPct), "success"
		}
		w.battPill.SetLabel(label)
		if cls != "" {
			w.battPill.AddCSSClass(cls)
		}
	}
}

// queueApply debounces rapid API calls from continuous inputs (color wheel,
// sliders). Discrete inputs (mode buttons, speed buttons, preset clicks)
// call sendApply directly.
func (w *Window) queueApply() {
	if w.syncing {
		return
	}
	if w.applyTimer != nil {
		w.applyTimer.Stop()
	}
	w.applyTimer = time.AfterFunc(150*time.Millisecond, func() {
		glib.IdleAdd(func() bool {
			w.sendApply()
			return false
		})
	})
}

// sendApply sends the current lighting state to the daemon. Guarded by
// w.syncing to prevent sending defaults during widget initialization.
func (w *Window) sendApply() {
	if w.syncing {
		return
	}
	color1 := defaultColor1
	if w.color1 != nil {
		color1 = w.color1.hex
	}
	color2 := defaultColor2
	if w.color2 != nil {
		color2 = w.color2.hex
	}

	mode := activeButton(w.modeButtons, defaultMode)
	speed := activeButton(w.speedBtns, defaultSpeed)

	brightness := defaultBrightness
	if w.brightScale != nil {
		brightness = int(w.brightScale.Value())
	}

	// "off" uses the daemon's dedicated off command so that Enabled=false
	// is persisted and survives a reboot.
	if mode == "off" {
		slog.Debug("sendApply: calling daemon off", "device", w.tab)
		start := time.Now()
		if _, err := api.SendOff(w.tab); err != nil {
			slog.Warn("off failed", "err", err, "elapsed", time.Since(start))
		} else {
			slog.Debug("sendApply: off done", "elapsed", time.Since(start))
		}
		return
	}

	slog.Debug("sendApply: calling daemon", "device", w.tab, "mode", mode, "brightness", brightness)
	start := time.Now()
	if _, err := api.SendApply(w.tab, color1, color2, mode, speed, brightness); err != nil {
		slog.Warn("apply failed", "err", err, "elapsed", time.Since(start))
	} else {
		slog.Debug("sendApply: done", "elapsed", time.Since(start))
	}
}

// sendProfileSet sends a profile change to the daemon.
func (w *Window) sendProfileSet(prof string) {
	slog.Debug("sendProfileSet: calling daemon", "profile", prof)
	start := time.Now()
	if _, err := api.SendProfileSet(prof); err != nil {
		slog.Warn("profile set failed", "profile", prof, "err", err, "elapsed", time.Since(start))
	} else {
		slog.Debug("sendProfileSet: done", "elapsed", time.Since(start))
	}
}

// initBatteryDebounce sets up debounced battery limit changes on the given scale.
func (w *Window) initBatteryDebounce(sc *gtk.Scale) {
	var debounce *time.Timer
	sc.ConnectValueChanged(func() {
		if debounce != nil {
			debounce.Stop()
		}
		debounce = time.AfterFunc(200*time.Millisecond, func() {
			glib.IdleAdd(func() bool {
				val := int(sc.Value())
				slog.Debug("sendBatteryLimitSet: calling daemon", "limit", val)
				start := time.Now()
				if _, err := api.SendBatteryLimitSet(val); err != nil {
					slog.Warn("battery limit set failed", "err", err, "elapsed", time.Since(start))
				} else {
					slog.Debug("sendBatteryLimitSet: done", "elapsed", time.Since(start))
				}
				return false
			})
		})
	})
}

// syncOverdrive sets the overdrive switch to match the daemon state.
func (w *Window) syncOverdrive() {
	if w.state == nil || w.overdriveSwitch == nil {
		return
	}
	w.overdriveSwitch.SetActive(w.state.PanelOverdrive != 0)
}

// syncBootSound sets the boot sound switch to match the daemon state.
func (w *Window) syncBootSound() {
	if w.state == nil || w.bootSoundSwitch == nil {
		return
	}
	w.bootSoundSwitch.SetActive(w.state.BootSound != 0)
}

// sendOverdriveSet sends a panel overdrive change to the daemon.
func (w *Window) sendOverdriveSet(value int) {
	slog.Debug("sendOverdriveSet: calling daemon", "value", value)
	start := time.Now()
	if _, err := api.SendPanelOverdriveSet(value); err != nil {
		slog.Warn("panel overdrive set failed", "value", value, "err", err, "elapsed", time.Since(start))
	} else {
		slog.Debug("sendOverdriveSet: done", "elapsed", time.Since(start))
	}
}

// sendBootSoundSet sends a boot sound change to the daemon.
func (w *Window) sendBootSoundSet(value int) {
	slog.Debug("sendBootSoundSet: calling daemon", "value", value)
	start := time.Now()
	if _, err := api.SendBootSoundSet(value); err != nil {
		slog.Warn("boot sound set failed", "value", value, "err", err, "elapsed", time.Since(start))
	} else {
		slog.Debug("sendBootSoundSet: done", "elapsed", time.Since(start))
	}
}

// sendRefreshRateSet switches eDP-1 to the requested refresh rate.
func (w *Window) sendRefreshRateSet(hz int) {
	slog.Debug("sendRefreshRateSet: calling daemon", "hz", hz)
	start := time.Now()
	if _, err := api.SendRefreshRateSet(hz); err != nil {
		slog.Warn("refresh rate set failed", "hz", hz, "err", err, "elapsed", time.Since(start))
	} else {
		slog.Debug("sendRefreshRateSet: done", "elapsed", time.Since(start))
	}
}

// sendBatteryLimitSet sends a battery charge limit change to the daemon.
// Called by the preset buttons; the scale's value-change path uses an inline
// debounce in initBatteryDebounce.
func (w *Window) sendBatteryLimitSet(limit int) {
	slog.Debug("sendBatteryLimitSet: calling daemon", "limit", limit)
	start := time.Now()
	if _, err := api.SendBatteryLimitSet(limit); err != nil {
		slog.Warn("battery limit set failed", "limit", limit, "err", err, "elapsed", time.Since(start))
	} else {
		slog.Debug("sendBatteryLimitSet: done", "elapsed", time.Since(start))
	}
}

// sendFanPreset encodes the named preset curve as "temp:pwm,..." and sends it
// to the daemon via the existing fancurve command — no new protocol needed.
func (w *Window) sendFanPreset(name string) {
	points := fanPresetPoints(name)
	if points == "" {
		slog.Warn("fan preset has no curve", "name", name)
		return
	}
	slog.Debug("sendFanPreset: calling daemon", "preset", name)
	start := time.Now()
	if _, err := api.SendFanCurveSet(points); err != nil {
		slog.Warn("fan preset set failed", "preset", name, "err", err, "elapsed", time.Since(start))
	} else {
		slog.Debug("sendFanPreset: done", "preset", name, "elapsed", time.Since(start))
	}
}

// sendNPUPowerMode sets the AMD XDNA NPU DPM mode (0–4).
//
// amdxdna's SET_STATE ioctl is DRM_ROOT_ONLY: it requires either a root
// daemon or CAP_SYS_ADMIN on the binary. We try the daemon path first
// (the common case — the daemon should be running as root via systemd).
//
// If the daemon path fails (no daemon, daemon not root, or daemon command
// unknown), we escalate by spawning pkexec, which shows a native polkit
// password prompt and re-runs the CLI as root. Falls back to `sudo -A`
// with SUDO_ASKPASS=kdialog if pkexec is unavailable.
//
// After escalation succeeds we re-fetch state so the active button matches
// the actual hardware mode.
func (w *Window) sendNPUPowerMode(mode int) {
	slog.Debug("sendNPUPowerMode: trying daemon", "mode", mode)
	start := time.Now()
	handled, err := api.SendNPUPowerModeSet(mode)
	if handled && err == nil {
		slog.Debug("sendNPUPowerMode: daemon ok", "mode", mode, "elapsed", time.Since(start))
		return
	}
	if err != nil {
		slog.Warn("daemon NPU set failed, escalating via pkexec", "mode", mode, "err", err)
	} else {
		slog.Warn("daemon did not handle NPU set, escalating via pkexec", "mode", mode)
	}
	w.escalateNPUPowerMode(mode)
}

// escalateNPUPowerMode runs `pkexec z13ctl npu --set <mode>` in a goroutine.
// pkexec shows a native polkit prompt; if unavailable, falls back to
// `sudo -A` with kdialog as the askpass helper. After completion (success
// or failure), re-fetches daemon state on the GTK main thread so the
// active button reflects hardware reality.
func (w *Window) escalateNPUPowerMode(mode int) {
	go func() {
		cmd := w.buildSudoCommand("z13ctl", "npu", "--set", strconv.Itoa(mode))
		slog.Info("escalating NPU power mode via GUI sudo", "mode", mode, "cmd", cmd.String())
		start := time.Now()
		out, err := cmd.CombinedOutput()
		elapsed := time.Since(start)
		if err != nil {
			slog.Error("NPU power escalation failed", "mode", mode, "err", err, "output", string(out), "elapsed", elapsed)
			return
		}
		slog.Info("NPU power mode set via sudo", "mode", mode, "elapsed", elapsed)
		// Refresh state so the active button syncs to the new mode.
		if ok, state, ferr := api.SendGetState(); ok && ferr == nil {
			glib.IdleAdd(func() bool {
				w.state = state
				w.syncing = true
				w.syncNPUPower()
				w.syncing = false
				return false
			})
		}
	}()
}

// buildSudoCommand returns a *exec.Cmd that runs the given args as root via
// whichever GUI escalation helper is available. Preference order:
//  1. pkexec — polkit-native, best KDE/GNOME integration.
//  2. sudo -A with SUDO_ASKPASS=kdialog — fallback when pkexec is missing.
//  3. sudo -A with SUDO_ASKPASS=zenity — last resort.
//  4. plain sudo (will fail without a TTY) — leaves the failure visible.
func (w *Window) buildSudoCommand(name string, args ...string) *exec.Cmd {
	if path, err := exec.LookPath("pkexec"); err == nil {
		full := append([]string{path, name}, args...)
		return exec.Command(full[0], full[1:]...)
	}
	askpass := ""
	if _, err := exec.LookPath("kdialog"); err == nil {
		askpass = "kdialog"
	} else if _, err := exec.LookPath("zenity"); err == nil {
		askpass = "zenity"
	}
	cmd := exec.Command("sudo", append([]string{"-A", name}, args...)...)
	if askpass == "kdialog" {
		cmd.Env = append(cmd.Environ(), "SUDO_ASKPASS=/usr/bin/kdialog")
	} else if askpass == "zenity" {
		cmd.Env = append(cmd.Environ(),
			"SUDO_ASKPASS=/bin/sh -c 'zenity --password --title=\"z13gui sudo\"'",
		)
	}
	return cmd
}

// syncNPUPower highlights the NPU power button matching the daemon state,
// and updates the section header suffix so the current mode is visible
// even when the section is collapsed (e.g., "NPU POWER · TURBO").
// Called from syncState and after a successful pkexec escalation.
func (w *Window) syncNPUPower() {
	if w.state == nil {
		return
	}
	setActiveIntButton(w.npuPowerBtns, w.state.NPUPowerMode)
	if w.npuHeader != nil {
		w.npuHeader.SetSuffix(strings.ToUpper(npuPowerModeName(w.state.NPUPowerMode)))
	}
}

// npuPowerModeName returns the lowercase human-readable name for a DPM mode.
// Mirrors cli.NPUPowerModeName in z13ctl so the GUI doesn't need a new API.
func npuPowerModeName(mode int) string {
	switch mode {
	case 0:
		return "default"
	case 1:
		return "low"
	case 2:
		return "medium"
	case 3:
		return "high"
	case 4:
		return "turbo"
	default:
		return fmt.Sprintf("%d", mode)
	}
}

// fanPresetPoints returns the named preset encoded as the wire-format curve
// string ("temp:pwm,..." with 8 points). Empty string for unknown presets.
// Mirrors internal/cli/fan.go FanCurvePreset() in z13ctl so the GUI does not
// need a new daemon command.
func fanPresetPoints(name string) string {
	var pts [][2]int
	switch name {
	case "silent":
		pts = [][2]int{{25, 0}, {40, 0}, {50, 13}, {60, 38}, {70, 76}, {80, 128}, {85, 178}, {90, 255}}
	case "balanced":
		pts = [][2]int{{30, 13}, {45, 25}, {55, 51}, {65, 89}, {75, 140}, {82, 191}, {88, 229}, {95, 255}}
	case "turbo":
		pts = [][2]int{{30, 76}, {45, 128}, {55, 178}, {65, 216}, {75, 242}, {80, 255}, {85, 255}, {90, 255}}
	default:
		return ""
	}
	parts := make([]string, len(pts))
	for i, p := range pts {
		parts[i] = fmt.Sprintf("%d:%d", p[0], p[1])
	}
	return strings.Join(parts, ",")
}
