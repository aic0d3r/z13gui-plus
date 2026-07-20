package gui

// widgets.go — premium Cairo widgets for the redesigned drawer.
//
// - RadialGauge: circular dial with arc progress, gradient stroke, and
//   centered value+label. Used in Power tab header (CPU temp, Fan RPM, TDP)
//   and System tab (battery capacity).
// - Sparkline: 30s rolling history sparkline with gradient fill. Used under
//   the gauges to show temp/fan trend.
//
// All widgets are stateless Cairo drawings driven from Window.state — they
// have no input handlers. Animation between values is handled via the GTK
// frame clock (queueDraw on each tick), with optional smoothing in the
// value-setter methods.

import (
	"fmt"
	"math"
	"sync"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// RadialGauge is a circular gauge widget drawn with Cairo.
//
// Display:
//   - 270° arc from -225° to +45° (opening at the bottom)
//   - Track arc in subtle border color
//   - Progress arc in accent gradient (red→orange for hot, accent-only otherwise)
//   - Centered value (mono, large) with unit suffix
//   - Label below value (small caps)
//
// Set via SetValue() / SetLabel(). The widget redraws on every set.
// Animated smoothing is built in: when SetValue() is called, the displayed
// value eases toward the target over ~250ms via a frame-clock tick.
type RadialGauge struct {
	area    *gtk.DrawingArea
	mu      sync.Mutex
	label   string  // small caption shown below value
	unit    string  // suffix appended to value (e.g. "°", "%", "W")
	current float64 // displayed (eased) value
	target  float64 // target value
	min     float64
	max     float64
	hot     bool // color arc orange→red (temperature-like) when true
}

// NewRadialGauge creates a gauge with the given label and unit. The value
// range defaults to 0–100; call SetRange to change it.
func NewRadialGauge(label, unit string) *RadialGauge {
	g := &RadialGauge{
		label: label,
		unit:  unit,
		min:   0,
		max:   100,
	}
	g.area = gtk.NewDrawingArea()
	g.area.AddCSSClass("gauge")
	g.area.SetDrawFunc(func(_ *gtk.DrawingArea, cr *cairo.Context, width, height int) {
		g.draw(cr, width, height)
	})
	return g
}

// Widget returns the underlying DrawingArea for layout.
func (g *RadialGauge) Widget() *gtk.DrawingArea { return g.area }

// SetRange sets the min/max for the arc. Value is clamped to [min,max].
func (g *RadialGauge) SetRange(min, max float64) {
	g.mu.Lock()
	g.min, g.max = min, max
	g.mu.Unlock()
	g.area.QueueDraw()
}

// SetHot switches the arc gradient between accent-only (false) and
// red-orange heat ramp (true). Use for temperature-style gauges.
func (g *RadialGauge) SetHot(hot bool) {
	g.mu.Lock()
	g.hot = hot
	g.mu.Unlock()
	g.area.QueueDraw()
}

// SetValue sets the target value. The displayed value eases toward it.
func (g *RadialGauge) SetValue(v float64) {
	g.mu.Lock()
	g.target = v
	// Ease 40% of the remaining distance per tick (~250ms total).
	diff := g.target - g.current
	g.current += diff * 0.4
	if math.Abs(diff) < 0.5 {
		g.current = g.target
	}
	g.mu.Unlock()
	g.area.QueueDraw()
}

// SetLabel changes the caption below the value.
func (g *RadialGauge) SetLabel(s string) {
	g.mu.Lock()
	g.label = s
	g.mu.Unlock()
	g.area.QueueDraw()
}

// draw renders the gauge. Geometry is computed for any size, but the widget
// is intended to be ~90×90 px (size-request set by caller).
func (g *RadialGauge) draw(cr *cairo.Context, width, height int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	w := float64(width)
	h := float64(height)
	cx := w / 2
	cy := h/2 + 2 // nudge down slightly to leave room for the caption
	stroke := math.Min(w, h) * 0.10
	radius := math.Min(w, h)/2 - stroke/2 - 2

	// Arc sweep from 135° to 405° (270° total, opening at bottom).
	const startAngle = 135.0 * math.Pi / 180.0
	const sweepAngle = 270.0 * math.Pi / 180.0

	// Track (background ring) — subtle border color.
	cr.SetSourceRGBA(1, 1, 1, 0.06)
	cr.SetLineWidth(stroke)
	cr.SetLineCap(cairo.LineCapRound)
	cr.Arc(cx, cy, radius, startAngle, startAngle+sweepAngle)
	cr.Stroke()

	// Progress arc.
	pct := 0.0
	if g.max > g.min {
		pct = (g.current - g.min) / (g.max - g.min)
	}
	if pct < 0 {
		pct = 0
	} else if pct > 1 {
		pct = 1
	}
	if pct > 0.01 {
		// Gradient stroke: hot ramps red→amber, else solid accent.
		if g.hot {
			x1, y1 := cx-radius, cy
			x2, y2 := cx+radius, cy
			grad, _ := cairo.NewPatternLinear(x1, y1, x2, y2)
			_ = grad.AddColorStopRGBA(0.2, 0.95, 0.62, 0.18, 1) // amber
			_ = grad.AddColorStopRGBA(0.7, 0.92, 0.30, 0.15, 1) // accent crimson
			_ = grad.AddColorStopRGBA(1.0, 0.95, 0.20, 0.10, 1) // hot red
			cr.SetSource(grad)
		} else {
			// Solid accent (matches theme @z13-accent #e2231a).
			cr.SetSourceRGBA(0.87, 0.14, 0.10, 1)
		}
		cr.SetLineWidth(stroke)
		cr.NewSubPath()
		cr.Arc(cx, cy, radius, startAngle, startAngle+sweepAngle*pct)
		cr.Stroke()

		// Soft glow at the leading edge — small radial dot at the arc tip.
		tipAngle := startAngle + sweepAngle*pct
		tx := cx + radius*math.Cos(tipAngle)
		ty := cy + radius*math.Sin(tipAngle)
		// Glow halo (bigger, translucent).
		cr.SetSourceRGBA(0.95, 0.30, 0.20, 0.3)
		cr.Arc(tx, ty, stroke*0.9, 0, 2*math.Pi)
		cr.Fill()
		// Bright core.
		cr.SetSourceRGBA(1, 0.95, 0.95, 1)
		cr.Arc(tx, ty, stroke*0.45, 0, 2*math.Pi)
		cr.Fill()
	}

	// Centered value text (large, mono-styled via class on parent box).
	cr.SetSourceRGBA(0.96, 0.96, 0.97, 1)
	cr.SetFontSize(math.Min(w, h) * 0.28)
	valueText := fmt.Sprintf("%.0f", g.current)
	extents := cr.TextExtents(valueText)
	cr.MoveTo(cx-extents.Width/2-extents.XBearing, cy-extents.Height/2-extents.YBearing-2)
	cr.ShowText(valueText)

	// Unit suffix — small, to the right of the value.
	if g.unit != "" {
		cr.SetFontSize(math.Min(w, h) * 0.14)
		cr.SetSourceRGBA(0.96, 0.96, 0.97, 0.55)
		unitExtents := cr.TextExtents(g.unit)
		cr.MoveTo(cx-extents.Width/2+extents.Width+2, cy+4)
		cr.ShowText(g.unit)
		_ = unitExtents
	}

	// Caption below value — small caps label.
	cr.SetFontSize(math.Min(w, h) * 0.12)
	cr.SetSourceRGBA(0.66, 0.66, 0.72, 1)
	labelExtents := cr.TextExtents(g.label)
	cr.MoveTo(cx-labelExtents.Width/2-labelExtents.XBearing, cy+extents.Height/2+12)
	cr.ShowText(g.label)
}

// Sparkline renders a rolling line+area chart from a fixed-size sample slice.
// Used to show 30s history of CPU temp or fan RPM.
type Sparkline struct {
	area    *gtk.DrawingArea
	mu      sync.Mutex
	samples []float64 // chronological samples, oldest first
	maxLen  int       // ring buffer capacity
	min     float64
	max     float64
	hot     bool // color red/orange when true
}

// NewSparkline creates a sparkline widget sized for `maxLen` samples.
// SetRange defines the y-axis. Defaults to 0–100.
func NewSparkline(maxLen int) *Sparkline {
	if maxLen < 2 {
		maxLen = 30
	}
	s := &Sparkline{
		samples: make([]float64, 0, maxLen),
		maxLen:  maxLen,
		min:     0,
		max:     100,
	}
	s.area = gtk.NewDrawingArea()
	s.area.AddCSSClass("sparkline")
	s.area.SetDrawFunc(func(_ *gtk.DrawingArea, cr *cairo.Context, width, height int) {
		s.draw(cr, width, height)
	})
	return s
}

// Widget returns the underlying DrawingArea.
func (s *Sparkline) Widget() *gtk.DrawingArea { return s.area }

// SetRange sets the y-axis min/max.
func (s *Sparkline) SetRange(min, max float64) {
	s.mu.Lock()
	s.min, s.max = min, max
	s.mu.Unlock()
	s.area.QueueDraw()
}

// SetHot switches the line color between accent-only and red-orange heat ramp.
func (s *Sparkline) SetHot(hot bool) {
	s.mu.Lock()
	s.hot = hot
	s.mu.Unlock()
	s.area.QueueDraw()
}

// Push appends a sample, dropping the oldest when full.
func (s *Sparkline) Push(v float64) {
	s.mu.Lock()
	if len(s.samples) >= s.maxLen {
		s.samples = s.samples[1:]
	}
	s.samples = append(s.samples, v)
	s.mu.Unlock()
	s.area.QueueDraw()
}

// Reset clears the samples.
func (s *Sparkline) Reset() {
	s.mu.Lock()
	s.samples = s.samples[:0]
	s.mu.Unlock()
	s.area.QueueDraw()
}

func (s *Sparkline) draw(cr *cairo.Context, width, height int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	w := float64(width)
	h := float64(height)
	if len(s.samples) < 2 || s.max <= s.min {
		return
	}

	xFor := func(i int) float64 {
		return float64(i) * w / float64(s.maxLen-1)
	}
	yFor := func(v float64) float64 {
		pct := (v - s.min) / (s.max - s.min)
		if pct < 0 {
			pct = 0
		} else if pct > 1 {
			pct = 1
		}
		// Invert: high value at top.
		return h - pct*h
	}

	// Gradient fill below line.
	var r, g, b, a float64
	if s.hot {
		r, g, b = 0.92, 0.30, 0.15
	} else {
		r, g, b = 0.87, 0.14, 0.10
	}
	fillGrad, _ := cairo.NewPatternLinear(0, 0, 0, h)
	_ = fillGrad.AddColorStopRGBA(0, r, g, b, 0.25)
	_ = fillGrad.AddColorStopRGBA(1, r, g, b, 0.0)
	cr.SetSource(fillGrad)
	cr.NewSubPath()
	cr.MoveTo(xFor(0), h)
	for i, v := range s.samples {
		cr.LineTo(xFor(i), yFor(v))
	}
	cr.LineTo(xFor(len(s.samples)-1), h)
	cr.ClosePath()
	cr.Fill()

	// Line stroke on top.
	a = 1.0
	cr.SetSourceRGBA(r, g, b, a)
	cr.SetLineWidth(1.5)
	cr.SetLineCap(cairo.LineCapRound)
	cr.SetLineJoin(cairo.LineJoinRound)
	for i, v := range s.samples {
		if i == 0 {
			cr.MoveTo(xFor(i), yFor(v))
		} else {
			cr.LineTo(xFor(i), yFor(v))
		}
	}
	cr.Stroke()

	// Dot at the latest sample.
	lastX := xFor(len(s.samples) - 1)
	lastY := yFor(s.samples[len(s.samples)-1])
	// Glow.
	cr.SetSourceRGBA(r, g, b, 0.4)
	cr.Arc(lastX, lastY, 4, 0, 2*math.Pi)
	cr.Fill()
	// Core.
	cr.SetSourceRGBA(1, 0.95, 0.95, 1)
	cr.Arc(lastX, lastY, 2, 0, 2*math.Pi)
	cr.Fill()
}

// formatPercent returns the integer percentage of `value` relative to `of`.
// Used by the battery hero card to compute health %, etc.
func formatPercent(value, of int) int {
	if of <= 0 {
		return 0
	}
	p := value * 100 / of
	if p > 100 {
		p = 100
	}
	return p
}
