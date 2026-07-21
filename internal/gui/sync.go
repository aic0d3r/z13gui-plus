package gui

// sync.go — daemon state synchronization and API communication.

import (
	"fmt"
	"log/slog"
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
	w.syncFanPreset()
	w.syncRefreshRate()
	w.syncOverdrive()
	w.syncBootSound()
	w.syncPresets()
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
	if w.cpuPowerBox == nil || w.state == nil {
		return
	}
	state := w.state.CPUPower
	if state == nil || w.state.Profile != "custom" {
		w.cpuPowerBox.SetVisible(false)
		if state == nil {
			return
		}
	} else {
		w.cpuPowerBox.SetVisible(true)
	}
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
	if _, ok := w.modeButtons[mode]; !ok {
		mode = defaultMode
	}
	setActiveButton(w.modeButtons, mode)
	if w.lightingSwitch != nil {
		w.lightingSwitch.SetActive(ls.Enabled)
	}
	if w.rgbControlsBox != nil {
		w.rgbControlsBox.SetSensitive(ls.Enabled)
	}
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
	if w.cpuPowerBox != nil {
		w.cpuPowerBox.SetVisible(w.state.Profile == "custom" && w.state.CPUPower != nil)
	}
}

// syncBattery sets the battery limit scale to match the daemon state.
// Also highlights the matching preset button (if any).
func (w *Window) syncBattery() {
	if w.state == nil {
		return
	}
	setActiveIntButton(w.battPresetBtns, w.state.Battery)
}

func (w *Window) syncFanPreset() {
	setActiveButton(w.fanPresetBtns, matchingFanPreset(w.state.FanCurve))
}

func matchingFanPreset(curve *api.FanCurveState) string {
	if curve == nil || curve.Mode != 1 {
		return ""
	}
	parts := make([]string, len(curve.Points))
	for i, point := range curve.Points {
		parts[i] = fmt.Sprintf("%d:%d", point.Temp, point.PWM)
	}
	encoded := strings.Join(parts, ",")
	for _, name := range fanPresets {
		if encoded == fanPresetPoints(name) {
			return name
		}
	}
	return ""
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
		w.cpuTempGauge.SetHot(t.CPUTemp >= thermalWarningC)
	}
	if w.gpuTempGauge != nil && t.GPUTemp > 0 {
		w.gpuTempGauge.SetValue(float64(t.GPUTemp))
		w.gpuTempGauge.SetHot(t.GPUTemp >= thermalWarningC)
	}
	if w.cpuUtilGauge != nil {
		w.cpuUtilGauge.SetValue(float64(t.CPUUtil))
	}
	if w.gpuUtilGauge != nil {
		w.gpuUtilGauge.SetValue(float64(t.GPUUtil))
	}
	if w.overviewSpark != nil && t.CPUTemp > 0 {
		w.overviewSpark.SetHot(t.CPUTemp >= thermalWarningC)
		w.overviewSpark.Push(float64(t.CPUTemp))
	}
	if w.overviewStatus != nil {
		status, class := "NORMAL", "success"
		maxTemp := max(t.CPUTemp, t.GPUTemp)
		memoryPressure := 0.0
		if t.MemoryTotalMB > 0 {
			memoryPressure = float64(t.MemoryUsedMB) / float64(t.MemoryTotalMB)
		}
		if maxTemp >= thermalCriticalC || memoryPressure >= memoryCritical {
			status, class = "CRITICAL", "danger"
		} else if maxTemp >= thermalWarningC {
			status, class = "THERMAL WARM", "warning"
		} else if memoryPressure >= memoryWarning {
			status, class = "MEMORY HIGH", "warning"
		}
		w.overviewStatus.SetLabel(status)
		w.overviewStatus.RemoveCSSClass("success")
		w.overviewStatus.RemoveCSSClass("warning")
		w.overviewStatus.RemoveCSSClass("danger")
		w.overviewStatus.AddCSSClass(class)
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
		w.npuLabel.SetLabel(formatNPU(t.NPUUtil, t.NPUPowerW))
		w.npuLabel.RemoveCSSClass("npu-high")
		w.npuLabel.RemoveCSSClass("npu-dim")
		if t.NPUPowerW < npuActivePowerW {
			w.npuLabel.AddCSSClass("npu-dim")
		} else if t.NPUPowerW >= npuActivePowerW {
			w.npuLabel.AddCSSClass("npu-high")
		}
	}
	if w.overviewMemClock != nil && t.MemClockMTs > 0 {
		w.overviewMemClock.SetLabel(fmt.Sprintf("%d MT/s", t.MemClockMTs))
	}
	memoryUsed, memoryTotal := unifiedMemory(t.MemoryUsedMB, t.MemoryTotalMB, t.VRAMUsedMB, t.VRAMTotalMB)
	if w.overviewMemoryLbl != nil {
		w.overviewMemoryLbl.SetLabel(formatMemory(memoryUsed, memoryTotal))
	}
	if w.overviewMemoryBar != nil && memoryTotal > 0 {
		w.overviewMemoryBar.SetFraction(float64(memoryUsed) / float64(memoryTotal))
	}
}

// updateBatteryHero updates the System tab battery card from live telemetry.
// Called from syncState (initial) and from the 1s telemetry poll.
func (w *Window) updateBatteryHero(b *api.BatteryState) {
	if w.battCapacityGauge != nil {
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
func (w *Window) setLightingEnabled(enabled bool) {
	if w.syncing {
		return
	}
	if w.rgbControlsBox != nil {
		w.rgbControlsBox.SetSensitive(enabled)
	}
	if enabled {
		setActiveButton(w.modeButtons, activeButton(w.modeButtons, defaultMode))
		w.syncModeVis()
		w.sendApply()
		return
	}
	if _, err := api.SendOff(w.tab); err != nil {
		slog.Warn("off failed", "device", w.tab, "err", err)
	}
}

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
