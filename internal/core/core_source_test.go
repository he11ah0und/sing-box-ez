package core

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	fwconfig "github.com/he11ah0und/config"
	"sing-box-ez/internal/config"
)

// fakeCore writes an executable shell script mimicking `sing-box version`.
func fakeCore(t *testing.T, dir, versionLine string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake core is unix-only")
	}
	path := filepath.Join(dir, "sing-box")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho '"+versionLine+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheckCoreCompatibility(t *testing.T) {
	dir := t.TempDir()

	ok := fakeCore(t, dir, "sing-box version 1.14.0 environment: go1.25")
	if _, err := CheckCoreCompatibility(ok); err != nil {
		t.Fatalf("1.14.0 should be compatible: %v", err)
	}

	edge := fakeCore(t, filepath.Join(dir, "edge"), "sing-box version "+MinCoreVersion)
	if _, err := CheckCoreCompatibility(edge); err != nil {
		t.Fatalf("MinCoreVersion itself should be compatible: %v", err)
	}

	old := fakeCore(t, filepath.Join(dir, "old"), "sing-box version 1.11.4")
	if _, err := CheckCoreCompatibility(old); !errors.Is(err, ErrCoreIncompatible) {
		t.Fatalf("1.11.4 should be incompatible, got %v", err)
	}

	garbage := fakeCore(t, filepath.Join(dir, "garbage"), "not a sing-box")
	if _, err := CheckCoreCompatibility(garbage); err == nil {
		t.Fatal("unparseable version output should fail")
	}

	if _, err := CheckCoreCompatibility(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing binary should fail")
	}
}

func newCoreSourceTestConfig(t *testing.T, mode, customPath string) *config.AppConfig {
	t.Helper()
	sheet := fwconfig.NewSheet(fwconfig.SheetOptions{})
	sheet.Register([]string{"core", "source", "mode"}, fwconfig.TypeString, mode)
	sheet.Register([]string{"core", "source", "custom_path"}, fwconfig.TypeString, customPath)
	return &config.AppConfig{Sheet: sheet, DataDir: t.TempDir()}
}

func TestResolveCoreBinary(t *testing.T) {
	base := t.TempDir()
	managed := managedCoreBinary(base)

	if got := ResolveCoreBinary(newCoreSourceTestConfig(t, CoreSourceOfficial, ""), base); got != managed {
		t.Fatalf("official: got %q, want %q", got, managed)
	}
	if got := ResolveCoreBinary(newCoreSourceTestConfig(t, "", ""), base); got != managed {
		t.Fatalf("unset mode: got %q, want %q", got, managed)
	}
	custom := filepath.Join(t.TempDir(), "my-sing-box")
	if got := ResolveCoreBinary(newCoreSourceTestConfig(t, CoreSourceCustom, custom), base); got != custom {
		t.Fatalf("custom: got %q, want %q", got, custom)
	}
	// A custom mode without a path falls back to the managed binary.
	if got := ResolveCoreBinary(newCoreSourceTestConfig(t, CoreSourceCustom, "  "), base); got != managed {
		t.Fatalf("custom without path: got %q, want %q", got, managed)
	}
	// System mode without a findable binary falls back to the managed binary.
	cfg := newCoreSourceTestConfig(t, CoreSourceSystem, "")
	if FindSystemCore() == "" {
		if got := ResolveCoreBinary(cfg, base); got != managed {
			t.Fatalf("system without binary: got %q, want %q", got, managed)
		}
	} else {
		t.Skip("a system sing-box exists; fallback case not testable here")
	}
}
