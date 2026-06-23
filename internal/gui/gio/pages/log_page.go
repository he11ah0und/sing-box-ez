package pages

import (
	"image/color"
	"io"
	"strings"
	"time"
	"unicode"

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
	"sing-box-ez/internal/gui/gio/theme"
	"sing-box-ez/internal/gui/gio/widgets"
)

const (
	logTabApp  = "app"
	logTabCore = "core"
)

// LogPage renders the log viewer with separate App and Core log tabs.
type LogPage struct {
	th   *material.Theme
	ctrl core.Backend

	copyBtn  widget.Clickable
	clearBtn widget.Clickable
	tabApp   widget.Clickable
	tabCore  widget.Clickable

	list     widget.List
	selected string
	lastText string
	lines    []string
}

// NewLogPage creates a new log page.
func NewLogPage(th *material.Theme, ctrl core.Backend) *LogPage {
	p := &LogPage{
		th:       th,
		ctrl:     ctrl,
		selected: logTabApp,
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

// NoShellScroll tells the shell not to wrap this page in a scroller because it
// already scrolls its own log list.
func (p *LogPage) NoShellScroll() bool { return true }

// Layout draws the log page.
func (p *LogPage) Layout(gtx layout.Context) layout.Dimensions {
	return widgets.SpacedList(gtx, p.Children(gtx)...)
}

// Children returns the page widgets; the shell adds standard vertical spacing.
func (p *LogPage) Children(gtx layout.Context) []layout.FlexChild {
	// Schedule periodic refresh to poll for new log lines.
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(100 * time.Millisecond)})

	if p.tabApp.Clicked(gtx) && p.selected != logTabApp {
		p.selected = logTabApp
		p.lastText = ""
		p.lines = nil
	}
	if p.tabCore.Clicked(gtx) && p.selected != logTabCore {
		p.selected = logTabCore
		p.lastText = ""
		p.lines = nil
	}

	if p.copyBtn.Clicked(gtx) {
		text := strings.Join(p.currentCleanLines(), "\n")
		gtx.Execute(clipboard.WriteCmd{
			Type: "text/plain",
			Data: io.NopCloser(strings.NewReader(text)),
		})
	}
	if p.clearBtn.Clicked(gtx) {
		p.clearCurrent()
	}

	lines := p.currentLines()
	text := strings.Join(lines, "\n")
	if text != p.lastText {
		p.lastText = text
	}
	p.lines = lines

	return []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceStart, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return p.tabButton(gtx, logTabApp, localengine.T("log", "tab", "app"))
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return widgets.HSpace(gtx, unit.Dp(4))
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return p.tabButton(gtx, logTabCore, localengine.T("log", "tab", "core"))
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceStart}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return material.Button(p.th, &p.copyBtn, localengine.T("log", "btn", "copy")).Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return widgets.HSpace(gtx, unit.Dp(8))
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return material.Button(p.th, &p.clearBtn, localengine.T("log", "btn", "clear")).Layout(gtx)
							}),
						)
					}),
				)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(p.lines) == 0 {
				return p.emptyPlaceholder(gtx)
			}
			listStyle := material.List(p.th, &p.list)
			listStyle.AnchorStrategy = material.Overlay
			return listStyle.Layout(gtx, len(p.lines), func(gtx layout.Context, index int) layout.Dimensions {
				return p.logLine(gtx, p.lines[index], p.selected == logTabCore)
			})
		}),
	}
}

func (p *LogPage) currentLines() []string {
	switch p.selected {
	case logTabCore:
		return p.ctrl.GetCoreLogLines()
	default:
		return p.ctrl.GetLogLinesAtLeast(levelFromString(p.ctrl.Config().MustGet("log", "level").String()))
	}
}

func (p *LogPage) currentCleanLines() []string {
	switch p.selected {
	case logTabCore:
		return p.ctrl.GetCoreLogCleanLines()
	default:
		return p.ctrl.GetLogLines()
	}
}

func (p *LogPage) clearCurrent() {
	switch p.selected {
	case logTabCore:
		p.ctrl.ClearCoreLogs()
	default:
		p.ctrl.ClearLogs()
	}
	p.lastText = ""
	p.lines = nil
}

