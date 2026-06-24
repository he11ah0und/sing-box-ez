// Package startup provides shared helpers for the GUI startup mode selection.
package startup

import (
	"sync"
	"time"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework/rpc"
	"sing-box-ez/internal/framework/svcman"
	"sing-box-ez/internal/framework/svcman/factory"
)

// OptionKind classifies a startup option.
type OptionKind int

const (
	KindEmbed OptionKind = iota
	KindService
	KindRemote
)

// Option represents a single selectable startup mode.
type Option struct {
	ID      string     // "embed", manager name, or "remote".
	Type    string     // "embed", service manager name, or "remote".
	Address string     // Connection address for remote options.
	Kind    OptionKind // Classification.
	Online  bool       // Whether the backend is reachable/running.
	Manager svcman.Manager
}

// Discover scans for available startup modes and their status.
// It always returns at least the embed option and the manual remote option.
// Available service managers are detected via svcman factory and added
// regardless of whether the service is installed; their online state reflects
// whether the service is currently running.
func Discover(cfg *config.AppConfig) []Option {
	options := []Option{{ID: "embed", Type: "embed", Kind: KindEmbed}}

	// Detect available service managers.
	for _, m := range factory.All("sing-box-ez") {
		opt := Option{
			ID:      m.Name(),
			Type:    m.Name(),
			Kind:    KindService,
			Manager: m,
		}
		if s, err := m.Status(); err == nil && s == svcman.StatusRunning {
			opt.Online = true
		}
		options = append(options, opt)
	}

	// Manual remote connection via TCP.
	remoteOpt := Option{ID: "remote", Type: "remote", Kind: KindRemote}
	if tcpAddr := cfg.MustGet("remote", "last_tcp_address").String(); tcpAddr != "" {
		remoteOpt.Address = tcpAddr
		remoteOpt.Online = probeTransportTCP(tcpAddr)
	}
	options = append(options, remoteOpt)

	return options
}

// DiscoverAsync is like Discover but performs status probes in the background.
func DiscoverAsync(cfg *config.AppConfig) ([]Option, chan Option) {
	options := Discover(cfg)
	updates := make(chan Option, len(options))

	var wg sync.WaitGroup
	for i := range options {
		if options[i].Kind != KindRemote {
			continue
		}
		idx := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			if options[idx].Address == "" {
				return
			}
			online := probeTransportTCP(options[idx].Address)
			if online != options[idx].Online {
				options[idx].Online = online
				updates <- options[idx]
			}
		}()
	}

	go func() {
		wg.Wait()
		close(updates)
	}()

	return options, updates
}

// ServiceManagers returns all detected service managers, keyed by name.
func ServiceManagers(options []Option) map[string]svcman.Manager {
	m := make(map[string]svcman.Manager)
	for i := range options {
		if options[i].Kind == KindService && options[i].Manager != nil {
			m[options[i].ID] = options[i].Manager
		}
	}
	return m
}

func probeTransportTCP(addr string) bool {
	if addr == "" {
		return false
	}
	t, err := rpc.ParseAddress(addr)
	if err != nil {
		return false
	}
	done := make(chan bool, 1)
	go func() {
		c, err := t.Dial()
		if err != nil {
			done <- false
			return
		}
		_ = c.Close()
		done <- true
	}()
	select {
	case <-time.After(1500 * time.Millisecond):
		return false
	case ok := <-done:
		return ok
	}
}
