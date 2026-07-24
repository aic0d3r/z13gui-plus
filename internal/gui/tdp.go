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
	// Clamp the dragged point first.
	if fc.points[idx].Temp < 35 {
		fc.points[idx].Temp = 35
	}
	if fc.points[idx].Temp > 105 {
		fc.points[idx].Temp = 105
	}
	if fc.points[idx].PWM < 0 {
		fc.points[idx].PWM = 0
	}
	if fc.points[idx].PWM > 255 {
		fc.points[idx].PWM = 255
	}

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
	// Final clamp pass.
	for i := range fc.points {
		if fc.points[i].Temp < 35 {
			fc.points[i].Temp = 35
		}
		if fc.points[i].Temp > 105 {
			fc.points[i].Temp = 105
		}
		if fc.points[i].PWM < 0 {
			fc.points[i].PWM = 0
		}
		if fc.points[i].PWM > 255 {
			fc.points[i].PWM = 255
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
	t := 35 + int(math.Round((x-fc.chartX)/fc.chartW*70.0))
	if t < 35 {
		t = 35
	}
	if t > 105 {
		t = 105
	}
	return t
}
func (fc *fanCurveEditor) yToPWM(y float64) int {
	p := int(math.Round((fc.chartY + fc.chartH - y) / fc.chartH * 255.0))
	if p < 0 {
		p = 0
	}
	if p > 255 {
		p = 255
	}
	return p
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
	fc.area.SetSizeRequest(-1, 240)
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

	// Header: back button + title.
	w.customBackBtn = gtk.NewButton()
	w.customBackBtn.SetIconName("go-previous-symbolic")
	w.customBackBtn.AddCSSClass("view-back-btn")
	w.customBackBtn.ConnectClicked(func() { w.showMainView() })

	header := gtk.NewBox(gtk.OrientationHorizontal, 8)
	header.SetMarginTop(10)
	header.SetMarginBottom(6)
	header.SetMarginStart(14)
	header.Append(w.customBackBtn)
	lbl := gtk.NewLabel("Power Tuning")
	lbl.SetHAlign(gtk.AlignStart)
	lbl.AddCSSClass("drawer-title")
	header.Append(lbl)
	view.Append(header)

	content := gtk.NewBox(gtk.OrientationVertical, 8)
	content.SetMarginTop(4)
	content.SetMarginBottom(12)
	content.SetMarginStart(12)
	content.SetMarginEnd(12)

	// --- TELEMETRY ---
	content.Append(sectionLabel("TELEMETRY"))
	telRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	w.telemetryTempLabel = gtk.NewLabel("APU: --°C")
	w.telemetryTempLabel.SetHAlign(gtk.AlignStart)
	w.telemetryTempLabel.AddCSSClass("telemetry-value")
	w.telemetryFanLabel = gtk.NewLabel("Fan: -- RPM")
	w.telemetryFanLabel.SetHAlign(gtk.AlignEnd)
	w.telemetryFanLabel.SetHExpand(true)
	w.telemetryFanLabel.AddCSSClass("telemetry-value")
	telRow.Append(w.telemetryTempLabel)
	telRow.Append(w.telemetryFanLabel)
	content.Append(telRow)

	// --- TDP ---
	tdpHdr, tdpMark := sectionHeader("TDP")
	w.tdpDirtyMark = tdpMark
	content.Append(tdpHdr)

	// Advanced checkbox — swaps basic/advanced sliders in-place.
	w.tdpAdvancedCheck = gtk.NewCheckButtonWithLabel("Advanced (PL1 / PL2 / PL3)")
	w.tdpAdvancedCheck.AddCSSClass("advanced-check")
	if w.gamescope {
		addTouchActivate(w.tdpAdvancedCheck, func() { w.tdpAdvancedCheck.SetActive(!w.tdpAdvancedCheck.Active()) })
	}
	content.Append(w.tdpAdvancedCheck)

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
	tdpBasicBox.Append(w.tdpBasicScale)
	tdpBasicBox.Append(w.tdpBasicLabel)
	content.Append(tdpBasicBox)

	// Advanced box (hidden by default) — replaces basic slider in-place.
	w.tdpAdvancedBox = gtk.NewBox(gtk.OrientationVertical, 4)
	w.tdpAdvancedBox.SetVisible(false)

	w.tdpWarningLabel = gtk.NewLabel("Values above 75W may throttle or damage hardware. Use caution.")
	w.tdpWarningLabel.SetWrap(true)
	w.tdpWarningLabel.SetHAlign(gtk.AlignStart)
	w.tdpWarningLabel.AddCSSClass("tdp-warning")
	w.tdpAdvancedBox.Append(w.tdpWarningLabel)

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

	// TDP inline actions.
	content.Append(w.tuningActionRow(&w.saveTdpBtn, "Save TDP", w.saveCustomTdp, &w.resetTdpBtn, "Reset TDP", w.resetTdp))

	content.Append(separator())

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
	w.uvBox.Append(w.tuningActionRow(&w.saveUvBtn, "Save UV", w.saveUndervolt, &w.resetUvBtn, "Reset UV", w.resetUndervolt))
	content.Append(w.uvBox)

	content.Append(separator())

	// --- FAN CURVE ---
	fanHdr, fanMark := sectionHeader("FAN CURVE")
	w.fanDirtyMark = fanMark
	content.Append(fanHdr)

	w.fanSafetyBanner = gtk.NewLabel("Fan control is locked while a safety profile is active.")
	w.fanSafetyBanner.SetWrap(true)
	w.fanSafetyBanner.SetHAlign(gtk.AlignStart)
	w.fanSafetyBanner.AddCSSClass("tdp-warning")
	w.fanSafetyBanner.SetVisible(false)
	content.Append(w.fanSafetyBanner)

	w.fanCurve = w.newFanCurveEditor()
	content.Append(w.fanCurve.area)
	content.Append(w.tuningActionRow(&w.saveFanBtn, "Save Fan Curve", w.saveCustomFanCurve, &w.resetFanBtn, "Reset Fan Curve", w.resetFanCurve))

	w.resetAllBtn = gtk.NewButtonWithLabel("Reset All Overrides")
	w.resetAllBtn.AddCSSClass("action-btn")
	w.resetAllBtn.ConnectClicked(func() {
		w.showConfirmation(
			"Reset All Overrides",
			"Reset fan control to Auto, TDP to firmware defaults, and undervolt to stock? Profile, display, and charge limit are not changed.",
			"Reset All",
			"custom",
			false,
			func() { w.resetAllOverrides() },
		)
	})
	content.Append(w.resetAllBtn)

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(content)
	w.customScroll = scroll
	view.Append(scroll)

	return view
}

// buildTdpScale creates a labeled TDP slider (5–93W) and appends it to the
// advanced box. Returns the scale and value label.
func (w *Window) buildTdpScale(label, desc string) (*gtk.Scale, *gtk.Label) {
	nameLabel := gtk.NewLabel(label)
	nameLabel.SetHAlign(gtk.AlignStart)
	nameLabel.AddCSSClass("scale-name")
	w.tdpAdvancedBox.Append(nameLabel)
	descLabel := gtk.NewLabel(desc)
	descLabel.SetHAlign(gtk.AlignStart)
	descLabel.SetWrap(true)
	descLabel.AddCSSClass("scale-desc")
	w.tdpAdvancedBox.Append(descLabel)
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
	w.tdpAdvancedBox.Append(sc)
	w.tdpAdvancedBox.Append(valLabel)
	return sc, valLabel
}

// buildUvScale creates a labeled undervolt slider and appends it to uvBox.
func (w *Window) buildUvScale(label string, lo, hi float64) (*gtk.Scale, *gtk.Label) {
	nameLabel := gtk.NewLabel(label)
	nameLabel.SetHAlign(gtk.AlignStart)
	nameLabel.AddCSSClass("scale-name")
	w.uvBox.Append(nameLabel)
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
	w.uvBox.Append(sc)
	w.uvBox.Append(valLabel)
	return sc, valLabel
}

// uvLabel formats an undervolt value label, e.g. "CPU Curve Optimizer: -20" or "... 0 (stock)".
func uvLabel(name string, val int) string {
	if val == 0 {
		return fmt.Sprintf("%s: 0 (stock)", name)
	}
	return fmt.Sprintf("%s: %d", name, val)
}

// syncCustomView populates the tuning widgets from daemon state.
func (w *Window) syncCustomView() {
	if w.state == nil {
		return
	}
	prev := w.syncing
	w.syncing = true
	defer func() { w.syncing = prev }()

	// TDP.
	if w.state.TDP != nil {
		tdp := w.state.TDP
		if w.tdpBasicScale != nil {
			v := float64(tdp.PL1SPL)
			if v > tdpMaxBasic {
				v = tdpMaxBasic
			}
			w.tdpBasicScale.SetValue(v)
			w.tdpBasicLabel.SetLabel(fmt.Sprintf("%d W", int(v)))
		}
		if w.tdpPL1Scale != nil {
			w.tdpPL1Scale.SetValue(float64(tdp.PL1SPL))
			w.tdpPL1Label.SetLabel(fmt.Sprintf("%d W", tdp.PL1SPL))
		}
		if w.tdpPL2Scale != nil {
			w.tdpPL2Scale.SetValue(float64(tdp.PL2SPPT))
			w.tdpPL2Label.SetLabel(fmt.Sprintf("%d W", tdp.PL2SPPT))
		}
		if w.tdpPL3Scale != nil {
			w.tdpPL3Scale.SetValue(float64(tdp.FPPT))
			w.tdpPL3Label.SetLabel(fmt.Sprintf("%d W", tdp.FPPT))
		}
	}

	// Fan curve.
	if w.state.FanCurve != nil && len(w.state.FanCurve.Points) == 8 && w.fanCurve != nil {
		copy(w.fanCurve.points[:], w.state.FanCurve.Points)
		w.fanCurve.area.QueueDraw()
	}

	// Undervolt (own section now; visibility is independent of the Advanced checkbox).
	if w.uvBox != nil {
		w.uvBox.SetVisible(w.state.UndervoltAvailable)
	}
	cpuCO := 0
	if w.state.UndervoltActive && w.state.Undervolt != nil {
		cpuCO = w.state.Undervolt.CPUCO
	}
	if w.uvCpuScale != nil {
		w.uvCpuScale.SetValue(float64(cpuCO))
		w.uvCpuLabel.SetLabel(uvLabel("CPU Curve Optimizer", cpuCO))
	}

	// Telemetry.
	applyTempColor(w.telemetryTempLabel, w.state.Temperature)
	if w.telemetryFanLabel != nil {
		w.telemetryFanLabel.SetLabel(fmt.Sprintf("Fan: %d RPM", w.state.FanRPM))
	}

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

	// Fan.
	fanLocked := w.state != nil && w.state.FanSafetyActive
	fanDirty := w.fanDirty()
	if w.fanSafetyBanner != nil {
		w.fanSafetyBanner.SetVisible(fanLocked)
	}
	if w.fanCurve != nil {
		w.fanCurve.area.SetSensitive(!fanLocked)
	}
	w.saveFanBtn.SetSensitive(fanDirty && !fanLocked)
	w.resetFanBtn.SetSensitive(!fanLocked)
	toggleMark(w.fanDirtyMark, fanDirty && !fanLocked)

	// Undervolt.
	uvDirty := w.uvDirty()
	w.saveUvBtn.SetSensitive(uvDirty)
	toggleMark(w.uvDirtyMark, uvDirty)
}

func toggleMark(mark *gtk.Label, dirty bool) {
	if mark != nil {
		mark.SetVisible(dirty)
	}
}

// tdpDirty reports whether the TDP sliders differ from the saved state.
func (w *Window) tdpDirty() bool {
	pl1, pl2, pl3 := 50, 50, 50
	if w.state != nil && w.state.TDP != nil {
		pl1, pl2, pl3 = w.state.TDP.PL1SPL, w.state.TDP.PL2SPPT, w.state.TDP.FPPT
	}
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
	if w.state == nil || w.state.FanCurve == nil || len(w.state.FanCurve.Points) != 8 {
		return w.fanCurve.points != defaultFanCurve()
	}
	for i := 0; i < 8; i++ {
		if w.fanCurve.points[i] != w.state.FanCurve.Points[i] {
			return true
		}
	}
	return false
}

// uvDirty reports whether the Curve Optimizer slider differs from the saved state.
func (w *Window) uvDirty() bool {
	if w.uvCpuScale == nil {
		return false
	}
	cur := 0
	if w.state != nil && w.state.UndervoltActive && w.state.Undervolt != nil {
		cur = w.state.Undervolt.CPUCO
	}
	return int(w.uvCpuScale.Value()) != cur
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
		label.SetLabel("APU: --°C")
		return
	}
	label.SetLabel(fmt.Sprintf("APU: %d°C", temp))
	switch {
	case temp >= 85:
		label.AddCSSClass("temp-hot")
	case temp >= 70:
		label.AddCSSClass("temp-warn")
	default:
		label.AddCSSClass("temp-ok")
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

func (w *Window) resetAllOverrides() {
	w.runStateAction("reset all tuning overrides", api.SendTuningReset)
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
		customActive := w.viewStack != nil && w.viewStack.VisibleChildName() == "custom"
		overviewActive := w.viewStack != nil && w.viewStack.VisibleChildName() == "main" &&
			w.tabStack != nil && w.tabStack.VisibleChildName() == "overview"
		powerActive := w.viewStack != nil && ((w.viewStack.VisibleChildName() == "main" &&
			w.tabStack != nil && w.tabStack.VisibleChildName() == "power") ||
			w.viewStack.VisibleChildName() == "presets" || w.viewStack.VisibleChildName() == "chooser" ||
			w.viewStack.VisibleChildName() == "confirm")
		systemActive := w.viewStack != nil && w.viewStack.VisibleChildName() == "main" &&
			w.tabStack != nil && w.tabStack.VisibleChildName() == "system"
		rgbActive := w.viewStack != nil && w.viewStack.VisibleChildName() == "main" &&
			w.tabStack != nil && w.tabStack.VisibleChildName() == "rgb"
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
					applyTempColor(w.telemetryTempLabel, state.Temperature)
					if w.telemetryFanLabel != nil {
						w.telemetryFanLabel.SetLabel(fmt.Sprintf("Fan: %d RPM", state.FanRPM))
					}
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

	// Row 1: basic TDP slider.
	if w.tdpBasicScale != nil {
		oL, oR, gV, sV := scaleAdjust(w.tdpBasicScale, 5)
		items = append(items, focusItem{
			widget: w.tdpBasicScale, row: 1, col: 0,
			section:  "tdp",
			editable: true,
			onLeft:   oL, onRight: oR,
			getValue: gV, setValue: sV,
			isVisible: func() bool { return w.tdpBasicScale.IsVisible() },
		})
	}

	// Row 2: advanced checkbox.
	if w.tdpAdvancedCheck != nil {
		items = append(items, focusItem{
			widget: w.tdpAdvancedCheck, row: 2, col: 0,
			section:    "tdp",
			onActivate: func() { w.tdpAdvancedCheck.SetActive(!w.tdpAdvancedCheck.Active()) },
		})
	}

	// Rows 3-5: PL1/PL2/PL3 sliders.
	advVis := func() bool { return w.tdpAdvancedBox.IsVisible() }
	for i, sc := range []*gtk.Scale{w.tdpPL1Scale, w.tdpPL2Scale, w.tdpPL3Scale} {
		oL, oR, gV, sV := scaleAdjust(sc, 1)
		items = append(items, focusItem{
			widget: sc, row: 3 + i, col: 0,
			section:   "tdp",
			editable:  true,
			isVisible: advVis,
			onLeft:    oL, onRight: oR,
			getValue: gV, setValue: sV,
		})
	}

	// Row 6: TDP save/reset (inline).
	items = append(items, focusItem{
		widget: w.saveTdpBtn, row: 6, col: 0,
		section: "tdp", onActivate: func() { w.saveTdpBtn.Activate() },
	})
	items = append(items, focusItem{
		widget: w.resetTdpBtn, row: 6, col: 1,
		section: "tdp", onActivate: func() { w.resetTdpBtn.Activate() },
	})

	// Rows 7-8: undervolt (own section; visible only when available).
	uvVis := func() bool { return w.uvBox != nil && w.uvBox.IsVisible() }
	if w.uvCpuScale != nil {
		oL, oR, gV, sV := scaleAdjust(w.uvCpuScale, 1)
		items = append(items, focusItem{
			widget: w.uvCpuScale, row: 7, col: 0,
			section: "undervolt", editable: true, isVisible: uvVis,
			onLeft: oL, onRight: oR, getValue: gV, setValue: sV,
		})
	}
	items = append(items, focusItem{
		widget: w.saveUvBtn, row: 8, col: 0,
		section: "undervolt", isVisible: uvVis,
		onActivate: func() { w.saveUvBtn.Activate() },
	})
	items = append(items, focusItem{
		widget: w.resetUvBtn, row: 8, col: 1,
		section: "undervolt", isVisible: uvVis,
		onActivate: func() { w.resetUvBtn.Activate() },
	})

	// Row 9: fan curve.
	if w.fanCurve != nil {
		items = append(items, focusItem{
			widget: w.fanCurve.area, row: 9, col: 0,
			section: "fan",
			// Fan curve is navigable but not editable via gamepad in this first pass.
			// Touch/mouse drag handles interaction.
		})
	}

	// Row 10: fan save/reset (inline).
	items = append(items, focusItem{
		widget: w.saveFanBtn, row: 10, col: 0,
		section: "fan", onActivate: func() { w.saveFanBtn.Activate() },
	})
	items = append(items, focusItem{
		widget: w.resetFanBtn, row: 10, col: 1,
		section: "fan", onActivate: func() { w.resetFanBtn.Activate() },
	})

	// Row 11: reset all.
	items = append(items, focusItem{
		widget: w.resetAllBtn, row: 11, col: 0,
		section: "actions", onActivate: func() { w.resetAllBtn.Activate() },
	})

	w.customFocusItems = items
}