func (p *LogPage) tabButton(gtx layout.Context, tab, label string) layout.Dimensions {
	colors := theme.Current().Colors()
	active := p.selected == tab
	btn := &p.tabApp
	if tab == logTabCore {
		btn = &p.tabCore
	}

	bg := colors.Surface
	fg := colors.Fg
	borderColor := colors.Border
	if active {
		bg = colors.SurfaceVariant
		fg = colors.Primary
		borderColor = colors.Primary
	}
	if btn.Hovered() {
		bg = colors.Hover
	}

	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return widgets.BorderedCard(gtx, borderColor, bg, unit.Dp(1), unit.Dp(4), unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(p.th, label)
			lbl.Color = fg
			if active {
				lbl.Font.Weight = 700
			}
			return lbl.Layout(gtx)
		})
	})
}

func (p *LogPage) emptyPlaceholder(gtx layout.Context) layout.Dimensions {
	colors := theme.Current().Colors()
	lbl := material.Body2(p.th, localengine.T("log", "empty"))
	lbl.Color = colors.DisabledFg
	return layout.Center.Layout(gtx, lbl.Layout)
}

func (p *LogPage) logLine(gtx layout.Context, line string, isCore bool) layout.Dimensions {
	bg := logLineBg(line)
	var parts []logPart
	if isCore {
		parts = parseANSILine(line)
	} else {
		parts = parseLogLine(line)
	}

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
	colors := theme.Current().Colors()
	dateColor := colors.LogDate
	sourceColor := colors.LogSource
	arrowColor := colors.LogArrow

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
	levelColor := colors.LogDebug
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

// parseCoreLogLine colorizes a raw sing-box log line. ANSI escape sequences
// are stripped before storage, so this works on the cleaned text.
func parseCoreLogLine(line string) []logPart {
	colors := theme.Current().Colors()

	// Find the level token (INFO, WARN, ERROR, FATAL, DEBUG) and split around it.
	level, idx, end := detectCoreLogLevel(line)
	if level == "" {
		// No recognizable level: render the whole line with the default date color.
		return []logPart{{text: line, color: colors.LogDate}}
	}

	levelColor := coreLevelColor(level)
	var parts []logPart
	if idx > 0 {
		parts = append(parts, logPart{text: line[:idx], color: colors.LogDate})
	}
	parts = append(parts, logPart{text: line[idx:end], color: levelColor})
	if end < len(line) {
		parts = append(parts, logPart{text: line[end:], color: colors.LogDate})
	}
	return parts
}

// detectCoreLogLevel returns the first recognizable level token in line and
// its byte offsets. It looks for whole words such as INFO, WARN, ERROR, FATAL,
// DEBUG (case-insensitive).
func detectCoreLogLevel(line string) (string, int, int) {
	levels := []string{"INFO", "WARN", "ERROR", "FATAL", "DEBUG"}
	lower := strings.ToLower(line)
	for _, lvl := range levels {
		target := strings.ToLower(lvl)
		for idx := 0; idx < len(lower); {
			pos := strings.Index(lower[idx:], target)
			if pos < 0 {
				break
			}
			pos += idx
			end := pos + len(target)
			// Ensure the token is a whole word.
			before := pos == 0 || !isWordRune(rune(lower[pos-1]))
			after := end >= len(lower) || !isWordRune(rune(lower[end]))
			if before && after {
				return lvl, pos, end
			}
			idx = end
		}
	}
	return "", 0, 0
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func coreLevelColor(level string) color.NRGBA {
	colors := theme.Current().Colors()
	switch strings.ToUpper(level) {
	case "DEBUG":
		return colors.LogDebug
	case "INFO":
		return colors.LogInfo
	case "WARN", "WARNING":
		return colors.LogWarn
	case "ERROR", "ERR", "FATAL":
		return colors.LogError
	default:
		return colors.LogDebug
	}
}

func levelColorFor(level string) color.NRGBA {
	colors := theme.Current().Colors()
	switch level {
	case "[DBG]":
		return colors.LogDebug
	case "[INF]":
		return colors.LogInfo
	case "[WRN]":
		return colors.LogWarn
	case "[ERR]":
		return colors.LogError
	default:
		return colors.LogDebug
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
	colors := theme.Current().Colors()
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "[dbg]") || strings.Contains(lower, " debug "):
		return colors.LogBgDebug
	case strings.Contains(lower, "[inf]") || strings.Contains(lower, " info "):
		return colors.LogBgInfo
	case strings.Contains(lower, "[wrn]") || strings.Contains(lower, " warn"):
		return colors.LogBgWarn
	case strings.Contains(lower, "[err]") || strings.Contains(lower, " error ") || strings.Contains(lower, " fatal "):
		return colors.LogBgError
	default:
		return colors.LogBgDefault
	}
}
