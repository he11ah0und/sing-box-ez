package state

import (
	"context"
	"testing"
	"time"

	"sing-box-ez/internal/core/api"
)

func newTestPoller(limit int) *Poller {
	return New(Deps{HistoryLimit: func() int { return limit }})
}

func TestSampleConnectionsFirstSight(t *testing.T) {
	p := newTestPoller(60)
	now := time.Now()
	rates := p.sampleConnections([]api.Connection{
		{ID: "a", UplinkTotal: 1000, DownlinkTotal: 2000},
	}, now)
	if r := rates["a"]; r.Up != 0 || r.Down != 0 {
		t.Fatalf("first sight must yield a zero rate, got %+v", r)
	}
	if got := len(p.ConnectionTrafficHistory("a").Points); got != 1 {
		t.Fatalf("expected 1 history point, got %d", got)
	}
}

func TestSampleConnectionsReportedRates(t *testing.T) {
	p := newTestPoller(60)
	now := time.Now()
	p.sampleConnections([]api.Connection{{ID: "a", UplinkTotal: 10, DownlinkTotal: 10}}, now)
	// API-reported rates win over the totals delta.
	rates := p.sampleConnections([]api.Connection{
		{ID: "a", Uplink: 500, Downlink: 700, UplinkTotal: 3000, DownlinkTotal: 5000},
	}, now.Add(time.Second))
	if r := rates["a"]; r.Up != 500 || r.Down != 700 {
		t.Fatalf("expected reported rates 500/700, got %+v", r)
	}
}

func TestSampleConnectionsDerivedRates(t *testing.T) {
	p := newTestPoller(60)
	now := time.Now()
	p.sampleConnections([]api.Connection{{ID: "a", UplinkTotal: 1000, DownlinkTotal: 2000}}, now)
	rates := p.sampleConnections([]api.Connection{
		{ID: "a", UplinkTotal: 3000, DownlinkTotal: 6000},
	}, now.Add(2*time.Second))
	if r := rates["a"]; r.Up != 1000 || r.Down != 2000 {
		t.Fatalf("expected derived rates 1000/2000, got %+v", r)
	}
}

func TestSampleConnectionsTotalsReset(t *testing.T) {
	p := newTestPoller(60)
	now := time.Now()
	p.sampleConnections([]api.Connection{{ID: "a", UplinkTotal: 5000, DownlinkTotal: 5000}}, now)
	// A negative delta (totals restarted) must not produce a negative rate.
	rates := p.sampleConnections([]api.Connection{
		{ID: "a", UplinkTotal: 10, DownlinkTotal: 10},
	}, now.Add(time.Second))
	if r := rates["a"]; r.Up != 0 || r.Down != 0 {
		t.Fatalf("expected zero rates on totals reset, got %+v", r)
	}
}

func TestSampleConnectionsTrimsHistory(t *testing.T) {
	p := newTestPoller(3)
	now := time.Now()
	for i := 0; i < 5; i++ {
		p.sampleConnections([]api.Connection{
			{ID: "a", Uplink: int64(i + 1), UplinkTotal: int64(i)},
		}, now.Add(time.Duration(i)*time.Second))
	}
	hist := p.ConnectionTrafficHistory("a").Points
	if len(hist) != 3 {
		t.Fatalf("expected history trimmed to 3, got %d", len(hist))
	}
	// The retained points are the most recent ones (rates 3, 4, 5).
	for i, want := range []int64{3, 4, 5} {
		if hist[i].Up != want {
			t.Fatalf("point %d: expected up=%d, got %d", i, want, hist[i].Up)
		}
	}
}

