package pages

import "gioui.org/layout"

// Page is the common interface for all shell pages.
type Page interface {
	Tag() string
	Name() string
	Layout(gtx layout.Context) layout.Dimensions
}
