package giogui

import (
	"fmt"
	"image"
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
	"sing-box-ez/internal/gui/gio/widgets"
)

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
	content widgets.DialogContent
	specs   []widgets.ButtonSpec

	// Effective buttons collected from specs each frame.
	buttons []widgets.DialogButton
	// Clickables for the effective buttons, grown as needed.
	buttonClicks []*widget.Clickable
	// Reused list for scrollable text/markdown/custom bodies.
	contentList widget.List

	// Cached markdown state for the current dialog.
	mdRenderer *markdown.Renderer
	mdSpans    []richtext.SpanStyle
	mdState    richtext.InteractiveText

	active bool
	th     *material.Theme
}

// NewDialog creates a new dialog.
func NewDialog() *Dialog {
	return &Dialog{}
}

// Show displays a dialog. If no ButtonSpec is provided, a default Close button
// is added for text/markdown/custom content. Loading/progress dialogs have no
// buttons unless explicitly supplied.
func (d *Dialog) Show(content widgets.DialogContent, specs ...widgets.ButtonSpec) {
	d.content = content
	d.specs = specs
	d.buttons = d.buttons[:0]
	// Invalidate cached markdown so a previous dialog doesn't reuse the wrong spans.
	d.mdSpans = nil
	d.active = true
}

// Hide closes the dialog programmatically.
func (d *Dialog) Hide() {
	d.active = false
	d.content = widgets.DialogContent{}
	d.specs = nil
	d.buttons = d.buttons[:0]
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
	if d.content.Kind == widgets.DialogLoading || d.content.Kind == widgets.DialogProgress {
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
				// Rigid so the dialog is only as tall as its content, not stretched
				// to fill the maximum available height.
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return d.layoutContent(gtx, th)
				}),
			}
			btns := d.collectButtons()
			if len(btns) > 0 {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return d.layoutButtons(gtx, th, btns)
					})
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (d *Dialog) collectButtons() []widgets.DialogButton {
	d.buttons = d.buttons[:0]
	for _, spec := range d.specs {
		if spec == nil {
			continue
		}
		d.buttons = append(d.buttons, spec()...)
	}
	if len(d.buttons) == 0 && !d.content.NoDefaultButtons {
		if d.content.Kind != widgets.DialogLoading && d.content.Kind != widgets.DialogProgress {
			d.buttons = append(d.buttons, widgets.DialogButton{
				Label: localengine.T("dialog", "btn", "close"),
			})
		}
	}
	return d.buttons
}

func (d *Dialog) layoutContent(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if d.content.Title != "" {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.H6(th, d.content.Title).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return d.layoutBody(gtx, th)
				})
			}),
		)
	}
	return d.layoutBody(gtx, th)
}

func (d *Dialog) layoutBody(gtx layout.Context, th *material.Theme) layout.Dimensions {
	gtx = limitMaxHeight(gtx, dialogMaxBodyHeight)

	switch d.content.Kind {
	case widgets.DialogText:
		body := func(gtx layout.Context) layout.Dimensions {
			return material.Body1(th, d.content.Text).Layout(gtx)
		}
		return d.scrollable(gtx, th, body)
	case widgets.DialogMarkdown:
		return d.layoutMarkdownBody(gtx, th)
	case widgets.DialogCustom:
		if d.content.Scrollable {
			return d.scrollable(gtx, th, d.content.Body)
		}
		if d.content.Body != nil {
			return d.content.Body(gtx)
		}
		return layout.Dimensions{}
	case widgets.DialogLoading:
		return d.layoutLoading(gtx, th)
	case widgets.DialogProgress:
		return d.layoutProgress(gtx, th)
	default:
		return layout.Dimensions{}
	}
}

func (d *Dialog) scrollable(gtx layout.Context, th *material.Theme, body layout.Widget) layout.Dimensions {
	d.contentList.Axis = layout.Vertical
	return material.List(th, &d.contentList).Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return body(gtx)
	})
}

func (d *Dialog) layoutMarkdownBody(gtx layout.Context, th *material.Theme) layout.Dimensions {
	// Reuse the markdown renderer across layouts for the same dialog.
	if d.mdRenderer == nil {
		d.mdRenderer = markdown.NewRenderer()
		d.mdRenderer.Config.DefaultSize = unit.Sp(13)
		d.mdRenderer.Config.H1Size = unit.Sp(18)
		d.mdRenderer.Config.H2Size = unit.Sp(16)
		d.mdRenderer.Config.H3Size = unit.Sp(15)
		d.mdRenderer.Config.H4Size = unit.Sp(14)
		d.mdRenderer.Config.H5Size = unit.Sp(13)
		d.mdRenderer.Config.H6Size = unit.Sp(13)
	}
	d.setMarkdownThemeColors(th.Palette.Fg, th.Palette.ContrastBg)

	if d.mdSpans == nil {
		spans, err := d.mdRenderer.Render([]byte(d.content.Markdown))
		if err != nil {
			return d.scrollable(gtx, th, func(gtx layout.Context) layout.Dimensions {
				return material.Body1(th, d.content.Markdown).Layout(gtx)
			})
		}
		d.mdSpans = spans
	}

	for {
		span, ev, ok := d.mdState.Update(gtx)
		if !ok {
			break
		}
		if ev.Type == richtext.Click {
			if url := span.Get(markdown.MetadataURL); url != nil {
				if urlStr, ok := url.(string); ok {
					_ = openurl.OpenURL(urlStr)
				}
			}
		}
	}

	return d.scrollable(gtx, th, func(gtx layout.Context) layout.Dimensions {
		style := richtext.Text(&d.mdState, th.Shaper, d.mdSpans...)
		style.Alignment = text.Start
		return style.Layout(gtx)
	})
}

