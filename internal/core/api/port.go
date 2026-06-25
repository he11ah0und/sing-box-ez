package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
)

// FindFreePort asks the OS for a free TCP port on the given host and returns
// it. The temporary listener is closed before returning, so there is a small
// race window where another process could bind the port before the caller.
func FindFreePort(host string) (int, error) {
	addr := net.JoinHostPort(host, "0")
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return 0, fmt.Errorf("find free port on %s: %w", addr, err)
	}
	defer ln.Close()

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected address type %T", ln.Addr())
	}
	return tcpAddr.Port, nil
}

// GenerateSecret returns a random hex string suitable for use as an API secret.
func GenerateSecret() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to a deterministic value only in the highly unlikely case
		// that the system's CSPRNG fails.
		return "sing-box-ez-default-secret"
	}
	return hex.EncodeToString(b)
}
