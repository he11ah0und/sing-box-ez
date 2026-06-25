package singbox

import (
	"context"
	"fmt"
	"io"
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
	addr    string
	secret  string
	conn    *grpc.ClientConn
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

// CloseConnection implements api.CoreAPIClient.
func (c *Client) CloseConnection(ctx context.Context, id string) error {
	_, err := c.started.CloseConnection(c.ctx(ctx), &pb.CloseConnectionRequest{Id: id})
	return err
}

// URLTest implements api.CoreAPIClient.
// The sing-box API only supports testing a single outbound. We interpret the
// provided group tag as the outbound tag for a best-effort test.
func (c *Client) URLTest(ctx context.Context, group, testURL string, timeout time.Duration) (map[string]int, error) {
	// The sing-box API URLTest request only accepts an outbound tag and does
	// not return delays synchronously. We trigger the test and rely on the
	// groups stream to refresh latencies.
	_, err := c.started.URLTest(c.ctx(ctx), &pb.URLTestRequest{
		OutboundTag: group,
	})
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// SubscribeStatus implements api.CoreAPIClient.
func (c *Client) SubscribeStatus(ctx context.Context, interval time.Duration) (<-chan *api.StatusEvent, func(), error) {
	stream, err := c.started.SubscribeStatus(c.ctx(ctx), &pb.SubscribeStatusRequest{Interval: int64(interval / time.Millisecond)})
	if err != nil {
		return nil, nil, err
	}

	ch := make(chan *api.StatusEvent, 1)
	ctx, cancel := context.WithCancel(ctx)
	stop := func() {
		cancel()
		_ = stream.CloseSend()
	}

	go func() {
		defer close(ch)
		for {
			msg, err := stream.Recv()
			if err != nil {
				if err == io.EOF || ctx.Err() != nil {
					return
				}
				select {
				case ch <- &api.StatusEvent{Error: err}:
				case <-ctx.Done():
				}
				return
			}
			select {
			case ch <- &api.StatusEvent{Status: statusFromProto(msg)}:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, stop, nil
}

// SubscribeConnections implements api.CoreAPIClient.
func (c *Client) SubscribeConnections(ctx context.Context, interval time.Duration) (<-chan *api.ConnectionEvent, func(), error) {
	stream, err := c.started.SubscribeConnections(c.ctx(ctx), &pb.SubscribeConnectionsRequest{Interval: int64(interval / time.Millisecond)})
	if err != nil {
		return nil, nil, err
	}

	ch := make(chan *api.ConnectionEvent, 4)
	ctx, cancel := context.WithCancel(ctx)
	stop := func() {
		cancel()
		_ = stream.CloseSend()
	}

	go func() {
		defer close(ch)
		for {
			msg, err := stream.Recv()
			if err != nil {
				if err == io.EOF || ctx.Err() != nil {
					return
				}
				select {
				case ch <- &api.ConnectionEvent{Error: err}:
				case <-ctx.Done():
				}
				return
			}
			for _, ev := range msg.GetEvents() {
				out := connectionEventFromProto(ev)
				select {
				case ch <- out:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ch, stop, nil
}

func statusFromProto(s *pb.Status) api.Status {
	return api.Status{
		Memory:           s.GetMemory(),
		Goroutines:       s.GetGoroutines(),
		ConnectionsIn:    s.GetConnectionsIn(),
		ConnectionsOut:   s.GetConnectionsOut(),
		TrafficAvailable: s.GetTrafficAvailable(),
		Uplink:           s.GetUplink(),
		Downlink:         s.GetDownlink(),
		UplinkTotal:      s.GetUplinkTotal(),
		DownlinkTotal:    s.GetDownlinkTotal(),
	}
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
			if n.GetTag() == "" {
				continue
			}
			node := api.Node{
				Tag:  n.GetTag(),
				Type: n.GetType(),
			}
			if t := n.GetUrlTestTime(); t > 0 {
				node.DelayValid = true
				node.DelayAt = time.UnixMilli(t)
				node.Delay = int(n.GetUrlTestDelay())
			}
			item.Nodes = append(item.Nodes, node)
		}
		out = append(out, item)
	}
	return out
}

func connectionEventFromProto(ev *pb.ConnectionEvent) *api.ConnectionEvent {
	out := &api.ConnectionEvent{
		UplinkDelta:   ev.GetUplinkDelta(),
		DownlinkDelta: ev.GetDownlinkDelta(),
	}
	switch ev.GetType() {
	case pb.ConnectionEventType_CONNECTION_EVENT_NEW:
		out.Type = api.ConnectionEventNew
	case pb.ConnectionEventType_CONNECTION_EVENT_UPDATE:
		out.Type = api.ConnectionEventUpdate
	case pb.ConnectionEventType_CONNECTION_EVENT_CLOSED:
		out.Type = api.ConnectionEventClosed
	}
	if conn := ev.GetConnection(); conn != nil {
		out.Connection = connectionFromProto(conn)
	}
	return out
}

func connectionFromProto(c *pb.Connection) api.Connection {
	return api.Connection{
		ID:            c.GetId(),
		Inbound:       c.GetInbound(),
		InboundType:   c.GetInboundType(),
		Network:       c.GetNetwork(),
		Source:        c.GetSource(),
		Destination:   c.GetDestination(),
		Domain:        c.GetDomain(),
		Protocol:      c.GetProtocol(),
		User:          c.GetUser(),
		Outbound:      c.GetOutbound(),
		OutboundType:  c.GetOutboundType(),
		Chain:         c.GetChainList(),
		Uplink:        c.GetUplink(),
		Downlink:      c.GetDownlink(),
		UplinkTotal:   c.GetUplinkTotal(),
		DownlinkTotal: c.GetDownlinkTotal(),
		CreatedAt:     time.UnixMilli(c.GetCreatedAt()),
		ClosedAt:      time.UnixMilli(c.GetClosedAt()),
		ProcessInfo:   processInfoFromProto(c.GetProcessInfo()),
	}
}

func processInfoFromProto(p *pb.ProcessInfo) api.ProcessInfo {
	if p == nil {
		return api.ProcessInfo{}
	}
	return api.ProcessInfo{
		ProcessID:    p.GetProcessId(),
		UserID:       p.GetUserId(),
		UserName:     p.GetUserName(),
		ProcessPath:  p.GetProcessPath(),
		PackageNames: p.GetPackageNames(),
	}
}

func connectionsFromProto(e *pb.ConnectionEvents) []api.Connection {
	var out []api.Connection
	for _, ev := range e.GetEvents() {
		if conn := ev.GetConnection(); conn != nil {
			out = append(out, connectionFromProto(conn))
		}
	}
	return out
}
