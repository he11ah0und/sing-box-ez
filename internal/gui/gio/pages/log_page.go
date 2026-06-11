package pages

import (
	"image"
	"io"
	"strings"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
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
		p.editor.SetText(text)
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
			return ed.Layout(gtx)
		}),
	)
}
