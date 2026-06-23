package giogui

import (
	"fmt"
	"image/color"

	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"gioui.org/x/markdown"
	"gioui.org/x/richtext"
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/framework/util/openurl"
	"sing-box-ez/internal/gui/gio/theme"
)

// DialogContent renders the inner part of a dialog card (title + body).
type DialogContent interface {
	Layout(gtx layout.Context, th *material.Theme) layout.Dimensions
	ShowsButtons() bool
}

const dialogMaxBodyHeight = unit.Dp(420)

func limitMaxHeight(gtx layout.Context, h unit.Dp) layout.Context {
	maxH := gtx.Dp(h)
	if gtx.Constraints.Max.Y > maxH {
		gtx.Constraints.Max.Y = maxH
	}
	if gtx.Constraints.Min.Y > gtx.Constraints.Max.Y {
		gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
	}
	return gtx
}

// Dialog is a reusable modal dialog.
type Dialog struct {
	content DialogContent

	closeLabel   string
	confirmLabel string
	cancelLabel  string

	dismiss widget.Clickable
	confirm widget.Clickable

	onDismiss func()
	onConfirm func()

	active bool
	th     *material.Theme
}

// NewDialog creates a new dialog.
func NewDialog() *Dialog {
	return &Dialog{}
}

func (d *Dialog) show(c DialogContent) {
	d.content = c
	d.active = true
}

// Show displays an informational dialog with a single Close button.
func (d *Dialog) Show(title, body string) {
	d.closeLabel = localengine.T("about", "dialog", "close")
	d.confirmLabel = ""
	d.cancelLabel = ""
	d.onConfirm = nil
	d.onDismiss = nil
	d.show(&textDialogContent{title: title, body: body})
}

// ShowMarkdown displays a dialog with Markdown-rendered body.
func (d *Dialog) ShowMarkdown(title, body string) {
	d.closeLabel = localengine.T("about", "dialog", "close")
	d.confirmLabel = ""
	d.cancelLabel = ""
	d.onConfirm = nil
	d.onDismiss = nil
	d.show(&markdownDialogContent{title: title, body: body})
}

// ShowLoading displays a centered loading dialog with a spinner.
func (d *Dialog) ShowLoading(title string) {
	d.show(&loadingDialogContent{title: title})
}

// HideLoading closes the loading dialog only if it is currently active.
func (d *Dialog) HideLoading() {
	if _, ok := d.content.(*loadingDialogContent); ok {
		d.active = false
		d.content = nil
	}
}

// ShowLoadingWithProgress displays a loading dialog with a determinate progress bar.
// The progress callback is invoked on every frame and should return a value in [0,1].
func (d *Dialog) ShowLoadingWithProgress(title string, progress func() float32) {
	d.show(&loadingDialogContent{title: title, progress: progress})
}

// ShowConfirm displays a dialog with Cancel and Confirm buttons.
// onDismiss is called when the dialog is dismissed without confirming.
func (d *Dialog) ShowConfirm(title, body string, onConfirm func(), onDismiss func()) {
	d.Show(title, body)
	d.confirmLabel = localengine.T("dialog", "btn", "confirm")
	d.cancelLabel = localengine.T("dialog", "btn", "cancel")
	d.onConfirm = onConfirm
	d.onDismiss = onDismiss
}

// ShowConfirmMarkdown displays a Markdown dialog with Cancel and Confirm buttons.
// onDismiss is called when the dialog is dismissed without confirming.
func (d *Dialog) ShowConfirmMarkdown(title, body string, onConfirm func(), onDismiss func()) {
	d.ShowMarkdown(title, body)
	d.confirmLabel = localengine.T("dialog", "btn", "update")
	d.cancelLabel = localengine.T("dialog", "btn", "ignore")
	d.onConfirm = onConfirm
	d.onDismiss = onDismiss
}

