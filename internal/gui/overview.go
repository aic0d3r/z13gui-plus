package gui

// overview.go — Overview tab: a live telemetry dashboard for the Strix Halo SoC.
//
// Layout (top-to-bottom). Everything above DETAILS fits one screen:
//   - status strip: health pill, profile, power source, sustained limit
//   - THERMALS & LOAD: CPU/GPU temperature and load tiles
//   - APU POWER vs the sustained limit, plus GPU/NPU power and NPU load; fans
//   - DETAILS: unified memory + clocks, then the compact battery card
//
// All values are populated by syncOverviewTelemetry() in sync.go, driven from
// the 1-second telemetry poll in tdp.go. When api.State.Telemetry is nil
// (older daemon), values show "—" placeholders.

import (
	"fmt"
	"math"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

// buildOverviewTab constructs the Overview tab content + scroll container.
func (w *Window) buildOverviewTab() *gtk.ScrolledWindow {
	inner := gtk.NewBox(gtk.OrientationVertical, 8)
	inner.SetMarginTop(4)
	inner.SetMarginBottom(8)
	inner.SetMarginStart(12)
	inner.SetMarginEnd(12)

	// Tier 1 (above DETAILS) answers "am I throttling or too hot?" at a glance;
	// tier 2 is the long tail for advanced users. Each sensor appears once.
	inner.Append(w.buildOverviewHero())
	power, fans, memory := w.buildOverviewPower(), w.buildOverviewFans(), w.buildOverviewMetrics()
	w.battCard = w.buildBatteryHero()
	w.registerCard("overview.power", power)
	w.registerCard("overview.fans", fans)
	w.registerCard("overview.memory", memory)
	w.registerCard("overview.battery", w.battCard)
	inner.Append(power)
	inner.Append(fans)
	inner.Append(sectionLabel("DETAILS"))
	inner.Append(memory)
	inner.Append(w.battCard)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(inner)
	w.overviewScroll = scroll
	return scroll
}

// buildOverviewHero is the status strip (health pill + context) and the four
// thermal/load tiles.
func (w *Window) buildOverviewHero() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 6)

	strip := gtk.NewBox(gtk.OrientationHorizontal, 8)
	strip.AddCSSClass("card")
	w.overviewStatus = gtk.NewLabel("NORMAL")
	w.overviewStatus.AddCSSClass("pill")
	strip.Append(w.overviewStatus)
	w.overviewContext = gtk.NewLabel("—")
	w.overviewContext.SetHAlign(gtk.AlignStart)
	w.overviewContext.SetHExpand(true)
	w.overviewContext.SetEllipsize(pango.EllipsizeEnd)
	w.overviewContext.AddCSSClass("card-sub")
	strip.Append(w.overviewContext)
	w.overviewFreshness = gtk.NewLabel("")
	w.overviewFreshness.AddCSSClass("overview-stale")
	w.overviewFreshness.SetVisible(false)
	strip.Append(w.overviewFreshness)
	box.Append(strip)

	box.Append(sectionLabel("THERMALS & LOAD"))
	metricRow := gtk.NewBox(gtk.OrientationHorizontal, 4)
	metricRow.SetHomogeneous(true)
	for _, m := range []struct {
		label string
		dst   **gtk.Label
	}{
		{"CPU TEMP", &w.cpuTempValue},
		{"GPU TEMP", &w.gpuTempValue},
		{"CPU LOAD", &w.cpuUtilValue},
		{"GPU LOAD", &w.gpuUtilValue},
	} {
		metric, value := overviewMetric(m.label)
		*m.dst = value
		metricRow.Append(metric)
	}
	box.Append(metricRow)
	return box
}

// buildOverviewPower shows APU power against the sustained limit, with the
// other power/NPU readings underneath.
func (w *Window) buildOverviewPower() *gtk.Box {
	card := gtk.NewBox(gtk.OrientationVertical, 4)
	card.AddCSSClass("card")

	header := gtk.NewBox(gtk.OrientationHorizontal, 4)
	title := gtk.NewLabel("APU POWER")
	title.SetHAlign(gtk.AlignStart)
	title.SetHExpand(true)
	title.AddCSSClass("card-title")
	header.Append(title)
	w.overviewAPUPower = gtk.NewLabel("—")
	w.overviewAPUPower.AddCSSClass("battery-summary-value")
	header.Append(w.overviewAPUPower)
	card.Append(header)

	w.overviewPowerBar = gtk.NewProgressBar()
	w.overviewPowerBar.SetHExpand(true)
	w.overviewPowerBar.AddCSSClass("memory-bar")
	card.Append(w.overviewPowerBar)
	w.overviewPowerLimit = gtk.NewLabel("")
	w.overviewPowerLimit.SetHAlign(gtk.AlignEnd)
	w.overviewPowerLimit.AddCSSClass("card-sub")
	card.Append(w.overviewPowerLimit)

	powerRow := gtk.NewBox(gtk.OrientationHorizontal, 4)
	powerRow.AddCSSClass("overview-power-row")
	powerRow.SetHomogeneous(true)
	for _, m := range []struct {
		label string
		dst   **gtk.Label
	}{
		{"GPU", &w.overviewGPUPower},
		{"NPU", &w.overviewNPUPower},
		{"NPU LOAD", &w.overviewNPUUtil},
	} {
		metric, value := overviewMetric(m.label)
		*m.dst = value
		powerRow.Append(metric)
	}
	card.Append(powerRow)
	return card
}

