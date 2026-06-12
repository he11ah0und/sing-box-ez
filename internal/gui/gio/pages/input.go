package pages

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

// LabeledInput renders a label + full-width bordered editor with optional dirty highlighting.
func LabeledInput(gtx layout.Context, th *material.Theme, label string, ed *widget.Editor, dirty bool) layout.Dimensions {
	bg := color.NRGBA{R: 40, G: 40, B: 40, A: 255}
	if dirty {
		bg = color.NRGBA{R: 80, G: 70, B: 20, A: 255}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(th, label).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Point{Y: gtx.Dp(unit.Dp(4))}}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			macro := op.Record(gtx.Ops)
			dims := widget.Border{
				Color:        color.NRGBA{R: 128, G: 128, B: 128, A: 128},
				CornerRadius: unit.Dp(4),
				Width:        unit.Dp(1),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return material.Editor(th, ed, "").Layout(gtx)
				})
			})
			call := macro.Stop()
			paint.FillShape(gtx.Ops, bg, clip.Rect{Max: dims.Size}.Op())
			call.Add(gtx.Ops)
			return dims
		}),
	)
}
