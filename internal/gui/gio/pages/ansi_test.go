package pages

import (
	"strings"
	"testing"
)

func TestParseANSILineStripsUnknownSequences(t *testing.T) {
	parts := parseANSILine("\x1b[?25lhello\x1b[?25h")
	text := concatenateParts(parts)
	if text != "hello" {
		t.Errorf("expected 'hello', got %q", text)
	}
}

func TestParseANSILineBasicColors(t *testing.T) {
	parts := parseANSILine("\x1b[31mRED\x1b[0m normal")
	text := concatenateParts(parts)
	if text != "RED normal" {
		t.Errorf("expected 'RED normal', got %q", text)
	}
	if len(parts) < 2 {
		t.Fatalf("expected at least two parts, got %d", len(parts))
	}
	if parts[0].text != "RED" {
		t.Errorf("expected first part 'RED', got %q", parts[0].text)
	}
}

func TestParseANSILine256Color(t *testing.T) {
	parts := parseANSILine("\x1b[38;5;196mred\x1b[0m")
	text := concatenateParts(parts)
	if text != "red" {
		t.Errorf("expected 'red', got %q", text)
	}
	if len(parts) < 1 {
		t.Fatalf("expected at least one part")
	}
	if parts[0].text != "red" {
		t.Errorf("expected part 'red', got %q", parts[0].text)
	}
}

func TestParseANSILineTrueColor(t *testing.T) {
	parts := parseANSILine("\x1b[38;2;255;128;0morange\x1b[0m")
	text := concatenateParts(parts)
	if text != "orange" {
		t.Errorf("expected 'orange', got %q", text)
	}
}

func TestParseANSILineSingBoxSample(t *testing.T) {
	line := "+0800 2026-06-23 19:08:21 \x1b[36mINFO\x1b[0m [\x1b[38;5;192m2153702695\x1b[0m 113ms] dns: cached CNAME"
	parts := parseANSILine(line)
	text := concatenateParts(parts)
	if strings.Contains(text, "\x1b[") {
		t.Errorf("expected ANSI escapes stripped, got %q", text)
	}
	if !strings.Contains(text, "INFO") {
		t.Errorf("expected INFO in text, got %q", text)
	}
}

func concatenateParts(parts []logPart) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p.text)
	}
	return b.String()
}
