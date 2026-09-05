// Package state tracks the runtime state of the sing-box core API: the
// connection phase, live traffic rates, the retained graph history and the
// latest groups/connections snapshot. It is UI-agnostic: frontends (Wails,
// CLI/TUI) subscribe to its callbacks and render what it pushes.
package state

import (
	"fmt"
	"time"

	"sing-box-ez/internal/core/api"
	"sing-box-ez/internal/framework/version"
)

// Core API connection phases reported to frontends in state updates.
const (
	// PhaseStopped: the core process is not running.
	PhaseStopped = "stopped"
	// PhasePreparingConfig: the active config is being checked/downloaded
	// before the core starts.
	PhasePreparingConfig = "preparing_config"
	// PhaseCheckingConfig: the config style is being detected before start.
	PhaseCheckingConfig = "checking_config"
	// PhaseStarting: a start/restart was requested, the process is spawning.
	PhaseStarting = "starting"
	// PhaseStopping: a stop was requested, the process is shutting down.
	PhaseStopping = "stopping"
	// PhaseWaiting: the process is up but the API does not answer yet.
	PhaseWaiting = "waiting_api"
	// PhaseConnected: the API answers; full snapshots are pushed.
	PhaseConnected = "connected"
)

// TrafficUpdate is pushed to frontends at most once per second.
type TrafficUpdate struct {
	Up          int64  `json:"up"`
	Down        int64  `json:"down"`
	UpTotal     int64  `json:"upTotal"`
	DownTotal   int64  `json:"downTotal"`
	UpRate      string `json:"upRate"`
	DownRate    string `json:"downRate"`
	Connected   bool   `json:"connected"`
	Backend     string `json:"backend"`
	Version     string `json:"version"`
	Connections int    `json:"connections"`
}

// TrafficPoint is a single traffic rate sample kept for the graph history.
type TrafficPoint struct {
	At   time.Time `json:"at"`
	Up   int64     `json:"up"`
	Down int64     `json:"down"`
}

// TrafficHistory is a snapshot of the retained traffic samples.
type TrafficHistory struct {
	Points []TrafficPoint `json:"points"`
}

// ConnSpan is the activity window of one member connection inside a group:
// one track per connection (DevTools/waterfall style). End is zero while
// the connection is alive.
type ConnSpan struct {
	ID    string    `json:"id"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ConnectionGroup aggregates connections to the same target (domain:port or
// destination) into one list entry. Totals include closed members; rates
// cover only the live ones.
type ConnectionGroup struct {
	Key       string    `json:"key"`
	Target    string    `json:"target"`
	Network   string    `json:"network"`
	Active    bool      `json:"active"`
	ConnCount int       `json:"connCount"`
	ConnIDs   []string  `json:"connIDs"`
	UpRate    int64     `json:"upRate"`
	DownRate  int64     `json:"downRate"`
	UpTotal   int64     `json:"upTotal"`
	DownTotal int64     `json:"downTotal"`
	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
	// LastAgo is the localized human-readable form of the elapsed time since
	// LastSeen.
	LastAgo string `json:"lastAgo"`
	// Spans are the activity windows of member connections, one per
	// connection, oldest first.
	Spans []ConnSpan `json:"spans"`
	// IPv4/IPv6 count the live members per IP version; for inactive groups
	// they hold the last known counts.
	IPv4 int `json:"ipv4"`
	IPv6 int `json:"ipv6"`
}

// Update is the full snapshot of the core API state pushed to frontends
// while the core is connected. Frontends never poll: they seed from
// Poller.State and then apply pushed updates.
type Update struct {
	Phase  string  `json:"phase"`
	Status *Status `json:"status"`
	Info   *Info   `json:"info"`
	Mode   string  `json:"mode"`
	// ModeList holds the clash modes the running core accepts for SetMode.
	// Empty for cores that do not report a mode list.
	ModeList    []string     `json:"modeList"`
	Groups      []Group      `json:"groups"`
	Connections []Connection `json:"connections"`
	// ConnGroups aggregates Connections by target; inactive groups are kept
	// until the configured retention elapses.
	ConnGroups []ConnectionGroup `json:"connGroups"`
}

// Info describes the runtime connection parameters for the active core API.
type Info struct {
	Backend string `json:"backend"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	Addr    string `json:"addr"`
}

