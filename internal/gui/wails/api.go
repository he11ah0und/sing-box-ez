//go:build !nogui

package wails

import (
	"context"
	"fmt"
	"time"

	"sing-box-ez/internal/core/api"
)

// apiClient returns the current core API client or an error if unavailable.
func (b *Bindings) apiClient() (api.CoreAPIClient, error) {
	client := b.app.Controller.APIClient()
	if client == nil {
		return nil, fmt.Errorf("core API not available")
	}
	return client, nil
}

// SetAPIMode changes the proxy mode.
func (b *Bindings) SetAPIMode(mode string) error {
	client, err := b.apiClient()
	if err != nil {
		b.toastErr(err)
		return err
	}
	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()
	if err := client.SetMode(ctx, mode); err != nil {
		b.toastErr(err)
		return err
	}
	return nil
}

// SelectAPINode selects an outbound inside a group.
func (b *Bindings) SelectAPINode(group, node string) error {
	client, err := b.apiClient()
	if err != nil {
		b.toastErr(err)
		return err
	}
	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()
	if err := client.SelectGroup(ctx, group, node); err != nil {
		b.toastErr(err)
		return err
	}
	return nil
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
	ctx, cancel := context.WithTimeout(b.ctx, 15*time.Second)
	defer cancel()
	results, err := client.URLTest(ctx, group, url, 5*time.Second)
	if err != nil {
		b.toastErr(err)
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
		b.toastErr(err)
		return err
	}
	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()
	if err := client.CloseConnections(ctx); err != nil {
		b.toastErr(err)
		return err
	}
	return nil
}

// CloseAPIConnection closes a single connection by ID.
func (b *Bindings) CloseAPIConnection(id string) error {
	client, err := b.apiClient()
	if err != nil {
		b.toastErr(err)
		return err
	}
	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()
	if err := client.CloseConnection(ctx, id); err != nil {
		b.toastErr(err)
		return err
	}
	return nil
}

// CloseAPIConnectionGroup closes the listed connections (the live members of
// one connection group).
func (b *Bindings) CloseAPIConnectionGroup(ids []string) error {
	client, err := b.apiClient()
	if err != nil {
		b.toastErr(err)
		return err
	}
	ctx, cancel := context.WithTimeout(b.ctx, 10*time.Second)
	defer cancel()
	for _, id := range ids {
		if err := client.CloseConnection(ctx, id); err != nil {
			b.toastErr(err)
			return err
		}
	}
	return nil
}
