package giogui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"gioui.org/x/markdown"
	"gioui.org/x/richtext"
	"sing-box-ez/internal/i18n"
	"sing-box-ez/internal/util"
)

// Dialog is a reusable modal dialog using a ModalLayer.
type Dialog struct {
	modal *component.ModalLayer

	title string
	body  string

	closeLabel   string
	confirmLabel string
	cancelLabel  string

	dismiss widget.Clickable
	confirm widget.Clickable

	onDismiss func()
	onConfirm func()

	active bool

	bodyList widget.List

	// Markdown support
	isMarkdown bool
	richState  richtext.InteractiveText
	mdRenderer *markdown.Renderer
	mdSpans    []richtext.SpanStyle

	// Theme reference for markdown colors.
	th *material.Theme

	// Loading state
	isLoading    bool
	loadingTitle string
}

// NewDialog creates a new dialog backed by a fresh ModalLayer.
func NewDialog() *Dialog {
	return &Dialog{
		modal: component.NewModal(),
		bodyList: widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		mdRenderer: markdown.NewRenderer(),
	}
}

// Show displays an informational dialog with a single Close button.
func (d *Dialog) Show(title, body string) {
	d.title = title
	d.body = body
	d.isMarkdown = false
	d.isLoading = false
	d.closeLabel = i18n.T("about.dialog.close")
	d.confirmLabel = ""
	d.cancelLabel = ""
	d.onConfirm = nil
	d.onDismiss = nil
	d.active = true
	d.bodyList.Position = layout.Position{}
}

// ShowMarkdown displays a dialog with Markdown-rendered body.
func (d *Dialog) ShowMarkdown(title, body string) {
	d.title = title
	d.body = body
	d.isMarkdown = true
	d.isLoading = false
	d.closeLabel = i18n.T("about.dialog.close")
	d.confirmLabel = ""
	d.cancelLabel = ""
	d.onConfirm = nil
	d.onDismiss = nil
	d.active = true
	d.bodyList.Position = layout.Position{}

	// Use compact sizes so markdown headings aren't overwhelming in a dialog.
	d.mdRenderer.Config.DefaultSize = unit.Sp(13)
	d.mdRenderer.Config.H1Size = unit.Sp(18)
	d.mdRenderer.Config.H2Size = unit.Sp(16)
	d.mdRenderer.Config.H3Size = unit.Sp(15)
	d.mdRenderer.Config.H4Size = unit.Sp(14)
	d.mdRenderer.Config.H5Size = unit.Sp(13)
	d.mdRenderer.Config.H6Size = unit.Sp(13)

	// Apply theme colors before rendering so spans have the right color.
	if d.th != nil {
		d.SetThemeColors(d.th.Palette.Fg, d.th.Palette.ContrastBg)
	}

	spans, err := d.mdRenderer.Render([]byte(body))
	if err != nil {
		// Fallback to plain text if markdown parsing fails.
		d.isMarkdown = false
		return
	}
	d.mdSpans = spans
}

// SetThemeColors updates the markdown renderer default colors from the theme.
func (d *Dialog) SetThemeColors(fg, link color.NRGBA) {
	d.mdRenderer.Config.DefaultColor = fg
	d.mdRenderer.Config.InteractiveColor = link
}

// ShowLoading displays a centered loading dialog with a spinner.
func (d *Dialog) ShowLoading(title string) {
	d.loadingTitle = title
	d.isLoading = true
	d.active = true
}

// HideLoading closes the loading dialog.
func (d *Dialog) HideLoading() {
	d.isLoading = false
	d.active = false
}

// ShowConfirm displays a dialog with Cancel and Confirm buttons.
func (d *Dialog) ShowConfirm(title, body string, onConfirm func()) {
	d.Show(title, body)
	d.confirmLabel = i18n.T("dialog.btn.confirm")
	d.cancelLabel = i18n.T("dialog.btn.cancel")
	d.onConfirm = onConfirm
}

// Dismiss closes the dialog programmatically.
func (d *Dialog) Dismiss() {
	d.active = false
}

// Visible reports whether the dialog is currently shown.
func (d *Dialog) Visible() bool {
	return d.active || d.modal.Visible()
}

// Layout renders the dialog if active.
func (d *Dialog) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	d.th = th
	if !d.active {
		if d.modal.Visible() {
			d.modal.Disappear(gtx.Now)
		}
		return layout.Dimensions{}
	}

	d.modal.Widget = func(gtx layout.Context, th *material.Theme, anim *component.VisibilityAnimation) layout.Dimensions {
		return d.layoutContent(gtx, th)
	}

	if !d.modal.Visible() {
		d.modal.Appear(gtx.Now)
	}

	return d.modal.Layout(gtx, th)
}

