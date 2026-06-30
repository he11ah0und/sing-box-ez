//go:build !nogui

package wails

import (
	"context"
	"fmt"
	"time"

	"sing-box-ez/internal/core/api"
)

// TrafficUpdate is emitted to the frontend at most once per second.
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

// trafficPoller subscribes to the core status stream and emits traffic
// updates to the frontend. It restarts automatically when the core restarts.
type trafficPoller struct {
	b             *Bindings
	lastClient    api.CoreAPIClient
	lastUpTotal   int64
	lastDownTotal int64
	lastTrafficAt time.Time
	backoff       time.Duration
}

func newTrafficPoller(b *Bindings) *trafficPoller {
	return &trafficPoller{b: b, backoff: 2 * time.Second}
}

// startTrafficPoller starts the poller in a background goroutine.
func (b *Bindings) startTrafficPoller() {
	go newTrafficPoller(b).run()
}

func (p *trafficPoller) run() {
	for {
		if p.b.ctx.Err() != nil {
			return
		}

		if !p.b.app.Controller.IsRunning() {
			p.reset(true)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		client := p.b.app.Controller.APIClient()
		if client == nil {
			p.disconnect()
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if client != p.lastClient {
			p.newClient(client)
		}

		if p.poll(client) {
			return
		}

		time.Sleep(p.backoff)
		p.increaseBackoff()
	}
}

func (p *trafficPoller) reset(full bool) {
	if full && p.lastClient != nil {
		p.lastClient = nil
		p.b.emit("traffic:updated", TrafficUpdate{Connected: false})
	}
	p.lastUpTotal = 0
	p.lastDownTotal = 0
	p.lastTrafficAt = time.Time{}
	p.backoff = 2 * time.Second
}

func (p *trafficPoller) disconnect() {
	if p.lastClient != nil {
		p.lastClient = nil
		p.b.emit("traffic:updated", TrafficUpdate{Connected: false})
	}
}

func (p *trafficPoller) newClient(client api.CoreAPIClient) {
	p.lastClient = client
	p.lastUpTotal = 0
	p.lastDownTotal = 0
	p.lastTrafficAt = time.Time{}
	p.backoff = 2 * time.Second
}

func (p *trafficPoller) increaseBackoff() {
	if p.backoff < 30*time.Second {
		p.backoff *= 2
	}
}

func (p *trafficPoller) poll(client api.CoreAPIClient) bool {
	ctx, cancel := context.WithCancel(p.b.ctx)
	ch, stop, err := client.SubscribeStatus(ctx, 1*time.Second)
	if err != nil {
		cancel()
		return false
	}
	p.backoff = 2 * time.Second

	const healthTimeout = 5 * time.Second
	watchdog := time.NewTimer(healthTimeout)
	received := false

	exit := p.readLoop(ch, stop, cancel, watchdog, &received, healthTimeout)

	watchdog.Stop()
	return exit
}

func (p *trafficPoller) readLoop(
	ch <-chan *api.StatusEvent,
	stop func(),
	cancel context.CancelFunc,
	watchdog *time.Timer,
	received *bool,
	healthTimeout time.Duration,
) bool {
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				stop()
				cancel()
				drainTimer(watchdog)
				return false
			}
			*received = true
			resetTimer(watchdog, healthTimeout)
			if ev.Error != nil {
				stop()
				cancel()
				return false
			}
			p.handleEvent(ev)

		case <-watchdog.C:
			if !*received {
				stop()
				cancel()
				return false
			}
			*received = false
			resetTimer(watchdog, healthTimeout)

		case <-p.b.ctx.Done():
			stop()
			cancel()
			drainTimer(watchdog)
			return true
		}
	}
}

func (p *trafficPoller) handleEvent(ev *api.StatusEvent) {
	now := time.Now()
	upRate := float64(ev.Status.Uplink)
	dnRate := float64(ev.Status.Downlink)

	if upRate == 0 && dnRate == 0 && ev.Status.TrafficAvailable {
		upRate, dnRate = p.deriveRates(ev.Status, now)
	}

	p.lastUpTotal = ev.Status.UplinkTotal
	p.lastDownTotal = ev.Status.DownlinkTotal
	p.lastTrafficAt = now

	backend := ""
	if info := p.b.app.Controller.APIInfo(); info != nil {
		backend = string(info.Backend)
	}

	p.b.emit("traffic:updated", TrafficUpdate{
		Up:          ev.Status.Uplink,
		Down:        ev.Status.Downlink,
		UpTotal:     ev.Status.UplinkTotal,
		DownTotal:   ev.Status.DownlinkTotal,
		UpRate:      formatSpeed(upRate),
		DownRate:    formatSpeed(dnRate),
		Connected:   true,
		Backend:     backend,
		Version:     ev.Status.Version,
		Connections: int(ev.Status.ConnectionsIn + ev.Status.ConnectionsOut),
	})
}

func (p *trafficPoller) deriveRates(s api.Status, now time.Time) (upRate, dnRate float64) {
	dt := now.Sub(p.lastTrafficAt).Seconds()
	if dt <= 0 || p.lastTrafficAt.IsZero() {
		return 0, 0
	}
	upDelta := s.UplinkTotal - p.lastUpTotal
	dnDelta := s.DownlinkTotal - p.lastDownTotal
	if upDelta >= 0 {
		upRate = float64(upDelta) / dt
	}
	if dnDelta >= 0 {
		dnRate = float64(dnDelta) / dt
	}
	return
}

func drainTimer(t *time.Timer) {
	if !t.Stop() {
		<-t.C
	}
}

func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}

func formatSpeed(v float64) string {
	if v < 1024 {
		return "0 B/s"
	}
	units := []string{"kB/s", "MB/s", "GB/s"}
	f := v / 1024
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}