// Status is a UI-facing snapshot of api.Status.
type Status struct {
	Version          string `json:"version"`
	Uptime           string `json:"uptime"`
	Memory           uint64 `json:"memory"`
	Goroutines       int32  `json:"goroutines"`
	ConnectionsIn    int32  `json:"connectionsIn"`
	ConnectionsOut   int32  `json:"connectionsOut"`
	TrafficAvailable bool   `json:"trafficAvailable"`
	Uplink           int64  `json:"uplink"`
	Downlink         int64  `json:"downlink"`
	UplinkTotal      int64  `json:"uplinkTotal"`
	DownlinkTotal    int64  `json:"downlinkTotal"`
	// ConnectedAt is when the core API first answered after the last failure.
	ConnectedAt time.Time `json:"connectedAt"`
	// ConnectedAgo is the localized human-readable form of the elapsed time
	// since ConnectedAt.
	ConnectedAgo string `json:"connectedAgo"`
}

// Node is a UI-facing snapshot of api.Node.
type Node struct {
	Tag        string    `json:"tag"`
	Type       string    `json:"type"`
	Delay      int       `json:"delay"`
	DelayValid bool      `json:"delayValid"`
	DelayAt    time.Time `json:"delayAt"`
}

// Group is a UI-facing snapshot of api.Group.
type Group struct {
	Tag        string `json:"tag"`
	Type       string `json:"type"`
	Selected   string `json:"selected"`
	Nodes      []Node `json:"nodes"`
	Delay      int    `json:"delay"`
	DelayValid bool   `json:"delayValid"`
}

// ProcessInfo is a UI-facing snapshot of api.ProcessInfo.
type ProcessInfo struct {
	ProcessID    uint32   `json:"processID"`
	UserID       int32    `json:"userID"`
	UserName     string   `json:"userName"`
	ProcessPath  string   `json:"processPath"`
	PackageNames []string `json:"packageNames"`
}

// Connection is a UI-facing snapshot of api.Connection.
type Connection struct {
	ID            string   `json:"id"`
	Inbound       string   `json:"inbound"`
	InboundType   string   `json:"inboundType"`
	Network       string   `json:"network"`
	Source        string   `json:"source"`
	Destination   string   `json:"destination"`
	Domain        string   `json:"domain"`
	Protocol      string   `json:"protocol"`
	User          string   `json:"user"`
	Outbound      string   `json:"outbound"`
	OutboundType  string   `json:"outboundType"`
	Chain         []string `json:"chain"`
	Uplink        int64    `json:"uplink"`
	Downlink      int64    `json:"downlink"`
	UplinkTotal   int64    `json:"uplinkTotal"`
	DownlinkTotal int64    `json:"downlinkTotal"`
	// UpRate/DownRate are the per-second rates computed by the poller for the
	// graph history sample (API-reported, or derived from the totals delta);
	// frontends read them directly instead of doing delta math.
	UpRate    int64     `json:"upRate"`
	DownRate  int64     `json:"downRate"`
	Rule      string    `json:"rule"`
	CreatedAt time.Time `json:"createdAt"`
	// CreatedAgo is the localized human-readable form of the elapsed time
	// since CreatedAt.
	CreatedAgo  string         `json:"createdAgo"`
	ClosedAt    time.Time      `json:"closedAt"`
	ProcessInfo ProcessInfo    `json:"processInfo"`
	Metadata    map[string]any `json:"metadata"`
}

func toInfo(info *api.Info) *Info {
	if info == nil {
		return nil
	}
	backend := ""
	switch info.Backend {
	case api.BackendClash:
		backend = "clash"
	case api.BackendSingBox:
		backend = "sing-box"
	default:
		backend = fmt.Sprintf("%v", info.Backend)
	}
	return &Info{
		Backend: backend,
		Host:    info.Host,
		Port:    info.Port,
		Addr:    info.Addr(),
	}
}

