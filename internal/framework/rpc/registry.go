package rpc

import (
	"context"
	"fmt"
	"reflect"
	"sort"
)

// Empty is a placeholder request/response with no fields.
type Empty struct{}

// HandlerFunc is the shape of a registered RPC method:
//
//	func(ctx context.Context, req Req) (Resp, error)
//
// Req and Resp must be encodable by the configured Codec. Resp may be Empty
// when the method returns no data.
type HandlerFunc = any

// MethodInfo describes a registered RPC method.
type MethodInfo struct {
	Namespace string
	Name      string
	ArgType   reflect.Type
	ReplyType reflect.Type
}

// handler wraps a registered function together with its argument/result types.
type handler struct {
	fn        reflect.Value
	argType   reflect.Type
	replyType reflect.Type
	codec     Codec
}

// Registry stores RPC method handlers keyed by namespace and name.
type Registry struct {
	codec   Codec
	methods map[string]map[string]*handler
	argType reflect.Type
}

// NewRegistry creates an empty registry using msgpack encoding.
func NewRegistry() *Registry {
	return NewRegistryWithCodec(MsgpackCodec{})
}

// NewRegistryWithCodec creates an empty registry with a custom codec.
func NewRegistryWithCodec(codec Codec) *Registry {
	return &Registry{
		codec:   codec,
		methods: make(map[string]map[string]*handler),
		argType: reflect.TypeOf((*context.Context)(nil)).Elem(),
	}
}

// Register adds a method handler. fn must have the signature:
//
//	func(ctx context.Context, req Req) (Resp, error)
func (r *Registry) Register(namespace, method string, fn HandlerFunc) error {
	if namespace == "" || method == "" {
		return fmt.Errorf("namespace and method are required")
	}
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		return fmt.Errorf("handler for %s/%s is not a function", namespace, method)
	}
	t := v.Type()
	if t.NumIn() != 2 || t.In(0) != r.argType {
		return fmt.Errorf("handler %s/%s must have signature func(context.Context, Req) (Resp, error)", namespace, method)
	}
	if t.NumOut() != 2 || t.Out(1) != reflect.TypeOf((*error)(nil)).Elem() {
		return fmt.Errorf("handler %s/%s must return (Resp, error)", namespace, method)
	}

	h := &handler{
		fn:        v,
		argType:   t.In(1),
		replyType: t.Out(0),
		codec:     r.codec,
	}

	if r.methods[namespace] == nil {
		r.methods[namespace] = make(map[string]*handler)
	}
	if _, ok := r.methods[namespace][method]; ok {
		return fmt.Errorf("method %s/%s already registered", namespace, method)
	}
	r.methods[namespace][method] = h
	return nil
}

// Namespaces returns all registered namespaces sorted alphabetically.
func (r *Registry) Namespaces() []string {
	out := make([]string, 0, len(r.methods))
	for ns := range r.methods {
		out = append(out, ns)
	}
	sort.Strings(out)
	return out
}

// Methods returns all method names registered under namespace, sorted.
func (r *Registry) Methods(namespace string) []string {
	m := r.methods[namespace]
	if m == nil {
		return nil
	}
	out := make([]string, 0, len(m))
	for name := range m {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Info returns metadata for a registered method.
func (r *Registry) Info(namespace, method string) (MethodInfo, bool) {
	h, ok := r.lookup(namespace, method)
	if !ok {
		return MethodInfo{}, false
	}
	return MethodInfo{
		Namespace: namespace,
		Name:      method,
		ArgType:   h.argType,
		ReplyType: h.replyType,
	}, true
}

func (r *Registry) lookup(namespace, method string) (*handler, bool) {
	m, ok := r.methods[namespace]
	if !ok {
		return nil, false
	}
	h, ok := m[method]
	return h, ok
}

// invoke decodes the payload, calls the handler, and encodes the reply.
func (h *handler) invoke(ctx context.Context, payload []byte) ([]byte, error) {
	arg := reflect.New(h.argType)
	if len(payload) > 0 {
		if err := h.codec.Unmarshal(payload, arg.Interface()); err != nil {
			return nil, fmt.Errorf("decode request: %w", err)
		}
	}

	reply := reflect.New(h.replyType)
	results := h.fn.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.Indirect(arg),
	})

	if err, _ := results[1].Interface().(error); err != nil {
		return nil, err
	}
	reply.Elem().Set(results[0])
	return h.codec.Marshal(reply.Interface())
}