func (d *Dialog) layoutContent(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if d.isLoading {
		return d.layoutLoading(gtx, th)
	}
	maxWidth := gtx.Dp(unit.Dp(480))
	if gtx.Constraints.Max.X < maxWidth {
		maxWidth = gtx.Constraints.Max.X - gtx.Dp(unit.Dp(32))
	}
	// Limit height so the dialog doesn't overflow the screen.
	maxHeight := gtx.Constraints.Max.Y - gtx.Dp(unit.Dp(64))

	macro := op.Record(gtx.Ops)
	cardGtx := gtx
	cardGtx.Constraints.Min.X = maxWidth
	cardGtx.Constraints.Max.X = maxWidth
	cardGtx.Constraints.Min.Y = 0
	cardGtx.Constraints.Max.Y = maxHeight

	cardDims := component.Surface(th).Layout(cardGtx, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.H6(th, d.title).Layout(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return d.layoutScrollableBody(gtx, th)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return d.layoutButtons(gtx, th)
				}),
			)
		})
	})
	call := macro.Stop()

	offset := image.Point{
		X: (gtx.Constraints.Max.X - cardDims.Size.X) / 2,
		Y: (gtx.Constraints.Max.Y - cardDims.Size.Y) / 2,
	}
	if offset.X < 0 {
		offset.X = 0
	}
	if offset.Y < 0 {
		offset.Y = 0
	}

	trans := op.Offset(offset).Push(gtx.Ops)
	call.Add(gtx.Ops)
	trans.Pop()

	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (d *Dialog) layoutScrollableBody(gtx layout.Context, th *material.Theme) layout.Dimensions {
	listStyle := material.List(th, &d.bodyList)
	listStyle.AnchorStrategy = material.Overlay
	return listStyle.Layout(gtx, 1, func(gtx layout.Context, index int) layout.Dimensions {
		if d.isMarkdown {
			return d.layoutMarkdown(gtx, th)
		}
		return material.Body1(th, d.body).Layout(gtx)
	})
}

func (d *Dialog) layoutLoading(gtx layout.Context, th *material.Theme) layout.Dimensions {
	maxWidth := gtx.Dp(unit.Dp(480))
	if gtx.Constraints.Max.X < maxWidth {
		maxWidth = gtx.Constraints.Max.X - gtx.Dp(unit.Dp(32))
	}

	macro := op.Record(gtx.Ops)
	cardGtx := gtx
	cardGtx.Constraints.Min.X = maxWidth
	cardGtx.Constraints.Max.X = maxWidth
	cardGtx.Constraints.Min.Y = 0
	cardGtx.Constraints.Max.Y = gtx.Constraints.Max.Y - gtx.Dp(unit.Dp(64))

	cardDims := component.Surface(th).Layout(cardGtx, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.H6(th, d.loadingTitle).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(24), Bottom: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Dp(48)
						gtx.Constraints.Min.Y = gtx.Dp(48)
						return material.Loader(th).Layout(gtx)
					})
				}),
			)
		})
	})
	call := macro.Stop()

	offset := image.Point{
		X: (gtx.Constraints.Max.X - cardDims.Size.X) / 2,
		Y: (gtx.Constraints.Max.Y - cardDims.Size.Y) / 2,
	}
	if offset.X < 0 {
		offset.X = 0
	}
	if offset.Y < 0 {
		offset.Y = 0
	}

	trans := op.Offset(offset).Push(gtx.Ops)
	call.Add(gtx.Ops)
	trans.Pop()

	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (d *Dialog) layoutMarkdown(gtx layout.Context, th *material.Theme) layout.Dimensions {
	// Ensure colors match the current theme.
	d.SetThemeColors(th.Palette.Fg, th.Palette.ContrastBg)

	// Handle link clicks.
	for {
		span, event, ok := d.richState.Update(gtx)
		if !ok {
			break
		}
		if event.Type == richtext.Click {
			if url := span.Get(markdown.MetadataURL); url != nil {
				if urlStr, ok := url.(string); ok {
					_ = util.OpenURL(urlStr)
				}
			}
		}
	}

	style := richtext.Text(&d.richState, th.Shaper, d.mdSpans...)
	style.Alignment = text.Start
	return style.Layout(gtx)
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
			d.active = false
			if d.onDismiss != nil {
				d.onDismiss()
			}
		}
		if len(d.confirm.History()) > prevConfirm {
			d.active = false
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
		d.active = false
		if d.onDismiss != nil {
			d.onDismiss()
		}
	}
	return dims
}