func toStatus(s *api.Status, connectedAt time.Time) Status {
	if s == nil {
		return Status{}
	}
	connectedAgo := ""
	if !connectedAt.IsZero() {
		connectedAgo = version.HumanDurationPlain(time.Since(connectedAt))
	}
	return Status{
		Version:          s.Version,
		Uptime:           s.Uptime.String(),
		Memory:           s.Memory,
		Goroutines:       s.Goroutines,
		ConnectionsIn:    s.ConnectionsIn,
		ConnectionsOut:   s.ConnectionsOut,
		TrafficAvailable: s.TrafficAvailable,
		Uplink:           s.Uplink,
		Downlink:         s.Downlink,
		UplinkTotal:      s.UplinkTotal,
		DownlinkTotal:    s.DownlinkTotal,
		ConnectedAt:      connectedAt,
		ConnectedAgo:     connectedAgo,
	}
}

func toNode(n api.Node) Node {
	return Node{
		Tag:        n.Tag,
		Type:       n.Type,
		Delay:      n.Delay,
		DelayValid: n.DelayValid,
		DelayAt:    n.DelayAt,
	}
}

func toGroup(g api.Group) Group {
	nodes := make([]Node, len(g.Nodes))
	for i, n := range g.Nodes {
		nodes[i] = toNode(n)
	}
	return Group{
		Tag:        g.Tag,
		Type:       g.Type,
		Selected:   g.Selected,
		Nodes:      nodes,
		Delay:      g.Delay,
		DelayValid: g.DelayValid,
	}
}

// toGroups converts a slice of api.Group to its UI-facing form.
func toGroups(groups []api.Group) []Group {
	out := make([]Group, len(groups))
	for i, g := range groups {
		out[i] = toGroup(g)
	}
	return out
}

func toProcessInfo(p api.ProcessInfo) ProcessInfo {
	return ProcessInfo{
		ProcessID:    p.ProcessID,
		UserID:       p.UserID,
		UserName:     p.UserName,
		ProcessPath:  p.ProcessPath,
		PackageNames: p.PackageNames,
	}
}

// createdAgoPlain formats the elapsed time since t as a compact plain
// duration ("5s", "3h 5m"), or an empty string when t is zero.
func createdAgoPlain(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return version.HumanDurationPlain(time.Since(t))
}

func toConnection(c api.Connection, rate TrafficPoint) Connection {
	chain := make([]string, len(c.Chain))
	copy(chain, c.Chain)
	metadata := make(map[string]any, len(c.Metadata))
	for k, v := range c.Metadata {
		metadata[k] = v
	}
	return Connection{
		ID:            c.ID,
		Inbound:       c.Inbound,
		InboundType:   c.InboundType,
		Network:       c.Network,
		Source:        c.Source,
		Destination:   c.Destination,
		Domain:        c.Domain,
		Protocol:      c.Protocol,
		User:          c.User,
		Outbound:      c.Outbound,
		OutboundType:  c.OutboundType,
		Chain:         chain,
		Uplink:        c.Uplink,
		Downlink:      c.Downlink,
		UplinkTotal:   c.UplinkTotal,
		DownlinkTotal: c.DownlinkTotal,
		UpRate:        rate.Up,
		DownRate:      rate.Down,
		Rule:          c.Rule,
		CreatedAt:     c.CreatedAt,
		CreatedAgo:    createdAgoPlain(c.CreatedAt),
		ClosedAt:      c.ClosedAt,
		ProcessInfo:   toProcessInfo(c.ProcessInfo),
		Metadata:      metadata,
	}
}

// toConnections converts a slice of api.Connection to its UI-facing form.
// rates carries the per-second rate samples computed by the poller for the
// same snapshot, keyed by connection ID.
func toConnections(conns []api.Connection, rates map[string]TrafficPoint) []Connection {
	out := make([]Connection, len(conns))
	for i, c := range conns {
		out[i] = toConnection(c, rates[c.ID])
	}
	return out
}
