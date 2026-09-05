package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"syscall"
	"testing"
	"time"

	fwconfig "github.com/he11ah0und/config"
	"github.com/he11ah0und/localengine"
	"sing-box-ez/internal/config"
)

func TestDownloadErrorKind(t *testing.T) {
	if got := DownloadErrorKind(nil); got != "" {
		t.Fatalf("nil error: got %q", got)
	}

	cases := []struct {
		name string
		err  error
		want string
	}{
		{"deadline", context.DeadlineExceeded, FailureTimeout},
		{"deadline wrapped", fmt.Errorf("download: %w", context.DeadlineExceeded), FailureTimeout},
		{"os timeout", os.ErrDeadlineExceeded, FailureTimeout},
		{"conn refused", syscall.ECONNREFUSED, FailureRefused},
		{"conn refused wrapped", fmt.Errorf("update failed: %w", syscall.ECONNREFUSED), FailureRefused},
		{"conn reset", &net.OpError{Op: "read", Err: syscall.ECONNRESET}, FailureRefused},
		{"host unreachable", syscall.EHOSTUNREACH, FailureRefused},
		{"dns", &net.DNSError{Err: "no such host", IsNotFound: true}, FailureDNS},
		{"dns wrapped in url.Error", &url.Error{Op: "Get", URL: "http://x", Err: &net.DNSError{Err: "no such host", IsNotFound: true}}, FailureDNS},
		{"net timeout", &net.OpError{Op: "dial", Err: os.ErrDeadlineExceeded}, FailureTimeout},
		// Status failures arrive as plain text errors (no *url.Error in chain).
		{"http status", errors.New("unexpected status: 403"), FailureHTTP},
		{"other with url.Error", &url.Error{Op: "Get", URL: "http://x", Err: errors.New("boom")}, FailureOther},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DownloadErrorKind(tc.err); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// A real refused connection must classify as FailureRefused through the whole
// net client chain — the guards above are worthless if the backend breaks the
// %w cause chain.
func TestDownloadErrorKindRealRefusedDial(t *testing.T) {
	_, err := http.Get("http://127.0.0.1:1/unreachable")
	if err == nil {
		t.Skip("port 1 unexpectedly reachable")
	}
	if got := DownloadErrorKind(err); got != FailureRefused {
		t.Fatalf("got %q, want %q (err: %v)", got, FailureRefused, err)
	}
}

func newFailureTestIC(t *testing.T) *InteractiveController {
	t.Helper()
	// The keyed TErrorf chain only keeps %w when the logs locales resolve the
	// format string — install them like the app boot does.
	if err := localengine.LoadLogsFromOSDir("../app/locales/logs"); err != nil {
		t.Fatalf("load logs locales: %v", err)
	}
	c := newUpdateTestController(t)
	c.terminal.SetResolver(localengine.LogResolver())
	sheet := c.Config().Sheet
	sheet.Register([]string{"updates", "auto_update_configs"}, fwconfig.TypeBool, true)
	return &InteractiveController{backend: c, Controller: c}
}

func staleRemote(name, url string) config.ConfigRecord {
	return config.ConfigRecord{
		Name:                name,
		URL:                 url,
		Type:                "remote",
		UpdateIntervalHours: 2,
		LastUpdate:          config.Timestamp{Time: time.Now().Add(-3 * time.Hour)},
	}
}

// When every due config fails, the run reports the failures (the GUI turns
// that into the connectivity dialog).
func TestUpdateOutdatedConfigsAllFailed(t *testing.T) {
	ic := newFailureTestIC(t)
	ic.backend.AddConfig(staleRemote("a", "http://127.0.0.1:1/a"))
	ic.backend.AddConfig(staleRemote("b", "http://127.0.0.1:1/b"))

	configs := ic.backend.GetConfigs()
	_, failures := ic.updateOutdatedConfigs(configs, nil)
	if len(failures) != 2 {
		t.Fatalf("expected 2 failures, got %+v", failures)
	}
	for _, f := range failures {
		if f.Kind != FailureRefused {
			t.Fatalf("config %q: kind %q, want %q", f.Name, f.Kind, FailureRefused)
		}
	}
}

// A single failed config stays silent: one sample proves nothing about
// connectivity (the per-config path reports it instead).
func TestUpdateOutdatedConfigsSingleFailureSilent(t *testing.T) {
	ic := newFailureTestIC(t)
	ic.backend.AddConfig(staleRemote("a", "http://127.0.0.1:1/a"))

	_, failures := ic.updateOutdatedConfigs(ic.backend.GetConfigs(), nil)
	if len(failures) != 0 {
		t.Fatalf("expected no reported failures for a single due config, got %+v", failures)
	}
}

// A partial wipeout stays silent: one config updated, so no dialog.
func TestUpdateOutdatedConfigsPartialFailureSilent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"outbounds":[]}`))
	}))
	defer srv.Close()

	ic := newFailureTestIC(t)
	ic.backend.AddConfig(staleRemote("ok", srv.URL))
	ic.backend.AddConfig(staleRemote("dead", "http://127.0.0.1:1/dead"))

	_, failures := ic.updateOutdatedConfigs(ic.backend.GetConfigs(), nil)
	if len(failures) != 0 {
		t.Fatalf("expected no reported failures on partial success, got %+v", failures)
	}
}

// Nothing due → no failures, no dialog.
func TestUpdateOutdatedConfigsNothingDue(t *testing.T) {
	ic := newFailureTestIC(t)
	fresh := staleRemote("fresh", "http://127.0.0.1:1/a")
	fresh.LastUpdate = config.Timestamp{Time: time.Now()}
	ic.backend.AddConfig(fresh)

	_, failures := ic.updateOutdatedConfigs(ic.backend.GetConfigs(), nil)
	if len(failures) != 0 {
		t.Fatalf("expected no failures when nothing is due, got %+v", failures)
	}
}
