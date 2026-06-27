package widgets

import (
	"gioui.org/layout"

	"sing-box-ez/internal/framework/localengine"
)

// ButtonStyle identifies the visual emphasis of a dialog button.
type ButtonStyle int

const (
	ButtonDefault ButtonStyle = iota
	ButtonPrimary
	ButtonDanger
)

// DialogButton describes a single button rendered in the dialog's bottom row.
type DialogButton struct {
	Label  string
	Style  ButtonStyle
	Action func()
}

// ButtonSpec returns the slice of buttons that should be shown this frame.
// It is evaluated on every layout so callers can change buttons dynamically.
type ButtonSpec func() []DialogButton

// Close returns a spec for a close button. The optional first argument overrides
// the default translated label.
func Close(labels ...string) ButtonSpec {
	label := localengine.T("dialog", "btn", "close")
	if len(labels) > 0 && labels[0] != "" {
		label = labels[0]
	}
	return func() []DialogButton {
		return []DialogButton{{Label: label, Style: ButtonDefault}}
	}
}

// Cancel returns a spec for a cancel button.
func Cancel() ButtonSpec {
	return func() []DialogButton {
		return []DialogButton{{Label: localengine.T("dialog", "btn", "cancel"), Style: ButtonDefault}}
	}
}

// Confirm returns a spec for a primary confirm button.
func Confirm(action func()) ButtonSpec {
	return func() []DialogButton {
		return []DialogButton{{Label: localengine.T("dialog", "btn", "confirm"), Style: ButtonPrimary, Action: action}}
	}
}

// Update returns a spec for a primary update button (used in place of confirm
// for update flows).
func Update(action func()) ButtonSpec {
	return func() []DialogButton {
		return []DialogButton{{Label: localengine.T("dialog", "btn", "update"), Style: ButtonPrimary, Action: action}}
	}
}

// Ignore returns a spec for a secondary ignore/skip button.
func Ignore(action func()) ButtonSpec {
	return func() []DialogButton {
		return []DialogButton{{Label: localengine.T("dialog", "btn", "ignore"), Style: ButtonDefault, Action: action}}
	}
}

// Action returns a spec for a generic button.
func Action(label string, action func()) ButtonSpec {
	return func() []DialogButton {
		return []DialogButton{{Label: label, Style: ButtonDefault, Action: action}}
	}
}

// Danger returns a spec for a destructive button.
func Danger(label string, action func()) ButtonSpec {
	return func() []DialogButton {
		return []DialogButton{{Label: label, Style: ButtonDanger, Action: action}}
	}
}

// LoadButtons returns a spec that invokes fn each frame to obtain buttons.
func LoadButtons(fn func() []DialogButton) ButtonSpec {
	return fn
}

// NoButtons explicitly requests no buttons. It overrides the default Close
// button that is otherwise added when Show is called without specs.
func NoButtons() ButtonSpec {
	return func() []DialogButton { return nil }
}

// DialogKind selects how the dialog body is rendered.
type DialogKind int

const (
	DialogText DialogKind = iota
	DialogMarkdown
	DialogCustom
	DialogLoading
	DialogProgress
)

// DialogContent describes the content part of a dialog.
type DialogContent struct {
	Title            string
	Kind             DialogKind
	Text             string
	Markdown         string
	Body             layout.Widget
	Progress         func() float32
	Scrollable       bool
	NoDefaultButtons bool
}

// Text returns a plain-text dialog content.
func Text(title, body string) DialogContent {
	return DialogContent{Title: title, Kind: DialogText, Text: body, Scrollable: true}
}

// Markdown returns a markdown-rendered dialog content.
func Markdown(title, body string) DialogContent {
	return DialogContent{Title: title, Kind: DialogMarkdown, Markdown: body, Scrollable: true}
}

// Custom returns a custom-widget dialog content. By default the body is wrapped
// in a vertical scroller so that action buttons remain visible.
func Custom(title string, body layout.Widget) DialogContent {
	return DialogContent{Title: title, Kind: DialogCustom, Body: body, Scrollable: true}
}

// CustomNoButtons returns a custom-widget dialog content without automatic
// scrolling and without a default Close button. Use this when the caller owns
// closing and the body already contains its own scrollable list.
func CustomNoButtons(title string, body layout.Widget) DialogContent {
	return DialogContent{Title: title, Kind: DialogCustom, Body: body, Scrollable: false}
}

// Loading returns a centered spinner dialog content.
func Loading(title string) DialogContent {
	return DialogContent{Title: title, Kind: DialogLoading}
}

// Progress returns a determinate progress dialog content.
func Progress(title string, p func() float32) DialogContent {
	return DialogContent{Title: title, Kind: DialogProgress, Progress: p}
}

// DialogProvider abstracts modal dialog operations for pages and widgets.
// Implemented by the shell's Dialog so pages don't import the gui package.
type DialogProvider interface {
	// Show displays a dialog. If no ButtonSpec is provided, a default Close
	// button is added. Every button action automatically hides the dialog.
	Show(content DialogContent, buttons ...ButtonSpec)
	Hide()
	Visible() bool
}