// buildOverviewFans is the CPU/GPU fan card; fan speed is part of "am I throttling".
func (w *Window) buildOverviewFans() *gtk.Box {
	card := gtk.NewBox(gtk.OrientationHorizontal, 12)
	card.AddCSSClass("card")
	card.SetHomogeneous(true)
	row, lbl := metricRow("CPU FAN")
	card.Append(row)
	w.cpuFanLabel = lbl
	row, lbl = metricRow("GPU FAN")
	card.Append(row)
	w.gpuFanLabel = lbl
	return card
}

func overviewMetric(label string) (*gtk.Box, *gtk.Label) {
	box := gtk.NewBox(gtk.OrientationVertical, 1)
	box.AddCSSClass("overview-metric")
	value := gtk.NewLabel("—")
	value.AddCSSClass("overview-metric-value")
	caption := gtk.NewLabel(label)
	caption.AddCSSClass("overview-metric-label")
	box.Append(value)
	box.Append(caption)
	return box, value
}

// metricRow builds one row of the 2-column metrics grid: a small-caps label
// on the left, a right-aligned mono value on the right. Returns the value
// label so callers can update it.
func metricRow(label string) (*gtk.Box, *gtk.Label) {
	row := gtk.NewBox(gtk.OrientationHorizontal, 4)
	row.SetMarginTop(2)
	row.SetMarginBottom(2)

	lbl := gtk.NewLabel(label)
	lbl.SetHAlign(gtk.AlignStart)
	lbl.AddCSSClass("scale-name")

	val := gtk.NewLabel("—")
	val.SetHAlign(gtk.AlignEnd)
	val.SetHExpand(true)
	val.AddCSSClass("scale-value")
	val.AddCSSClass("overview-value")

	row.Append(lbl)
	row.Append(val)
	return row, val
}

// buildOverviewMetrics constructs the metrics grid: two columns of label/value
// rows below the hero card.
func (w *Window) buildOverviewMetrics() *gtk.Box {
	card := gtk.NewBox(gtk.OrientationVertical, 6)
	card.AddCSSClass("card")
	card.AddCSSClass("overview-system-card")

	memoryRow := gtk.NewBox(gtk.OrientationVertical, 2)
	memoryHeader := gtk.NewBox(gtk.OrientationHorizontal, 4)
	memoryTitle := gtk.NewLabel("UNIFIED MEMORY")
	memoryTitle.SetHAlign(gtk.AlignStart)
	memoryTitle.AddCSSClass("scale-name")
	memoryHeader.Append(memoryTitle)
	w.overviewMemoryLbl = gtk.NewLabel("—")
	w.overviewMemoryLbl.SetHAlign(gtk.AlignEnd)
	w.overviewMemoryLbl.SetHExpand(true)
	w.overviewMemoryLbl.AddCSSClass("scale-value")
	memoryHeader.Append(w.overviewMemoryLbl)
	memoryRow.Append(memoryHeader)
	w.overviewMemoryBar = gtk.NewProgressBar()
	w.overviewMemoryBar.SetHExpand(true)
	w.overviewMemoryBar.AddCSSClass("memory-bar")
	memoryRow.Append(w.overviewMemoryBar)
	card.Append(memoryRow)

	clocks := gtk.NewBox(gtk.OrientationHorizontal, 4)
	clocks.SetHomogeneous(true)
	for _, m := range []struct {
		label string
		dst   **gtk.Label
	}{
		{"CPU CLOCK", &w.overviewCPUClock},
		{"GPU CLOCK", &w.overviewGPUClock},
		{"MEM CLOCK", &w.overviewMemClock},
	} {
		metric, value := overviewMetric(m.label)
		*m.dst = value
		clocks.Append(metric)
	}
	card.Append(clocks)

	return card
}

// formatGHz converts MHz to a "X.Y GHz" string. Below 1000 MHz shows as MHz.
func formatGHz(mhz int) string {
	if mhz <= 0 {
		return "—"
	}
	if mhz < 1000 {
		return fmt.Sprintf("%d MHz", mhz)
	}
	return fmt.Sprintf("%.1f GHz", float64(mhz)/1000.0)
}

// unifiedMemory combines Linux memory with the reserved GPU carveout.
func unifiedMemory(used, total, vramUsed, vramTotal int) (combinedUsed, combinedTotal int) {
	if total <= 0 {
		return 0, 0
	}
	used += vramUsed
	total += vramTotal
	// Linux excludes firmware reservations from MemTotal. Round the combined
	// system + GPU pool up to a standard 8 GiB installed capacity.
	totalGiB := math.Ceil(float64(total)/(8*1024)) * 8
	return used, int(totalGiB * 1024)
}

func formatMemory(used, total int) string {
	if total <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f / %.0f GB", float64(used)/1024.0, float64(total)/1024.0)
}

const (
	npuActivePowerW = 0.5
	// Ryzen AI Max+ 395 has a 100°C Tjmax; warn before throttling territory.
	thermalWarningC  = 85
	thermalCriticalC = 95
)

// formatNPU keeps the power tile honest on drivers without power sensors.
func formatNPU(available bool, powerW float64, clockMHz int) string {
	if !available {
		return "IDLE"
	}
	if powerW > 0 {
		return fmt.Sprintf("%.1f W", powerW)
	}
	if clockMHz > 0 {
		return "0.0 W"
	}
	return "ACTIVE"
}

func formatNPUUtil(available bool, util int, powerW float64, clockMHz int) string {
	if !available {
		return "0%"
	}
	if powerW == 0 && util == 0 && clockMHz == 0 {
		return "—"
	}
	return fmt.Sprintf("%d%%", util)
}
