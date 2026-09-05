package core

import (
	"os"
	"path/filepath"
	"testing"

	fwconfig "github.com/he11ah0und/config"
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

func TestDeleteConfigRemovesCacheWhenEnabled(t *testing.T) {
	c := newUpdateTestController(t)
	c.cfg.Sheet.Register([]string{"configs", "delete_cache_on_remove"}, fwconfig.TypeBool, true)
	c.cfg.Profiles.AddConfig(config.ConfigRecord{Name: "loc", Type: "local"})
	if err := c.manager.CreateLocalConfig("loc"); err != nil {
		t.Fatal(err)
	}

	if err := c.DeleteConfig("loc"); err != nil {
		t.Fatalf("DeleteConfig: %v", err)
	}
	if c.HasCachedConfig("loc") {
		t.Fatal("expected the cached config file to be removed")
	}
}

func TestDeleteConfigKeepsCacheWhenDisabled(t *testing.T) {
	c := newUpdateTestController(t)
	c.cfg.Sheet.Register([]string{"configs", "delete_cache_on_remove"}, fwconfig.TypeBool, false)
	c.cfg.Profiles.AddConfig(config.ConfigRecord{Name: "loc", Type: "local"})
	if err := c.manager.CreateLocalConfig("loc"); err != nil {
		t.Fatal(err)
	}

	if err := c.DeleteConfig("loc"); err != nil {
		t.Fatalf("DeleteConfig: %v", err)
	}
	if !c.HasCachedConfig("loc") {
		t.Fatal("expected the cached config file to survive")
	}
	if got := c.OrphanedConfigs(); len(got) != 1 || got[0].Name != "loc" {
		t.Fatalf("expected the leftover to be reported as orphan, got %v", got)
	}
	if err := c.DeleteCachedConfig("loc"); err != nil {
		t.Fatalf("DeleteCachedConfig: %v", err)
	}
	if len(c.OrphanedConfigs()) != 0 {
		t.Fatal("expected no orphans after cleanup")
	}
}

func TestOrphanedConfigsIgnoresProfilesAndForeignFiles(t *testing.T) {
	c := newUpdateTestController(t)
	c.cfg.Profiles.AddConfig(config.ConfigRecord{Name: "active", Type: "local"})
	if err := c.manager.CreateLocalConfig("active"); err != nil {
		t.Fatal(err)
	}
	if err := c.manager.CreateLocalConfig("leftover"); err != nil {
		t.Fatal(err)
	}
	// A non-JSON file in the configs dir is not a config cache leftover.
	if err := c.fwApp.FS.Root().File(filepath.Join(c.manager.baseDir, "configs", "notes.txt")).Write([]byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	got := c.OrphanedConfigs()
	if len(got) != 1 || got[0].Name != "leftover" {
		t.Fatalf("expected only 'leftover', got %v", got)
	if got[0].ModTime == "" {
		t.Fatal("expected the orphan mtime to be reported")
	}
	}
}
