package widgets

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// Sparkline draws a simple line chart from a rolling set of values.
type Sparkline struct {
	color     color.NRGBA
	maxPoints int
	data      []float64
	times     []time.Time
	smoothing float64
	ema       float64
	hasEma    bool
	lastAdd   time.Time
	interval  time.Duration
}

// NewSparkline creates a sparkline with the given line color and maximum
// number of retained data points.
func NewSparkline(color color.NRGBA, maxPoints int) *Sparkline {
	if maxPoints < 2 {
		maxPoints = 2
	}
	return &Sparkline{
		color:     color,
		maxPoints: maxPoints,
		data:      make([]float64, 0, maxPoints),
		times:     make([]time.Time, 0, maxPoints),
		smoothing: 1,
		interval:  time.Second,
	}
}

// SetSmoothing controls exponential moving average on newly added values.
// alpha=1 disables smoothing, alpha=0 keeps the value frozen. Default is 0.4.
func (s *Sparkline) SetSmoothing(alpha float64) {
	if alpha < 0 {
		alpha = 0
	} else if alpha > 1 {
		alpha = 1
	}
	s.smoothing = alpha
}

// Add appends a new value, dropping the oldest value once the buffer is full.
func (s *Sparkline) Add(v float64) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		v = 0
	}
	if !s.hasEma {
		s.ema = v
		s.hasEma = true
	} else {
		s.ema = s.smoothing*v + (1-s.smoothing)*s.ema
	}

	now := time.Now()
	s.lastAdd = now
	s.data = append(s.data, s.ema)
	s.times = append(s.times, now)
	if len(s.data) > s.maxPoints {
		s.data = s.data[1:]
		s.times = s.times[1:]
	}
}

// Values returns the current data slice.
func (s *Sparkline) Values() []float64 { return s.data }

// SetMaxPoints changes the rolling window size and trims excess old data.
func (s *Sparkline) SetMaxPoints(maxPoints int) {
	if maxPoints < 2 {
		maxPoints = 2
	}
	s.maxPoints = maxPoints
	if len(s.data) > s.maxPoints {
		s.data = s.data[len(s.data)-s.maxPoints:]
		s.times = s.times[len(s.times)-s.maxPoints:]
	}
}

// Reset clears the buffer and smoothing state.
func (s *Sparkline) Reset() {
	s.data = s.data[:0]
	s.times = s.times[:0]
	s.hasEma = false
	s.lastAdd = time.Time{}
}

// MaxPoints returns the current rolling window size.
func (s *Sparkline) MaxPoints() int { return s.maxPoints }

// Stats returns the minimum, maximum and average of the retained values.
func (s *Sparkline) Stats() (min, max, avg float64) {
	if len(s.data) == 0 {
		return 0, 0, 0
	}
	min = s.data[0]
	max = s.data[0]
	var sum float64
	for _, v := range s.data {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}
	avg = sum / float64(len(s.data))
	return
}

// Layout draws the sparkline into a rectangle of the requested size.
func (s *Sparkline) Layout(gtx layout.Context, width, height unit.Dp) layout.Dimensions {
	w := float32(gtx.Dp(width))
	h := float32(gtx.Dp(height))
	size := image.Point{X: int(w), Y: int(h)}

	if len(s.data) < 2 || w <= 0 || h <= 0 || math.IsNaN(float64(w)) || math.IsNaN(float64(h)) {
		return layout.Dimensions{Size: size}
	}

	maxVal := s.data[0]
	for _, v := range s.data {
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal <= 0 {
		maxVal = 1
	}

	// Smooth horizontal scroll: position each point by its age so older
	// values drift left while the newest value stays near the right edge.
	now := gtx.Now
	if now.IsZero() {
		now = time.Now()
	}
	span := time.Duration(s.maxPoints-1) * s.interval
	if span <= 0 {
		span = time.Second
	}
	spanSecs := span.Seconds()

	pointAt := func(i int, v float64) f32.Point {
		age := now.Sub(s.times[i]).Seconds()
		x := w - float32(age/spanSecs)*w
		y := h - (float32(v)/float32(maxVal))*h
		return f32.Point{X: x, Y: y}
	}

	clipRect := clip.Rect{Max: size}.Push(gtx.Ops)
	defer clipRect.Pop()

	// Fill area under the line.
	{
		var p clip.Path
		p.Begin(gtx.Ops)
		first := pointAt(0, s.data[0])
		last := pointAt(len(s.data)-1, s.data[len(s.data)-1])
		p.MoveTo(f32.Point{X: 0, Y: h})
		p.LineTo(first)
		for i, v := range s.data {
			p.LineTo(pointAt(i, v))
		}
		p.LineTo(f32.Point{X: last.X, Y: h})
		p.Close()
		fill := s.color
		fill.A = uint8(float32(fill.A) * 0.25)
		paint.FillShape(gtx.Ops, fill, clip.Outline{Path: p.End()}.Op())
	}

	// Animate: schedule another frame so the scroll continues smoothly.
	gtx.Execute(op.InvalidateCmd{At: now.Add(16 * time.Millisecond)})

	return layout.Dimensions{Size: size}
}
