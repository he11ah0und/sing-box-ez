// Package tcp implements an IPC transport backed by a TCP socket.
// It is intended for remote GUI connections and development scenarios
// where Unix sockets or named pipes are not suitable.
package tcp

import (
	"net"
)

// Transport uses a TCP socket for IPC.
type Transport struct {
	addr string
}

// New creates a TCP transport for the given address.
// The address must be in "host:port" format.
func New(addr string) *Transport {
	return &Transport{addr: addr}
}

// Listen implements ipc.Transport.
func (t *Transport) Listen() (net.Listener, error) {
	return net.Listen("tcp", t.addr)
}

// Dial implements ipc.Transport.
func (t *Transport) Dial() (net.Conn, error) {
	return net.Dial("tcp", t.addr)
}

// Addr implements ipc.Transport.
func (t *Transport) Addr() string {
	return "tcp://" + t.addr
}
