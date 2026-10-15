package gui

// tdp.go — Advanced tuning view: TDP sliders, fan curve editor, telemetry.

import (
	"fmt"
	"math"
	"strings"

	"github.com/dahui/z13ctl/api"
	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// TDP limits (matching daemon constants).
const (
	tdpMin         = 5
	tdpMaxBasic    = 70 // basic slider max
	tdpMaxSafe     = 75 // warning threshold
	tdpMaxAdvanced = 93 // advanced slider max (force=true above 75)
)

// fanCurveEditor renders and handles interaction for the 8-point fan curve.
type fanCurveEditor struct {
	area     *gtk.DrawingArea
	points   [8]api.FanCurvePoint // temp: 0–120°C, pwm: 0–255
	dragging int                  // point index being dragged, -1 if none
	hovered  int                  // point index under cursor, -1 if none
	w        *Window              // parent for theme colors + telemetry

	// Chart area within the DrawingArea (set during draw).
	chartX, chartY, chartW, chartH float64
}

// defaultFanCurve returns a reasonable default fan curve.
func defaultFanCurve() [8]api.FanCurvePoint {
	return [8]api.FanCurvePoint{
		{Temp: 35, PWM: 0},
		{Temp: 45, PWM: 25},
		{Temp: 50, PWM: 50},
		{Temp: 60, PWM: 80},
		{Temp: 70, PWM: 120},
		{Temp: 80, PWM: 170},
		{Temp: 90, PWM: 220},
		{Temp: 100, PWM: 255},
	}
}

func fanPWMAtTemp(points [8]api.FanCurvePoint, temp int) int {
	if temp <= points[0].Temp {
		return points[0].PWM
	}
	pwm := float64(points[0].PWM)
	for i := 1; i < len(points); i++ {
		prev, next := points[i-1], points[i]
		if temp <= next.Temp {
			span := next.Temp - prev.Temp
			if span > 0 {
				fraction := float64(temp-prev.Temp) / float64(span)
				pwm = float64(prev.PWM) + fraction*float64(next.PWM-prev.PWM)
			}
			break
		}
		pwm = float64(next.PWM)
	}
	return int(math.Round(pwm))
}

// curveString returns the curve in "temp:pwm,temp:pwm,..." format for the API.
func (fc *fanCurveEditor) curveString() string {
	var parts []string
	for _, p := range fc.points {
		parts = append(parts, fmt.Sprintf("%d:%d", p.Temp, p.PWM))
	}
	return strings.Join(parts, ",")
}

// enforceConstraints ensures temps are strictly increasing and PWM non-decreasing.
func (fc *fanCurveEditor) enforceConstraints(idx int) {
	// Clamp the dragged point so every neighbour still fits a distinct degree
	// inside 35–105°C; the cascades below then never leave that range and
	// points the user didn't push (e.g. a preset's 30°C start) stay put.
	fc.points[idx].Temp = min(max(fc.points[idx].Temp, 35+idx), 105-(7-idx))
	fc.points[idx].PWM = min(max(fc.points[idx].PWM, 0), 255)

	// Cascade temps forward (must be strictly increasing).
	for i := idx + 1; i < 8; i++ {
		if fc.points[i].Temp <= fc.points[i-1].Temp {
			fc.points[i].Temp = fc.points[i-1].Temp + 1
		}
	}
	// Cascade temps backward.
	for i := idx - 1; i >= 0; i-- {
		if fc.points[i].Temp >= fc.points[i+1].Temp {
			fc.points[i].Temp = fc.points[i+1].Temp - 1
		}
	}
	// Cascade PWM forward (must be non-decreasing).
	for i := idx + 1; i < 8; i++ {
		if fc.points[i].PWM < fc.points[i-1].PWM {
			fc.points[i].PWM = fc.points[i-1].PWM
		}
	}
	// Cascade PWM backward.
	for i := idx - 1; i >= 0; i-- {
		if fc.points[i].PWM > fc.points[i+1].PWM {
			fc.points[i].PWM = fc.points[i+1].PWM
		}
	}
}

// Coordinate mapping.
func (fc *fanCurveEditor) tempToX(temp int) float64 {
	return fc.chartX + (float64(temp-35)/70.0)*fc.chartW // 35–105°C range
}
func (fc *fanCurveEditor) pwmToY(pwm int) float64 {
	return fc.chartY + fc.chartH - (float64(pwm)/255.0)*fc.chartH // inverted
}
func (fc *fanCurveEditor) xToTemp(x float64) int {
	return min(max(35+int(math.Round((x-fc.chartX)/fc.chartW*70.0)), 35), 105)
}
func (fc *fanCurveEditor) yToPWM(y float64) int {
	return min(max(int(math.Round((fc.chartY+fc.chartH-y)/fc.chartH*255.0)), 0), 255)
}

// hitTest returns the index of the point nearest to (x,y) within tolerance, or -1.
func (fc *fanCurveEditor) hitTest(x, y float64) int {
	const tolerance = 20.0
	best := -1
	bestDist := tolerance * tolerance
	for i, p := range fc.points {
		px := fc.tempToX(p.Temp)
		py := fc.pwmToY(p.PWM)
		dx := x - px
		dy := y - py
		d := dx*dx + dy*dy
		if d < bestDist {
			bestDist = d
			best = i
		}
	}
	return best
}

// draw renders the fan curve chart.
func (fc *fanCurveEditor) draw(cr *cairo.Context, width, height int) {
	w := float64(width)
	h := float64(height)

	// Chart margins.
	const leftMargin = 36.0
	const bottomMargin = 20.0
	const topMargin = 8.0
	const rightMargin = 8.0
	fc.chartX = leftMargin
	fc.chartY = topMargin
	fc.chartW = w - leftMargin - rightMargin
	fc.chartH = h - topMargin - bottomMargin

	// Background.
	cr.SetSourceRGBA(0, 0, 0, 0) // transparent — CSS handles bg
	cr.Paint()

	// Grid lines.
	cr.SetSourceRGBA(0.4, 0.4, 0.4, 0.3)
	cr.SetLineWidth(0.5)
	// Horizontal: 0%, 25%, 50%, 75%, 100%.
	for _, pct := range []float64{0, 25, 50, 75, 100} {
		y := fc.pwmToY(int(pct / 100.0 * 255))
		cr.MoveTo(fc.chartX, y)
		cr.LineTo(fc.chartX+fc.chartW, y)
	}
	// Vertical: every 10°C from 35 to 105.
	for temp := 35; temp <= 105; temp += 10 {
		x := fc.tempToX(temp)
		cr.MoveTo(x, fc.chartY)
		cr.LineTo(x, fc.chartY+fc.chartH)
	}
	cr.Stroke()

	// Axis labels.
	cr.SetSourceRGBA(0.6, 0.6, 0.6, 1)
	cr.SetFontSize(9)
	// Y-axis labels.
	for _, pct := range []int{0, 25, 50, 75, 100} {
		y := fc.pwmToY(int(float64(pct) / 100.0 * 255))
		cr.MoveTo(2, y+3)
		cr.ShowText(fmt.Sprintf("%d%%", pct))
	}
	// X-axis labels.
	for temp := 40; temp <= 100; temp += 20 {
		x := fc.tempToX(temp)
		cr.MoveTo(x-8, fc.chartY+fc.chartH+14)
		cr.ShowText(fmt.Sprintf("%d°", temp))
	}

	// Current APU temperature indicator line.
	if fc.w != nil && fc.w.state != nil && fc.w.state.Temperature > 0 {
		apuTemp := fc.w.state.Temperature
		if apuTemp >= 35 && apuTemp <= 105 {
			tx := fc.tempToX(apuTemp)
			cr.SetSourceRGBA(1, 1, 1, 0.4)
			cr.SetLineWidth(1)
			cr.SetDash([]float64{4, 3}, 0)
			cr.MoveTo(tx, fc.chartY)
			cr.LineTo(tx, fc.chartY+fc.chartH)
			cr.Stroke()
			cr.SetDash(nil, 0)
		}
	}

	// Filled area under curve.
	cr.SetSourceRGBA(0.8, 0.1, 0.1, 0.15) // accent-ish, semi-transparent
	cr.MoveTo(fc.tempToX(fc.points[0].Temp), fc.pwmToY(0))
	for _, p := range fc.points {
		cr.LineTo(fc.tempToX(p.Temp), fc.pwmToY(p.PWM))
	}
	cr.LineTo(fc.tempToX(fc.points[7].Temp), fc.pwmToY(0))
	cr.ClosePath()
	cr.Fill()

	// Line connecting points.
	cr.SetSourceRGBA(0.8, 0.1, 0.1, 1) // accent color
	cr.SetLineWidth(2)
	for i, p := range fc.points {
		x := fc.tempToX(p.Temp)
		y := fc.pwmToY(p.PWM)
		if i == 0 {
			cr.MoveTo(x, y)
		} else {
			cr.LineTo(x, y)
		}
	}
	cr.Stroke()

	// Point circles.
	for i, p := range fc.points {
		x := fc.tempToX(p.Temp)
		y := fc.pwmToY(p.PWM)
		radius := 6.0
		if i == fc.dragging || i == fc.hovered {
			radius = 8.0
			// Outer ring.
			cr.SetSourceRGBA(1, 1, 1, 0.6)
			cr.Arc(x, y, radius+2, 0, 2*math.Pi)
			cr.Stroke()
		}
		cr.SetSourceRGBA(0.8, 0.1, 0.1, 1)
		cr.Arc(x, y, radius, 0, 2*math.Pi)
		cr.Fill()
	}

	// Current operating point, interpolated along the configured curve.
	if fc.w != nil && fc.w.state != nil {
		temp := fc.w.state.Temperature
		if temp >= 35 && temp <= 105 {
			cr.SetSourceRGBA(1, 1, 1, 0.9)
			cr.SetLineWidth(2)
			cr.Arc(fc.tempToX(temp), fc.pwmToY(fanPWMAtTemp(fc.points, temp)), 7, 0, 2*math.Pi)
			cr.Stroke()
		}
	}
}

// newFanCurveEditor creates the DrawingArea and sets up input handling.
func (w *Window) newFanCurveEditor() *fanCurveEditor {
	fc := &fanCurveEditor{
		dragging: -1,
		hovered:  -1,
		w:        w,
		points:   defaultFanCurve(),
	}

	fc.area = gtk.NewDrawingArea()
	fc.area.AddCSSClass("fan-curve-area")
	fc.area.SetSizeRequest(-1, 280)
	fc.area.SetVExpand(true) // the Fan page has the whole height to itself
	fc.area.SetDrawFunc(func(_ *gtk.DrawingArea, cr *cairo.Context, width, height int) {
		fc.draw(cr, width, height)
	})

	// Drag gesture for point dragging (CAPTURE phase for gamescope touch).
	drag := gtk.NewGestureDrag()
	drag.SetPropagationPhase(gtk.PhaseCapture)

	var startX, startY float64

	drag.ConnectDragBegin(func(x, y float64) {
		idx := fc.hitTest(x, y)
		if idx < 0 {
			drag.SetState(gtk.EventSequenceDenied)
			return
		}
		fc.dragging = idx
		startX, startY = x, y
		fc.area.QueueDraw()
	})

	drag.ConnectDragUpdate(func(offsetX, offsetY float64) {
		if fc.dragging < 0 {
			return
		}
		x := startX + offsetX
		y := startY + offsetY
		fc.points[fc.dragging].Temp = fc.xToTemp(x)
		fc.points[fc.dragging].PWM = fc.yToPWM(y)
		fc.enforceConstraints(fc.dragging)
		fc.area.QueueDraw()
		if fc.w != nil {
			fc.w.refreshTuningDirty()
		}
	})

	drag.ConnectDragEnd(func(_, _ float64) {
		fc.dragging = -1
		fc.area.QueueDraw()
	})

	fc.area.AddController(drag)

	// Hover tracking (mouse only).
	motion := gtk.NewEventControllerMotion()
	motion.ConnectMotion(func(x, y float64) {
		if fc.dragging >= 0 {
			return
		}
		prev := fc.hovered
		fc.hovered = fc.hitTest(x, y)
		if fc.hovered != prev {
			fc.area.QueueDraw()
		}
	})
	motion.ConnectLeave(func() {
		if fc.hovered >= 0 {
			fc.hovered = -1
			fc.area.QueueDraw()
		}
	})
	fc.area.AddController(motion)

	return fc
}

// buildCustomView builds the advanced TDP/fan/undervolt view.
// sectionHeader makes a section label with a trailing "MODIFIED" marker that is
// shown when the section has unsaved changes.
func sectionHeader(title string) (*gtk.Box, *gtk.Label) {
	row := gtk.NewBox(gtk.OrientationHorizontal, 6)
	l := gtk.NewLabel(title)
	l.SetHAlign(gtk.AlignStart)
	l.SetHExpand(true)
	l.AddCSSClass("section-label")
	m := gtk.NewLabel("MODIFIED")
	m.AddCSSClass("dirty-mark")
	m.SetVisible(false)
	row.Append(l)
	row.Append(m)
	return row, m
}

// tuningActionRow builds a [Save][Reset] row for one tuning section. The buttons
// are created here and assigned through the provided pointers.
func (w *Window) tuningActionRow(save **gtk.Button, saveLabel string, onSave func(), reset **gtk.Button, resetLabel string, onReset func()) *gtk.Box {
	row := gtk.NewBox(gtk.OrientationHorizontal, 4)
	row.AddCSSClass("custom-actions")
	s := gtk.NewButtonWithLabel(saveLabel)
	s.AddCSSClass("suggested-action")
	s.SetHExpand(true)
	s.ConnectClicked(onSave)
	*save = s
	r := gtk.NewButtonWithLabel(resetLabel)
	r.SetHExpand(true)
	r.ConnectClicked(onReset)
	*reset = r
	row.Append(s)
	row.Append(r)
	return row
}

func (w *Window) buildCustomView() *gtk.Box {
	view := gtk.NewBox(gtk.OrientationVertical, 0)

	header, back, _ := viewHeader("Power Tuning", func() { w.showMainView() })
	w.customBackBtn = back
	view.Append(header)

	telemetry := gtk.NewBox(gtk.OrientationHorizontal, 8)
	telemetry.AddCSSClass("card")
	telemetry.AddCSSClass("tuning-telemetry")
	telemetry.SetMarginTop(4)
	telemetry.SetMarginBottom(4)
	telemetry.SetMarginStart(12)
	telemetry.SetMarginEnd(12)
	w.telemetryTempLabel = gtk.NewLabel("APU --°C")
	w.telemetryTempLabel.SetHAlign(gtk.AlignStart)
	w.telemetryTempLabel.SetHExpand(true)
	w.telemetryTempLabel.AddCSSClass("telemetry-value")
	w.telemetryPowerLabel = gtk.NewLabel("POWER —")
	w.telemetryPowerLabel.SetHAlign(gtk.AlignCenter)
	w.telemetryPowerLabel.SetHExpand(true)
	w.telemetryPowerLabel.AddCSSClass("telemetry-value")
	w.telemetryFanLabel = gtk.NewLabel("FAN -- RPM")
	w.telemetryFanLabel.SetHAlign(gtk.AlignEnd)
	w.telemetryFanLabel.SetHExpand(true)
	w.telemetryFanLabel.AddCSSClass("telemetry-value")
	telemetry.Append(w.telemetryTempLabel)
	telemetry.Append(w.telemetryPowerLabel)
	telemetry.Append(w.telemetryFanLabel)
	view.Append(telemetry)

	// One page per tuning area; only one is on screen so nothing needs a long scroll.
	newPage := func() *gtk.Box {
		b := gtk.NewBox(gtk.OrientationVertical, 8)
		b.SetMarginTop(4)
		b.SetMarginBottom(8)
		b.SetMarginStart(12)
		b.SetMarginEnd(16) // clear the overlay scrollbar
		return b
	}
	content, uvPage, fanPage := newPage(), newPage(), newPage()
	cpuPage := newPage()

	// --- TDP ---
	tdpHdr, tdpMark := sectionHeader("TDP")
	w.tdpDirtyMark = tdpMark
	content.Append(tdpHdr)

	// Simple / Advanced segmented control (a radio pair styled like the RGB device
	// picker). Advanced swaps the single slider for PL1/PL2/PL3 in place.
	modeRow := gtk.NewBox(gtk.OrientationHorizontal, 4)
	segment := func(label string) *gtk.CheckButton {
		b := gtk.NewCheckButton()
		l := gtk.NewLabel(label)
		l.SetHExpand(true)
		b.SetChild(l)
		b.AddCSSClass("tab-btn")
		b.SetHExpand(true)
		modeRow.Append(b)
		return b
	}
	w.tdpSimpleCheck = segment("Simple")
	w.tdpSimpleCheck.SetActive(true)
	w.tdpAdvancedCheck = segment("Advanced (PL1/2/3)")
	w.tdpAdvancedCheck.SetGroup(w.tdpSimpleCheck)
	if w.gamescope {
		addTouchActivate(w.tdpSimpleCheck, func() { w.tdpSimpleCheck.SetActive(true) })
		addTouchActivate(w.tdpAdvancedCheck, func() { w.tdpAdvancedCheck.SetActive(true) })
	}
	content.Append(modeRow)

	// Basic TDP box (visible by default).
	tdpBasicBox := gtk.NewBox(gtk.OrientationVertical, 4)
	w.tdpBasicScale = gtk.NewScaleWithRange(gtk.OrientationHorizontal, tdpMin, tdpMaxBasic, 1)
	w.tdpBasicScale.SetDigits(0)
	w.tdpBasicScale.SetDrawValue(false)
	w.tdpBasicScale.SetValue(float64(50))
	w.tdpBasicScale.SetFocusable(false)
	w.tdpBasicLabel = gtk.NewLabel("50 W")
	w.tdpBasicLabel.AddCSSClass("scale-value")
	w.tdpBasicScale.ConnectValueChanged(func() {
		w.tdpBasicLabel.SetLabel(fmt.Sprintf("%d W", int(w.tdpBasicScale.Value())))
		if !w.syncing {
			w.refreshTuningDirty()
		}
	})
	basicName := gtk.NewLabel("TDP")
	basicName.SetHAlign(gtk.AlignStart)
	basicName.SetHExpand(true)
	basicName.AddCSSClass("scale-name")
	w.tdpBasicLabel.SetHAlign(gtk.AlignEnd)
	basicHeader := gtk.NewBox(gtk.OrientationHorizontal, 4)
	basicHeader.Append(basicName)
	basicHeader.Append(w.tdpBasicLabel)
	tdpBasicBox.Append(basicHeader)
	tdpBasicBox.Append(w.tdpBasicScale)
	content.Append(tdpBasicBox)

	// Advanced box (hidden by default) — replaces basic slider in-place.
	w.tdpAdvancedBox = gtk.NewBox(gtk.OrientationVertical, 4)
	w.tdpAdvancedBox.SetVisible(false)

	// Shown only while a limit is above 75 W (refreshTuningDirty), so the one
	// real danger is the only red on the page.
	w.tdpWarnLabel = gtk.NewLabel("Values above 75W may throttle or damage hardware. Use caution.")
	w.tdpWarnLabel.SetWrap(true)
	w.tdpWarnLabel.SetHAlign(gtk.AlignStart)
	w.tdpWarnLabel.AddCSSClass("tdp-warning")
	w.tdpWarnLabel.SetVisible(false)
	w.tdpAdvancedBox.Append(w.tdpWarnLabel)

	w.tdpPL1Scale, w.tdpPL1Label = w.buildTdpScale("PL1 (SPL)", "Sustained power limit — the long-term average power the CPU targets.")
	w.tdpPL2Scale, w.tdpPL2Label = w.buildTdpScale("PL2 (SPPT)", "Short boost — maximum power during brief burst workloads.")
	w.tdpPL3Scale, w.tdpPL3Label = w.buildTdpScale("PL3 (FPPT)", "Fast boost — peak instantaneous power for single-threaded spikes.")
	content.Append(w.tdpAdvancedBox)

	// Basic-mode clamp guard: shown when active sustained power exceeds the basic range.
	w.tdpClampWarn = gtk.NewLabel("Current sustained power is above the basic range — switch to Advanced to edit without losing it.")
	w.tdpClampWarn.SetWrap(true)
	w.tdpClampWarn.SetHAlign(gtk.AlignStart)
	w.tdpClampWarn.AddCSSClass("tdp-warning")
	w.tdpClampWarn.SetVisible(false)
	content.Append(w.tdpClampWarn)

	w.tdpAdvancedCheck.ConnectToggled(func() {
		adv := w.tdpAdvancedCheck.Active()
		w.tdpAdvancedBox.SetVisible(adv)
		tdpBasicBox.SetVisible(!adv)
		if !w.syncing {
			w.refreshTuningDirty()
		}
	})

	// Actions are pinned under each page, outside its scroll.
	tdpActions := w.tuningActionRow(&w.saveTdpBtn, "Save TDP", w.saveCustomTdp, &w.resetTdpBtn, "Reset TDP", w.resetTdp)

	// --- UNDERVOLT (own section; hidden when unavailable) ---
	w.uvBox = gtk.NewBox(gtk.OrientationVertical, 4)
	w.uvBox.SetVisible(false)
	uvHdr, uvMark := sectionHeader("UNDERVOLT")
	w.uvDirtyMark = uvMark
	w.uvBox.Append(uvHdr)
	uvWarn := gtk.NewLabel("Independent override. Unstable values may cause crashes.")
	uvWarn.SetWrap(true)
	uvWarn.SetHAlign(gtk.AlignStart)
	uvWarn.AddCSSClass("tdp-warning")
	w.uvBox.Append(uvWarn)
	w.uvCpuScale, w.uvCpuLabel = w.buildUvScale("CPU Curve Optimizer", -40, 0)
	uvActions := w.tuningActionRow(&w.saveUvBtn, "Save UV", w.saveUndervolt, &w.resetUvBtn, "Reset UV", w.resetUndervolt)
	uvPage.Append(w.uvBox)

	// --- FAN CURVE ---
	fanHdr, fanMark := sectionHeader("FAN CURVE")
	w.fanDirtyMark = fanMark
	w.fanModeLabel = gtk.NewLabel("—")
	w.fanModeLabel.SetVAlign(gtk.AlignCenter)
	w.fanModeLabel.AddCSSClass("pill")
	fanHdr.Append(w.fanModeLabel)
	fanPage.Append(fanHdr)

	w.fanSafetyBanner = gtk.NewLabel("Above 75 W the safety fan curve runs. To use your own, keep every point at 50% or higher.")
	w.fanSafetyBanner.SetWrap(true)
	w.fanSafetyBanner.SetHAlign(gtk.AlignStart)
	w.fanSafetyBanner.AddCSSClass("tdp-warning")
	w.fanSafetyBanner.SetVisible(false)
	fanPage.Append(w.fanSafetyBanner)

	w.fanCurve = w.newFanCurveEditor()
	fanPage.Append(w.fanCurve.area)
	fanActions := w.tuningActionRow(&w.saveFanBtn, "Save Fan Curve", w.saveCustomFanCurve, &w.resetFanBtn, "Reset Fan Curve", w.resetFanCurve)

	// --- CPU (rarely used) ---
	cpuPage.Append(w.buildCPUTuningBox())

	w.resetAllBtn = gtk.NewButtonWithLabel("Reset All Overrides")
	w.resetAllBtn.AddCSSClass("action-btn")
	w.resetAllBtn.ConnectClicked(func() {
		w.showConfirmation(
			"Reset All Overrides",
			"Reset fan control to Auto, TDP to firmware defaults, and undervolt to stock? Profile, display, and charge limit are not changed.",
			"Reset All",
			"custom",
			false,
			func() { w.runStateAction("reset all tuning overrides", api.SendTuningReset) },
		)
	})
	cpuPage.Append(w.resetAllBtn)

	tabRow := gtk.NewBox(gtk.OrientationHorizontal, 4)
	tabRow.SetMarginStart(12)
	tabRow.SetMarginEnd(12)
	tabRow.SetMarginBottom(4)
	tabRow.AddCSSClass("main-tab-row")
	view.Append(tabRow)

	w.tuningStack = gtk.NewStack()
	w.tuningStack.SetTransitionType(gtk.StackTransitionTypeNone)
	w.tuningStack.SetVExpand(true)
	for _, p := range []struct {
		name, label string
		body        *gtk.Box
		actions     gtk.Widgetter
	}{
		{"tdp", "TDP", content, tdpActions},
		{"fan", "Fan", fanPage, fanActions},
		{"uv", "UV", uvPage, uvActions},
		{"cpu", "CPU", cpuPage, nil},
	} {
		scroll := gtk.NewScrolledWindow()
		scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
		scroll.SetVExpand(true)
		scroll.SetChild(p.body)
		page := gtk.NewBox(gtk.OrientationVertical, 0)
		page.Append(scroll)
		if p.actions != nil {
			page.Append(p.actions)
		}
		w.tuningScrolls[p.name] = scroll
		w.tuningStack.AddNamed(page, p.name)

		btn := gtk.NewButtonWithLabel(p.label)
		btn.SetHExpand(true)
		name := p.name
		btn.ConnectClicked(func() { w.switchTuningTab(name) })
		w.tuningTabBtns[name] = btn
		tabRow.Append(btn)
	}
	view.Append(w.tuningStack)
	w.customScroll = w.tuningScrolls[w.tuningTab]
	w.tuningStack.SetVisibleChildName(w.tuningTab)
	setActiveButton(w.tuningTabBtns, w.tuningTab)

	return view
}

// switchTuningTab shows one tuning page and rebuilds the gamepad focus list.
func (w *Window) switchTuningTab(name string) {
	if w.tuningStack == nil {
		return
	}
	w.tuningTab = name
	w.tuningStack.SetVisibleChildName(name)
	w.customScroll = w.tuningScrolls[name]
	setActiveButton(w.tuningTabBtns, name)
	w.buildCustomFocusList()
	w.swapFocusList(w.customFocusItems)
}

// buildTdpScale creates a labeled TDP slider (5–93W) and appends it to the
// advanced box. Returns the scale and value label.
func (w *Window) buildTdpScale(label, desc string) (*gtk.Scale, *gtk.Label) {
	// One header line: name left, live value right. The long description moves
	// to the tooltip, saving about 55px per slider.
	nameLabel := gtk.NewLabel(label)
	nameLabel.SetHAlign(gtk.AlignStart)
	nameLabel.SetHExpand(true)
	nameLabel.AddCSSClass("scale-name")
	nameLabel.SetTooltipText(desc)
	header := gtk.NewBox(gtk.OrientationHorizontal, 4)
	header.SetTooltipText(desc)
	header.Append(nameLabel)
	w.tdpAdvancedBox.Append(header)
	sc := gtk.NewScaleWithRange(gtk.OrientationHorizontal, tdpMin, tdpMaxAdvanced, 1)
	sc.SetDigits(0)
	sc.SetDrawValue(false)
	sc.SetValue(50)
	sc.SetFocusable(false)
	valLabel := gtk.NewLabel("50 W")
	valLabel.AddCSSClass("scale-value")
	sc.ConnectValueChanged(func() {
		valLabel.SetLabel(fmt.Sprintf("%d W", int(sc.Value())))
		if !w.syncing {
			w.refreshTuningDirty()
		}
	})
	valLabel.SetHAlign(gtk.AlignEnd)
	header.Append(valLabel)
	w.tdpAdvancedBox.Append(sc)
	return sc, valLabel
}

// buildUvScale creates a labeled undervolt slider and appends it to uvBox.
func (w *Window) buildUvScale(label string, lo, hi float64) (*gtk.Scale, *gtk.Label) {
	nameLabel := gtk.NewLabel(label)
	nameLabel.SetHAlign(gtk.AlignStart)
	nameLabel.SetHExpand(true)
	nameLabel.AddCSSClass("scale-name")
	header := gtk.NewBox(gtk.OrientationHorizontal, 4)
	header.Append(nameLabel)
	w.uvBox.Append(header)
	sc := gtk.NewScaleWithRange(gtk.OrientationHorizontal, lo, hi, 1)
	sc.SetDigits(0)
	sc.SetDrawValue(false)
	sc.SetValue(0)
	sc.SetFocusable(false)
	valLabel := gtk.NewLabel(uvLabel(label, 0))
	valLabel.AddCSSClass("scale-value")
	sc.ConnectValueChanged(func() {
		valLabel.SetLabel(uvLabel(label, int(sc.Value())))
		if !w.syncing {
			w.refreshTuningDirty()
		}
	})
	valLabel.SetHAlign(gtk.AlignEnd)
	header.Append(valLabel)
	w.uvBox.Append(sc)
	return sc, valLabel
}

// uvLabel formats an undervolt value label, e.g. "CPU Curve Optimizer: -20" or "... 0 (stock)".
func uvLabel(name string, val int) string {
	if val == 0 {
		return fmt.Sprintf("%s: 0 (stock)", name)
	}
	return fmt.Sprintf("%s: %d", name, val)
}

// highTDPMinPWM mirrors z13ctl-plus cli.HighTDPMinPWM: above 75 W a user
// fan curve is applied only if every point is at or above it.
const highTDPMinPWM = 127

// curveMeetsHighTDPFloor mirrors the daemon rule of the same name.
func curveMeetsHighTDPFloor(points []api.FanCurvePoint) bool {
	for _, p := range points {
		if p.PWM < highTDPMinPWM {
			return false
		}
	}
	return len(points) > 0
}

// tuningSnapshot is the effective saved tuning, with daemon "unset" (nil)
// mapped to the values the widgets show for it.
type tuningSnapshot struct {
	tdp   [3]int // PL1, PL2, PL3
	fan   [8]api.FanCurvePoint
	cpuCO int
}

// tuningTarget derives the saved tuning from daemon state. TDP and FanCurve
// are nil after a reset, which means 50 W and the default curve.
func tuningTarget(s *api.State) tuningSnapshot {
	t := tuningSnapshot{tdp: [3]int{50, 50, 50}, fan: defaultFanCurve()}
	if s == nil {
		return t
	}
	if s.TDP != nil {
		t.tdp = [3]int{s.TDP.PL1SPL, s.TDP.PL2SPPT, s.TDP.FPPT}
	}
	if s.FanCurve != nil && len(s.FanCurve.Points) == 8 {
		copy(t.fan[:], s.FanCurve.Points)
	}
	if s.UndervoltActive && s.Undervolt != nil {
		t.cpuCO = s.Undervolt.CPUCO
	}
	return t
}

// syncCustomView populates the tuning widgets from daemon state. Only
// sections whose saved value changed since the last sync are rewritten, so
// unsaved edits survive unrelated actions and polls; set w.tuningSynced = nil
// to force a full rewrite.
func (w *Window) syncCustomView() {
	if w.state == nil {
		return
	}
	prev := w.syncing
	w.syncing = true
	defer func() { w.syncing = prev }()

	t := tuningTarget(w.state)
	last := w.tuningSynced
	w.tuningSynced = &t

	// TDP.
	if last == nil || last.tdp != t.tdp {
		pl1, pl2, pl3 := t.tdp[0], t.tdp[1], t.tdp[2]
		if w.tdpBasicScale != nil {
			v := min(float64(pl1), tdpMaxBasic)
			w.tdpBasicScale.SetValue(v)
			w.tdpBasicLabel.SetLabel(fmt.Sprintf("%d W", int(v)))
		}
		if w.tdpPL1Scale != nil {
			w.tdpPL1Scale.SetValue(float64(pl1))
			w.tdpPL1Label.SetLabel(fmt.Sprintf("%d W", pl1))
		}
		if w.tdpPL2Scale != nil {
			w.tdpPL2Scale.SetValue(float64(pl2))
			w.tdpPL2Label.SetLabel(fmt.Sprintf("%d W", pl2))
		}
		if w.tdpPL3Scale != nil {
			w.tdpPL3Scale.SetValue(float64(pl3))
			w.tdpPL3Label.SetLabel(fmt.Sprintf("%d W", pl3))
		}
	}

	// Fan curve.
	if w.fanCurve != nil && (last == nil || last.fan != t.fan) {
		w.fanCurve.points = t.fan
		w.fanCurve.area.QueueDraw()
	}

	// Undervolt (own section now; visibility is independent of the Advanced checkbox).
	if w.uvBox != nil {
		w.uvBox.SetVisible(w.state.UndervoltAvailable)
		if btn := w.tuningTabBtns["uv"]; btn != nil {
			btn.SetVisible(w.state.UndervoltAvailable)
		}
		if !w.state.UndervoltAvailable && w.tuningTab == "uv" {
			w.tuningTab = "tdp"
			w.tuningStack.SetVisibleChildName("tdp")
			w.customScroll = w.tuningScrolls["tdp"]
			setActiveButton(w.tuningTabBtns, "tdp")
		}
	}
	if w.uvCpuScale != nil && (last == nil || last.cpuCO != t.cpuCO) {
		w.uvCpuScale.SetValue(float64(t.cpuCO))
		w.uvCpuLabel.SetLabel(uvLabel("CPU Curve Optimizer", t.cpuCO))
	}

	w.syncTuningTelemetry(w.state)
	w.syncCPUPower() // CPU page widgets exist only once this view is built

	// Refresh Save sensitivity, dirty markers, and conditional banners.
	w.refreshTuningDirty()
}

// saveCustomTdp commits only the TDP values.
// refreshTuningDirty updates Save sensitivity, dirty markers, and conditional
// banners based on whether each section's widgets differ from the saved state.
func (w *Window) refreshTuningDirty() {
	if w.saveTdpBtn == nil || w.saveFanBtn == nil || w.saveUvBtn == nil {
		return
	}
	// TDP.
	clamped := w.tdpBasicCapped()
	tdpDirty := w.tdpDirty()
	if w.tdpClampWarn != nil {
		w.tdpClampWarn.SetVisible(clamped)
	}
	w.saveTdpBtn.SetSensitive(tdpDirty && !clamped)
	toggleMark(w.tdpDirtyMark, tdpDirty && !clamped)
	if w.tdpWarnLabel != nil {
		high := w.tdpPL1Scale.Value() > 75 || w.tdpPL2Scale.Value() > 75 || w.tdpPL3Scale.Value() > 75
		w.tdpWarnLabel.SetVisible(high)
	}

	// Fan.
	// Under the high-TDP safety lock the daemon still honours a user curve
	// that meets the PWM floor, so only block saving curves below it.
	fanLocked := w.state != nil && w.state.FanSafetyActive
	fanSaveable := w.fanDirty() && (!fanLocked || (w.fanCurve != nil && curveMeetsHighTDPFloor(w.fanCurve.points[:])))
	if w.fanSafetyBanner != nil {
		w.fanSafetyBanner.SetVisible(fanLocked)
	}
	w.saveFanBtn.SetSensitive(fanSaveable)
	w.resetFanBtn.SetSensitive(!fanLocked)
	toggleMark(w.fanDirtyMark, fanSaveable)

	// Undervolt.
	uvDirty := w.uvDirty()
	w.saveUvBtn.SetSensitive(uvDirty)
	toggleMark(w.uvDirtyMark, uvDirty)

	// A dot on the tab names pages with unsaved edits, even when another page is open.
	for name, dirty := range map[string]bool{"tdp": tdpDirty && !clamped, "fan": fanSaveable, "uv": uvDirty} {
		if btn := w.tuningTabBtns[name]; btn != nil {
			label := map[string]string{"tdp": "TDP", "fan": "Fan", "uv": "UV"}[name]
			if dirty {
				label += " ●"
			}
			btn.SetLabel(label)
		}
	}
}

func toggleMark(mark *gtk.Label, dirty bool) {
	if mark != nil {
		mark.SetVisible(dirty)
	}
}

// tdpDirty reports whether the TDP sliders differ from the saved state.
func (w *Window) tdpDirty() bool {
	tdp := tuningTarget(w.state).tdp
	pl1, pl2, pl3 := tdp[0], tdp[1], tdp[2]
	if w.tdpAdvancedCheck != nil && w.tdpAdvancedCheck.Active() {
		return int(w.tdpPL1Scale.Value()) != pl1 ||
			int(w.tdpPL2Scale.Value()) != pl2 ||
			int(w.tdpPL3Scale.Value()) != pl3
	}
	if w.tdpBasicScale == nil {
		return false
	}
	// Basic slider can't represent values above its max — guarded separately.
	if pl1 > tdpMaxBasic {
		return false
	}
	return int(w.tdpBasicScale.Value()) != pl1
}

// tdpBasicCapped reports whether the active sustained power exceeds the basic
// slider range (so basic editing would silently clobber it).
func (w *Window) tdpBasicCapped() bool {
	if w.tdpAdvancedCheck != nil && w.tdpAdvancedCheck.Active() {
		return false
	}
	if w.state == nil || w.state.TDP == nil {
		return false
	}
	return w.state.TDP.PL1SPL > tdpMaxBasic
}

// fanDirty reports whether the edited fan curve differs from the saved state.
func (w *Window) fanDirty() bool {
	if w.fanCurve == nil {
		return false
	}
	return w.fanCurve.points != tuningTarget(w.state).fan
}

// uvDirty reports whether the Curve Optimizer slider differs from the saved state.
func (w *Window) uvDirty() bool {
	if w.uvCpuScale == nil {
		return false
	}
	return int(w.uvCpuScale.Value()) != tuningTarget(w.state).cpuCO
}

// applyTempColor sets the telemetry temp label and a threshold-based color class.
func applyTempColor(label *gtk.Label, temp int) {
	if label == nil {
		return
	}
	for _, c := range []string{"temp-ok", "temp-warn", "temp-hot"} {
		label.RemoveCSSClass(c)
	}
	if temp <= 0 {
		label.SetLabel("APU --°C")
		return
	}
	label.SetLabel(fmt.Sprintf("APU %d°C", temp))
	switch {
	case temp >= 85:
		label.AddCSSClass("temp-hot")
	case temp >= 70:
		label.AddCSSClass("temp-warn")
	default:
		label.AddCSSClass("temp-ok")
	}
}

func (w *Window) syncTuningTelemetry(state *api.State) {
	if state == nil {
		return
	}
	applyTempColor(w.telemetryTempLabel, state.Temperature)
	if w.telemetryPowerLabel != nil {
		value := "POWER —"
		if state.Telemetry != nil && state.Telemetry.APUPowerAvailable {
			value = fmt.Sprintf("POWER %.1f W", state.Telemetry.APUPowerW)
		}
		w.telemetryPowerLabel.SetLabel(value)
	}
	if w.telemetryFanLabel != nil {
		w.telemetryFanLabel.SetLabel(fmt.Sprintf("FAN %d RPM", state.FanRPM))
	}
	if w.fanModeLabel != nil {
		_, mode, _, _ := fanStatus(state)
		if !state.FanCurveActive && !state.FanSafetyActive && state.Profile != "" {
			mode = "AUTO · " + strings.ToUpper(state.Profile)
		}
		w.fanModeLabel.SetLabel(mode)
		w.fanModeLabel.RemoveCSSClass("danger")
		if state.FanSafetyActive {
			w.fanModeLabel.AddCSSClass("danger")
		}
	}
}

func (w *Window) saveCustomTdp() {
	if w.tdpAdvancedCheck != nil && w.tdpAdvancedCheck.Active() {
		pl1 := fmt.Sprintf("%d", int(w.tdpPL1Scale.Value()))
		pl2 := fmt.Sprintf("%d", int(w.tdpPL2Scale.Value()))
		pl3 := fmt.Sprintf("%d", int(w.tdpPL3Scale.Value()))
		maxPL := int(math.Max(w.tdpPL1Scale.Value(), math.Max(w.tdpPL2Scale.Value(), w.tdpPL3Scale.Value())))
		w.runStateAction("save TDP override", func() (bool, error) {
			return api.SendTdpSet(pl1, pl1, pl2, pl3, maxPL > tdpMaxSafe)
		})
		return
	}
	watts := fmt.Sprintf("%d", int(w.tdpBasicScale.Value()))
	w.runStateAction("save TDP override", func() (bool, error) {
		return api.SendTdpSet(watts, "", "", "", false)
	})
}

// saveCustomFanCurve commits only the fan curve.
func (w *Window) saveCustomFanCurve() {
	curve := w.fanCurve.curveString()
	w.runStateAction("save fan curve override", func() (bool, error) { return api.SendFanCurveSet(curve) })
}

// resetTdp resets TDP to firmware defaults.
func (w *Window) resetTdp() {
	w.showConfirmation("Reset TDP", "Reset TDP to firmware defaults?", "Reset", "custom", false, func() {
		w.runStateAction("reset TDP override", api.SendTdpReset)
	})
}

// resetFanCurve resets fan curves to firmware auto mode.
func (w *Window) resetFanCurve() {
	w.showConfirmation("Reset Fan Curve", "Reset fan control to Auto?", "Reset", "custom", false, func() {
		w.runStateAction("reset fan curve override", api.SendFanCurveReset)
	})
}

// saveUndervolt commits the current Curve Optimizer offsets to the daemon.
func (w *Window) saveUndervolt() {
	cpu := fmt.Sprintf("%d", int(w.uvCpuScale.Value()))
	w.runStateAction("save undervolt override", func() (bool, error) { return api.SendUndervoltSet(cpu) })
}

// resetUndervolt resets Curve Optimizer to stock (0).
func (w *Window) resetUndervolt() {
	w.showConfirmation("Reset Undervolt", "Reset Curve Optimizer to stock (0)?", "Reset", "custom", false, func() {
		w.runStateAction("reset undervolt override", api.SendUndervoltReset)
	})
}

// startTelemetryPolling keeps live telemetry and power automation state current
// while their corresponding views are visible.
func (w *Window) startTelemetryPolling() {
	w.telemetryGen++
	gen := w.telemetryGen
	glib.TimeoutAdd(1000, func() bool {
		if gen != w.telemetryGen || !w.visible {
			return false
		}
		w.syncing = true
		w.syncTablet()
		w.syncing = false
		view, tab := "", ""
		if w.viewStack != nil {
			view = w.viewStack.VisibleChildName()
		}
		if view == "main" && w.tabStack != nil {
			tab = w.tabStack.VisibleChildName()
		}
		customActive := view == "custom"
		overviewActive := tab == "overview"
		powerActive := tab == "power" || view == "presets" || view == "preset-detail" || view == "confirm"
		systemActive := tab == "system"
		rgbActive := tab == "rgb"
		if (!customActive && !overviewActive && !powerActive && !systemActive && !rgbActive) || w.stateActionBusy || w.telemetryPollBusy {
			return true
		}
		w.telemetryPollBusy = true
		w.stateRequestGen++
		requestGen := w.stateRequestGen
		go func() {
			ok, state, err := api.SendGetState()
			if !ok || err != nil {
				glib.IdleAdd(func() {
					w.telemetryPollBusy = false
					if gen == w.telemetryGen && overviewActive {
						w.markOverviewStale()
					}
				})
				return
			}
			glib.IdleAdd(func() {
				w.telemetryPollBusy = false
				if gen != w.telemetryGen || requestGen != w.stateRequestGen || w.stateActionBusy {
					return
				}
				presetsChanged := presetStateChanged(w.state, state)
				w.state = state
				w.syncing = true
				w.syncTablet()
				w.syncing = false
				if overviewActive {
					if state.BatteryDetail != nil {
						w.updateBatteryHero(state.BatteryDetail)
					}
					w.syncOverviewTelemetry()
				}
				if powerActive {
					w.syncPowerState(presetsChanged)
				}
				if systemActive {
					w.syncing = true
					w.syncRefreshRate()
					w.syncOverdrive()
					w.syncBootSound()
					w.syncing = false
				}
				if rgbActive {
					w.syncLightingSection()
				}

				if customActive {
					w.syncFanPreset()
					// Picks up tuning changed underneath (automation, other
					// clients) and refreshes Save/safety-lock state; edits in
					// unchanged sections are kept.
					w.syncCustomView()
					if w.fanCurve != nil {
						w.fanCurve.area.QueueDraw()
					}
				}
			})
		}()
		return true
	})
}

// buildCustomFocusList builds the 2D focus grid for the advanced tuning view.
func (w *Window) buildCustomFocusList() {
	var items []focusItem

	// Row 0: back button.
	items = append(items, focusItem{
		widget: w.customBackBtn, row: 0, col: 0,
		section:    "nav",
		onActivate: func() { w.showMainView() },
	})

	// Row 1: sub-tab buttons (hidden ones are skipped via isVisible).
	for col, name := range []string{"tdp", "fan", "uv", "cpu"} {
		btn := w.tuningTabBtns[name]
		if btn == nil {
			continue
		}
		items = append(items, focusItem{
			widget: btn, row: 1, col: col,
			section:    "tuning-tabs",
			isVisible:  btn.IsVisible,
			onActivate: func() { btn.Activate() },
		})
	}

	// Page contents. Each page lists only its own widgets from row 2 down; the
	// focus list is rebuilt on every sub-tab switch.
	switch w.tuningTab {
	case "tdp":
		if w.tdpBasicScale != nil {
			oL, oR, gV, sV := scaleAdjust(w.tdpBasicScale, 5)
			items = append(items, focusItem{
				widget: w.tdpBasicScale, row: 3, col: 0,
				section:  "tdp",
				editable: true,
				onLeft:   oL, onRight: oR,
				getValue: gV, setValue: sV,
				isVisible: func() bool { return w.tdpBasicScale.IsVisible() },
			})
		}
		if w.tdpAdvancedCheck != nil {
			items = append(items, focusItem{
				widget: w.tdpSimpleCheck, row: 2, col: 0,
				section:    "tdp",
				onActivate: func() { w.tdpSimpleCheck.SetActive(true) },
			}, focusItem{
				widget: w.tdpAdvancedCheck, row: 2, col: 1,
				section:    "tdp",
				onActivate: func() { w.tdpAdvancedCheck.SetActive(true) },
			})
		}
		advVis := func() bool { return w.tdpAdvancedBox.IsVisible() }
		for i, sc := range []*gtk.Scale{w.tdpPL1Scale, w.tdpPL2Scale, w.tdpPL3Scale} {
			oL, oR, gV, sV := scaleAdjust(sc, 1)
			items = append(items, focusItem{
				widget: sc, row: 4 + i, col: 0,
				section:   "tdp",
				editable:  true,
				isVisible: advVis,
				onLeft:    oL, onRight: oR,
				getValue: gV, setValue: sV,
			})
		}
		items = append(items, focusItem{
			widget: w.saveTdpBtn, row: 7, col: 0,
			section: "tdp", onActivate: func() { w.saveTdpBtn.Activate() },
		}, focusItem{
			widget: w.resetTdpBtn, row: 7, col: 1,
			section: "tdp", onActivate: func() { w.resetTdpBtn.Activate() },
		})
	case "uv":
		if w.uvCpuScale != nil {
			oL, oR, gV, sV := scaleAdjust(w.uvCpuScale, 1)
			items = append(items, focusItem{
				widget: w.uvCpuScale, row: 2, col: 0,
				section: "undervolt", editable: true,
				onLeft: oL, onRight: oR, getValue: gV, setValue: sV,
			})
		}
		items = append(items, focusItem{
			widget: w.saveUvBtn, row: 3, col: 0,
			section: "undervolt", onActivate: func() { w.saveUvBtn.Activate() },
		}, focusItem{
			widget: w.resetUvBtn, row: 3, col: 1,
			section: "undervolt", onActivate: func() { w.resetUvBtn.Activate() },
		})
	case "fan":
		if w.fanCurve != nil {
			// Navigable but not editable via gamepad; touch/mouse drag edits.
			items = append(items, focusItem{widget: w.fanCurve.area, row: 2, col: 0, section: "fan"})
		}
		items = append(items, focusItem{
			widget: w.saveFanBtn, row: 3, col: 0,
			section: "fan", onActivate: func() { w.saveFanBtn.Activate() },
		}, focusItem{
			widget: w.resetFanBtn, row: 3, col: 1,
			section: "fan", onActivate: func() { w.resetFanBtn.Activate() },
		})
	case "cpu":
		row := 2
		if w.cpuMinScale != nil {
			left, right, get, set := scaleAdjust(w.cpuMinScale, 25)
			items = append(items, focusItem{
				widget: w.cpuMinScale, row: row, col: 0,
				section: "cpu", editable: true,
				onLeft: left, onRight: right, getValue: get, setValue: set,
			})
			row++
		}
		for i, option := range cpuEPPs {
			btn := w.cpuEPPBtns[option.value]
			items = append(items, focusItem{
				widget: btn, row: row + i/2, col: i % 2,
				section:    "cpu",
				isVisible:  btn.IsVisible,
				onActivate: func() { btn.Activate() },
			})
		}
		row += (len(cpuEPPs) + 1) / 2
		items = append(items, focusItem{
			widget: w.resetAllBtn, row: row, col: 0,
			section: "actions", onActivate: func() { w.resetAllBtn.Activate() },
		})
	}

	w.customFocusItems = items
}
