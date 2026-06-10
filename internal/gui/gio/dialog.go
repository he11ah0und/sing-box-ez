package giogui

import (
	"image/color"

	"gioui.org/io/event"
	"gioui.org/layout"
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
)

// Dialog is a reusable modal dialog.
type Dialog struct {
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

	// Custom content state
	isCustom    bool
	customTitle string
	customBody  layout.Widget
}

// NewDialog creates a new dialog.
func NewDialog() *Dialog {
	return &Dialog{
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
	d.isCustom = false
	d.closeLabel = localengine.T("about", "dialog", "close")
	d.confirmLabel = ""
	d.cancelLabel = ""
	d.onConfirm = nil
	d.onDismiss = nil
	d.active = true
	d.bodyList = widget.List{List: layout.List{Axis: layout.Vertical}}
}

// ShowMarkdown displays a dialog with Markdown-rendered body.
func (d *Dialog) ShowMarkdown(title, body string) {
	d.title = title
	d.body = body
	d.isMarkdown = true
	d.isLoading = false
	d.isCustom = false
	d.closeLabel = localengine.T("about", "dialog", "close")
	d.confirmLabel = ""
	d.cancelLabel = ""
	d.onConfirm = nil
	d.onDismiss = nil
	d.active = true
	d.bodyList = widget.List{List: layout.List{Axis: layout.Vertical}}

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
	d.isCustom = false
	d.active = true
}

// HideLoading closes the loading dialog only if it is currently active.
func (d *Dialog) HideLoading() {
	if d.isLoading {
		d.isLoading = false
		d.active = false
	}
}

// ShowConfirm displays a dialog with Cancel and Confirm buttons.
func (d *Dialog) ShowConfirm(title, body string, onConfirm func()) {
	d.Show(title, body)
	d.confirmLabel = localengine.T("dialog", "btn", "confirm")
	d.cancelLabel = localengine.T("dialog", "btn", "cancel")
	d.onConfirm = onConfirm
}

// ShowConfirmMarkdown displays a Markdown dialog with Cancel and Confirm buttons.
func (d *Dialog) ShowConfirmMarkdown(title, body string, onConfirm func()) {
	d.ShowMarkdown(title, body)
	d.confirmLabel = localengine.T("dialog", "btn", "update")
	d.cancelLabel = localengine.T("dialog", "btn", "ignore")
	d.onConfirm = onConfirm
}

// ShowCustom displays a dialog with arbitrary widget content and a Cancel button.
func (d *Dialog) ShowCustom(title string, content layout.Widget) {
	d.customTitle = title
	d.customBody = content
	d.isCustom = true
	d.isLoading = false
	d.isMarkdown = false
	d.closeLabel = localengine.T("dialog", "btn", "cancel")
	d.confirmLabel = ""
	d.cancelLabel = ""
	d.onConfirm = nil
	d.onDismiss = nil
	d.active = true
}

// HideCustom closes the custom dialog.
func (d *Dialog) HideCustom() {
	d.isCustom = false
	d.active = false
}

// Dismiss closes the dialog programmatically.
func (d *Dialog) Dismiss() {
	d.active = false
	d.isCustom = false
	d.isMarkdown = false
	d.isLoading = false
}

// Visible reports whether the dialog is currently shown.
func (d *Dialog) Visible() bool {
	return d.active
}

// Layout renders the dialog if active.
func (d *Dialog) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	d.th = th
	if !d.active {
		return layout.Dimensions{}
	}

	// Draw semi-transparent black backdrop over the entire area.
	backdrop := color.NRGBA{R: 0, G: 0, B: 0, A: 160}
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
			return d.layoutContent(gtx, th)
		}),
	)
}

func (d *Dialog) layoutContent(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if d.isLoading {
		return d.layoutLoading(gtx, th)
	}
	if d.isCustom {
		return d.layoutCustom(gtx, th)
	}
	maxWidth := gtx.Dp(unit.Dp(480))
	if gtx.Constraints.Max.X < maxWidth {
		maxWidth = gtx.Constraints.Max.X - gtx.Dp(unit.Dp(32))
	}
	// Limit height so the dialog doesn't overflow the screen.
	maxHeight := gtx.Constraints.Max.Y - gtx.Dp(unit.Dp(64))

	cardGtx := gtx
	cardGtx.Constraints.Min.X = 0
	cardGtx.Constraints.Max.X = maxWidth
	cardGtx.Constraints.Min.Y = 0
	cardGtx.Constraints.Max.Y = maxHeight

	return component.Surface(th).Layout(cardGtx, func(gtx layout.Context) layout.Dimensions {
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
	maxWidth := gtx.Dp(unit.Dp(360))
	if gtx.Constraints.Max.X < maxWidth {
		maxWidth = gtx.Constraints.Max.X - gtx.Dp(unit.Dp(32))
	}

	cardGtx := gtx
	cardGtx.Constraints.Min.X = 0
	cardGtx.Constraints.Max.X = maxWidth
	cardGtx.Constraints.Min.Y = 0

	return component.Surface(th).Layout(cardGtx, func(gtx layout.Context) layout.Dimensions {
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
}

func (d *Dialog) layoutCustom(gtx layout.Context, th *material.Theme) layout.Dimensions {
	maxWidth := gtx.Dp(unit.Dp(480))
	if gtx.Constraints.Max.X < maxWidth {
		maxWidth = gtx.Constraints.Max.X - gtx.Dp(unit.Dp(32))
	}
	maxHeight := gtx.Constraints.Max.Y - gtx.Dp(unit.Dp(64))

	cardGtx := gtx
	cardGtx.Constraints.Min.X = 0
	cardGtx.Constraints.Max.X = maxWidth
	cardGtx.Constraints.Min.Y = 0
	cardGtx.Constraints.Max.Y = maxHeight

	return component.Surface(th).Layout(cardGtx, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.H6(th, d.customTitle).Layout(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						if d.customBody != nil {
							return d.customBody(gtx)
						}
						return layout.Dimensions{}
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return d.layoutButtons(gtx, th)
				}),
			)
		})
	})
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
					_ = openurl.OpenURL(urlStr)
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
