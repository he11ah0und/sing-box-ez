package state

import (
	"context"
	"fmt"
	"sync"
	"time"

	"sing-box-ez/internal/core/api"
)

// Deps wires the Poller to the core controller without importing it, so the
// package stays usable from any frontend (GUI, CLI/TUI).
type Deps struct {
	// IsRunning reports whether the core process is alive.
	IsRunning func() bool
	// PhaseHint returns the current lifecycle stage reported by the start/stop
	// flow (e.g. PhasePreparingConfig, PhaseStarting, PhaseStopping), or an
	// empty string when no transition is in flight.
	PhaseHint func() string
	// Client returns the current core API client, or nil when unavailable.
	Client func() api.CoreAPIClient
	// Info returns the runtime API connection parameters, or nil.
	Info func() *api.Info
	// HistoryLimit returns the configured number of retained graph samples.
	HistoryLimit func() int
}

// Poller subscribes to the core status stream and drives everything a
// frontend needs from the core API: live traffic updates, the retained graph
// history and per-second state snapshots with the connection phase.
// It restarts automatically when the core restarts.
type Poller struct {
	d Deps

	onTraffic func(TrafficUpdate)
	onState   func(Update)

	lastClient    api.CoreAPIClient
	lastUpTotal   int64
	lastDownTotal int64
	lastTrafficAt time.Time
	backoff       time.Duration

	// connectedAt is when the core API first answered after the last
	// failure; frontends show it as the connection session start.
	connectedAt time.Time

	// mu guards history, state and phase: the poller goroutine writes while
	// snapshot readers run on other goroutines.
	mu      sync.Mutex
	history []TrafficPoint
	state   Update
	phase   string
}

// New creates a Poller. Run it with Run.
func New(d Deps) *Poller {
	return &Poller{d: d, backoff: 2 * time.Second, phase: PhaseStopped}
}

// OnTraffic registers the traffic update callback (at most one per second).
func (p *Poller) OnTraffic(fn func(TrafficUpdate)) { p.onTraffic = fn }

// OnState registers the state snapshot callback.
func (p *Poller) OnState(fn func(Update)) { p.onState = fn }

func (p *Poller) emitTraffic(u TrafficUpdate) {
	if p.onTraffic != nil {
		p.onTraffic(u)
	}
}

func (p *Poller) emitState(s Update) {
	if p.onState != nil {
		p.onState(s)
	}
}

// Run drives the poll loop until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		if !p.d.IsRunning() {
			p.setPhase(p.disconnectedPhase())
			p.resetConn(true)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		client := p.d.Client()
		if client == nil {
			p.setPhase(p.waitingPhase())
			p.resetConn(false)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if client != p.lastClient {
			p.newClient(client)
		}

		if p.poll(ctx, client) {
			return
		}

		// The status stream broke while the process is alive: back to waiting.
		p.setPhase(p.waitingPhase())
		p.resetConn(false)
		time.Sleep(p.backoff)
		p.increaseBackoff()
	}
}

// disconnectedPhase picks the phase for a dead core process: an explicit
// lifecycle hint from the start/stop flow wins over the plain stopped phase.
func (p *Poller) disconnectedPhase() string {
	if hint := p.phaseHint(); hint != "" {
		return hint
	}
	return PhaseStopped
}

// waitingPhase picks the phase for a live process with a dead API.
func (p *Poller) waitingPhase() string {
	if hint := p.phaseHint(); hint != "" {
		return hint
	}
	return PhaseWaiting
}

func (p *Poller) phaseHint() string {
	if p.d.PhaseHint == nil {
		return ""
	}
	return p.d.PhaseHint()
}

// setPhase switches the connection phase, dropping the API snapshot and the
// connection session start. State events are pushed only on transitions.
func (p *Poller) setPhase(phase string) {
	p.resetConnectedAt()
	if p.lastClient != nil {
		p.lastClient = nil
		p.emitTraffic(TrafficUpdate{Connected: false})
	}
	p.mu.Lock()
	changed := p.phase != phase
	p.phase = phase
	p.state = Update{Phase: phase}
	p.mu.Unlock()
	if changed {
		p.emitState(Update{Phase: phase})
	}
}

func (p *Poller) resetConn(clearHistory bool) {
	p.lastUpTotal = 0
	p.lastDownTotal = 0
	p.lastTrafficAt = time.Time{}
	p.backoff = 2 * time.Second
	if clearHistory {
		// The core stopped: the retained samples describe a dead session.
		p.mu.Lock()
		p.history = nil
		p.mu.Unlock()
	}
}

func (p *Poller) newClient(client api.CoreAPIClient) {
	p.lastClient = client
	p.lastUpTotal = 0
	p.lastDownTotal = 0
	p.lastTrafficAt = time.Time{}
	p.backoff = 2 * time.Second
}

