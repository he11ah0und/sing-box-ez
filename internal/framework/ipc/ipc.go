// Package ipc provides a cross-platform transport abstraction for communication
// between the sing-box-ez GUI and daemon.
package ipc

import "net"

// Transport abstracts the underlying inter-process communication mechanism.
// Implementations must be safe for concurrent use and must not assume a
// specific platform unless documented.
type Transport interface {
	// Listen returns a listener for the daemon side.
	Listen() (net.Listener, error)

	// Dial connects a client (usually the GUI) to the daemon.
	Dial() (net.Conn, error)

	// Addr returns a human-readable address for logging and debugging.
	Addr() string
}

// Default returns the best available transport for the current platform.
func Default() (Transport, error) {
	return defaultTransport()
}
