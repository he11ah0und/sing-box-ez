//go:build !nogui

package wails

import (
	"context"
	"fmt"
	"time"

	"sing-box-ez/internal/core/api"
)

// APIInfo describes the runtime connection parameters for the active core API.
type APIInfo struct {
	Backend string `json:"backend"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	Addr    string `json:"addr"`
}

// APIStatus is a UI-facing snapshot of api.Status.
type APIStatus struct {
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
}

// APINode is a UI-facing snapshot of api.Node.
type APINode struct {
	Tag        string    `json:"tag"`
	Type       string    `json:"type"`
	Delay      int       `json:"delay"`
	DelayValid bool      `json:"delayValid"`
	DelayAt    time.Time `json:"delayAt"`
}

// APIGroup is a UI-facing snapshot of api.Group.
type APIGroup struct {
	Tag        string    `json:"tag"`
	Type       string    `json:"type"`
	Selected   string    `json:"selected"`
	Nodes      []APINode `json:"nodes"`
	Delay      int       `json:"delay"`
	DelayValid bool      `json:"delayValid"`
}

// APIProcessInfo is a UI-facing snapshot of api.ProcessInfo.
type APIProcessInfo struct {
	ProcessID    uint32   `json:"processID"`
	UserID       int32    `json:"userID"`
	UserName     string   `json:"userName"`
	ProcessPath  string   `json:"processPath"`
	PackageNames []string `json:"packageNames"`
}

// APIConnection is a UI-facing snapshot of api.Connection.
type APIConnection struct {
	ID            string         `json:"id"`
	Inbound       string         `json:"inbound"`
	InboundType   string         `json:"inboundType"`
	Network       string         `json:"network"`
	Source        string         `json:"source"`
	Destination   string         `json:"destination"`
	Domain        string         `json:"domain"`
	Protocol      string         `json:"protocol"`
	User          string         `json:"user"`
	Outbound      string         `json:"outbound"`
	OutboundType  string         `json:"outboundType"`
	Chain         []string       `json:"chain"`
	Uplink        int64          `json:"uplink"`
	Downlink      int64          `json:"downlink"`
	UplinkTotal   int64          `json:"uplinkTotal"`
	DownlinkTotal int64          `json:"downlinkTotal"`
	Rule          string         `json:"rule"`
	CreatedAt     time.Time      `json:"createdAt"`
	ClosedAt      time.Time      `json:"closedAt"`
	ProcessInfo   APIProcessInfo `json:"processInfo"`
	Metadata      map[string]any `json:"metadata"`
}

func toAPIInfo(info *api.Info) *APIInfo {
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
	return &APIInfo{
		Backend: backend,
		Host:    info.Host,
		Port:    info.Port,
		Addr:    info.Addr(),
	}
}

func toAPIStatus(s *api.Status) APIStatus {
	if s == nil {
		return APIStatus{}
	}
	return APIStatus{
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
	}
}

func toAPINode(n api.Node) APINode {
	return APINode{
		Tag:        n.Tag,
		Type:       n.Type,
		Delay:      n.Delay,
		DelayValid: n.DelayValid,
		DelayAt:    n.DelayAt,
	}
}

func toAPIGroup(g api.Group) APIGroup {
	nodes := make([]APINode, len(g.Nodes))
	for i, n := range g.Nodes {
		nodes[i] = toAPINode(n)
	}
	return APIGroup{
		Tag:        g.Tag,
		Type:       g.Type,
		Selected:   g.Selected,
		Nodes:      nodes,
		Delay:      g.Delay,
		DelayValid: g.DelayValid,
	}
}

func toAPIProcessInfo(p api.ProcessInfo) APIProcessInfo {
	return APIProcessInfo{
		ProcessID:    p.ProcessID,
		UserID:       p.UserID,
		UserName:     p.UserName,
		ProcessPath:  p.ProcessPath,
		PackageNames: p.PackageNames,
	}
}

func toAPIConnection(c api.Connection) APIConnection {
	chain := make([]string, len(c.Chain))
	copy(chain, c.Chain)
	metadata := make(map[string]any, len(c.Metadata))
	for k, v := range c.Metadata {
		metadata[k] = v
	}
	return APIConnection{
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
		Rule:          c.Rule,
		CreatedAt:     c.CreatedAt,
		ClosedAt:      c.ClosedAt,
		ProcessInfo:   toAPIProcessInfo(c.ProcessInfo),
		Metadata:      metadata,
	}
}

// apiClient returns the current core API client or an error if unavailable.
func (b *Bindings) apiClient() (api.CoreAPIClient, error) {
	client := b.app.Controller.APIClient()
	if client == nil {
		return nil, fmt.Errorf("core API not available")
	}
	return client, nil
}

// GetAPIInfo returns runtime connection parameters for the active core API.
func (b *Bindings) GetAPIInfo() *APIInfo {
	return toAPIInfo(b.app.Controller.APIInfo())
}

// GetAPIStatus returns a one-shot snapshot of the running core instance.
func (b *Bindings) GetAPIStatus() (APIStatus, error) {
	client, err := b.apiClient()
	if err != nil {
		return APIStatus{}, err
	}
	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()
	status, err := client.Status(ctx)
	if err != nil {
		return APIStatus{}, err
	}
	return toAPIStatus(status), nil
}

// GetAPIMode returns the current proxy mode (rule / global / direct).
func (b *Bindings) GetAPIMode() (string, error) {
	client, err := b.apiClient()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()
	return client.Mode(ctx)
}

// SetAPIMode changes the proxy mode.
func (b *Bindings) SetAPIMode(mode string) error {
	client, err := b.apiClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()
	return client.SetMode(ctx, mode)
}

// GetAPIGroups returns the configured outbound groups.
func (b *Bindings) GetAPIGroups() ([]APIGroup, error) {
	client, err := b.apiClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()
	groups, err := client.Groups(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]APIGroup, len(groups))
	for i, g := range groups {
		out[i] = toAPIGroup(g)
	}
	return out, nil
}

// GetAPIConnections returns the active connections.
func (b *Bindings) GetAPIConnections() ([]APIConnection, error) {
	client, err := b.apiClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()
	conns, err := client.Connections(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]APIConnection, len(conns))
	for i, c := range conns {
		out[i] = toAPIConnection(c)
	}
	return out, nil
}

// SelectAPINode selects an outbound inside a group.
func (b *Bindings) SelectAPINode(group, node string) error {
	client, err := b.apiClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()
	return client.SelectGroup(ctx, group, node)
}

// URLTestResult maps outbound tags to latency in milliseconds.
type URLTestResult struct {
	Results map[string]int `json:"results"`
	Average int            `json:"average"`
	Count   int            `json:"count"`
}

// URLTestAPIGroup runs a latency test against all nodes in a group.
func (b *Bindings) URLTestAPIGroup(group string) (URLTestResult, error) {
	client, err := b.apiClient()
	if err != nil {
		return URLTestResult{}, err
	}
	cfg := b.app.Controller.Config()
	url := cfg.String("core", "url_test_url")
	if url == "" {
		url = "http://cp.cloudflare.com/generate_204"
	}
	ctx, cancel := context.WithTimeout(b.ctx, 15*time.Second)
	defer cancel()
	results, err := client.URLTest(ctx, group, url, 5*time.Second)
	if err != nil {
		return URLTestResult{}, err
	}
	var total, count int
	for _, delay := range results {
		total += delay
		count++
	}
	return URLTestResult{Results: results, Average: total / max(count, 1), Count: count}, nil
}

// CloseAPIConnections closes all active connections.
func (b *Bindings) CloseAPIConnections() error {
	client, err := b.apiClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()
	return client.CloseConnections(ctx)
}

// CloseAPIConnection closes a single connection by ID.
func (b *Bindings) CloseAPIConnection(id string) error {
	client, err := b.apiClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()
	return client.CloseConnection(ctx, id)
}
