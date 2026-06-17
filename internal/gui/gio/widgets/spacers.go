package widgets

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
)

// Standard spacing values used across Gio UI pages and dialogs.
const (
	// PageSpacing is the default vertical gap between major widgets on a page.
	PageSpacing = unit.Dp(16)
	// DialogSpacing is the default vertical gap between widgets inside a dialog.
	DialogSpacing = unit.Dp(12)
)

// VSpace returns a vertical spacer with the given height.
func VSpace(gtx layout.Context, height unit.Dp) layout.Dimensions {
	return layout.Dimensions{Size: image.Point{Y: gtx.Dp(height)}}
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
