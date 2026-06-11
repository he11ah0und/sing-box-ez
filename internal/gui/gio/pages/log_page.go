package pages

import (
	"image"
	"image/color"
	"io"
	"strings"
	"time"

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
	list     widget.List
	lastText string
	lines    []string
}

// NewLogPage creates a new log page.
func NewLogPage(th *material.Theme, ctrl *core.Controller) *LogPage {
	return &LogPage{
		th:   th,
		ctrl: ctrl,
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
			return material.List(p.th, &p.list).Layout(gtx, len(p.lines), func(gtx layout.Context, index int) layout.Dimensions {
				return p.logLine(gtx, p.lines[index])
			})
		}),
	)
}

func (p *LogPage) logLine(gtx layout.Context, line string) layout.Dimensions {
	bg, fg := logLineColors(line)
	// Fill background for the whole row.
	paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())

	lbl := material.Label(p.th, unit.Sp(12), line)
	lbl.Font.Typeface = "Go Mono"
	lbl.Color = fg
	return layout.UniformInset(unit.Dp(2)).Layout(gtx, lbl.Layout)
}

func logLineColors(line string) (bg, fg color.NRGBA) {
	// Default colors.
	bg = color.NRGBA{R: 20, G: 20, B: 20, A: 255}
	fg = color.NRGBA{R: 200, G: 200, B: 200, A: 255}

	// Extract level from format: [HH:MM:SS] [LEVEL] ...
	if idx := strings.Index(line, "["); idx >= 0 {
		rest := line[idx+1:]
		if idx2 := strings.Index(rest, "]"); idx2 >= 0 {
			level := rest[:idx2]
			switch level {
			case "DBG":
				bg = color.NRGBA{R: 25, G: 35, B: 25, A: 255}
				fg = color.NRGBA{R: 100, G: 200, B: 100, A: 255}
			case "INF":
				bg = color.NRGBA{R: 20, G: 30, B: 50, A: 255}
				fg = color.NRGBA{R: 100, G: 150, B: 255, A: 255}
			case "WRN":
				bg = color.NRGBA{R: 50, G: 40, B: 20, A: 255}
				fg = color.NRGBA{R: 255, G: 200, B: 100, A: 255}
			case "ERR":
				bg = color.NRGBA{R: 50, G: 20, B: 20, A: 255}
				fg = color.NRGBA{R: 255, G: 100, B: 100, A: 255}
			}
		}
	}
	return
}