func TestSampleConnectionsEvictsOldest(t *testing.T) {
	p := newTestPoller(60)
	now := time.Now()
	// Fill the map beyond the cap; the connection with the oldest last
	// sample must be evicted.
	for i := 0; i < connHistoryMax; i++ {
		p.sampleConnections([]api.Connection{
			{ID: string(rune('a'+i%26)) + string(rune('A'+i/26)), UplinkTotal: 1},
		}, now.Add(time.Duration(i)*time.Second))
	}
	p.sampleConnections([]api.Connection{{ID: "new", UplinkTotal: 1}}, now.Add(connHistoryMax*time.Second))
	if len(p.connHistory) > connHistoryMax {
		t.Fatalf("expected at most %d histories, got %d", connHistoryMax, len(p.connHistory))
	}
	if _, ok := p.connHistory["aA"]; ok {
		t.Fatal("expected the oldest history to be evicted")
	}
	if _, ok := p.connLast["aA"]; ok {
		t.Fatal("expected the evicted connection's delta baseline to be dropped")
	}
	if got := len(p.ConnectionTrafficHistory("new").Points); got != 1 {
		t.Fatalf("expected the newest history to survive, got %d points", got)
	}
}

func TestConnectionTrafficHistoryCopy(t *testing.T) {
	p := newTestPoller(60)
	p.sampleConnections([]api.Connection{{ID: "a", Uplink: 5}}, time.Now())
	first := p.ConnectionTrafficHistory("a")
	first.Points[0].Up = 999
	second := p.ConnectionTrafficHistory("a")
	if second.Points[0].Up == 999 {
		t.Fatal("ConnectionTrafficHistory must return a copy")
	}
	if got := len(p.ConnectionTrafficHistory("unknown").Points); got != 0 {
		t.Fatalf("expected an empty history for an unknown connection, got %d", got)
	}
}

// stubAPIClient implements only the methods handleEvent calls; the embedded
// interface satisfies the rest.
type stubAPIClient struct {
	api.CoreAPIClient
}

func (stubAPIClient) Groups(context.Context) ([]api.Group, error) { return nil, nil }
func (stubAPIClient) Mode(context.Context) (string, error)        { return "rule", nil }
func (stubAPIClient) Connections(context.Context) ([]api.Connection, error) {
	return []api.Connection{{ID: "a", UplinkTotal: 10, DownlinkTotal: 20}}, nil
}

// TestHandleEventNoDeadlock guards against reentrant mu usage in handleEvent:
// connection sampling locks mu internally, so it must not run under the state
// update lock.
func TestHandleEventNoDeadlock(t *testing.T) {
	p := New(Deps{Info: func() *api.Info { return nil }})
	p.lastClient = stubAPIClient{}

	done := make(chan struct{})
	go func() {
		p.handleEvent(context.Background(), &api.StatusEvent{})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handleEvent deadlocked")
	}

	st := p.State()
	if st.Phase != PhaseConnected {
		t.Fatalf("phase = %q, want %q", st.Phase, PhaseConnected)
	}
	if len(st.Connections) != 1 || st.Connections[0].ID != "a" {
		t.Fatalf("connections = %+v", st.Connections)
	}
}

func TestFilterGroupsByName(t *testing.T) {
	groups := []api.Group{
		{Tag: "Proxy"},
		{Tag: "Auto"},
		{Tag: "proxy-us"},
	}
	if got := filterGroupsByName(groups, ""); len(got) != 3 {
		t.Fatalf("empty filter must keep all groups, got %d", len(got))
	}
	got := filterGroupsByName(groups, "PROXY")
	if len(got) != 2 || got[0].Tag != "Proxy" || got[1].Tag != "proxy-us" {
		t.Fatalf("case-insensitive substring match failed: %+v", got)
	}
	if got := filterGroupsByName(groups, "missing"); len(got) != 0 {
		t.Fatalf("expected no matches, got %+v", got)
	}
}

func TestSetGroupFilterNormalizes(t *testing.T) {
	p := newTestPoller(60)
	p.SetGroupFilter("  Proxy ")
	if p.groupFilter != "proxy" {
		t.Fatalf("groupFilter = %q, want %q", p.groupFilter, "proxy")
	}
	p.SetGroupFilter("")
	if p.groupFilter != "" {
		t.Fatalf("groupFilter = %q, want empty", p.groupFilter)
	}
}
