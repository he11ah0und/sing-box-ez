package widgets

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// Standard spacing values used across Gio UI pages and dialogs.
const (
	// PageSpacing is the default vertical gap between major widgets on a page.
	PageSpacing = unit.Dp(16)
	// DialogSpacing is the default vertical gap between widgets inside a dialog.
	DialogSpacing = unit.Dp(12)
)

// HSpace returns a horizontal spacer with the given width.
func HSpace(gtx layout.Context, w unit.Dp) layout.Dimensions {
	return layout.Dimensions{Size: image.Point{X: gtx.Dp(w), Y: 0}}
}

// VSpace returns a vertical spacer with the given height.
func VSpace(gtx layout.Context, h unit.Dp) layout.Dimensions {
	return layout.Dimensions{Size: image.Point{X: 0, Y: gtx.Dp(h)}}
}

// PageSpacer returns a standard vertical spacer for page content.
func PageSpacer(gtx layout.Context) layout.Dimensions {
	return VSpace(gtx, PageSpacing)
}

// DialogSpacer returns a standard vertical spacer for dialog content.
func DialogSpacer(gtx layout.Context) layout.Dimensions {
	return VSpace(gtx, DialogSpacing)
}

// SpacedList lays out children vertically with PageSpacing between each item.
func SpacedList(gtx layout.Context, children ...layout.FlexChild) layout.Dimensions {
	switch len(children) {
	case 0:
		return layout.Dimensions{}
	case 1:
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children[0])
	}
	spaced := make([]layout.FlexChild, 0, len(children)*2-1)
	for i, c := range children {
		if i > 0 {
			spaced = append(spaced, layout.Rigid(PageSpacer))
		}
		spaced = append(spaced, c)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, spaced...)
}

// DialogSpacedList lays out children vertically with DialogSpacing between each item.
func DialogSpacedList(gtx layout.Context, children ...layout.FlexChild) layout.Dimensions {
	switch len(children) {
	case 0:
		return layout.Dimensions{}
	case 1:
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children[0])
	}
	spaced := make([]layout.FlexChild, 0, len(children)*2-1)
	for i, c := range children {
		if i > 0 {
			spaced = append(spaced, layout.Rigid(DialogSpacer))
		}
		spaced = append(spaced, c)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, spaced...)
}

// Button is a small wrapper around material.Button for consistent sizing.
func Button(th *material.Theme, btn *widget.Clickable, label string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return material.Button(th, btn, label).Layout(gtx)
	}
}

// BorderedCard draws a rounded rectangle with a border and background color,
// then lays out content inside it with a uniform inset.
// It avoids widget.Border's clip.Stroke because that stroke flattener can loop
// forever on degenerate rectangles on Windows.
func BorderedCard(gtx layout.Context, border, bg color.NRGBA, borderWidth, radius, inset unit.Dp, content layout.Widget) layout.Dimensions {
	// Record the content first so we can paint the background and border
	// underneath it. widget.Border did the same ordering: content (including
	// its own background) was painted, then the border stroke. We emulate
	// that by drawing the filled border/background before replaying content.
	macro := op.Record(gtx.Ops)
	dims := layout.UniformInset(inset).Layout(gtx, content)
	call := macro.Stop()
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		return dims
	}

	bw := gtx.Dp(borderWidth)
	if bw < 0 {
		bw = 0
	}
	rr := gtx.Dp(radius)
	if rr < 0 {
		rr = 0
	}
	// Clamp corner radius so arcs cannot exceed the rectangle dimensions.
	maxRR := dims.Size.X
	if dims.Size.Y < maxRR {
		maxRR = dims.Size.Y
	}
	if maxRR > 0 && rr > maxRR/2 {
		rr = maxRR / 2
	}

	// Outer rectangle filled with border color.
	outer := image.Rectangle{Max: dims.Size}
	paint.FillShape(gtx.Ops, border, clip.Outline{
		Path: clip.UniformRRect(outer, rr).Path(gtx.Ops),
	}.Op())

	// Inner rectangle filled with background color, leaving the border ring.
	inner := image.Rectangle{
		Min: image.Point{X: bw, Y: bw},
		Max: image.Point{X: dims.Size.X - bw, Y: dims.Size.Y - bw},
	}
	if inner.Min.X > inner.Max.X {
		inner.Min.X, inner.Max.X = inner.Max.X, inner.Min.X
	}
	if inner.Min.Y > inner.Max.Y {
		inner.Min.Y, inner.Max.Y = inner.Max.Y, inner.Min.Y
	}
	innerRR := rr - bw
	if innerRR < 0 {
		innerRR = 0
	}
	maxInner := inner.Dx()
	if inner.Dy() < maxInner {
		maxInner = inner.Dy()
	}
	if maxInner > 0 && innerRR > maxInner/2 {
		innerRR = maxInner / 2
	}
	paint.FillShape(gtx.Ops, bg, clip.Outline{
		Path: clip.UniformRRect(inner, innerRR).Path(gtx.Ops),
	}.Op())

	// Replay content on top of the background/border.
	call.Add(gtx.Ops)
	return dims
}