// ShowCustom displays a dialog with arbitrary widget content and a Cancel button.
func (d *Dialog) ShowCustom(title string, body layout.Widget) {
	d.closeLabel = localengine.T("dialog", "btn", "cancel")
	d.confirmLabel = ""
	d.cancelLabel = ""
	d.onConfirm = nil
	d.onDismiss = nil
	d.show(&customDialogContent{title: title, body: body, showButtons: true})
}

// ShowCustomNoCancel displays a dialog with arbitrary widget content and no buttons.
// The caller is responsible for closing the dialog programmatically.
func (d *Dialog) ShowCustomNoCancel(title string, body layout.Widget) {
	d.closeLabel = ""
	d.confirmLabel = ""
	d.cancelLabel = ""
	d.onConfirm = nil
	d.onDismiss = nil
	d.show(&customDialogContent{title: title, body: body, showButtons: false})
}

// HideCustom closes the custom dialog.
func (d *Dialog) HideCustom() {
	if _, ok := d.content.(*customDialogContent); ok {
		d.active = false
		d.content = nil
	}
}

// Dismiss closes the dialog programmatically.
func (d *Dialog) Dismiss() {
	d.active = false
	d.content = nil
}

// Visible reports whether the dialog is currently shown.
func (d *Dialog) Visible() bool {
	return d.active
}

// Layout renders the dialog if active.
func (d *Dialog) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	d.th = th
	if !d.active || d.content == nil {
		return layout.Dimensions{}
	}

	// Keep redrawing while the dialog is shown so state changes from other
	// goroutines become visible without waiting for the next input event.
	gtx.Execute(op.InvalidateCmd{})

	// Draw semi-transparent backdrop over the entire area.
	backdrop := theme.Current().Colors().Backdrop
	paint.FillShape(gtx.Ops, backdrop, clip.Rect{Max: gtx.Constraints.Max}.Op())

	// Capture pointer events on the backdrop so they don't leak through to the UI below.
	area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, d)
	area.Pop()

	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return d.layoutCard(gtx, th)
		}),
	)
}

func (d *Dialog) layoutCard(gtx layout.Context, th *material.Theme) layout.Dimensions {
	maxWidth := gtx.Dp(unit.Dp(480))
	if _, ok := d.content.(*loadingDialogContent); ok {
		maxWidth = gtx.Dp(unit.Dp(360))
	}
	if gtx.Constraints.Max.X < maxWidth {
		maxWidth = gtx.Constraints.Max.X - gtx.Dp(unit.Dp(32))
	}
	maxHeight := gtx.Constraints.Max.Y - gtx.Dp(unit.Dp(64))

	inset := unit.Dp(16)
	cardGtx := gtx
	cardGtx.Constraints.Min.X = 0
	cardGtx.Constraints.Max.X = maxWidth
	cardGtx.Constraints.Min.Y = 0
	cardGtx.Constraints.Max.Y = maxHeight

	return component.Surface(th).Layout(cardGtx, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(inset).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return d.content.Layout(gtx, th)
				}),
			}
			if d.content.ShowsButtons() {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return d.layoutButtons(gtx, th)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (d *Dialog) layoutButtons(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if d.onConfirm != nil {
		prevDismiss := len(d.dismiss.History())
		prevConfirm := len(d.confirm.History())

		dims := layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceEnd}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return material.Button(th, &d.dismiss, d.cancelLabel).Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Button(th, &d.confirm, d.confirmLabel).Layout(gtx)
			}),
		)

		if len(d.dismiss.History()) > prevDismiss {
			d.Dismiss()
			if d.onDismiss != nil {
				d.onDismiss()
			}
		}
		if len(d.confirm.History()) > prevConfirm {
			d.Dismiss()
			if d.onConfirm != nil {
				d.onConfirm()
			}
		}
		return dims
	}

	prevDismiss := len(d.dismiss.History())
	dims := layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceEnd}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Button(th, &d.dismiss, d.closeLabel).Layout(gtx)
		}),
	)
	if len(d.dismiss.History()) > prevDismiss {
		d.Dismiss()
		if d.onDismiss != nil {
			d.onDismiss()
		}
	}
	return dims
}

