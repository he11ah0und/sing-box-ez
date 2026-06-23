//go:build windows

// Package namedpipe implements an IPC transport backed by a Windows named pipe.
package namedpipe

import (
	"fmt"
	"net"

	"gopkg.in/natefinch/npipe.v2"
)

// Transport uses a Windows named pipe for IPC.
type Transport struct {
	name string
}

// New creates a named pipe transport with the given pipe name (without the
// \\.\pipe\ prefix).
func New(name string) *Transport {
	return &Transport{name: name}
}

// DefaultName returns the default pipe name for the application.
func DefaultName() string {
	return "sing-box-ez"
}

func (t *Transport) pipePath() string {
	return fmt.Sprintf(`\\.\pipe\%s`, t.name)
}

// Listen implements ipc.Transport.
func (t *Transport) Listen() (net.Listener, error) {
	return npipe.Listen(t.pipePath())
}

// Dial implements ipc.Transport.
func (t *Transport) Dial() (net.Conn, error) {
	return npipe.Dial(t.pipePath())
}

// Addr implements ipc.Transport.
func (t *Transport) Addr() string {
	return t.pipePath()
}
