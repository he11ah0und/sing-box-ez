package core

import (
	"strings"
	"testing"
)

func TestCoreLogBufferLimit(t *testing.T) {
	b := NewCoreLogBuffer(3)
	b.Add("one")
	b.Add("two")
	b.Add("three")
	b.Add("four")

	lines := b.GetLines()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "two" || lines[2] != "four" {
		t.Fatalf("unexpected lines: %v", lines)
	}
}

func TestCoreLogBufferClears(t *testing.T) {
	b := NewCoreLogBuffer(100)
	b.Add("line")
	if len(b.GetLines()) != 1 {
		t.Fatal("expected one line")
	}
	b.Clear()
	if len(b.GetLines()) != 0 {
		t.Fatal("expected empty buffer")
	}
}

func TestCoreLogBufferKeepsRawANSI(t *testing.T) {
	b := NewCoreLogBuffer(100)
	b.Add("\x1b[36mINFO\x1b[0m [\x1b[38;5;192m123\x1b[0m] message")
	lines := b.GetLines()
	if len(lines) != 1 {
		t.Fatal("expected one line")
	}
	if !strings.Contains(lines[0], "\x1b[") {
		t.Errorf("expected raw ANSI escapes preserved, got %q", lines[0])
	}

	clean := b.GetCleanLines()
	if len(clean) != 1 {
		t.Fatal("expected one clean line")
	}
	if strings.Contains(clean[0], "\x1b[") {
		t.Errorf("expected clean line to have no ANSI escapes, got %q", clean[0])
	}
	expected := "INFO [123] message"
	if clean[0] != expected {
		t.Errorf("expected clean line %q, got %q", expected, clean[0])
	}
}
