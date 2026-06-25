package singbox

import (
	"context"
	"fmt"
	"time"

	"sing-box-ez/internal/core/api"
	pb "sing-box-ez/internal/core/api/singbox/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Client implements api.CoreAPIClient over the native sing-box gRPC API.
type Client struct {
	addr   string
	secret string
	conn   *grpc.ClientConn
	started pb.StartedServiceClient
}

// NewClient creates a sing-box API client connected to addr (host:port).
func NewClient(addr, secret string) (*Client, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial sing-box API: %w", err)
	}
	return &Client{
		addr:    addr,
		secret:  secret,
		conn:    conn,
		started: pb.NewStartedServiceClient(conn),
	}, nil
}

// Close closes the underlying gRPC connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) ctx(ctx context.Context) context.Context {
	if c.secret == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+c.secret)
}

// Status implements api.CoreAPIClient.
func (c *Client) Status(ctx context.Context) (*api.Status, error) {
	resp, err := c.started.GetVersion(c.ctx(ctx), &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	return &api.Status{Version: resp.Version}, nil
}

// Groups implements api.CoreAPIClient.
func (c *Client) Groups(ctx context.Context) ([]api.Group, error) {
	stream, err := c.started.SubscribeGroups(c.ctx(ctx), &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	defer stream.CloseSend()

	msg, err := stream.Recv()
	if err != nil {
		return nil, err
	}
	return groupsFromProto(msg), nil
}

// SelectGroup implements api.CoreAPIClient.
func (c *Client) SelectGroup(ctx context.Context, group, outbound string) error {
	_, err := c.started.SelectOutbound(c.ctx(ctx), &pb.SelectOutboundRequest{
		GroupTag:    group,
		OutboundTag: outbound,
	})
	return err
}

// Mode implements api.CoreAPIClient.
func (c *Client) Mode(ctx context.Context) (string, error) {
	resp, err := c.started.GetClashModeStatus(c.ctx(ctx), &emptypb.Empty{})
	if err != nil {
		return "", err
	}
	return resp.CurrentMode, nil
}

// SetMode implements api.CoreAPIClient.
func (c *Client) SetMode(ctx context.Context, mode string) error {
	_, err := c.started.SetClashMode(c.ctx(ctx), &pb.ClashMode{Mode: mode})
	return err
}

// Connections implements api.CoreAPIClient.
func (c *Client) Connections(ctx context.Context) ([]api.Connection, error) {
	stream, err := c.started.SubscribeConnections(c.ctx(ctx), &pb.SubscribeConnectionsRequest{})
	if err != nil {
		return nil, err
	}
	defer stream.CloseSend()

	msg, err := stream.Recv()
	if err != nil {
		return nil, err
	}
	return connectionsFromProto(msg), nil
}

// CloseConnections implements api.CoreAPIClient.
func (c *Client) CloseConnections(ctx context.Context) error {
	_, err := c.started.CloseAllConnections(c.ctx(ctx), &emptypb.Empty{})
	return err
}

// URLTest implements api.CoreAPIClient.
// The sing-box API only supports testing a single outbound. We interpret the
// provided group tag as the outbound tag for a best-effort test.
func (c *Client) URLTest(ctx context.Context, group, testURL string, timeout time.Duration) (map[string]int, error) {
	// The sing-box API URLTest request only accepts an outbound tag and does
	// not return delays synchronously. For now we trigger the test and leave
	// result observation to the caller.
	_, err := c.started.URLTest(c.ctx(ctx), &pb.URLTestRequest{
		OutboundTag: group,
	})
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func groupsFromProto(g *pb.Groups) []api.Group {
	var out []api.Group
	for _, group := range g.GetGroup() {
		item := api.Group{
			Tag:      group.GetTag(),
			Type:     group.GetType(),
			Selected: group.GetSelected(),
		}
		for _, n := range group.GetItems() {
			if n.GetTag() != "" {
				item.Nodes = append(item.Nodes, n.GetTag())
			}
		}
		out = append(out, item)
	}
	return out
}

func connectionsFromProto(e *pb.ConnectionEvents) []api.Connection {
	var out []api.Connection
	for _, ev := range e.Events {
		if conn := ev.GetConnection(); conn != nil {
			out = append(out, api.Connection{
				ID: conn.Id,
				Metadata: map[string]any{
					"inbound":      conn.Inbound,
					"inbound_type": conn.InboundType,
					"network":      conn.Network,
					"source":       conn.Source,
					"destination":  conn.Destination,
					"domain":       conn.Domain,
					"protocol":     conn.Protocol,
					"user":         conn.User,
					"from":         conn.FromOutbound,
					"outbound":     conn.Outbound,
					"outbound_type": conn.OutboundType,
				},
			})
		}
	}
	return out
}
