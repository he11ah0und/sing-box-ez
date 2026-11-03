package updater

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/he11ah0und/logger"
)

const atomFeedFixture = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>tag:github.com,2008:Repository/1/v0.1.0-testing.abc1234</id>
    <updated>2026-11-03T07:35:47Z</updated>
    <link rel="alternate" type="text/html" href="https://github.com/o/r/releases/tag/v0.1.0-testing.abc1234"/>
    <title>v0.1.0-testing</title>
    <content type="html">notes-testing</content>
  </entry>
  <entry>
    <id>tag:github.com,2008:Repository/1/v0.0.9</id>
    <updated>2026-10-01T00:00:00Z</updated>
    <link rel="alternate" type="text/html" href="https://github.com/o/r/releases/tag/v0.0.9"/>
    <title>v0.0.9</title>
    <content type="html">notes-stable</content>
  </entry>
</feed>`

const testingAssetsFixture = `
<a href="/o/r/releases/download/v0.1.0-testing.abc1234/sing-box-ez-amd64-linux-gcc-gui">gui</a>
<a href="/o/r/releases/download/v0.1.0-testing.abc1234/sing-box-ez-amd64-windows-gcc-gui.exe">win</a>
<a href="/o/r/releases/download/v0.1.0-testing.abc1234/sing-box-ez-amd64-linux-gcc-gui">dup</a>`

const stableAssetsFixture = `
<a href="/o/r/releases/download/v0.0.9/sing-box-ez-amd64-linux-gcc-gui">gui</a>`

func newAtomTestBackend(t *testing.T) (*GitHubBackend, *httptest.Server) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/o/r/releases.atom", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, atomFeedFixture)
	})
	mux.HandleFunc("/o/r/releases/expanded_assets/", func(w http.ResponseWriter, r *http.Request) {
		tag := strings.TrimPrefix(r.URL.Path, "/o/r/releases/expanded_assets/")
		if tag == "v0.1.0-testing.abc1234" {
			_, _ = io.WriteString(w, testingAssetsFixture)
			return
		}
		_, _ = io.WriteString(w, stableAssetsFixture)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	b := NewGitHubBackend(logger.NewLogger(0).Root, "o", "r")
	b.BaseURL = srv.URL
	return b, srv
}

func TestLatestFromAtomStable(t *testing.T) {
	b, srv := newAtomTestBackend(t)
	r, err := b.LatestRelease(context.Background(), "")
	if err != nil {
		t.Fatalf("LatestRelease: %v", err)
	}
	if r.Version != "v0.0.9" {
		t.Fatalf("want the newest non-prerelease v0.0.9, got %q", r.Version)
	}
	if r.Prerelease {
		t.Fatal("stable release must not be marked as prerelease")
	}
	if r.Body != "notes-stable" {
		t.Fatalf("body = %q", r.Body)
	}
	if len(r.Assets) != 1 {
		t.Fatalf("want 1 asset, got %d", len(r.Assets))
	}
	wantURL := srv.URL + "/o/r/releases/download/v0.0.9/sing-box-ez-amd64-linux-gcc-gui"
	if r.Assets[0].URL != wantURL {
		t.Fatalf("asset URL = %q, want %q", r.Assets[0].URL, wantURL)
	}
	if strings.Contains(r.Assets[0].URL, "api.github.com") {
		t.Fatal("asset URL must not point at the REST API host")
	}
}

func TestLatestFromAtomTestingChannel(t *testing.T) {
	b, _ := newAtomTestBackend(t)
	r, err := b.LatestRelease(context.Background(), "testing")
	if err != nil {
		t.Fatalf("LatestRelease: %v", err)
	}
	if r.Version != "v0.1.0-testing.abc1234" {
		t.Fatalf("version = %q", r.Version)
	}
	if !r.Prerelease {
		t.Fatal("testing build must be marked as prerelease")
	}
	if r.Channel != "testing" {
		t.Fatalf("channel = %q", r.Channel)
	}
	if len(r.Assets) != 2 {
		t.Fatalf("assets must be deduplicated to 2, got %d", len(r.Assets))
	}
}

func TestReleaseByVersionAtom(t *testing.T) {
	b, _ := newAtomTestBackend(t)
	r, err := b.ReleaseByVersion(context.Background(), "v0.0.9")
	if err != nil {
		t.Fatalf("ReleaseByVersion: %v", err)
	}
	if r.Version != "v0.0.9" {
		t.Fatalf("version = %q", r.Version)
	}
	if _, err := b.releaseByVersionAtom(context.Background(), "v0.0.1"); err == nil {
		t.Fatal("an unknown tag must fail so the REST API fallback can take over")
	}
}

func TestAtomPrerelease(t *testing.T) {
	cases := map[string]bool{
		"v1.0.0":                  false,
		"v1.0.0-testing.abc1234":  true,
		"v1.0.0-beta.1":           true,
		"v1.0.0-rc.2":             true,
		"v3.2.0-nightly.20261102": true,
	}
	for tag, want := range cases {
		if got := atomPrerelease(tag); got != want {
			t.Errorf("atomPrerelease(%q) = %v, want %v", tag, got, want)
		}
	}
}
