package singbox

import (
	"context"
	"net"
	"testing"

	pb "sing-box-ez/internal/core/api/singbox/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type mockStartedService struct {
	pb.UnimplementedStartedServiceServer
	secret string
}

func (m *mockStartedService) GetVersion(ctx context.Context, _ *emptypb.Empty) (*pb.Version, error) {
	return &pb.Version{Version: "1.14.0-alpha.34"}, nil
}

func TestClientStatus(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	pb.RegisterStartedServiceServer(srv, &mockStartedService{})
	go srv.Serve(ln)
	defer srv.Stop()

	c, err := NewClient(ln.Addr().String(), "")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Close()

	status, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Version != "1.14.0-alpha.34" {
		t.Fatalf("unexpected version: %s", status.Version)
	}
}
