package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/logger"
)

func TestManagerLifecycle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a shell script; skipping on windows")
	}

	baseDir := t.TempDir()
	configsDir := filepath.Join(baseDir, "configs")
	if err := os.MkdirAll(configsDir, 0750); err != nil {
		t.Fatalf("mkdir configs: %v", err)
	}

	// Create a fake core binary that sleeps long enough for the test.
	corePath := filepath.Join(baseDir, "sing-box")
	script := "#!/bin/sh\nexec sleep 60\n"
	if err := os.WriteFile(corePath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake core: %v", err)
	}

	cfgPath := filepath.Join(configsDir, "test.json")
	if err := os.WriteFile(cfgPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("write fake config: %v", err)
	}

	log := logger.NewLogger(100)
	m := NewManager(baseDir, fs.NewOS(baseDir), nil, log)
	m.SetConfigName("test")

	if m.IsRunning() {
		t.Fatal("expected manager to be stopped before Start")
	}

	if err := m.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	if !m.IsRunning() {
		t.Fatal("expected manager to be running after Start")
	}
	if m.GetPID() == 0 {
		t.Fatal("expected non-zero PID")
	}

	if err := m.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if m.IsRunning() {
		t.Fatal("expected manager to be stopped after Stop")
	}

	// A second Start after Stop should succeed.
	if err := m.Start(); err != nil {
		t.Fatalf("second start failed: %v", err)
	}
	if !m.IsRunning() {
		t.Fatal("expected manager to be running after second Start")
	}
	_ = m.Stop()
}

func TestManagerCreateLocalConfig(t *testing.T) {
	baseDir := t.TempDir()
	log := logger.NewLogger(100)
	m := NewManager(baseDir, fs.NewOS(baseDir), nil, log)

	if err := m.CreateLocalConfig("local"); err != nil {
		t.Fatalf("CreateLocalConfig failed: %v", err)
	}
	path := filepath.Join(baseDir, "configs", "local.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read local config: %v", err)
	}
	if string(data) != "{}" {
		t.Fatalf("unexpected local config content: %s", string(data))
	}
}

func TestManagerRenameConfigFile(t *testing.T) {
	baseDir := t.TempDir()
	log := logger.NewLogger(100)
	m := NewManager(baseDir, fs.NewOS(baseDir), nil, log)

	if err := m.CreateLocalConfig("old"); err != nil {
		t.Fatalf("CreateLocalConfig failed: %v", err)
	}
	if err := m.RenameConfigFile("old", "new"); err != nil {
		t.Fatalf("RenameConfigFile failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "configs", "old.json")); !os.IsNotExist(err) {
		t.Fatalf("expected old config file to be removed")
	}
	if _, err := os.Stat(filepath.Join(baseDir, "configs", "new.json")); err != nil {
		t.Fatalf("expected new config file to exist: %v", err)
	}
}

func TestManagerRestartWaitsForProcessExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a shell script; skipping on windows")
	}

	baseDir := t.TempDir()
	configsDir := filepath.Join(baseDir, "configs")
	if err := os.MkdirAll(configsDir, 0750); err != nil {
		t.Fatalf("mkdir configs: %v", err)
	}

	corePath := filepath.Join(baseDir, "sing-box")
	script := "#!/bin/sh\nexec sleep 60\n"
	if err := os.WriteFile(corePath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake core: %v", err)
	}

	cfgPath := filepath.Join(configsDir, "test.json")
	if err := os.WriteFile(cfgPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("write fake config: %v", err)
	}

	log := logger.NewLogger(100)
	m := NewManager(baseDir, fs.NewOS(baseDir), nil, log)
	m.SetConfigName("test")

	if err := m.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	pid1 := m.GetPID()
	if pid1 == 0 {
		t.Fatal("expected non-zero PID")
	}

	if err := m.Restart(); err != nil {
		t.Fatalf("restart failed: %v", err)
	}
	if !m.IsRunning() {
		t.Fatal("expected manager to be running after Restart")
	}
	pid2 := m.GetPID()
	if pid2 == 0 {
		t.Fatal("expected non-zero PID after restart")
	}
	if pid1 == pid2 {
		t.Fatalf("expected new PID after restart, got same PID %d", pid1)
	}

	_ = m.Stop()
}
