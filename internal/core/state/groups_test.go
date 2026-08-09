package state

import (
	"testing"
	"time"

	"sing-box-ez/internal/core/api"
)

func conn(id, domain, dest string, up, down int64) api.Connection {
	return api.Connection{
		ID:            id,
		Domain:        domain,
		Destination:   dest,
		UplinkTotal:   up,
		DownlinkTotal: down,
		CreatedAt:     time.Now(),
	}
}

func TestGroupTrackerGroupsByTarget(t *testing.T) {
	tr := newGroupTracker()
	now := time.Now()

	// Two parallel connections to the same target aggregate into one group.
	tr.update([]api.Connection{
		conn("a", "example.com", "1.2.3.4:443", 10, 100),
		conn("b", "example.com", "5.6.7.8:443", 20, 200),
		conn("c", "other.com", "9.9.9.9:443", 1, 1),
	}, now, time.Hour)

	groups := tr.snapshot(nil, now)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	g := groups[0]
	if g.Key != "example.com:443" {
		t.Fatalf("expected key example.com:443, got %q", g.Key)
	}
	if g.ConnCount != 2 || !g.Active {
		t.Fatalf("expected 2 active members, got count=%d active=%v", g.ConnCount, g.Active)
	}
	if g.UpTotal != 30 || g.DownTotal != 300 {
		t.Fatalf("expected totals 30/300, got %d/%d", g.UpTotal, g.DownTotal)
	}
	if len(g.Spans) != 2 || !g.Spans[0].End.IsZero() || !g.Spans[1].End.IsZero() {
		t.Fatalf("expected two open spans, got %+v", g.Spans)
	}
}

func TestGroupTrackerClosesIntervalAndKeepsTotals(t *testing.T) {
	tr := newGroupTracker()
	now := time.Now()

	tr.update([]api.Connection{conn("a", "example.com", "1.2.3.4:443", 10, 100)}, now, time.Hour)
	// The connection is gone: the interval closes but the group stays.
	tr.update(nil, now.Add(time.Minute), time.Hour)

	groups := tr.snapshot(nil, now.Add(time.Minute))
	if len(groups) != 1 {
		t.Fatalf("expected 1 retained group, got %d", len(groups))
	}
	g := groups[0]
	if g.Active || g.ConnCount != 0 {
		t.Fatalf("expected inactive group, got active=%v count=%d", g.Active, g.ConnCount)
	}
	if g.UpTotal != 10 || g.DownTotal != 100 {
		t.Fatalf("expected totals to survive close, got %d/%d", g.UpTotal, g.DownTotal)
	}
	if len(g.Spans) != 1 || g.Spans[0].End.IsZero() {
		t.Fatalf("expected closed span, got %+v", g.Spans)
	}

	// A new connection to the same target reopens the group with a second span.
	tr.update([]api.Connection{conn("b", "example.com", "1.2.3.4:443", 5, 50)}, now.Add(2*time.Minute), time.Hour)
	groups = tr.snapshot(nil, now.Add(2*time.Minute))
	g = groups[0]
	if !g.Active || len(g.Spans) != 2 {
		t.Fatalf("expected active group with 2 spans, got active=%v spans=%+v", g.Active, g.Spans)
	}
	if g.UpTotal != 15 || g.DownTotal != 150 {
		t.Fatalf("expected accumulated totals 15/150, got %d/%d", g.UpTotal, g.DownTotal)
	}
}

func TestGroupTrackerRetentionPrunes(t *testing.T) {
	tr := newGroupTracker()
	now := time.Now()

	tr.update([]api.Connection{conn("a", "example.com", "1.2.3.4:443", 10, 100)}, now, time.Hour)
	tr.update(nil, now.Add(time.Minute), time.Hour)
	// Past retention with no members: the group is dropped.
	tr.update(nil, now.Add(2*time.Hour), time.Hour)

	if groups := tr.snapshot(nil, now.Add(2*time.Hour)); len(groups) != 0 {
		t.Fatalf("expected group pruned after retention, got %d", len(groups))
	}
}

func TestGroupTrafficHistory(t *testing.T) {
	p := newTestPoller(60)
	now := time.Now()

	conns := []api.Connection{
		conn("a", "example.com", "1.2.3.4:443", 10, 100),
		conn("b", "example.com", "5.6.7.8:443", 20, 200),
	}
	rates := map[string]TrafficPoint{
		"a": {At: now, Up: 1, Down: 2},
		"b": {At: now, Up: 3, Down: 4},
	}
	p.updateGroups(conns, rates, now)

	h := p.ConnectionGroupTrafficHistory("example.com:443")
	if len(h.Points) != 1 {
		t.Fatalf("expected 1 group history point, got %d", len(h.Points))
	}
	if h.Points[0].Up != 4 || h.Points[0].Down != 6 {
		t.Fatalf("expected summed rates 4/6, got %+v", h.Points[0])
	}

	// The group goes inactive: no new samples, history frozen.
	p.updateGroups(nil, nil, now.Add(time.Minute))
	if got := len(p.ConnectionGroupTrafficHistory("example.com:443").Points); got != 1 {
		t.Fatalf("inactive group must not gain samples, got %d", got)
	}

	// Past retention: the group is pruned and its history is dropped.
	p.updateGroups(nil, nil, now.Add(2*time.Hour))
	if got := len(p.ConnectionGroupTrafficHistory("example.com:443").Points); got != 0 {
		t.Fatalf("pruned group must lose its history, got %d", got)
	}
}
