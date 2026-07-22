package gui

// overview.go — Overview tab: a live telemetry dashboard for the Strix Halo SoC.
//
// Layout (top-to-bottom, all on a single screen with no scroll at native res):
//   - THERMALS: CPU/GPU temperature and load tiles, plus NPU state and power
//   - BATTERY: charge, source, draw, health, and active strategy
//   - SYSTEM: unified memory, memory clock, fans, and CPU/GPU clocks
//
// All values are populated by syncOverviewTelemetry() in sync.go, driven from
// the 1-second telemetry poll in tdp.go. When api.State.Telemetry is nil
// (older daemon), values show "—" placeholders.

import (
	"fmt"
	"math"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// buildOverviewTab constructs the Overview tab content + scroll container.
func (w *Window) buildOverviewTab() *gtk.ScrolledWindow {
	inner := gtk.NewBox(gtk.OrientationVertical, 8)
	inner.SetMarginTop(4)
	inner.SetMarginBottom(8)
	inner.SetMarginStart(12)
	inner.SetMarginEnd(12)

	inner.Append(w.buildOverviewHero())
	inner.Append(w.buildBatteryHero())
	inner.Append(w.buildOverviewMetrics())

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(inner)
	w.overviewScroll = scroll
	return scroll
}

// buildOverviewHero constructs the hero card: status, metrics, and trend.
func (w *Window) buildOverviewHero() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 4)
	box.SetMarginBottom(4)

	card := gtk.NewBox(gtk.OrientationVertical, 6)
	card.AddCSSClass("card")

	header := gtk.NewBox(gtk.OrientationHorizontal, 4)
	title := gtk.NewLabel("THERMALS & LOAD")
	title.SetHAlign(gtk.AlignStart)
	title.SetHExpand(true)
	title.AddCSSClass("card-title")
	header.Append(title)
	w.overviewStatus = gtk.NewLabel("NORMAL")
	w.overviewStatus.AddCSSClass("pill")
	w.overviewStatus.AddCSSClass("success")
	header.Append(w.overviewStatus)
	card.Append(header)

	contextRow := gtk.NewBox(gtk.OrientationHorizontal, 4)
	w.overviewContext = gtk.NewLabel("—")
	w.overviewContext.SetHAlign(gtk.AlignStart)
	w.overviewContext.SetHExpand(true)
	w.overviewContext.AddCSSClass("card-sub")
	contextRow.Append(w.overviewContext)
	w.overviewFreshness = gtk.NewLabel("")
	w.overviewFreshness.AddCSSClass("overview-stale")
	w.overviewFreshness.SetVisible(false)
	contextRow.Append(w.overviewFreshness)
	card.Append(contextRow)

	metricRow := gtk.NewBox(gtk.OrientationHorizontal, 4)
	metricRow.SetHomogeneous(true)
	metric, value := overviewMetric("CPU TEMP")
	w.cpuTempValue = value
	metricRow.Append(metric)
	metric, value = overviewMetric("GPU TEMP")
	w.gpuTempValue = value
	metricRow.Append(metric)
	metric, value = overviewMetric("CPU LOAD")
	w.cpuUtilValue = value
	metricRow.Append(metric)
	metric, value = overviewMetric("GPU LOAD")
	w.gpuUtilValue = value
	metricRow.Append(metric)
	card.Append(metricRow)

	powerRow := gtk.NewBox(gtk.OrientationHorizontal, 4)
	powerRow.AddCSSClass("overview-power-row")
	powerRow.SetHomogeneous(true)
	metric, value = overviewMetric("SYSTEM POWER")
	w.overviewSystemPower = value
	powerRow.Append(metric)
	metric, value = overviewMetric("APU POWER")
	w.overviewAPUPower = value
	powerRow.Append(metric)
	metric, value = overviewMetric("GPU POWER")
	w.overviewGPUPower = value
	powerRow.Append(metric)
	card.Append(powerRow)

	npuRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	npuRow.AddCSSClass("overview-npu-row")
	npuTitle := gtk.NewLabel("NPU")
	npuTitle.SetHAlign(gtk.AlignStart)
	npuTitle.SetHExpand(true)
	npuTitle.AddCSSClass("scale-name")
	npuRow.Append(npuTitle)
	w.npuLabel = gtk.NewLabel("—")
	w.npuLabel.AddCSSClass("overview-value")
	npuRow.Append(w.npuLabel)
	w.overviewNPUPower = gtk.NewLabel("—")
	w.overviewNPUPower.AddCSSClass("overview-value")
	npuRow.Append(w.overviewNPUPower)
	card.Append(npuRow)

	box.Append(card)
	w.overviewHero = box
	return box
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
	card := gtk.NewBox(gtk.OrientationVertical, 4)
	card.AddCSSClass("card")

	title := gtk.NewLabel("SYSTEM")
	title.SetHAlign(gtk.AlignStart)
	title.AddCSSClass("card-title")
	card.Append(title)

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

	row, lbl := metricRow("MEMORY CLOCK")
	card.Append(row)
	w.overviewMemClock = lbl

	grid := gtk.NewBox(gtk.OrientationHorizontal, 12)
	grid.SetHomogeneous(true)
	leftCol := gtk.NewBox(gtk.OrientationVertical, 4)
	rightCol := gtk.NewBox(gtk.OrientationVertical, 4)

	row, lbl = metricRow("CPU FAN")
	leftCol.Append(row)
	w.cpuFanLabel = lbl
	row, lbl = metricRow("GPU FAN")
	leftCol.Append(row)
	w.gpuFanLabel = lbl
	row, lbl = metricRow("CPU CLOCK")
	rightCol.Append(row)
	w.overviewCPUClock = lbl
	row, lbl = metricRow("GPU CLOCK")
	rightCol.Append(row)
	w.overviewGPUClock = lbl

	grid.Append(leftCol)
	grid.Append(rightCol)
	card.Append(grid)

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
func unifiedMemory(used, total, vramUsed, vramTotal int) (int, int) {
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

// formatNPU renders power first because it distinguishes low-power background
// clients from real computation better than the driver's raw column occupancy.
func formatNPU(available bool, util int, powerW float64) (string, string) {
	if !available {
		return "UNAVAILABLE", "NO DATA"
	}
	if util == 0 && powerW == 0 {
		return "IDLE", "0.00 W"
	}
	if powerW <= 0 {
		return "LOW POWER", "POWER N/A"
	}
	status := "LOW POWER"
	if powerW >= npuActivePowerW {
		status = "ACTIVE"
	}
	return status, fmt.Sprintf("%.2f W", powerW)
}
