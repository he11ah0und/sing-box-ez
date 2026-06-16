package pages

import (
	"image"
	"image/color"
	"io"
	"strings"
	"time"

	"gio.tools/icons"
	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/styledtext"

	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/framework/logger"
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
	p := &LogPage{
		th:   th,
		ctrl: ctrl,
	}
	p.list.Axis = layout.Vertical
	return p
}

// Tag returns the page tag.
func (p *LogPage) Tag() string { return "logs" }

// Name returns the page name.
func (p *LogPage) Name() string       { return localengine.T("tab", "log") }
func (p *LogPage) Icon() *widget.Icon { return icons.ActionBugReport }

// NoInset tells the shell not to wrap this page in padding.
func (p *LogPage) NoInset() bool { return true }

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

	lines := p.ctrl.GetLogLinesAtLeast(levelFromString(p.ctrl.Config().GetLogLevel()))
	text := strings.Join(lines, "\n")
	if text != p.lastText {
		p.lastText = text
		p.lines = lines
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
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
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Point{Y: gtx.Dp(unit.Dp(8))}}
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			listStyle := material.List(p.th, &p.list)
			listStyle.AnchorStrategy = material.Overlay
			return listStyle.Layout(gtx, len(p.lines), func(gtx layout.Context, index int) layout.Dimensions {
				return p.logLine(gtx, p.lines[index])
			})
		}),
	)
}

func (p *LogPage) logLine(gtx layout.Context, line string) layout.Dimensions {
	bg := logLineBg(line)
	parts := parseLogLine(line)

	styles := make([]styledtext.SpanStyle, len(parts))
	for i, part := range parts {
		styles[i] = styledtext.SpanStyle{
			Content: part.text,
			Color:   part.color,
			Font:    font.Font{Typeface: "Go Mono"},
			Size:    unit.Sp(12),
		}
	}

	macro := op.Record(gtx.Ops)
	txt := styledtext.Text(p.th.Shaper, styles...)
	dims := txt.Layout(gtx, nil)
	call := macro.Stop()

	paint.FillShape(gtx.Ops, bg, clip.Rect{Max: dims.Size}.Op())
	call.Add(gtx.Ops)
	return dims
}

type logPart struct {
	text  string
	color color.NRGBA
}

// parseLogLine splits a log line into colored parts.
// Expected format: [HH:MM:SS] [LEVEL] source -> message
func parseLogLine(line string) []logPart {
	var parts []logPart

	// Distinct non-gray accent colors for structural tokens.
	dateColor := color.NRGBA{R: 200, G: 140, B: 60, A: 255}   // burnt orange
	sourceColor := color.NRGBA{R: 140, G: 200, B: 80, A: 255} // lime green
	arrowColor := color.NRGBA{R: 255, G: 255, B: 255, A: 255} // white

	arrowIdx := strings.Index(line, " -> ")
	if arrowIdx < 0 {
		// Unrecognized format: return as single gold part.
		parts = append(parts, logPart{text: line, color: dateColor})
		return parts
	}

	message := line[arrowIdx+4:]
	header := line[:arrowIdx]

	// Extract timestamp: first [HH:MM:SS]
	if idx := strings.Index(header, "] "); idx >= 0 {
		parts = append(parts, logPart{text: header[:idx+1] + " ", color: dateColor})
		header = strings.TrimSpace(header[idx+2:])
	} else {
		parts = append(parts, logPart{text: header + " ", color: dateColor})
		header = ""
	}

	// Extract level: next [LEVEL]
	levelColor := color.NRGBA{R: 200, G: 200, B: 200, A: 255}
	if idx := strings.Index(header, "] "); idx >= 0 {
		level := header[:idx+1]
		levelColor = levelColorFor(level)
		parts = append(parts, logPart{text: level + " ", color: levelColor})
		header = strings.TrimSpace(header[idx+2:])
	} else if header != "" {
		levelColor = levelColorFor(header)
		parts = append(parts, logPart{text: header + " ", color: levelColor})
		header = ""
	}

	// Remaining header is source.
	if header != "" {
		parts = append(parts, logPart{text: header + " ", color: sourceColor})
	}

	// Arrow.
	parts = append(parts, logPart{text: "-> ", color: arrowColor})

	// Message uses the level color.
	parts = append(parts, logPart{text: message, color: levelColor})

	return parts
}

func levelColorFor(level string) color.NRGBA {
	switch level {
	case "[DBG]":
		return color.NRGBA{R: 180, G: 180, B: 180, A: 255} // light gray
	case "[INF]":
		return color.NRGBA{R: 100, G: 150, B: 255, A: 255} // light blue
	case "[WRN]":
		return color.NRGBA{R: 255, G: 200, B: 100, A: 255} // light orange
	case "[ERR]":
		return color.NRGBA{R: 255, G: 100, B: 100, A: 255} // light red
	default:
		return color.NRGBA{R: 200, G: 200, B: 200, A: 255}
	}
}

func levelFromString(s string) logger.LogLevel {
	switch s {
	case "debug":
		return logger.LogLevelDebug
	case "info":
		return logger.LogLevelInfo
	case "warn":
		return logger.LogLevelWarn
	case "error":
		return logger.LogLevelError
	}
	return logger.LogLevelDebug
}

func logLineBg(line string) color.NRGBA {
	switch {
	case strings.Contains(line, "[DBG]"):
		return color.NRGBA{R: 30, G: 30, B: 30, A: 255}
	case strings.Contains(line, "[INF]"):
		return color.NRGBA{R: 20, G: 30, B: 50, A: 255}
	case strings.Contains(line, "[WRN]"):
		return color.NRGBA{R: 50, G: 40, B: 20, A: 255}
	case strings.Contains(line, "[ERR]"):
		return color.NRGBA{R: 50, G: 20, B: 20, A: 255}
	default:
		return color.NRGBA{R: 20, G: 20, B: 20, A: 255}
	}
}
