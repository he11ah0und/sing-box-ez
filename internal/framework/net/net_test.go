package net

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/he11ah0und/logger"
)

// uaServer starts a test server that records the User-Agent of the first
// request and returns the recorder.
func uaServer(t *testing.T) (*httptest.Server, *string) {
	t.Helper()
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("User-Agent")
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func TestUserAgentApplied(t *testing.T) {
	SetDefaultUserAgent("test-agent/1.0/linux")
	t.Cleanup(func() { SetDefaultUserAgent("") })

	srv, seen := uaServer(t)
	c := NewClient(logger.NewLogger(10).Root)
	r, _, err := c.GetReader(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	_ = r.Close()
	if *seen != "test-agent/1.0/linux" {
		t.Errorf("User-Agent: want %q, got %q", "test-agent/1.0/linux", *seen)
	}
}

func TestUserAgentNotOverridden(t *testing.T) {
	SetDefaultUserAgent("test-agent/1.0/linux")
	t.Cleanup(func() { SetDefaultUserAgent("") })

	srv, seen := uaServer(t)
	c := NewClient(logger.NewLogger(10).Root)
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("User-Agent", "custom-agent")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	_ = resp.Body.Close()
	if *seen != "custom-agent" {
		t.Errorf("User-Agent: want explicit %q preserved, got %q", "custom-agent", *seen)
	}
}

func TestNoDefaultUserAgent(t *testing.T) {
	SetDefaultUserAgent("")

	srv, seen := uaServer(t)
	c := NewClient(logger.NewLogger(10).Root)
	r, _, err := c.GetReader(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	_ = r.Close()
	// Without a configured default the net/http default applies.
	if *seen == "" || *seen == "test-agent/1.0/linux" {
		t.Errorf("User-Agent: want net/http default, got %q", *seen)
	}
}
