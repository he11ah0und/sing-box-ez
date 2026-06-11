package pages

import (
	"image"
	"image/color"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gio.tools/icons"

	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework/localengine"
)

// LogPage renders the log viewer.
type LogPage struct {
	th   *material.Theme
	ctrl *core.Controller

	copyBtn  widget.Clickable
	clearBtn widget.Clickable
	editor   widget.Editor
	lastText string
	lines    []string
	// runeOffsets[i] = [start,end) rune offsets of lines[i] inside the editor text.
	runeOffsets [][2]int
}

// NewLogPage creates a new log page.
func NewLogPage(th *material.Theme, ctrl *core.Controller) *LogPage {
	ed := widget.Editor{
		SingleLine: false,
		ReadOnly:   true,
	}
	return &LogPage{
		th:     th,
		ctrl:   ctrl,
		editor: ed,
	}
}

// Tag returns the page tag.
func (p *LogPage) Tag() string { return "logs" }

// Name returns the page name.
func (p *LogPage) Name() string { return localengine.T("tab", "log") }
func (p *LogPage) Icon() *widget.Icon { return icons.ActionBugReport }

// Layout draws the log page.
func (p *LogPage) Layout(gtx layout.Context) layout.Dimensions {
	// Schedule periodic refresh to poll for new log lines.
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(100 * time.Millisecond)})

	if p.copyBtn.Clicked(gtx) {
		lines := p.ctrl.GetLogLines()
		text := strings.Join(lines, "\n")
		gtx.Execute(clipboard.WriteCmd{
			Type: "text/plain",
			Data: io.NopCloser(strings.NewReader(text)),
		})
	}
	if p.clearBtn.Clicked(gtx) {
		p.ctrl.ClearLogs()
	}

	lines := p.ctrl.GetLogLines()
	text := strings.Join(lines, "\n")
	if text != p.lastText {
		p.lastText = text
		p.lines = lines
		p.editor.SetText(text)
		// Compute rune offsets for each line so we can query Regions later.
		p.runeOffsets = make([][2]int, len(lines))
		offset := 0
		for i, line := range lines {
			p.runeOffsets[i] = [2]int{offset, offset + utf8.RuneCountInString(line)}
			offset += utf8.RuneCountInString(line) + 1 // +1 for '\n'
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceStart}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Button(p.th, &p.copyBtn, localengine.T("log", "btn", "copy")).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Point{X: gtx.Dp(unit.Dp(8)), Y: 0}}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Button(p.th, &p.clearBtn, localengine.T("log", "btn", "clear")).Layout(gtx)
				}),
			)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			ed := material.Editor(p.th, &p.editor, "")
			ed.Font.Typeface = "Go Mono"
			ed.TextSize = unit.Sp(12)

			// Record the editor so we can paint backgrounds behind the text.
			macro := op.Record(gtx.Ops)
			dims := ed.Layout(gtx)
			call := macro.Stop()

			// Paint per-line backgrounds using the shaped text regions.
			for i := range p.lines {
				if i >= len(p.runeOffsets) {
					break
				}
				start, end := p.runeOffsets[i][0], p.runeOffsets[i][1]
				regions := p.editor.Regions(start, end, nil)
				bg, _ := logLineColors(p.lines[i])
				for _, r := range regions {
					paint.FillShape(gtx.Ops, bg, clip.Rect(r.Bounds).Op())
				}
			}

			call.Add(gtx.Ops)
			return dims
		}),
	)
}

func logLineColors(line string) (bg, fg color.NRGBA) {
	// Default colors.
	bg = color.NRGBA{R: 20, G: 20, B: 20, A: 255}
	fg = color.NRGBA{R: 200, G: 200, B: 200, A: 255}

	switch {
	case strings.Contains(line, "[DBG]"):
		bg = color.NRGBA{R: 25, G: 35, B: 25, A: 255}
		fg = color.NRGBA{R: 100, G: 200, B: 100, A: 255}
	case strings.Contains(line, "[INF]"):
		bg = color.NRGBA{R: 20, G: 30, B: 50, A: 255}
		fg = color.NRGBA{R: 100, G: 150, B: 255, A: 255}
	case strings.Contains(line, "[WRN]"):
		bg = color.NRGBA{R: 50, G: 40, B: 20, A: 255}
		fg = color.NRGBA{R: 255, G: 200, B: 100, A: 255}
	case strings.Contains(line, "[ERR]"):
		bg = color.NRGBA{R: 50, G: 20, B: 20, A: 255}
		fg = color.NRGBA{R: 255, G: 100, B: 100, A: 255}
	}
	return
}
