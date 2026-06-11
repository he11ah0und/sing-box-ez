package pages

import (
	"gioui.org/layout"
	"gioui.org/widget"
)

// Page is the common interface for all shell pages.
type Page interface {
	Tag() string
	Name() string
	Icon() *widget.Icon
	Layout(gtx layout.Context) layout.Dimensions
}