func (p *Poller) increaseBackoff() {
	if p.backoff < 30*time.Second {
		p.backoff *= 2
	}
}

func (p *Poller) poll(ctx context.Context, client api.CoreAPIClient) bool {
	streamCtx, cancel := context.WithCancel(ctx)
	ch, stop, err := client.SubscribeStatus(streamCtx, 1*time.Second)
	if err != nil {
		cancel()
		return false
	}
	p.backoff = 2 * time.Second

	const healthTimeout = 5 * time.Second
	watchdog := time.NewTimer(healthTimeout)
	received := false

	exit := p.readLoop(ctx, ch, stop, cancel, watchdog, &received, healthTimeout)

	watchdog.Stop()
	return exit
}

func (p *Poller) readLoop(
	ctx context.Context,
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
			p.handleEvent(ctx, ev)

		case <-watchdog.C:
			if !*received {
				stop()
				cancel()
				return false
			}
			*received = false
			resetTimer(watchdog, healthTimeout)

		case <-ctx.Done():
			stop()
			cancel()
			drainTimer(watchdog)
			return true
		}
	}
}

// handleEvent processes one status stream event (~1/s): it updates the
// traffic history and pushes both the traffic update and the full API state
// snapshot built from the same moment in time.
func (p *Poller) handleEvent(ctx context.Context, ev *api.StatusEvent) {
	now := time.Now()
	upRate := float64(ev.Status.Uplink)
	dnRate := float64(ev.Status.Downlink)

	if upRate == 0 && dnRate == 0 && ev.Status.TrafficAvailable {
		upRate, dnRate = p.deriveRates(ev.Status, now)
	}

	p.lastUpTotal = ev.Status.UplinkTotal
	p.lastDownTotal = ev.Status.DownlinkTotal
	p.lastTrafficAt = now

	p.appendHistory(TrafficPoint{At: now, Up: ev.Status.Uplink, Down: ev.Status.Downlink})

	// The first answered status event owns the waiting -> connected
	// transition of the connection session.
	connectedAt := p.markConnected()

	backend := ""
	if info := p.d.Info(); info != nil {
		backend = string(info.Backend)
	}

	p.emitTraffic(TrafficUpdate{
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

	// Secondary API data keeps its previous values on transient errors.
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	groups, gErr := p.lastClient.Groups(callCtx)
	conns, cErr := p.lastClient.Connections(callCtx)
	mode, mErr := p.lastClient.Mode(callCtx)
	cancel()

	status := toStatus(&ev.Status, connectedAt)

	p.mu.Lock()
	next := Update{
		Phase:       PhaseConnected,
		Status:      &status,
		Info:        toInfo(p.d.Info()),
		Mode:        p.state.Mode,
		Groups:      p.state.Groups,
		Connections: p.state.Connections,
	}
	if mErr == nil {
		next.Mode = mode
	}
	if gErr == nil {
		next.Groups = toGroups(groups)
	}
	if cErr == nil {
		next.Connections = toConnections(conns)
	}
	p.state = next
	p.phase = PhaseConnected
	p.mu.Unlock()

	p.emitState(next)
}

// appendHistory retains a rate sample, trimming the window to the configured
// graph history length (in seconds, one sample per second).
func (p *Poller) appendHistory(pt TrafficPoint) {
	limit := p.historyLimit()
	p.mu.Lock()
	p.history = append(p.history, pt)
	if len(p.history) > limit {
		p.history = p.history[len(p.history)-limit:]
	}
	p.mu.Unlock()
}

// historyLimit returns the configured number of retained samples.
func (p *Poller) historyLimit() int {
	n := 0
	if p.d.HistoryLimit != nil {
		n = p.d.HistoryLimit()
	}
	if n < 2 {
		n = 60
	}
	return n
}

// History returns the retained traffic rate samples.
func (p *Poller) History() TrafficHistory {
	p.mu.Lock()
	defer p.mu.Unlock()
	points := make([]TrafficPoint, len(p.history))
	copy(points, p.history)
	return TrafficHistory{Points: points}
}

// State returns the latest cached core API snapshot.
func (p *Poller) State() Update {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// markConnected records the first successful API answer after a failure and
// returns the time the current connection session started.
func (p *Poller) markConnected() time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.connectedAt.IsZero() {
		p.connectedAt = time.Now()
	}
	return p.connectedAt
}

// resetConnectedAt drops the connection session start on API failure.
func (p *Poller) resetConnectedAt() {
	p.mu.Lock()
	p.connectedAt = time.Time{}
	p.mu.Unlock()
}

func (p *Poller) deriveRates(s api.Status, now time.Time) (upRate, dnRate float64) {
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
