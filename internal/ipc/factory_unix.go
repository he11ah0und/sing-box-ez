//go:build !windows

package ipc

import (
	"sing-box-ez/internal/ipc/unixsocket"
)

func defaultTransport() (Transport, error) {
	return unixsocket.New(unixsocket.DefaultPath()), nil
}
