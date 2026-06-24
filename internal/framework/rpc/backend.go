package rpc

import (
	"context"
	"fmt"
)

// ProgressFunc reports download progress for RPC methods that transfer files.
type ProgressFunc func(downloaded, total int64)

// Backend is the RPC abstraction implemented by both local and remote backends.
// A backend exposes methods in namespaces; callers use Call to invoke a method
// and receive its reply.
type Backend interface {
	// Call invokes the named method. args must be a value of the method's
	// request type (or nil for Empty requests). reply must be a pointer to the
	// method's response type.
	Call(ctx context.Context, namespace, method string, args, reply any) error

	// Namespaces returns all available namespaces.
	Namespaces() []string

	// Methods returns all method names inside a namespace.
	Methods(namespace string) []string
}

// LocalBackend dispatches RPC calls to an in-process registry.
type LocalBackend struct {
	registry *Registry
}

// NewLocalBackend creates a backend backed by the given registry.
func NewLocalBackend(registry *Registry) *LocalBackend {
	return &LocalBackend{registry: registry}
}

// Call implements Backend.
func (b *LocalBackend) Call(ctx context.Context, namespace, method string, args, reply any) error {
	h, ok := b.registry.lookup(namespace, method)
	if !ok {
		return fmt.Errorf("unknown method %s/%s", namespace, method)
	}

	var payload []byte
	if args != nil {
		var err error
		payload, err = h.codec.Marshal(args)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
	}

	replyBytes, err := h.invoke(ctx, payload)
	if err != nil {
		return err
	}

	if reply != nil && len(replyBytes) > 0 {
		if err := h.codec.Unmarshal(replyBytes, reply); err != nil {
			return fmt.Errorf("decode reply: %w", err)
		}
	}
	return nil
}

// Namespaces implements Backend.
func (b *LocalBackend) Namespaces() []string {
	return b.registry.Namespaces()
}

// Methods implements Backend.
func (b *LocalBackend) Methods(namespace string) []string {
	return b.registry.Methods(namespace)
}
