package clash

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sing-box-ez/internal/core/api"
)

// Client implements api.CoreAPIClient over the Clash REST API.
type Client struct {
	baseURL string
	secret  string
	client  *http.Client
}

// NewClient creates a Clash API client.
func NewClient(baseURL, secret string) *Client {
	return &Client{
		baseURL: baseURL,
		secret:  secret,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) req(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}
	u, err := base.Parse(path)
	if err != nil {
		return nil, err
	}
	r, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	if c.secret != "" {
		r.Header.Set("Authorization", "Bearer "+c.secret)
	}
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	return r, nil
}

func (c *Client) doJSON(req *http.Request, out any) error {
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("clash API %s %s: %d %s", req.Method, req.URL.Path, resp.StatusCode, string(body))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Status implements api.CoreAPIClient.
func (c *Client) Status(ctx context.Context) (*api.Status, error) {
	req, err := c.req(ctx, "GET", "/version", nil)
	if err != nil {
		return nil, err
	}
	var v struct {
		Version string `json:"version"`
	}
	if err := c.doJSON(req, &v); err != nil {
		return nil, err
	}
	return &api.Status{Version: v.Version}, nil
}

// Groups implements api.CoreAPIClient.
func (c *Client) Groups(ctx context.Context) ([]api.Group, error) {
	req, err := c.req(ctx, "GET", "/proxies", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Proxies map[string]proxy `json:"proxies"`
	}
	if err := c.doJSON(req, &resp); err != nil {
		return nil, err
	}
	var groups []api.Group
	for name, p := range resp.Proxies {
		if p.Type != "Selector" && p.Type != "URLTest" && p.Type != "Fallback" && p.Type != "LoadBalance" {
			continue
		}
		g := api.Group{
			Tag:      name,
			Type:     p.Type,
			Selected: p.Now,
		}
		if len(p.History) > 0 {
			last := p.History[len(p.History)-1]
			if last.Delay > 0 {
				g.Delay = last.Delay
				g.DelayValid = true
			}
		}
		for _, n := range p.All {
			if n == "" {
				continue
			}
			node := api.Node{Tag: n}
			if leaf, ok := resp.Proxies[n]; ok {
				node.Type = leaf.Type
				if len(leaf.History) > 0 {
					last := leaf.History[len(leaf.History)-1]
					if last.Delay > 0 {
						node.Delay = last.Delay
						node.DelayValid = true
					}
				}
			}
			g.Nodes = append(g.Nodes, node)
		}
		groups = append(groups, g)
	}
	return groups, nil
}

// SelectGroup implements api.CoreAPIClient.
func (c *Client) SelectGroup(ctx context.Context, group, outbound string) error {
	path := "/proxies/" + url.PathEscape(group)
	body, _ := json.Marshal(map[string]string{"name": outbound})
	req, err := c.req(ctx, "PUT", path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	return c.doJSON(req, nil)
}

// Mode implements api.CoreAPIClient.
func (c *Client) Mode(ctx context.Context) (string, error) {
	req, err := c.req(ctx, "GET", "/configs", nil)
	if err != nil {
		return "", err
	}
	var cfg struct {
		Mode string `json:"mode"`
	}
	if err := c.doJSON(req, &cfg); err != nil {
		return "", err
	}
	return cfg.Mode, nil
}

// SetMode implements api.CoreAPIClient.
func (c *Client) SetMode(ctx context.Context, mode string) error {
	body, _ := json.Marshal(map[string]string{"mode": mode})
	req, err := c.req(ctx, "PUT", "/configs", bytes.NewReader(body))
	if err != nil {
		return err
	}
	return c.doJSON(req, nil)
}

// Connections implements api.CoreAPIClient.
func (c *Client) Connections(ctx context.Context) ([]api.Connection, error) {
	req, err := c.req(ctx, "GET", "/connections", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Connections []connection `json:"connections"`
	}
	if err := c.doJSON(req, &resp); err != nil {
		return nil, err
	}
	out := make([]api.Connection, len(resp.Connections))
	for i, conn := range resp.Connections {
		out[i] = connectionFromClash(conn)
	}
	return out, nil
}

// CloseConnections implements api.CoreAPIClient.
func (c *Client) CloseConnections(ctx context.Context) error {
	req, err := c.req(ctx, "DELETE", "/connections", nil)
	if err != nil {
		return err
	}
	return c.doJSON(req, nil)
}

// CloseConnection implements api.CoreAPIClient.
func (c *Client) CloseConnection(ctx context.Context, id string) error {
	path := "/connections/" + url.PathEscape(id)
	req, err := c.req(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}
	return c.doJSON(req, nil)
}

// URLTest implements api.CoreAPIClient.
func (c *Client) URLTest(ctx context.Context, group, testURL string, timeout time.Duration) (map[string]int, error) {
	path := fmt.Sprintf("/group/%s/delay?url=%s&timeout=%d", url.PathEscape(group), url.QueryEscape(testURL), int(timeout.Milliseconds()))
	req, err := c.req(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var delays map[string]int
	if err := c.doJSON(req, &delays); err != nil {
		return nil, err
	}
	return delays, nil
}

// SubscribeStatus implements api.CoreAPIClient.
// Clash provides a live traffic stream at /traffic; if it is unavailable we
// fall back to periodic version pings with TrafficAvailable=false.
func (c *Client) SubscribeStatus(ctx context.Context, interval time.Duration) (<-chan *api.StatusEvent, func(), error) {
	_ = interval
	ch := make(chan *api.StatusEvent, 1)
	ctx, cancel := context.WithCancel(ctx)
	stop := func() { cancel() }

	go func() {
		defer close(ch)

		// Only the SSE traffic stream carries live speed data on the Clash API.
		// If it is unavailable, let the caller fall back to connection totals
		// instead of keeping a useless polling loop alive.
		if err := c.readTrafficStream(ctx, ch); err != nil {
			if ctx.Err() != nil {
				return
			}
			select {
			case ch <- &api.StatusEvent{Status: api.Status{TrafficAvailable: false}, Error: err}:
			case <-ctx.Done():
			}
		}
	}()

	return ch, stop, nil
}

func (c *Client) readTrafficStream(ctx context.Context, ch chan<- *api.StatusEvent) error {
	req, err := c.req(ctx, "GET", "/traffic", nil)
	if err != nil {
		return err
	}
	// Streaming request: do not use the default timeout client.
	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("clash traffic stream: %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Some cores send raw JSON lines, others wrap them in SSE "data:".
		data := line
		if strings.HasPrefix(line, "data:") {
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" {
				continue
			}
		}
		var t struct {
			Up   int64 `json:"up"`
			Down int64 `json:"down"`
		}
		if err := json.Unmarshal([]byte(data), &t); err != nil {
			continue
		}
		select {
		case ch <- &api.StatusEvent{Status: api.Status{
			TrafficAvailable: true,
			Uplink:           t.Up,
			Downlink:         t.Down,
		}}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return scanner.Err()
}

// SubscribeConnections implements api.CoreAPIClient.
func (c *Client) SubscribeConnections(ctx context.Context, interval time.Duration) (<-chan *api.ConnectionEvent, func(), error) {
	ch := make(chan *api.ConnectionEvent, 4)
	ctx, cancel := context.WithCancel(ctx)
	stop := func() { cancel() }

	go func() {
		defer close(ch)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			conns, err := c.Connections(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				select {
				case ch <- &api.ConnectionEvent{Error: err}:
				case <-ctx.Done():
				}
				return
			}
			for _, conn := range conns {
				select {
				case ch <- &api.ConnectionEvent{Type: api.ConnectionEventNew, Connection: conn}:
				case <-ctx.Done():
					return
				}
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, stop, nil
}

type proxy struct {
	Type    string    `json:"type"`
	Now     string    `json:"now"`
	All     []string  `json:"all"`
	History []history `json:"history"`
}

type history struct {
	Time  string `json:"time"`
	Delay int    `json:"delay"`
}

type connection struct {
	ID          string         `json:"id"`
	Metadata    map[string]any `json:"metadata"`
	Chains      []string       `json:"chains"`
	Upload      int64          `json:"upload"`
	Download    int64          `json:"download"`
	Start       string         `json:"start"`
	Rule        string         `json:"rule"`
	RulePayload string         `json:"rulePayload"`
}

func connectionFromClash(c connection) api.Connection {
	// Clash returns chains from innermost (final) outbound to outermost selector,
	// so reverse them for top-down display (selector → group → final node).
	chain := make([]string, len(c.Chains))
	for i, v := range c.Chains {
		chain[len(c.Chains)-1-i] = v
	}
	conn := api.Connection{
		ID:            c.ID,
		Metadata:      c.Metadata,
		Chain:         chain,
		UplinkTotal:   c.Upload,
		DownlinkTotal: c.Download,
		Rule:          c.Rule,
	}
	if len(conn.Chain) > 0 {
		conn.Outbound = conn.Chain[len(conn.Chain)-1]
	}
	if c.Start != "" {
		if t, err := time.Parse(time.RFC3339, c.Start); err == nil {
			conn.CreatedAt = t
		}
	}
	if c.Metadata == nil {
		return conn
	}
	if v, ok := c.Metadata["host"].(string); ok {
		conn.Domain = v
	}
	if v, ok := c.Metadata["network"].(string); ok {
		conn.Network = v
	}
	if v, ok := c.Metadata["type"].(string); ok {
		conn.InboundType = v
	}
	if v, ok := c.Metadata["sourceIP"].(string); ok {
		if sp, ok := c.Metadata["sourcePort"].(string); ok {
			conn.Source = v + ":" + sp
		} else {
			conn.Source = v
		}
	}
	if v, ok := c.Metadata["destinationIP"].(string); ok {
		if dp, ok := c.Metadata["destinationPort"].(string); ok {
			conn.Destination = v + ":" + dp
		} else {
			conn.Destination = v
		}
	}
	return conn
}
