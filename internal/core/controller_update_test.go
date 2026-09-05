package core

import (
	"context"
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

func newStaleRemoteController(t *testing.T, url string) *Controller {
	t.Helper()
	c := newUpdateTestController(t)
	c.cfg.Profiles.AddConfig(config.ConfigRecord{
		Name:                "sub",
		URL:                 url,
		Type:                "remote",
		UpdateIntervalHours: 2,
		LastUpdate:          config.Timestamp{Time: time.Now().Add(-3 * time.Hour)},
	})
	if err := c.manager.CreateLocalConfig("sub"); err != nil {
		t.Fatal(err)
	}
	// PrepareConfig normally wires these before refreshActiveConfig runs.
	c.manager.SetConfigURL(url)
	c.manager.SetConfigName("sub")
	return c
}

// A genuine download failure with a cached fallback must trigger the
// download-failed hook (the GUI offers a retry through the core's proxy).
// This regressed when downloadCancel() ran before the skip check, making
// downloadCtx.Err() always non-nil and the hook unreachable.
func TestRefreshActiveConfigReportsDownloadFailure(t *testing.T) {
	c := newStaleRemoteController(t, "http://127.0.0.1:1/sub")

	var gotName string
	var gotErr error
	c.OnConfigDownloadFailed = func(name string, err error) { gotName, gotErr = name, err }

	if err := c.refreshActiveConfig(context.Background()); err != nil {
		t.Fatalf("refreshActiveConfig: %v", err)
	}
	if gotName != "sub" || gotErr == nil {
		t.Fatalf("expected the download-failed hook for sub, got name=%q err=%v", gotName, gotErr)
	}
}

// A user skip (the "skip" button of the config-update phase) falls back to
// the cached config silently — it is not a failure and must not trigger the
// download-failed hook.
func TestRefreshActiveConfigSkipDoesNotReport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // hang until the client goes away
	}))
	defer srv.Close()

	c := newStaleRemoteController(t, srv.URL)

	hookCalled := false
	c.OnConfigDownloadFailed = func(string, error) { hookCalled = true }

	go func() {
		time.Sleep(100 * time.Millisecond)
		c.SkipConfigDownload()
	}()
	if err := c.refreshActiveConfig(context.Background()); err != nil {
		t.Fatalf("refreshActiveConfig: %v", err)
	}
	if hookCalled {
		t.Fatal("a user skip must not trigger the download-failed hook")
	}
}
