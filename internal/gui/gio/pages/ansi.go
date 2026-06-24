package pages

import (
	"image/color"
	"strconv"
	"strings"

	"sing-box-ez/internal/gui/gio/theme"
)

// parseANSILine converts a string containing ANSI SGR escape sequences into
// colored text spans. Sequences that are not SGR codes (ending with 'm') are
// stripped without producing spans.
func parseANSILine(line string) []logPart {
	defaultColor := ansiDefaultColor()

	var parts []logPart
	state := ansiState{fg: defaultColor}
	start := 0

	for i := 0; i < len(line); {
		if line[i] != '\x1b' {
			i++
			continue
		}

		// Append text before the escape sequence.
		if start < i {
			parts = append(parts, logPart{text: line[start:i], color: state.fg})
		}

		// Only CSI sequences (ESC [) are parsed; other escape sequences are skipped.
		if i+1 >= len(line) || line[i+1] != '[' {
			i++
			start = i
			continue
		}

		// Find the final byte of the CSI sequence.
		seqEnd := i + 2
		for seqEnd < len(line) && !isCSIFinalByte(line[seqEnd]) {
			seqEnd++
		}
		if seqEnd >= len(line) {
			break
		}
		final := line[seqEnd]
		seq := line[i+2 : seqEnd]
		if final == 'm' {
			state.applySGR(seq)
		}
		// Non-SGR CSI sequences are ignored.

		seqEnd++
		i = seqEnd
		start = i
	}

	if start < len(line) {
		parts = append(parts, logPart{text: line[start:], color: state.fg})
	}

	// Avoid returning empty parts slice for an empty/fully-stripped line.
	if len(parts) == 0 {
		parts = append(parts, logPart{text: "", color: defaultColor})
	}
	return parts
}

type ansiState struct {
	fg   color.NRGBA
	bold bool
}

// applySGR parses a CSI sequence body (without the leading ESC and trailing 'm')
// and updates the current style. The body may start with '['.
func (s *ansiState) applySGR(seq string) {
	seq = strings.TrimPrefix(seq, "[")
	if seq == "" {
		*s = ansiState{fg: ansiDefaultColor()}
		return
	}

	params := strings.Split(seq, ";")
	for i := 0; i < len(params); i++ {
		code, _ := strconv.Atoi(params[i])
		switch {
		case code == 0:
			*s = ansiState{fg: ansiDefaultColor()}
		case code == 1:
			s.bold = true
		case code == 22:
			s.bold = false
		case code >= 30 && code <= 37:
			s.fg = ansiBasicColor(code-30, s.bold)
		case code == 39:
			s.fg = ansiDefaultColor()
		case code >= 90 && code <= 97:
			s.fg = ansiBasicColor(code-90, true)
		case code == 38:
			if c, ok := parseExtendedColor(params, &i); ok {
				s.fg = c
			}
		}
	}
}

func parseExtendedColor(params []string, i *int) (color.NRGBA, bool) {
	if *i+2 >= len(params) {
		return color.NRGBA{}, false
	}
	mode, _ := strconv.Atoi(params[*i+1])
	switch mode {
	case 5:
		if *i+2 >= len(params) {
			return color.NRGBA{}, false
		}
		if c, ok := ansi256Color(params[*i+2]); ok {
			*i += 2
			return c, true
		}
	case 2:
		if *i+4 >= len(params) {
			return color.NRGBA{}, false
		}
		r, _ := strconv.Atoi(params[*i+2])
		g, _ := strconv.Atoi(params[*i+3])
		b, _ := strconv.Atoi(params[*i+4])
		*i += 4
		return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}, true
	}
	return color.NRGBA{}, false
}

func ansiBasicColor(index int, bright bool) color.NRGBA {
	palette := []color.NRGBA{
		{R: 0, G: 0, B: 0, A: 255},
		{R: 205, G: 49, B: 49, A: 255},
		{R: 13, G: 188, B: 121, A: 255},
		{R: 229, G: 229, B: 16, A: 255},
		{R: 36, G: 114, B: 200, A: 255},
		{R: 188, G: 63, B: 188, A: 255},
		{R: 17, G: 168, B: 205, A: 255},
		{R: 229, G: 229, B: 229, A: 255},
	}
	c := palette[index%len(palette)]
	if bright {
		c = brighten(c, 1.4)
	}
	return c
}

func ansi256Color(s string) (color.NRGBA, bool) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return color.NRGBA{}, false
	}
	return ansi256ToColor(n), true
}

func ansi256ToColor(n int) color.NRGBA {
	switch {
	case n < 16:
		if n < 8 {
			return ansiBasicColor(n, false)
		}
		return ansiBasicColor(n-8, true)
	case n < 232:
		// 6x6x6 color cube.
		n -= 16
		r := n / 36
		g := (n % 36) / 6
		b := n % 6
		return color.NRGBA{
			R: uint8(55 + r*40),
			G: uint8(55 + g*40),
			B: uint8(55 + b*40),
			A: 255,
		}
	default:
		// Grayscale ramp.
		gray := uint8(8 + (n-232)*10)
		return color.NRGBA{R: gray, G: gray, B: gray, A: 255}
	}
}

// isCSIFinalByte returns true for bytes that terminate a CSI sequence.
// Final bytes are in the range 0x40-0x7E (@ to ~).
func isCSIFinalByte(b byte) bool {
	return b >= 0x40 && b <= 0x7E
}

func ansiDefaultColor() color.NRGBA {
	th := theme.Current()
	if th.Material != nil {
		return th.Colors().Fg
	}
	return color.NRGBA{R: 200, G: 200, B: 200, A: 255}
}

func brighten(c color.NRGBA, factor float64) color.NRGBA {
	clamp := func(v float64) uint8 {
		if v > 255 {
			return 255
		}
		if v < 0 {
			return 0
		}
		return uint8(v)
	}
	return color.NRGBA{
		R: clamp(float64(c.R) * factor),
		G: clamp(float64(c.G) * factor),
		B: clamp(float64(c.B) * factor),
		A: c.A,
	}
}
