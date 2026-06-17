package core

import (
	"strings"
	"testing"
	"time"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework/logger"
)

func TestIsCoreFatalError(t *testing.T) {
	fatalCases := []string{
		"FATAL[0000] something went wrong",
		"panic: runtime error: index out of range",
	}
	nonFatalCases := []string{
		"INFO[0000] fetch rule-set geoip-cn: fetching...",
		"initial rule-set: geoip-cn",
		"save rule-set geoip-cn",
		"ERROR[0000] some error",
		"regular log line",
	}

	for _, c := range fatalCases {
		if !isCoreFatalError(c) {
			t.Errorf("expected fatal for %q", c)
		}
	}
	for _, c := range nonFatalCases {
		if isCoreFatalError(c) {
			t.Errorf("expected non-fatal for %q", c)
		}
	}
}

func TestCoreLogWriterLineSplitting(t *testing.T) {
	w := NewCoreLogWriter()

	n, err := w.Write([]byte("line1\nline2\npartial"))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if n != 19 {
		t.Fatalf("expected write length 19, got %d", n)
	}

	if got := <-w.Ch; got != "line1" {
		t.Errorf("expected line1, got %q", got)
	}
	if got := <-w.Ch; got != "line2" {
		t.Errorf("expected line2, got %q", got)
	}

	// Complete the partial line.
	if _, err := w.Write([]byte(" rest\n")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if got := <-w.Ch; got != "partial rest" {
		t.Errorf("expected 'partial rest', got %q", got)
	}
}

func TestCoreLogWriterClose(t *testing.T) {
	w := NewCoreLogWriter()
	if _, err := w.Write([]byte("before close\n")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if got := <-w.Ch; got != "before close" {
		t.Errorf("expected 'before close', got %q", got)
	}

	w.Close()
	if _, err := w.Write([]byte("after close\n")); err == nil {
		t.Error("expected error writing to closed writer")
	}

	if _, ok := <-w.Ch; ok {
		t.Error("expected channel to be closed")
	}
}

func TestCoreLogProcessorDrainsOnStop(t *testing.T) {
	cfg := &config.AppConfig{
		WatchCoreLogs:   true,
		CoreAutoRestart: false,
	}

	log := logger.NewLogger(100)
	writer := NewCoreLogWriter()

	// manager is nil because auto-restart is disabled in this test.
	p := NewCoreLogProcessor(cfg, nil, writer, log.Root)
	p.Start()

	if _, err := writer.Write([]byte("hello world\n")); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	p.Stop()

	// Give the asynchronous log append a moment to land.
	time.Sleep(50 * time.Millisecond)

	lines := log.GetLines()
	found := false
	for _, line := range lines {
		if strings.Contains(line, "hello world") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected drained log line to be mirrored, got: %v", lines)
	}
}
