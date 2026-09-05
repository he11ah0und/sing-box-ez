package core

import (
	"os"
	"path/filepath"
	"testing"

	"sing-box-ez/internal/config"
)

func TestConfigNameAvailable(t *testing.T) {
	c := newUpdateTestController(t)
	c.cfg.Profiles.AddConfig(config.ConfigRecord{Name: "taken", Type: "remote", URL: "http://x"})

	if c.ConfigNameAvailable("taken", "") {
		t.Fatal("expected taken name to be unavailable")
	}
	if !c.ConfigNameAvailable("taken", "taken") {
		t.Fatal("the profile being edited may keep its own name")
	}
	if !c.ConfigNameAvailable("free", "") {
		t.Fatal("expected a fresh name to be available")
	}
	if c.ConfigNameAvailable("", "") {
		t.Fatal("empty name must not be available")
	}
}

func TestImportLocalConfigFile(t *testing.T) {
	c := newUpdateTestController(t)
	c.cfg.Profiles.AddConfig(config.ConfigRecord{Name: "loc", Type: "local"})

	src := filepath.Join(t.TempDir(), "source.json")
	if err := os.WriteFile(src, []byte(`{"outbounds":[]}`), 0600); err != nil {
		t.Fatal(err)
	}

	if err := c.ImportLocalConfigFile("loc", src); err != nil {
		t.Fatalf("ImportLocalConfigFile: %v", err)
	}
	data, err := c.manager.ReadConfigByName("loc")
	if err != nil {
		t.Fatalf("read imported config: %v", err)
	}
	if string(data) != `{"outbounds":[]}` {
		t.Fatalf("unexpected imported content: %s", data)
	}
	rec := c.cfg.GetConfigByName("loc")
	if rec.Hash != config.HashConfig(data) {
		t.Fatal("expected the stored hash to match the imported content")
	}
}

func TestImportLocalConfigFileRejects(t *testing.T) {
	c := newUpdateTestController(t)
	c.cfg.Profiles.AddConfig(config.ConfigRecord{Name: "loc", Type: "local"})
	c.cfg.Profiles.AddConfig(config.ConfigRecord{Name: "rem", Type: "remote", URL: "http://x"})

	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte(`not json`), 0600); err != nil {
		t.Fatal(err)
	}

	if err := c.ImportLocalConfigFile("loc", bad); err == nil {
		t.Fatal("expected an error for non-JSON content")
	}
	if err := c.ImportLocalConfigFile("rem", bad); err == nil {
		t.Fatal("expected an error for a remote profile")
	}
	if err := c.ImportLocalConfigFile("missing", bad); err == nil {
		t.Fatal("expected an error for an unknown profile")
	}
	if err := c.ImportLocalConfigFile("loc", filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Fatal("expected an error for a missing source file")
	}
	if c.HasCachedConfig("loc") {
		t.Fatal("failed imports must not create the cached config")
	}
}

func TestLocalConfigModTime(t *testing.T) {
	c := newUpdateTestController(t)
	if err := c.manager.CreateLocalConfig("loc"); err != nil {
		t.Fatal(err)
	}
	mt, err := c.LocalConfigModTime("loc")
	if err != nil {
		t.Fatalf("LocalConfigModTime: %v", err)
	}
	if mt.IsZero() {
		t.Fatal("expected a non-zero mtime")
	}
	if _, err := c.LocalConfigModTime("absent"); err == nil {
		t.Fatal("expected an error for a missing config file")
	}
}
