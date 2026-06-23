package remote

import (
	"fmt"
	"net/url"
	"runtime"
	"strings"

	"sing-box-ez/internal/ipc"
	"sing-box-ez/internal/ipc/namedpipe"
	"sing-box-ez/internal/ipc/tcp"
	"sing-box-ez/internal/ipc/unixsocket"
)

// ParseAddress converts a remote address string into an IPC transport.
// Supported forms:
//   - "auto"                      -> platform default (unix socket / named pipe)
//   - "unix:///path/to/socket"    -> Unix domain socket
//   - "tcp://host:port"           -> TCP socket
//   - "npipe://name"              -> Windows named pipe
//   - "\\.\pipe\name"             -> Windows named pipe (raw form)
func ParseAddress(addr string) (ipc.Transport, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" || addr == "auto" {
		return ipc.Default()
	}

	if runtime.GOOS == "windows" && strings.HasPrefix(addr, `\\.\pipe\`) {
		return namedpipe.New(strings.TrimPrefix(addr, `\\.\pipe\`)), nil
	}

	u, err := url.Parse(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid remote address %q: %w", addr, err)
	}

	switch u.Scheme {
	case "unix":
		path := u.Path
		if path == "" {
			path = u.Host
		}
		if path == "" {
			return nil, fmt.Errorf("unix socket path required")
		}
		return unixsocket.New(path), nil
	case "tcp":
		host := u.Hostname()
		port := u.Port()
		if port == "" {
			return nil, fmt.Errorf("tcp port required")
		}
		return tcp.New(fmt.Sprintf("%s:%s", host, port)), nil
	case "npipe":
		name := strings.TrimPrefix(u.Path, "/")
		if name == "" {
			name = u.Host
		}
		if name == "" {
			return nil, fmt.Errorf("named pipe name required")
		}
		return namedpipe.New(name), nil
	default:
		return nil, fmt.Errorf("unsupported remote scheme %q", u.Scheme)
	}
}
