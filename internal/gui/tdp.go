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
	content.Append(sectionLabel("TDP"))

	// Advanced checkbox — placed above sliders so toggle swaps content in-place.
	w.tdpAdvancedCheck = gtk.NewCheckButtonWithLabel("Advanced")
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
	})
	tdpBasicBox.Append(w.tdpBasicScale)
	tdpBasicBox.Append(w.tdpBasicLabel)
	content.Append(tdpBasicBox)

	// Advanced box (hidden by default) — replaces basic slider in-place.
	w.tdpAdvancedBox = gtk.NewBox(gtk.OrientationVertical, 4)
	w.tdpAdvancedBox.SetVisible(false)

	w.tdpWarningLabel = gtk.NewLabel("WARNING: Values above 75W may cause thermal throttling, instability, or hardware damage. Use at your own risk — we are not responsible for any damages.")
	w.tdpWarningLabel.SetWrap(true)
	w.tdpWarningLabel.SetHAlign(gtk.AlignStart)
	w.tdpWarningLabel.AddCSSClass("tdp-warning")
	w.tdpAdvancedBox.Append(w.tdpWarningLabel)

	w.tdpPL1Scale, w.tdpPL1Label = w.buildTdpScale("PL1 (SPL)", "Sustained power limit — the long-term average power the CPU targets.")
	w.tdpPL2Scale, w.tdpPL2Label = w.buildTdpScale("PL2 (SPPT)", "Short boost — maximum power during brief burst workloads.")
	w.tdpPL3Scale, w.tdpPL3Label = w.buildTdpScale("PL3 (FPPT)", "Fast boost — peak instantaneous power for single-threaded spikes.")

	// --- UNDERVOLT (inside advanced box) ---
	w.uvBox = gtk.NewBox(gtk.OrientationVertical, 4)
	// Hidden by default; syncCustomView shows it when UndervoltAvailable.
	w.uvBox.SetVisible(false)

	w.uvBox.Append(sectionLabel("UNDERVOLT"))

	uvWarn := gtk.NewLabel("Undervolt is an independent override. Unstable values may cause crashes.")
	uvWarn.SetWrap(true)
	uvWarn.SetHAlign(gtk.AlignStart)
	uvWarn.AddCSSClass("tdp-warning")
	w.uvBox.Append(uvWarn)

	w.uvCpuScale, w.uvCpuLabel = w.buildUvScale("CPU Curve Optimizer", -40, 0)

	// UV buttons: Save UV | Reset UV
	uvBtnRow := gtk.NewBox(gtk.OrientationHorizontal, 4)
	uvBtnRow.AddCSSClass("custom-actions")

	w.saveUvBtn = gtk.NewButtonWithLabel("Save UV")
	w.saveUvBtn.AddCSSClass("save-btn")
	w.saveUvBtn.SetHExpand(true)
	w.saveUvBtn.ConnectClicked(func() { w.saveUndervolt() })
	uvBtnRow.Append(w.saveUvBtn)

	w.resetUvBtn = gtk.NewButtonWithLabel("Reset UV")
	w.resetUvBtn.SetHExpand(true)
	w.resetUvBtn.ConnectClicked(func() { w.resetUndervolt() })
	uvBtnRow.Append(w.resetUvBtn)

	w.uvBox.Append(uvBtnRow)
	w.tdpAdvancedBox.Append(w.uvBox)

	content.Append(w.tdpAdvancedBox)

	w.tdpAdvancedCheck.ConnectToggled(func() {
		adv := w.tdpAdvancedCheck.Active()
		w.tdpAdvancedBox.SetVisible(adv)
		tdpBasicBox.SetVisible(!adv)
	})

	content.Append(separator())

	// --- FAN CURVE ---
	content.Append(sectionLabel("FAN CURVE"))
	w.fanCurve = w.newFanCurveEditor()
	content.Append(w.fanCurve.area)

	content.Append(separator())

	// --- BUTTONS ---
	// Targeted save actions do not alter the other tuning overrides.
	saveRow := gtk.NewBox(gtk.OrientationHorizontal, 4)
	saveRow.AddCSSClass("custom-actions")

	w.saveTdpBtn = gtk.NewButtonWithLabel("Save TDP")
	w.saveTdpBtn.AddCSSClass("save-btn")
	w.saveTdpBtn.SetHExpand(true)
	w.saveTdpBtn.ConnectClicked(func() { w.saveCustomTdp() })
	saveRow.Append(w.saveTdpBtn)

	w.saveFanBtn = gtk.NewButtonWithLabel("Save Fan Curve")
	w.saveFanBtn.AddCSSClass("save-btn")
	w.saveFanBtn.SetHExpand(true)
	w.saveFanBtn.ConnectClicked(func() { w.saveCustomFanCurve() })
	saveRow.Append(w.saveFanBtn)

	content.Append(saveRow)

	// Reset row: Reset TDP | Reset Fans
	resetRow := gtk.NewBox(gtk.OrientationHorizontal, 4)
	resetRow.AddCSSClass("custom-actions")

	w.resetTdpBtn = gtk.NewButtonWithLabel("Reset TDP")
	w.resetTdpBtn.SetHExpand(true)
	w.resetTdpBtn.ConnectClicked(func() { w.resetTdp() })
	resetRow.Append(w.resetTdpBtn)

	w.resetFanBtn = gtk.NewButtonWithLabel("Reset Fan Curve")
	w.resetFanBtn.SetHExpand(true)
	w.resetFanBtn.ConnectClicked(func() { w.resetFanCurve() })
	resetRow.Append(w.resetFanBtn)

	content.Append(resetRow)

	w.resetAllBtn = gtk.NewButtonWithLabel("Reset All Overrides")
	w.resetAllBtn.AddCSSClass("action-btn")
	w.resetAllBtn.AddCSSClass("destructive-action")
	w.resetAllBtn.ConnectClicked(func() {
		w.showConfirmation(
			"Reset All Overrides",
			"Reset fan control to Auto, TDP to firmware defaults, and undervolt to stock? Profile, display, and charge limit are not changed.",
			"Reset All",
			"custom",
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

	// Undervolt.
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
	if w.fanCurve != nil {
		w.fanCurve.area.SetSensitive(!w.state.FanSafetyActive)
		w.saveFanBtn.SetSensitive(!w.state.FanSafetyActive)
		w.resetFanBtn.SetSensitive(!w.state.FanSafetyActive)
	}

	// Telemetry.
	if w.telemetryTempLabel != nil {
		w.telemetryTempLabel.SetLabel(fmt.Sprintf("APU: %d°C", w.state.Temperature))
	}
	if w.telemetryFanLabel != nil {
		w.telemetryFanLabel.SetLabel(fmt.Sprintf("Fan: %d RPM", w.state.FanRPM))
	}
}

// saveCustomTdp commits only the TDP values.
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
	w.runStateAction("reset TDP override", api.SendTdpReset)
}

// resetFanCurve resets fan curves to firmware auto mode.
func (w *Window) resetFanCurve() {
	w.runStateAction("reset fan curve override", api.SendFanCurveReset)
}

// saveUndervolt commits the current Curve Optimizer offsets to the daemon.
func (w *Window) saveUndervolt() {
	cpu := fmt.Sprintf("%d", int(w.uvCpuScale.Value()))
	w.runStateAction("save undervolt override", func() (bool, error) { return api.SendUndervoltSet(cpu) })
}

// resetUndervolt resets Curve Optimizer to stock (0).
func (w *Window) resetUndervolt() {
	w.runStateAction("reset undervolt override", api.SendUndervoltReset)
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
					if w.telemetryTempLabel != nil {
						w.telemetryTempLabel.SetLabel(fmt.Sprintf("APU: %d°C", state.Temperature))
					}
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

	// Row 6: fan curve (editable with custom behavior).
	if w.fanCurve != nil {
		items = append(items, focusItem{
			widget: w.fanCurve.area, row: 6, col: 0,
			section: "fan",
			// Fan curve is navigable but not editable via gamepad in this first pass.
			// Touch/mouse drag handles interaction.
		})
	}

	// Rows 7-9: undervolt (visible only when available).
	uvVis := func() bool { return w.tdpAdvancedBox.IsVisible() && w.uvBox != nil && w.uvBox.IsVisible() }
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

	// Row 9: targeted save buttons.
	items = append(items, focusItem{
		widget: w.saveTdpBtn, row: 9, col: 0,
		section:    "actions",
		onActivate: func() { w.saveTdpBtn.Activate() },
	})
	items = append(items, focusItem{
		widget: w.saveFanBtn, row: 9, col: 1,
		section:    "actions",
		onActivate: func() { w.saveFanBtn.Activate() },
	})
	// Row 10: targeted reset buttons.
	items = append(items, focusItem{
		widget: w.resetTdpBtn, row: 10, col: 0,
		section:    "actions",
		onActivate: func() { w.resetTdpBtn.Activate() },
	})
	items = append(items, focusItem{
		widget: w.resetFanBtn, row: 10, col: 1,
		section:    "actions",
		onActivate: func() { w.resetFanBtn.Activate() },
	})
	items = append(items, focusItem{
		widget: w.resetAllBtn, row: 11, col: 0,
		section:    "actions",
		onActivate: func() { w.resetAllBtn.Activate() },
	})

	w.customFocusItems = items
}
