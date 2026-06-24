package rpc

import (
	"context"
	"fmt"

	"sing-box-ez/internal/framework/ipc"
)

// RemoteBackend sends RPC calls over an IPC transport.
type RemoteBackend struct {
	transport ipc.Transport
	codec     Codec
}

// NewRemoteBackend creates a remote backend for the given IPC transport.
func NewRemoteBackend(transport ipc.Transport) *RemoteBackend {
	return &RemoteBackend{transport: transport, codec: MsgpackCodec{}}
}

// Call implements Backend.
func (b *RemoteBackend) Call(ctx context.Context, namespace, method string, args, reply any) error {
	var payload []byte
	if args != nil {
		var err error
		payload, err = b.codec.Marshal(args)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
	}

	conn, err := b.transport.Dial()
	if err != nil {
		return fmt.Errorf("dial %s: %w", b.transport.Addr(), err)
	}
	defer conn.Close()

	req := requestEnvelope{Namespace: namespace, Method: method, Payload: payload}
	reqBytes, err := b.codec.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	if err := writeFrame(conn, reqBytes); err != nil {
		return fmt.Errorf("write request: %w", err)
	}

	respBytes, err := readFrame(conn)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var resp responseEnvelope
	if err := b.codec.Unmarshal(respBytes, &resp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("remote: %s", resp.Error)
	}

	if reply != nil && len(resp.Payload) > 0 {
		if err := b.codec.Unmarshal(resp.Payload, reply); err != nil {
			return fmt.Errorf("decode reply: %w", err)
		}
	}
	return nil
}

// Namespaces is not available without a connection; returns nil.
func (b *RemoteBackend) Namespaces() []string { return nil }

// Methods is not available without a connection; returns nil.
func (b *RemoteBackend) Methods(namespace string) []string { return nil }
