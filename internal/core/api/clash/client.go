package clash

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
		if p.Type != "Selector" && p.Type != "URLTest" && p.Type != "Fallback" {
			continue
		}
		g := api.Group{
			Tag:      name,
			Type:     p.Type,
			Selected: p.Now,
		}
		for _, n := range p.All {
			if n != "" {
				g.Nodes = append(g.Nodes, n)
			}
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
		out[i] = api.Connection{
			ID:       conn.ID,
			Metadata: conn.Metadata,
		}
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

type proxy struct {
	Type string   `json:"type"`
	Now  string   `json:"now"`
	All  []string `json:"all"`
}

type connection struct {
	ID       string         `json:"id"`
	Metadata map[string]any `json:"metadata"`
}
