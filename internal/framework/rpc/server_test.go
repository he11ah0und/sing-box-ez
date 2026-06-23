package rpc

import (
	"context"
	"testing"
	"time"

	"sing-box-ez/internal/framework/ipc/tcp"
)

type echoReq struct {
	Message string `msgpack:"message"`
}

type echoResp struct {
	Message string `msgpack:"message"`
}

func TestServerClientRoundTrip(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register("test", "echo", func(ctx context.Context, req echoReq) (echoResp, error) {
		return echoResp(req), nil
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	transport := tcp.New(":0")
	server := NewServer(reg, transport)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := server.Run(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("server error: %v", err)
		}
	}()

	// Wait for the listener to be ready.
	for i := 0; i < 50 && server.Addr() == transport.Addr(); i++ {
		time.Sleep(10 * time.Millisecond)
	}

	clientTransport := tcp.New(server.Addr())
	client := NewRemoteBackend(clientTransport)

	var resp echoResp
	if err := client.Call(context.Background(), "test", "echo", echoReq{Message: "hello"}, &resp); err != nil {
		t.Fatalf("call: %v", err)
	}
	if resp.Message != "hello" {
		t.Fatalf("expected hello, got %s", resp.Message)
	}

	cancel()
}
