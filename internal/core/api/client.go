package api

import (
	"context"
	"time"
)

// Status summarizes the running core instance.
type Status struct {
	Version          string
	Uptime           time.Duration
	Memory           uint64
	Goroutines       int32
	ConnectionsIn    int32
	ConnectionsOut   int32
	TrafficAvailable bool
	Uplink           int64
	Downlink         int64
	UplinkTotal      int64
	DownlinkTotal    int64
}

// StatusEvent is delivered by SubscribeStatus.
type StatusEvent struct {
	Status Status
	Error  error
}

// Node describes one outbound inside a Group.
type Node struct {
	Tag        string
	Type       string
	Delay      int
	DelayValid bool
	DelayAt    time.Time
}

// Group describes an outbound group (Selector, URLTest, etc.).
type Group struct {
	Tag        string
	Type       string
	Selected   string
	Nodes      []Node
	Delay      int
	DelayValid bool
}

// ProcessInfo describes the process owning a connection.
type ProcessInfo struct {
	ProcessID    uint32
	UserID       int32
	UserName     string
	ProcessPath  string
	PackageNames []string
}

// Connection describes an active (or recently closed) connection.
type Connection struct {
	ID            string
	Inbound       string
	InboundType   string
	Network       string
	Source        string
	Destination   string
	Domain        string
	Protocol      string
	User          string
	Outbound      string
	OutboundType  string
	Chain         []string
	Uplink        int64
	Downlink      int64
	UplinkTotal   int64
	DownlinkTotal int64
	CreatedAt     time.Time
	ClosedAt      time.Time
	ProcessInfo   ProcessInfo

	// Metadata is populated by REST-style backends that do not expose typed
	// fields; it is also kept for backwards compatibility.
	Metadata map[string]any
}

// ConnectionEventType describes what happened to a connection stream event.
type ConnectionEventType int

const (
	ConnectionEventNew ConnectionEventType = iota
	ConnectionEventUpdate
	ConnectionEventClosed
)

// ConnectionEvent is delivered by SubscribeConnections.
type ConnectionEvent struct {
	Type          ConnectionEventType
	Connection    Connection
	UplinkDelta   int64
	DownlinkDelta int64
	Error         error
}

// CoreAPIClient is the common interface exposed by both Clash API and
// sing-box API backends. Callers should not depend on transport details.
type CoreAPIClient interface {
	Status(ctx context.Context) (*Status, error)
	Groups(ctx context.Context) ([]Group, error)
	SelectGroup(ctx context.Context, group, outbound string) error
	Mode(ctx context.Context) (string, error)
	SetMode(ctx context.Context, mode string) error
	Connections(ctx context.Context) ([]Connection, error)
	CloseConnection(ctx context.Context, id string) error
	CloseConnections(ctx context.Context) error
	URLTest(ctx context.Context, group, url string, timeout time.Duration) (map[string]int, error)

	// SubscribeStatus delivers a stream of status events (memory, goroutines,
	// connections, traffic). The returned stop function must be called to release
	// resources. The channel is closed after the stream ends or an error occurs.
	SubscribeStatus(ctx context.Context, interval time.Duration) (<-chan *StatusEvent, func(), error)

	// SubscribeConnections delivers a stream of connection events (new, update,
	// closed). The returned stop function must be called to release resources.
	SubscribeConnections(ctx context.Context, interval time.Duration) (<-chan *ConnectionEvent, func(), error)
}
