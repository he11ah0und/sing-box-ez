package remote

import (
	"strings"
	"testing"
)

func TestParseAddress(t *testing.T) {
	cases := []struct {
		addr      string
		wantAddr  string
		wantError bool
	}{
		{"tcp://127.0.0.1:8080", "tcp://127.0.0.1:8080", false},
		{"unix:///tmp/sing-box-ez.sock", "unix:/tmp/sing-box-ez.sock", false},
		{"auto", "", false},
		{"tcp://127.0.0.1", "", true},
		{"", "", false},
		{"unknown://host", "", true},
	}

	for _, c := range cases {
		tr, err := ParseAddress(c.addr)
		if c.wantError {
			if err == nil {
				t.Fatalf("ParseAddress(%q) expected error", c.addr)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseAddress(%q): %v", c.addr, err)
		}
		if tr == nil {
			t.Fatalf("ParseAddress(%q) returned nil transport", c.addr)
		}
		if c.wantAddr != "" && tr.Addr() != c.wantAddr {
			t.Fatalf("ParseAddress(%q) addr = %q, want %q", c.addr, tr.Addr(), c.wantAddr)
		}
	}
}

func TestParseAddressTCPHostPort(t *testing.T) {
	tr, err := ParseAddress("tcp://0.0.0.0:9000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(tr.Addr(), "tcp://") {
		t.Fatalf("expected tcp transport, got %q", tr.Addr())
	}
}
