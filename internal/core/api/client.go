package api

import (
	"context"
	"time"
)

// Status summarizes the running core instance.
type Status struct {
	Version string
	Uptime  time.Duration
}

// Group describes an outbound group (Selector, URLTest, etc.).
type Group struct {
	Tag      string
	Type     string
	Selected string
	Nodes    []string
}

// Connection describes an active connection.
type Connection struct {
	ID       string
	Metadata map[string]any
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
	CloseConnections(ctx context.Context) error
	URLTest(ctx context.Context, group, url string, timeout time.Duration) (map[string]int, error)
}
