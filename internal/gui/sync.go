package gui

// sync.go — daemon state synchronization and API communication.

import (
	"fmt"
	"log/slog"
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
type modeVis struct{ color1, color2, speed bool }

// modeVisMap maps lighting mode names to their subsection visibility.
var modeVisMap = map[string]modeVis{
	"static":  {true, false, false},
	"breathe": {true, true, true},
	"cycle":   {false, false, true},
	"rainbow": {false, false, true},
	"strobe":  {true, false, true},
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

func setOnOffSummary(label *gtk.Label, active bool) {
	if label == nil {
		return
	}
	if active {
		label.SetLabel("ON")
	} else {
		label.SetLabel("OFF")
	}
}

// syncModeVis shows/hides color and speed sections based on the active mode.
// Safe to call at any time (including during sync).
func (w *Window) syncModeVis() {
	mode := activeButton(w.modeButtons, "static")
	if w.rgbEffectSummary != nil {
		w.rgbEffectSummary.SetLabel(strings.ToUpper(mode))
	}
	v := modeVisMap[mode]
	if w.color1Box != nil {
		w.color1Box.SetVisible(v.color1)
	}
	if w.color2Box != nil {
		w.color2Box.SetVisible(v.color2)
	}
	if w.speedBox != nil {
		w.speedBox.SetVisible(v.speed)
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
	w.syncBattery()
	w.syncFanPreset()
	w.syncCPUPower()
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

func (w *Window) syncPowerState(syncAutomation bool) {
	prev := w.syncing
	w.syncing = true
	defer func() { w.syncing = prev }()
	w.syncProfile()
	w.syncBattery()
	w.syncFanPreset()
	w.syncCPUPower()
	w.syncRefreshRate()
	w.syncOverdrive()
	if syncAutomation {
		w.syncPresets()
	}
}

func (w *Window) syncCPUPower() {
	if w.state == nil || w.state.CPUPower == nil || w.cpuMinScale == nil {
		return
	}
	state := w.state.CPUPower
	w.cpuMinScale.SetRange(float64(state.MinLimitKHz)/1000, float64(state.MaxLimitKHz)/1000)
	w.cpuMinScale.SetValue(float64(state.MinFrequencyKHz) / 1000)
	setActiveButton(w.cpuEPPBtns, state.EPP)
	if w.cpuBoostSwitch != nil {
		w.cpuBoostSwitch.SetActive(state.Boost)
	}
}

func (w *Window) initCPUMinDebounce(scale *gtk.Scale) {
	var debounce *time.Timer
	scale.ConnectValueChanged(func() {
		if w.syncing {
			return
		}
		if debounce != nil {
			debounce.Stop()
		}
		debounce = time.AfterFunc(200*time.Millisecond, func() {
			glib.IdleAdd(func() bool {
				khz := int(scale.Value()) * 1000
				w.runStateAction("set CPU minimum frequency", func() (bool, error) {
					return api.SendCPUMinFrequencySet(khz)
				})
				return false
			})
		})
	})
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
	setOnOffSummary(w.lightingSummary, ls.Enabled)
	if w.rgbControlsBox != nil {
		w.rgbControlsBox.SetSensitive(ls.Enabled)
	}
	if w.rgbEffectCard != nil {
		if ls.Enabled {
			w.rgbEffectCard.RemoveCSSClass("card-inactive")
		} else {
			w.rgbEffectCard.AddCSSClass("card-inactive")
		}
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
	if w.state == nil {
		return
	}
	setActiveButton(w.profileBtns, w.state.Profile)
	if w.profileSummary != nil {
		w.profileSummary.SetLabel(profileSummary(w.state.Profile))
	}
}

func profileSummary(profile string) string {
	if profile == "" {
		return "—"
	}
	return strings.ToUpper(profile)
}

// syncBattery sets the battery limit scale to match the daemon state.
// Also highlights the matching preset button (if any).
func (w *Window) syncBattery() {
	if w.state == nil {
		return
	}
	setActiveButton(w.battPresetBtns, w.state.Battery)
	if w.batterySummary != nil {
		w.batterySummary.SetLabel(batteryStrategySummary(w.state.Battery))
	}
}

func batteryStrategySummary(limit int) string {
	switch limit {
	case 100:
		return "STANDARD · 100%"
	case 80:
		return "BALANCED · 80%"
	case 60:
		return "MAX LIFE · 60%"
	case 0:
		return "—"
	default:
		return fmt.Sprintf("CUSTOM · %d%%", limit)
	}
}

func (w *Window) syncFanPreset() {
	if w.state == nil {
		return
	}
	active, summary, detail, locked := fanStatus(w.state)
	setActiveButton(w.fanPresetBtns, active)
	for _, button := range w.fanPresetBtns {
		button.SetSensitive(!locked)
	}
	if w.fanSummary != nil {
		w.fanSummary.SetLabel(summary)
		w.fanSummary.RemoveCSSClass("warning")
		if locked {
			w.fanSummary.AddCSSClass("warning")
		}
	}
	if w.fanSafetyLabel != nil {
		w.fanSafetyLabel.SetLabel(detail)
		w.fanSafetyLabel.SetVisible(detail != "")
		w.fanSafetyLabel.RemoveCSSClass("warning")
		if locked {
			w.fanSafetyLabel.AddCSSClass("warning")
		}
	}
	if w.tuningSummary != nil {
		w.tuningSummary.SetLabel(tuningStatus(w.state))
	}
}

func matchingFanPreset(curve *api.FanCurveState) string {
	if curve == nil {
		return ""
	}
	if curve.Mode == 2 {
		return "auto"
	}
	if curve.Mode != 1 {
		return ""
	}
	parts := make([]string, len(curve.Points))
	for i, point := range curve.Points {
		parts[i] = fmt.Sprintf("%d:%d", point.Temp, point.PWM)
	}
	encoded := strings.Join(parts, ",")
	for _, name := range fanPresets[1:] {
		if encoded == fanPresetPoints(name) {
			return name
		}
	}
	return ""
}

func fanStatus(state *api.State) (active, summary, detail string, locked bool) {
	if state == nil {
		return "", "—", "", false
	}
	if state.FanSafetyActive {
		return "", "SAFETY LOCK", "High-TDP safety cooling is active. Reset or lower TDP to unlock fan choices.", true
	}
	if !state.FanCurveActive {
		return "auto", "AUTO", "", false
	}
	if preset := matchingFanPreset(state.FanCurve); preset != "" {
		return preset, strings.ToUpper(preset), "", false
	}
	return "", "CUSTOM", "", false
}

func tuningStatus(state *api.State) string {
	if state == nil {
		return "CPU — · TDP — · UV —"
	}
	cpu := "CPU —"
	if state.CPUPower != nil {
		epp := strings.ToUpper(strings.ReplaceAll(state.CPUPower.EPP, "_", " "))
		switch state.CPUPower.EPP {
		case "balance_performance":
			epp = "RESPONSIVE"
		case "balance_power":
			epp = "EFFICIENT"
		case "performance":
			epp = "PERFORMANCE"
		case "power":
			epp = "POWER SAVER"
		}
		cpu = "CPU " + epp
		if state.CPUPower.Boost {
			cpu += " + BOOST"
		}
	}
	tdp := "FIRMWARE"
	if state.TDPActive && state.TDP != nil {
		tdp = fmt.Sprintf("%d W", state.TDP.PL1SPL)
	}
	uv := "STOCK"
	if state.UndervoltActive && state.Undervolt != nil {
		uv = fmt.Sprintf("%d", state.Undervolt.CPUCO)
	}
	return fmt.Sprintf("%s · TDP %s · UV %s", cpu, tdp, uv)
}

// syncRefreshRate highlights the refresh rate button matching the daemon state.
func (w *Window) syncRefreshRate() {
	if w.state == nil {
		return
	}
	if w.pendingRefreshRate != 0 {
		if w.state.RefreshRate == w.pendingRefreshRate || time.Now().After(w.refreshPendingUntil) {
			w.pendingRefreshRate = 0
		} else {
			setActiveButton(w.refreshBtns, w.pendingRefreshRate)
			if w.displaySummary != nil {
				w.displaySummary.SetLabel(fmt.Sprintf("%d HZ", w.pendingRefreshRate))
			}
			return
		}
	}
	setActiveButton(w.refreshBtns, w.state.RefreshRate)
	if w.displaySummary != nil {
		if _, ok := w.refreshBtns[w.state.RefreshRate]; ok {
			w.displaySummary.SetLabel(fmt.Sprintf("%d HZ", w.state.RefreshRate))
		} else {
			w.displaySummary.SetLabel("—")
		}
	}
}

// syncOverviewTelemetry populates Overview tab widgets from the live Telemetry
// sub-state. Called from syncState (initial) and from the 1 s telemetry poll.
// Gracefully no-ops on daemons that don't populate Telemetry (legacy); gauges
// simply show their last-known value and text rows keep their "—" placeholder.
func (w *Window) syncOverviewTelemetry() {
	t := w.state.Telemetry
	if t == nil {
		w.markOverviewStale()
		return
	}
	w.overviewLastUpdate = time.Now()
	if w.overviewFreshness != nil {
		w.overviewFreshness.SetVisible(false)
	}
	if w.overviewScroll != nil {
		w.overviewScroll.RemoveCSSClass("telemetry-stale")
	}

	setTemp := func(label *gtk.Label, temp int) {
		if label == nil {
			return
		}
		label.RemoveCSSClass("warning")
		label.RemoveCSSClass("danger")
		if temp <= 0 {
			label.SetLabel("—")
			return
		}
		label.SetLabel(fmt.Sprintf("%d°", temp))
		if temp >= thermalCriticalC {
			label.AddCSSClass("danger")
		} else if temp >= thermalWarningC {
			label.AddCSSClass("warning")
		}
	}
	setTemp(w.cpuTempValue, t.CPUTemp)
	setTemp(w.gpuTempValue, t.GPUTemp)
	if w.cpuUtilValue != nil {
		w.cpuUtilValue.SetLabel(fmt.Sprintf("%d%%", t.CPUUtil))
	}
	if w.gpuUtilValue != nil {
		w.gpuUtilValue.SetLabel(fmt.Sprintf("%d%%", t.GPUUtil))
	}
	if w.overviewStatus != nil {
		maxTemp := max(t.CPUTemp, t.GPUTemp)
		status, class := "UNKNOWN", ""
		switch {
		case maxTemp >= thermalCriticalC:
			status, class = "HOT", "danger"
		case maxTemp >= thermalWarningC:
			status, class = "WARM", "warning"
		case maxTemp > 0:
			status = "NORMAL"
		}
		w.overviewStatus.SetLabel(status)
		w.overviewStatus.RemoveCSSClass("warning")
		w.overviewStatus.RemoveCSSClass("danger")
		if class != "" {
			w.overviewStatus.AddCSSClass(class)
		}
	}
	if w.overviewContext != nil {
		parts := make([]string, 0, 2)
		if w.state.Profile != "" {
			parts = append(parts, strings.ToUpper(w.state.Profile))
		}
		if w.state.PowerSource != "" {
			parts = append(parts, strings.ToUpper(w.state.PowerSource))
		}
		if len(parts) == 0 {
			parts = append(parts, "—")
		}
		w.overviewContext.SetLabel(strings.Join(parts, "  ·  "))
	}
	if w.overviewAPUPower != nil {
		value := "—"
		if t.APUPowerAvailable {
			value = fmt.Sprintf("%.1f W", t.APUPowerW)
		}
		w.overviewAPUPower.SetLabel(value)
	}
	if w.overviewGPUPower != nil {
		value := "—"
		if t.GPUPowerAvailable {
			value = fmt.Sprintf("%.1f W", t.GPUPowerW)
		}
		w.overviewGPUPower.SetLabel(value)
	}
	if w.cpuFanLabel != nil {
		if t.FansAvailable {
			w.cpuFanLabel.SetLabel(fmt.Sprintf("%d RPM", t.CPUFanRPM))
		} else {
			w.cpuFanLabel.SetLabel("—")
		}
	}
	if w.gpuFanLabel != nil {
		if t.FansAvailable {
			w.gpuFanLabel.SetLabel(fmt.Sprintf("%d RPM", t.GPUFanRPM))
		} else {
			w.gpuFanLabel.SetLabel("—")
		}
	}
	if w.overviewCPUClock != nil {
		w.overviewCPUClock.SetLabel(formatGHz(t.CPUClockMHz))
	}
	if w.overviewGPUClock != nil {
		w.overviewGPUClock.SetLabel(formatGHz(t.GPUClockMHz))
	}
	if w.overviewNPUPower != nil {
		w.overviewNPUPower.SetLabel(formatNPU(t.NPUAvailable, t.NPUUtil, t.NPUPowerW))
		w.overviewNPUPower.RemoveCSSClass("npu-high")
		w.overviewNPUPower.RemoveCSSClass("npu-dim")
		if t.NPUAvailable && t.NPUPowerW < npuActivePowerW {
			w.overviewNPUPower.AddCSSClass("npu-dim")
		} else if t.NPUPowerW >= npuActivePowerW {
			w.overviewNPUPower.AddCSSClass("npu-high")
		}
	}
	if w.overviewMemClock != nil {
		if t.MemClockAvailable {
			w.overviewMemClock.SetLabel(fmt.Sprintf("%d MHz", t.MemClockMHz))
		} else {
			w.overviewMemClock.SetLabel("—")
		}
	}
	memoryUsed, memoryTotal := unifiedMemory(t.MemoryUsedMB, t.MemoryTotalMB, t.VRAMUsedMB, t.VRAMTotalMB)
	if w.overviewMemoryLbl != nil {
		w.overviewMemoryLbl.SetLabel(formatMemory(memoryUsed, memoryTotal))
	}
	if w.overviewMemoryBar != nil {
		w.overviewMemoryBar.RemoveCSSClass("warning")
		w.overviewMemoryBar.RemoveCSSClass("danger")
		fraction := 0.0
		if memoryTotal > 0 {
			fraction = float64(memoryUsed) / float64(memoryTotal)
		}
		w.overviewMemoryBar.SetFraction(fraction)
		if fraction >= 0.9 {
			w.overviewMemoryBar.AddCSSClass("danger")
		} else if fraction >= 0.8 {
			w.overviewMemoryBar.AddCSSClass("warning")
		}
	}
}

func (w *Window) markOverviewStale() {
	if w.overviewFreshness == nil {
		return
	}
	label := "TELEMETRY UNAVAILABLE"
	if !w.overviewLastUpdate.IsZero() {
		seconds := max(1, int(time.Since(w.overviewLastUpdate).Seconds()))
		label = fmt.Sprintf("STALE %d SEC", seconds)
	}
	w.overviewFreshness.SetLabel(label)
	w.overviewFreshness.SetVisible(true)
	if w.overviewScroll != nil {
		w.overviewScroll.AddCSSClass("telemetry-stale")
	}
}

// updateBatteryHero updates the System tab battery card from live telemetry.
// Called from syncState (initial) and from the 1s telemetry poll.
func (w *Window) updateBatteryHero(b *api.BatteryState) {
	capacity := min(100, max(0, b.CapacityPct))
	if w.battCapacityLabel != nil {
		w.battCapacityLabel.SetLabel(fmt.Sprintf("%d%%", capacity))
	}
	if w.battProgress != nil {
		w.battProgress.SetFraction(float64(capacity) / 100)
	}

	statusKind := strings.ToLower(b.Status)
	source := strings.ToLower(w.state.PowerSource)
	onAC := source == "ac"
	onBattery := source == "battery"
	atLimit := onAC && b.ThresholdPct > 0 && b.ThresholdPct < 100 &&
		capacity >= b.ThresholdPct && statusKind != "charging"
	if w.battStatusLabel != nil {
		status := b.Status
		switch {
		case atLimit:
			status = fmt.Sprintf("Holding at %d%%", b.ThresholdPct)
		case statusKind == "charging" && onAC:
			status = "Charging on AC"
		case statusKind == "charging":
			status = "Charging"
		case statusKind == "full":
			status = "Fully charged"
		case statusKind == "discharging" || onBattery:
			status = "On battery"
		case onAC && status != "":
			status += " on AC"
		case status == "":
			status = "—"
		}
		w.battStatusLabel.SetLabel(status)
	}
	if w.battHealthLabel != nil {
		w.battHealthLabel.SetLabel(fmt.Sprintf("%d%%", b.HealthPct))
	}
	if w.battEnergyLabel != nil {
		w.battEnergyLabel.SetLabel(fmt.Sprintf("%.1f / %.1f Wh", b.EnergyNowWh, b.EnergyFullWh))
	}
	if w.battVoltageLabel != nil {
		voltage := formatBatteryDecimal(b.VoltageVolts)
		if voltage != "—" {
			voltage += " V"
		}
		w.battVoltageLabel.SetLabel(voltage)
	}
	if w.battDesignLabel != nil {
		w.battDesignLabel.SetLabel(fmt.Sprintf("%.1f Wh", b.EnergyDesignWh))
	}

	draw, timeCaption, timeValue := "—", "REMAINING", "—"
	showDraw := statusKind == "discharging" || onBattery
	powerW, err := strconv.ParseFloat(strings.TrimSpace(b.PowerWatts), 64)
	canRate := err == nil && powerW > 0
	switch {
	case showDraw:
		if canRate && b.EnergyNowWh > 0 {
			draw = fmt.Sprintf("%.1f W", powerW)
			timeValue = formatBatteryRuntime(b.EnergyNowWh / powerW)
		}
	case statusKind == "charging":
		timeCaption = "TO FULL"
		if canRate {
			target := b.EnergyFullWh
			if b.ThresholdPct > 0 && b.ThresholdPct < 100 {
				target = b.EnergyFullWh * float64(b.ThresholdPct) / 100
			}
			if target > b.EnergyNowWh {
				timeValue = formatBatteryRuntime((target - b.EnergyNowWh) / powerW)
			}
		}
	case atLimit:
		timeCaption = "AT LIMIT"
		timeValue = fmt.Sprintf("%d%%", b.ThresholdPct)
	case statusKind == "full":
		timeCaption = "STATUS"
		timeValue = "FULL"
	case onAC:
		timeCaption = "TO FULL"
	}
	if w.battDrawMetric != nil {
		w.battDrawMetric.SetVisible(showDraw)
	}
	if w.battDrawLabel != nil {
		w.battDrawLabel.SetLabel(draw)
	}
	if w.battTimeCaption != nil {
		w.battTimeCaption.SetLabel(timeCaption)
	}
	if w.battTimeLabel != nil {
		w.battTimeLabel.SetLabel(timeValue)
	}
	if w.battPill != nil {
		// Threshold preset chip: 100=Standard, 80=Balanced, <80=Max Life.
		w.battPill.RemoveCSSClass("warning")
		w.battPill.RemoveCSSClass("accent")
		w.battPill.RemoveCSSClass("success")
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

func formatBatteryDecimal(value string) string {
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return "—"
	}
	return fmt.Sprintf("%.2f", n)
}

// formatBatteryRuntime renders an estimated runtime as "Xh Ym".
func formatBatteryRuntime(hours float64) string {
	hours = max(hours, 0)
	h := int(hours)
	m := int((hours - float64(h)) * 60)
	return fmt.Sprintf("%dh %dm", h, m)
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
	setOnOffSummary(w.lightingSummary, enabled)
	if w.rgbEffectCard != nil {
		if enabled {
			w.rgbEffectCard.RemoveCSSClass("card-inactive")
		} else {
			w.rgbEffectCard.AddCSSClass("card-inactive")
		}
	}
	if enabled {
		setActiveButton(w.modeButtons, activeButton(w.modeButtons, defaultMode))
		w.syncModeVis()
		// Floor brightness at the minimum visible level; a synced brightness
		// of 0 (e.g. device was off) would otherwise re-apply as fully dark.
		if w.brightScale != nil && w.brightScale.Value() == 0 {
			w.brightScale.SetValue(1)
		}
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
	setOnOffSummary(w.startupSummary, w.state.BootSound != 0)
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
