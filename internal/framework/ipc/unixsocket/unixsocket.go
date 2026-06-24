// Package unixsocket implements an IPC transport backed by a Unix domain socket.
// It is intended for Linux, macOS, and other Unix-like systems.
package unixsocket

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// Transport uses a Unix domain socket for IPC.
type Transport struct {
	path string
}

// New creates a Unix socket transport at the given path.
func New(path string) *Transport {
	return &Transport{path: path}
}

// DefaultPath returns a reasonable default socket path for the application.
func DefaultPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "sing-box-ez", "daemon.sock")
}

// Listen implements ipc.Transport.
func (t *Transport) Listen() (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(t.path), 0o755); err != nil {
		return nil, fmt.Errorf("create socket directory: %w", err)
	}
	_ = os.Remove(t.path)
	return net.Listen("unix", t.path)
}

// Dial implements ipc.Transport.
func (t *Transport) Dial() (net.Conn, error) {
	return net.Dial("unix", t.path)
}

// Addr implements ipc.Transport.
func (t *Transport) Addr() string {
	return "unix:" + t.path
}
