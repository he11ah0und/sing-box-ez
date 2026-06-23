//go:build !windows

// Package namedpipe stubs out the Windows named pipe transport on non-Windows
// platforms so that the package is always importable.
package namedpipe

import (
	"errors"
	"net"
)

// Transport is a no-op stub on non-Windows platforms.
type Transport struct{}

// New returns a stub transport.
func New(name string) *Transport {
	return &Transport{}
}

// DefaultName returns the default pipe name.
func DefaultName() string {
	return "sing-box-ez"
}

// Listen always returns an error on non-Windows platforms.
func (t *Transport) Listen() (net.Listener, error) {
	return nil, errors.New("named pipes are only supported on Windows")
}

// Dial always returns an error on non-Windows platforms.
func (t *Transport) Dial() (net.Conn, error) {
	return nil, errors.New("named pipes are only supported on Windows")
}

// Addr returns a placeholder address.
func (t *Transport) Addr() string {
	return "npipe:unsupported"
}
