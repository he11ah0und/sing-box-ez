package widgets

import "gioui.org/layout"

// DialogProvider abstracts modal dialog operations for pages and widgets.
// Implemented by the shell's Dialog so pages don't import the gui package.
type DialogProvider interface {
	Show(title, body string)
	ShowMarkdown(title, body string)
	ShowLoading(title string)
	HideLoading()
	ShowLoadingWithProgress(title string, progress func() float32)
	ShowConfirm(title, body string, onConfirm func(), onDismiss func())
	ShowCustom(title string, content layout.Widget)
	ShowCustomNoCancel(title string, content layout.Widget)
	HideCustom()
}
