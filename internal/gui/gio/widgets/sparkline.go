package widgets

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// Sparkline draws a simple line chart from a rolling set of values.
type Sparkline struct {
	color     color.NRGBA
	maxPoints int
	data      []float64
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
	}
}

// Add appends a new value, dropping the oldest value once the buffer is full.
func (s *Sparkline) Add(v float64) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		v = 0
	}
	s.data = append(s.data, v)
	if len(s.data) > s.maxPoints {
		s.data = s.data[1:]
	}
}

// Values returns the current data slice.
func (s *Sparkline) Values() []float64 { return s.data }

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

	step := w / float32(len(s.data)-1)

	// Fill area under the line.
	{
		var p clip.Path
		p.Begin(gtx.Ops)
		p.MoveTo(f32.Point{X: 0, Y: h})
		for i, v := range s.data {
			y := h - (float32(v)/float32(maxVal))*h
			p.LineTo(f32.Point{X: float32(i) * step, Y: y})
		}
		p.LineTo(f32.Point{X: w, Y: h})
		p.Close()
		fill := s.color
		fill.A = uint8(float32(fill.A) * 0.25)
		paint.FillShape(gtx.Ops, fill, clip.Outline{Path: p.End()}.Op())
	}

	return layout.Dimensions{Size: size}
}
