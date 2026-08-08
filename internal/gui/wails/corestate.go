//go:build !nogui

package wails

import (
	"sing-box-ez/internal/core/state"
)

// startCorePoller starts the core state poller in a background goroutine and
// forwards its pushes as Wails events.
func (b *Bindings) startCorePoller() {
	b.core = state.New(state.Deps{
		IsRunning: b.app.Controller.IsRunning,
		PhaseHint: b.phaseHint,
		Client:    b.app.Controller.APIClient,
		Info:      b.app.Controller.APIInfo,
		HistoryLimit: func() int {
			return b.app.Controller.Config().MustGet("core", "traffic_graph_history").Int()
		},
	})
	b.core.OnTraffic(func(u state.TrafficUpdate) { b.emit("traffic:updated", u) })
	b.core.OnState(func(s state.Update) { b.emit("api:state", s) })
	go b.core.Run(b.ctx)
}

// GetTrafficHistory returns the retained traffic rate samples.
func (b *Bindings) GetTrafficHistory() state.TrafficHistory {
	if b.core == nil {
		return state.TrafficHistory{}
	}
	return b.core.History()
}

// GetAPIState returns the latest cached core API snapshot.
func (b *Bindings) GetAPIState() state.Update {
	if b.core == nil {
		return state.Update{Phase: state.PhaseStopped}
	}
	return b.core.State()
}