// ---------- Content implementations ----------

type textDialogContent struct {
	title string
	body  string
	list  widget.List
}

func (c *textDialogContent) ShowsButtons() bool { return true }

func (c *textDialogContent) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.H6(th, c.title).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx = limitMaxHeight(gtx, dialogMaxBodyHeight)
				c.list.Axis = layout.Vertical
				list := material.List(th, &c.list)
				list.AnchorStrategy = material.Overlay
				return list.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
					return material.Body1(th, c.body).Layout(gtx)
				})
			})
		}),
	)
}

type markdownDialogContent struct {
	title     string
	body      string
	renderer  *markdown.Renderer
	richState richtext.InteractiveText
	spans     []richtext.SpanStyle
	list      widget.List
}

func (c *markdownDialogContent) ShowsButtons() bool { return true }

func (c *markdownDialogContent) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if c.renderer == nil {
		c.renderer = markdown.NewRenderer()
		c.renderer.Config.DefaultSize = unit.Sp(13)
		c.renderer.Config.H1Size = unit.Sp(18)
		c.renderer.Config.H2Size = unit.Sp(16)
		c.renderer.Config.H3Size = unit.Sp(15)
		c.renderer.Config.H4Size = unit.Sp(14)
		c.renderer.Config.H5Size = unit.Sp(13)
		c.renderer.Config.H6Size = unit.Sp(13)
	}
	c.setThemeColors(th.Palette.Fg, th.Palette.ContrastBg)
	if c.spans == nil {
		spans, err := c.renderer.Render([]byte(c.body))
		if err != nil {
			c.spans = nil
			return (&textDialogContent{title: c.title, body: c.body, list: widget.List{List: layout.List{Axis: layout.Vertical}}}).Layout(gtx, th)
		}
		c.spans = spans
	}

	// Handle link clicks.
	for {
		span, event, ok := c.richState.Update(gtx)
		if !ok {
			break
		}
		if event.Type == richtext.Click {
			if url := span.Get(markdown.MetadataURL); url != nil {
				if urlStr, ok := url.(string); ok {
					_ = openurl.OpenURL(urlStr)
				}
			}
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.H6(th, c.title).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx = limitMaxHeight(gtx, dialogMaxBodyHeight)
				c.list.Axis = layout.Vertical
				list := material.List(th, &c.list)
				list.AnchorStrategy = material.Overlay
				return list.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
					style := richtext.Text(&c.richState, th.Shaper, c.spans...)
					style.Alignment = text.Start
					return style.Layout(gtx)
				})
			})
		}),
	)
}

func (c *markdownDialogContent) setThemeColors(fg, link color.NRGBA) {
	if c.renderer == nil {
		return
	}
	c.renderer.Config.DefaultColor = fg
	c.renderer.Config.InteractiveColor = link
}

type loadingDialogContent struct {
	title    string
	progress func() float32
}

func (c *loadingDialogContent) ShowsButtons() bool { return false }

func (c *loadingDialogContent) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(material.H6(th, c.title).Layout),
	}

	if c.progress != nil {
		gtx.Execute(op.InvalidateCmd{})
		progress := c.progress()
		if progress < 0 {
			progress = 0
		}
		if progress > 1 {
			progress = 1
		}
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(24), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return material.ProgressBar(th, progress).Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Body2(th, fmt.Sprintf("%d%%", int(progress*100))).Layout(gtx)
			}),
		)
	} else {
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(24), Bottom: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Dp(unit.Dp(48))
					gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(48))
					return material.Loader(th).Layout(gtx)
				})
			}),
		)
	}

	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, children...)
}

type customDialogContent struct {
	title       string
	body        layout.Widget
	showButtons bool
}

func (c *customDialogContent) ShowsButtons() bool { return c.showButtons }

func (c *customDialogContent) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.H6(th, c.title).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx = limitMaxHeight(gtx, dialogMaxBodyHeight)
				if c.body != nil {
					return c.body(gtx)
				}
				return layout.Dimensions{}
			})
		}),
	)
}
