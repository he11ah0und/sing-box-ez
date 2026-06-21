package widgets

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"sing-box-ez/internal/gui/gio/theme"
)

// LabeledInput renders a label + full-width bordered editor with optional dirty highlighting.
func LabeledInput(gtx layout.Context, th *material.Theme, label string, ed *widget.Editor, dirty bool) layout.Dimensions {
	bg := theme.Current().Colors().InputBg
	if dirty {
		bg = theme.Current().Colors().InputDirtyBg
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(th, label).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Point{Y: gtx.Dp(unit.Dp(4))}}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// Force the input to fill the available width even when the parent
			// layout does not set a non-zero minimum width (e.g. inside a dialog).
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			macro := op.Record(gtx.Ops)
			dims := widget.Border{
				Color:        theme.Current().Colors().InputBorder,
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
