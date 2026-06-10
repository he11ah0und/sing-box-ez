package pages

import "gioui.org/layout"

// DialogProvider abstracts modal dialog operations for pages.
// Implemented by the shell's Dialog so pages don't import the gui package.
type DialogProvider interface {
	Show(title, body string)
	ShowMarkdown(title, body string)
	ShowLoading(title string)
	HideLoading()
	ShowCustom(title string, content layout.Widget)
	HideCustom()
}
