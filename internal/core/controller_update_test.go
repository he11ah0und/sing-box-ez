package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	fwconfig "github.com/he11ah0und/config"
	"github.com/he11ah0und/logger"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework"
	"sing-box-ez/internal/framework/fs"
)

func newUpdateTestController(t *testing.T) *Controller {
	t.Helper()
	baseDir := t.TempDir()
	sheet := fwconfig.NewSheet(fwconfig.SheetOptions{})
	sheet.Register([]string{"updates", "auto_update_on_hash_mismatch"}, fwconfig.TypeBool, false)
	cfg := &config.AppConfig{
		Sheet:    sheet,
		Root:     fs.NewOS(baseDir).Root(),
		DataDir:  baseDir,
		Profiles: &config.Profiles{Configs: []config.ConfigRecord{}},
	}
	log := logger.NewLogger(100)
	return &Controller{
		cfg:      cfg,
		fwApp:    &framework.App{FS: fs.NewOS(baseDir), Logger: log},
		manager:  NewManager(baseDir, fs.NewOS(baseDir), nil, log),
		terminal: log.Root,
	}
}

// A background update that finds the profile already refreshed (e.g. by the
// start flow's PrepareConfig) must skip the download: the second download
// used to land right after the core came up and trigger an auto-restart,
// visible as a connected -> waiting_api -> connected flicker.
func TestUpdateConfigIfDueSkipsRefreshedProfile(t *testing.T) {
	c := newUpdateTestController(t)
	rec := config.ConfigRecord{
		Name:                "sub",
		URL:                 "http://127.0.0.1:1/unreachable",
		Type:                "remote",
		UpdateIntervalHours: 2,
		LastUpdate:          config.Timestamp{Time: time.Now()},
	}
	c.cfg.Profiles.AddConfig(rec)

	updated, err := c.UpdateConfigIfDue("sub", true, false)
	if err != nil {
		t.Fatalf("UpdateConfigIfDue: %v", err)
	}
	if updated {
		t.Fatal("expected updated=false for a freshly updated profile")
	}
}

func TestUpdateConfigIfDueDownloadsStaleProfile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"outbounds":[]}`))
	}))
	defer srv.Close()

	c := newUpdateTestController(t)
	rec := config.ConfigRecord{
		Name:                "sub",
		URL:                 srv.URL,
		Type:                "remote",
		UpdateIntervalHours: 2,
		LastUpdate:          config.Timestamp{Time: time.Now().Add(-3 * time.Hour)},
	}
	c.cfg.Profiles.AddConfig(rec)

	updated, err := c.UpdateConfigIfDue("sub", true, false)
	if err != nil {
		t.Fatalf("UpdateConfigIfDue: %v", err)
	}
	if !updated {
		t.Fatal("expected updated=true for a stale profile")
	}
	fresh := c.cfg.GetConfigByName("sub")
	if fresh == nil || time.Since(fresh.LastUpdate.Time) > time.Minute {
		t.Fatalf("expected last_update to be refreshed, got %+v", fresh)
	}
	if !c.HasCachedConfig("sub") {
		t.Fatal("expected the downloaded config to be cached")
	}
}

func TestUpdateConfigIfDueIgnoresLocalAndUnknown(t *testing.T) {
	c := newUpdateTestController(t)
	c.cfg.Profiles.AddConfig(config.ConfigRecord{Name: "local", Type: "local"})

	for _, name := range []string{"local", "missing"} {
		updated, err := c.UpdateConfigIfDue(name, true, true)
		if err != nil {
			t.Fatalf("UpdateConfigIfDue(%q): %v", name, err)
		}
		if updated {
			t.Fatalf("expected updated=false for %q", name)
		}
	}
}
