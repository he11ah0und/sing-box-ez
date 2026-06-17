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

// SpacedPage is implemented by pages whose children should be rendered by the
// shell with standard vertical spacing between them.
type SpacedPage interface {
	Page
	Children(gtx layout.Context) []layout.FlexChild
}
