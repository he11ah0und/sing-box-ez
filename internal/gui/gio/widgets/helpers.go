package widgets

import (
	"image"
	"image/color"

	"gioui.org/layout"
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
	if len(children) == 0 {
		return layout.Dimensions{}
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
	if len(children) == 0 {
		return layout.Dimensions{}
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
func BorderedCard(gtx layout.Context, border, bg color.NRGBA, borderWidth, radius, inset unit.Dp, content layout.Widget) layout.Dimensions {
	return widget.Border{
		Color:        border,
		Width:        borderWidth,
		CornerRadius: radius,
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
		paint.Fill(gtx.Ops, bg)
		return layout.UniformInset(inset).Layout(gtx, content)
	})
}
