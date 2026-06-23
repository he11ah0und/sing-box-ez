//go:build windows

package ipc

import (
	"sing-box-ez/internal/framework/ipc/namedpipe"
)

func defaultTransport() (Transport, error) {
	return namedpipe.New(namedpipe.DefaultName()), nil
}
