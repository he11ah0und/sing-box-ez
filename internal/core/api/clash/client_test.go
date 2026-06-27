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

func TestClientConnections(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connections" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"connections": []any{
				map[string]any{
					"id":       "d5949562-40bd-43ff-b1a0-a54ba4f88584",
					"chains":   []string{"direct"},
					"upload":   918,
					"download": 3290,
					"start":    "2026-06-27T06:16:31.661995348+08:00",
					"rule":     "network=udp port_range=1024:65535 => route(direct)",
					"metadata": map[string]any{
						"network":         "udp",
						"type":            "tun/0",
						"sourceIP":        "172.19.0.1",
						"sourcePort":      "34239",
						"destinationIP":   "104.29.132.88",
						"destinationPort": "19320",
					},
				},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	conns, err := c.Connections(context.Background())
	if err != nil {
		t.Fatalf("Connections: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}
	conn := conns[0]
	if conn.ID != "d5949562-40bd-43ff-b1a0-a54ba4f88584" {
		t.Fatalf("unexpected id: %s", conn.ID)
	}
	if len(conn.Chain) != 1 || conn.Chain[0] != "direct" {
		t.Fatalf("unexpected chain: %v", conn.Chain)
	}
	if conn.Outbound != "direct" {
		t.Fatalf("unexpected outbound: %s", conn.Outbound)
	}
	if conn.UplinkTotal != 918 {
		t.Fatalf("unexpected uplink total: %d", conn.UplinkTotal)
	}
	if conn.DownlinkTotal != 3290 {
		t.Fatalf("unexpected downlink total: %d", conn.DownlinkTotal)
	}
	if conn.Rule == "" {
		t.Fatalf("expected rule to be populated")
	}
	if conn.Network != "udp" {
		t.Fatalf("unexpected network: %s", conn.Network)
	}
	if conn.Source != "172.19.0.1:34239" {
		t.Fatalf("unexpected source: %s", conn.Source)
	}
	if conn.Destination != "104.29.132.88:19320" {
		t.Fatalf("unexpected destination: %s", conn.Destination)
	}
	if conn.CreatedAt.IsZero() {
		t.Fatalf("expected created at to be parsed")
	}
}

func TestClientConnectionsChainReversal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"connections": []any{
				map[string]any{
					"id":     "b906a600-8d8d-49f9-943a-f1c3038be1fe",
					"chains": []string{"1.vless-ws.direct", "auto", "proxy"},
					"rule":   "final",
				},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	conns, err := c.Connections(context.Background())
	if err != nil {
		t.Fatalf("Connections: %v", err)
	}
	conn := conns[0]
	want := []string{"proxy", "auto", "1.vless-ws.direct"}
	for i, v := range want {
		if conn.Chain[i] != v {
			t.Fatalf("chain[%d] = %s, want %s; chain=%v", i, conn.Chain[i], v, conn.Chain)
		}
	}
	if conn.Outbound != "1.vless-ws.direct" {
		t.Fatalf("unexpected outbound: %s", conn.Outbound)
	}
}
