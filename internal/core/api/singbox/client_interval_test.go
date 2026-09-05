package singbox

import (
	"context"
	"net"
	"testing"
	"time"

	pb "sing-box-ez/internal/core/api/singbox/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type intervalRecorder struct {
	pb.UnimplementedStartedServiceServer
	got chan int64
}

func (m *intervalRecorder) SubscribeStatus(req *pb.SubscribeStatusRequest, stream grpc.ServerStreamingServer[pb.Status]) error {
	m.got <- req.GetInterval()
	<-stream.Context().Done()
	return nil
}

func (m *intervalRecorder) SubscribeConnections(req *pb.SubscribeConnectionsRequest, stream grpc.ServerStreamingServer[pb.ConnectionEvents]) error {
	m.got <- req.GetInterval()
	<-stream.Context().Done()
	return nil
}

// The sing-box daemon interprets the interval field as a time.Duration
// (nanoseconds): sending milliseconds instead makes the server tick every
// microsecond and flood the client.
func TestSubscribeIntervalIsNanoseconds(t *testing.T) {
	mock := &intervalRecorder{got: make(chan int64, 2)}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	pb.RegisterStartedServiceServer(srv, mock)
	go srv.Serve(ln)
	defer srv.Stop()

	c, err := NewClient(ln.Addr().String(), "")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	_, stopStatus, err := c.SubscribeStatus(ctx, time.Second)
	if err != nil {
		t.Fatalf("SubscribeStatus: %v", err)
	}
	defer stopStatus()
	_, stopConns, err := c.SubscribeConnections(ctx, 2*time.Second)
	if err != nil {
		t.Fatalf("SubscribeConnections: %v", err)
	}
	defer stopConns()

	if got := <-mock.got; got != int64(time.Second) {
		t.Fatalf("SubscribeStatus interval: got %d, want %d (ns)", got, int64(time.Second))
	}
	if got := <-mock.got; got != int64(2*time.Second) {
		t.Fatalf("SubscribeConnections interval: got %d, want %d (ns)", got, int64(2*time.Second))
	}
}
