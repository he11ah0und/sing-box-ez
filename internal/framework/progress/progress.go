// Package progress provides a unified progress callback mechanism for
// long-running operations such as network downloads and file copies.
package progress

import (
	"sync"
	"time"
)

// State describes the current progress of an operation.
type State struct {
	Op      string // operation name, e.g. "download", "copy"
	Label   string // human-readable subject, e.g. URL or path
	Current int64  // completed bytes/items
	Total   int64  // total bytes/items; -1 if unknown
}

// Done reports whether the operation has reached its total.
func (s State) Done() bool {
	return s.Total > 0 && s.Current >= s.Total
}

// Percent returns the completion percentage in the range [0, 100].
// When Total is unknown or zero, it returns 0.
func (s State) Percent() float64 {
	if s.Total <= 0 {
		return 0
	}
	p := float64(s.Current) / float64(s.Total) * 100
	if p > 100 {
		return 100
	}
	return p
}

// Callback is invoked whenever progress should be reported.
type Callback func(state State)

// Config groups a callback with throttling settings.
type Config struct {
	Callback Callback
	Interval time.Duration // minimum time between callbacks; zero means every update
}

// Reporter wraps a Config and handles throttling.
type Reporter struct {
	cfg      *Config
	last     time.Time
	lastSent State
	mu       sync.Mutex
}

// NewReporter creates a reporter from cfg. If cfg is nil, reports are dropped.
func NewReporter(cfg *Config) *Reporter {
	return &Reporter{cfg: cfg}
}

// Report sends state to the callback if the configured interval has elapsed
// or if the operation is complete. It is safe to call from multiple goroutines.
func (r *Reporter) Report(state State) {
	if r.cfg == nil || r.cfg.Callback == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if state.Total <= 0 {
		state.Total = -1
	}

	now := time.Now()
	if !r.last.IsZero() &&
		!state.Done() &&
		r.cfg.Interval > 0 &&
		now.Sub(r.last) < r.cfg.Interval {
		return
	}

	r.last = now
	r.lastSent = state
	r.cfg.Callback(state)
}

// Finish forces a final callback with the current total reached.
func (r *Reporter) Finish(op, label string, total int64) {
	r.Report(State{Op: op, Label: label, Current: total, Total: total})
}