func (d *Dialog) setMarkdownThemeColors(fg, link color.NRGBA) {
	if d.mdRenderer == nil {
		return
	}
	d.mdRenderer.Config.DefaultColor = fg
	d.mdRenderer.Config.InteractiveColor = link
}

func (d *Dialog) layoutLoading(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(24), Bottom: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Dp(unit.Dp(48))
				gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(48))
				return material.Loader(th).Layout(gtx)
			})
		}),
	)
}

func (d *Dialog) layoutProgress(gtx layout.Context, th *material.Theme) layout.Dimensions {
	gtx.Execute(op.InvalidateCmd{})
	progress := float32(0)
	if d.content.Progress != nil {
		progress = d.content.Progress()
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(24), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.ProgressBar(th, progress).Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(th, fmt.Sprintf("%d%%", int(progress*100))).Layout(gtx)
		}),
	)
}

func (d *Dialog) layoutButtons(gtx layout.Context, th *material.Theme, buttons []widgets.DialogButton) layout.Dimensions {
	// Ensure we have enough clickables for the current buttons.
	for len(d.buttonClicks) < len(buttons) {
		d.buttonClicks = append(d.buttonClicks, &widget.Clickable{})
	}

	// Measure natural widths to decide wrapping.
	const gapDp = 8
	gap := gtx.Dp(unit.Dp(gapDp))
	minBtn := gtx.Dp(unit.Dp(64))
	avail := gtx.Constraints.Max.X
	if avail <= 0 {
		avail = gtx.Constraints.Min.X
	}

	natural := make([]int, len(buttons))
	for i, b := range buttons {
		w := d.measureButtonWidth(gtx, th, b.Label)
		if w < minBtn {
			w = minBtn
		}
		natural[i] = w
	}

	// Split buttons into rows that fit.
	var rows [][]int
	var row []int
	rowWidth := 0
	for i, w := range natural {
		add := w
		if len(row) > 0 {
			add += gap
		}
		if len(row) > 0 && rowWidth+add > avail {
			rows = append(rows, row)
			row = []int{i}
			rowWidth = w
		} else {
			row = append(row, i)
			rowWidth += add
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	// Render rows.
	rowChildren := make([]layout.FlexChild, 0, len(rows)*2-1)
	for ri, row := range rows {
		if ri > 0 {
			rowChildren = append(rowChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widgets.VSpace(gtx, unit.Dp(gapDp))
			}))
		}
		rowChildren = append(rowChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.layoutButtonRow(gtx, th, buttons, row)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rowChildren...)
}

func (d *Dialog) layoutButtonRow(gtx layout.Context, th *material.Theme, buttons []widgets.DialogButton, indices []int) layout.Dimensions {
	const gapDp = 8
	children := make([]layout.FlexChild, 0, len(indices)*2-1)
	for i, idx := range indices {
		if i > 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widgets.HSpace(gtx, unit.Dp(gapDp))
			}))
		}
		idx := idx
		children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return d.layoutButton(gtx, th, &buttons[idx], d.buttonClicks[idx])
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx, children...)
}

func (d *Dialog) layoutButton(gtx layout.Context, th *material.Theme, button *widgets.DialogButton, click *widget.Clickable) layout.Dimensions {
	btn := material.Button(th, click, button.Label)
	colors := theme.Current().Colors()
	switch button.Style {
	case widgets.ButtonDefault:
		btn.Background = colors.Primary
		btn.Color = colors.OnPrimary
	case widgets.ButtonPrimary:
		btn.Background = colors.Success
		btn.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	case widgets.ButtonDanger:
		btn.Background = colors.Error
		btn.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}
	// Check for a click before Layout consumes the pointer event.
	clicked := click.Clicked(gtx)
	dims := btn.Layout(gtx)
	if clicked {
		// Always close the dialog when a button is clicked. Run the action in a
		// goroutine so long-running callbacks don't block the UI thread.
		d.Hide()
		if button.Action != nil {
			go button.Action()
		}
	}
	return dims
}

func (d *Dialog) measureButtonWidth(gtx layout.Context, th *material.Theme, label string) int {
	// Use a throw-away clickable so measuring does not consume events from the
	// real clickables used for rendering.
	dummy := &widget.Clickable{}
	macro := op.Record(gtx.Ops)
	mgtx := gtx
	mgtx.Constraints.Min = image.Point{}
	mgtx.Constraints.Max.X = int(1e6)
	dims := material.Button(th, dummy, label).Layout(mgtx)
	_ = macro.Stop()
	return dims.Size.X
}
