package clash

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/version" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer secret" {
			t.Fatalf("unexpected auth: %s", auth)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"version": "1.13.13"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret")
	status, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Version != "1.13.13" {
		t.Fatalf("unexpected version: %s", status.Version)
	}
}

func TestClientGroups(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"proxies": map[string]any{
				"Proxy": map[string]any{"type": "Selector", "now": "Node1", "all": []string{"Node1", "Node2"}},
				"Node1": map[string]any{"type": "Shadowsocks"},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	groups, err := c.Groups(context.Background())
	if err != nil {
		t.Fatalf("Groups: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Tag != "Proxy" || groups[0].Selected != "Node1" || len(groups[0].Nodes) != 2 {
		t.Fatalf("unexpected group: %+v", groups[0])
	}
}

func TestClientSetMode(t *testing.T) {
	var received string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" || r.URL.Path != "/configs" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		received = body["mode"]
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	if err := c.SetMode(context.Background(), "direct"); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	if received != "direct" {
		t.Fatalf("unexpected mode: %s", received)
	}
}

func TestClientURLTest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/group/Proxy/delay" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]int{"Node1": 150, "Node2": 200})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	delays, err := c.URLTest(context.Background(), "Proxy", "http://example.com", 5*time.Second)
	if err != nil {
		t.Fatalf("URLTest: %v", err)
	}
	if delays["Node1"] != 150 {
		t.Fatalf("unexpected delay: %v", delays)
	}
}
