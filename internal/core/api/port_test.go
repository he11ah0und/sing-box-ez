package api

import (
	"net"
	"strconv"
	"testing"
)

func TestFindFreePort(t *testing.T) {
	port, err := FindFreePort("127.0.0.1")
	if err != nil {
		t.Fatalf("FindFreePort: %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Fatalf("invalid port: %d", port)
	}

	// The returned port should actually be bindable.
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("port %d is not free: %v", port, err)
	}
	ln.Close()
}

func TestGenerateSecret(t *testing.T) {
	s1 := GenerateSecret()
	s2 := GenerateSecret()
	if len(s1) == 0 || len(s2) == 0 {
		t.Fatal("empty secret")
	}
	if s1 == s2 {
		t.Fatal("secrets should be random")
	}
}
