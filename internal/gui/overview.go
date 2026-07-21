package gui

// overview.go — Overview tab: a live telemetry dashboard for the Strix Halo SoC.
//
// Layout (top-to-bottom, all on a single screen with no scroll at native res):
//   - HERO CARD: 4 compact radial gauges in a row
//       CPU °C (hot ramp) | GPU °C (hot ramp) | CPU % | GPU %
//   - Sparkline (30s CPU temp history, hot ramp)
//   - METRICS GRID (2 columns of label/value rows):
//       Fan        3400 RPM      | VRAM       ████░░ 4.2/16 GB
//       CPU clock  3.8 GHz       | Memory     6400 MT/s
//       GPU clock  1.7 GHz       |
//
// All values are populated by syncOverviewTelemetry() in sync.go, driven from
// the 1-second telemetry poll in tdp.go. When api.State.Telemetry is nil
// (older daemon), values show "—" placeholders.

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// buildOverviewTab constructs the Overview tab content + scroll container.
func (w *Window) buildOverviewTab() *gtk.ScrolledWindow {
	inner := gtk.NewBox(gtk.OrientationVertical, 4)
	inner.SetMarginTop(4)
	inner.SetMarginBottom(8)
	inner.SetMarginStart(12)
	inner.SetMarginEnd(12)

	inner.Append(groupLabel("OVERVIEW"))
	inner.Append(w.buildOverviewHero())
	inner.Append(w.buildOverviewMetrics())

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(inner)
	w.overviewScroll = scroll
	return scroll
}

// buildOverviewHero constructs the hero card: 4 gauges + sparkline.
func (w *Window) buildOverviewHero() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 4)
	box.SetMarginBottom(4)

	card := gtk.NewBox(gtk.OrientationVertical, 6)
	card.AddCSSClass("card")

	// Gauge row — 4 equal-width compact gauges.
	gaugeRow := gtk.NewBox(gtk.OrientationHorizontal, 4)
	gaugeRow.SetHomogeneous(true)

	w.cpuTempGauge = NewRadialGauge("CPU", "°")
	w.cpuTempGauge.SetRange(20, 100)
	w.cpuTempGauge.SetHot(true)
	w.cpuTempGauge.Widget().SetSizeRequest(-1, 56)
	gaugeRow.Append(w.cpuTempGauge.Widget())

	w.gpuTempGauge = NewRadialGauge("GPU", "°")
	w.gpuTempGauge.SetRange(20, 100)
	w.gpuTempGauge.SetHot(true)
	w.gpuTempGauge.Widget().SetSizeRequest(-1, 56)
	gaugeRow.Append(w.gpuTempGauge.Widget())

	w.cpuUtilGauge = NewRadialGauge("CPU", "%")
	w.cpuUtilGauge.SetRange(0, 100)
	w.cpuUtilGauge.Widget().SetSizeRequest(-1, 56)
	gaugeRow.Append(w.cpuUtilGauge.Widget())

	w.gpuUtilGauge = NewRadialGauge("GPU", "%")
	w.gpuUtilGauge.SetRange(0, 100)
	w.gpuUtilGauge.Widget().SetSizeRequest(-1, 56)
	gaugeRow.Append(w.gpuUtilGauge.Widget())

	card.Append(gaugeRow)

	// Sparkline — 30s CPU temperature trend (compact).
	w.overviewSpark = NewSparkline(30)
	w.overviewSpark.SetRange(20, 100)
	w.overviewSpark.SetHot(true)
	w.overviewSpark.Widget().SetSizeRequest(-1, 28)
	card.Append(w.overviewSpark.Widget())

	box.Append(card)
	w.overviewHero = box
	return box
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
	card.SetMarginTop(4)

	// Two-column grid via a horizontal Box of two vertical Boxes.
	grid := gtk.NewBox(gtk.OrientationHorizontal, 12)
	grid.SetHomogeneous(true)

	leftCol := gtk.NewBox(gtk.OrientationVertical, 4)
	rightCol := gtk.NewBox(gtk.OrientationVertical, 4)

	// Left column.
	row, lbl := metricRow("CPU FAN")
	leftCol.Append(row)
	w.cpuFanLabel = lbl

	row, lbl = metricRow("GPU FAN")
	leftCol.Append(row)
	w.gpuFanLabel = lbl

	row, lbl = metricRow("CPU CLOCK")
	leftCol.Append(row)
	w.overviewCPUClock = lbl

	row, lbl = metricRow("GPU CLOCK")
	leftCol.Append(row)
	w.overviewGPUClock = lbl

	// Right column.
	row, lbl = metricRow("MEMORY")
	rightCol.Append(row)
	w.overviewMemClock = lbl

	// VRAM row: label + progress bar + text (custom layout).
	vramRow := gtk.NewBox(gtk.OrientationVertical, 2)
	vramRow.SetMarginTop(2)
	vramRow.SetMarginBottom(2)

	vramLabelRow := gtk.NewBox(gtk.OrientationHorizontal, 4)
	vramLbl := gtk.NewLabel("VRAM")
	vramLbl.SetHAlign(gtk.AlignStart)
	vramLbl.AddCSSClass("scale-name")
	vramLabelRow.Append(vramLbl)

	w.overviewVRAMLbl = gtk.NewLabel("—")
	w.overviewVRAMLbl.SetHAlign(gtk.AlignEnd)
	w.overviewVRAMLbl.SetHExpand(true)
	w.overviewVRAMLbl.AddCSSClass("scale-value")
	vramLabelRow.Append(w.overviewVRAMLbl)
	vramRow.Append(vramLabelRow)

	w.overviewVRAMBar = gtk.NewProgressBar()
	w.overviewVRAMBar.SetHExpand(true)
	w.overviewVRAMBar.AddCSSClass("vram-bar")
	vramRow.Append(w.overviewVRAMBar)
	rightCol.Append(vramRow)

	grid.Append(leftCol)
	grid.Append(rightCol)
	card.Append(grid)

	// NPU summary row — full width below the grid. Combines util/clock/power
	// into a single scannable line; matches the format users expect from
	// tools like amdxdna-top. Stays "—" until the daemon reports non-zero NPU.
	npuRow, npuLbl := metricRow("NPU")
	card.Append(npuRow)
	w.npuLabel = npuLbl

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

// formatVRAM renders "used / total GB". Returns "—" if either is zero.
func formatVRAM(used, total int) string {
	if total <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f / %.0f GB", float64(used)/1024.0, float64(total)/1024.0)
}

// formatNPU renders the NPU summary line: "util% · clock · powerW".
// Shows "—" if every sensor is zero (NPU unavailable or fully idle).
// Individual zero fields are replaced with "—" so a partial read still
// surfaces the available metrics.
func formatNPU(util, clockMHz, powerW int) string {
	if util == 0 && clockMHz == 0 && powerW == 0 {
		return "—"
	}
	utilStr := "—"
	if util > 0 {
		utilStr = fmt.Sprintf("%d%%", util)
	}
	clockStr := "—"
	if clockMHz > 0 {
		clockStr = formatGHz(clockMHz)
	}
	powerStr := "—"
	if powerW > 0 {
		powerStr = fmt.Sprintf("%d W", powerW)
	}
	return fmt.Sprintf("%s · %s · %s", utilStr, clockStr, powerStr)
}
