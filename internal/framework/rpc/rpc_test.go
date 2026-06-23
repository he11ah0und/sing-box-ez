package rpc

import (
	"context"
	"errors"
	"testing"
)

type addReq struct {
	A int `msgpack:"a"`
	B int `msgpack:"b"`
}

type addResp struct {
	Result int `msgpack:"result"`
}

func TestLocalBackendCall(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register("math", "add", func(ctx context.Context, req addReq) (addResp, error) {
		return addResp{Result: req.A + req.B}, nil
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	back := NewLocalBackend(reg)
	var resp addResp
	if err := back.Call(context.Background(), "math", "add", addReq{A: 2, B: 3}, &resp); err != nil {
		t.Fatalf("call: %v", err)
	}
	if resp.Result != 5 {
		t.Fatalf("expected 5, got %d", resp.Result)
	}
}

func TestLocalBackendUnknownMethod(t *testing.T) {
	reg := NewRegistry()
	back := NewLocalBackend(reg)
	var resp addResp
	err := back.Call(context.Background(), "math", "add", addReq{A: 1, B: 2}, &resp)
	if err == nil {
		t.Fatal("expected error for unknown method")
	}
}

func TestRegistryIntrospection(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register("math", "add", func(ctx context.Context, req addReq) (addResp, error) {
		return addResp{}, nil
	})
	_ = reg.Register("math", "sub", func(ctx context.Context, req addReq) (addResp, error) {
		return addResp{}, nil
	})

	ns := reg.Namespaces()
	if len(ns) != 1 || ns[0] != "math" {
		t.Fatalf("unexpected namespaces: %v", ns)
	}
	methods := reg.Methods("math")
	if len(methods) != 2 {
		t.Fatalf("expected 2 methods, got %v", methods)
	}

	info, ok := reg.Info("math", "add")
	if !ok {
		t.Fatal("expected info for math/add")
	}
	if info.ArgType.Name() != "addReq" {
		t.Fatalf("unexpected arg type: %s", info.ArgType.Name())
	}
	if info.ReplyType.Name() != "addResp" {
		t.Fatalf("unexpected reply type: %s", info.ReplyType.Name())
	}
}

func TestLocalBackendError(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register("math", "fail", func(ctx context.Context, req Empty) (Empty, error) {
		return Empty{}, errors.New("boom")
	})
	back := NewLocalBackend(reg)
	err := back.Call(context.Background(), "math", "fail", Empty{}, nil)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("unexpected error: %v", err)
	}
}
