package state

import (
	"net"
	"sort"
	"time"

	"sing-box-ez/internal/core/api"
)

// groupKey identifies a connection group: connections to the same target
// (domain:port or destination host:port) are aggregated into one entry.
func groupKey(c api.Connection) string {
	if c.Domain != "" {
		if _, port, err := net.SplitHostPort(c.Destination); err == nil && port != "" {
			return c.Domain + ":" + port
		}
		return c.Domain
	}
	return c.Destination
}

// ipVersionOf returns 4 or 6 for the destination host of a connection, or 0
// when the host is not a parseable IP literal.
func ipVersionOf(c api.Connection) int {
	host := c.Destination
	if h, _, err := net.SplitHostPort(c.Destination); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return 0
	}
	if ip.To4() != nil {
		return 4
	}
	return 6
}

// connGroup is the tracked state of one connection group.
type connGroup struct {
	target    string
	network   string
	firstSeen time.Time
	lastSeen  time.Time
	// members holds the last totals snapshot of each active connection,
	// keyed by connection ID.
	members map[string]connTotals
	// closedUp/closedDown accumulate the final totals of closed members so
	// the group totals survive their disconnect.
	closedUp   int64
	closedDown int64
	// spans records the activity window of every member connection, one
	// track per connection (DevTools/waterfall style). End is zero while
	// the connection is alive.
	spans []ConnSpan
	// ip4/ip6 count the live members per IP version; lastIp4/lastIp6 keep
	// the last non-zero counts so inactive groups still show the versions
	// their members used.
	ip4, ip6         int
	lastIp4, lastIp6 int
}

// connGroupsMax caps how many groups are retained.
const connGroupsMax = 256

// groupSpansMax caps how many connection spans one group retains;
// the oldest are dropped first.
const groupSpansMax = 64

// groupTracker aggregates connections into groups by target and tracks
// their activity intervals. All methods must be called with Poller.mu held.
type groupTracker struct {
	groups map[string]*connGroup
}

func newGroupTracker() *groupTracker {
	return &groupTracker{groups: make(map[string]*connGroup)}
}

// reset drops all tracked groups (the core stopped).
func (t *groupTracker) reset() {
	t.groups = make(map[string]*connGroup)
}

// update folds the current connections snapshot into the tracked groups:
// it opens spans for new member connections, closes spans for members
// that are gone, and prunes inactive groups past the retention.
func (t *groupTracker) update(conns []api.Connection, now time.Time, retention time.Duration) {
	for _, g := range t.groups {
		g.ip4, g.ip6 = 0, 0
	}
	current := make(map[string]api.Connection, len(conns))
	for _, c := range conns {
		current[c.ID] = c
		key := groupKey(c)
		g := t.groups[key]
		if g == nil {
			g = &connGroup{
				target:  key,
				network: c.Network,
				members: make(map[string]connTotals),
				spans:   []ConnSpan{},
			}
			t.groups[key] = g
		}
		if g.firstSeen.IsZero() {
			g.firstSeen = now
		}
		switch ipVersionOf(c) {
		case 4:
			g.ip4++
		case 6:
			g.ip6++
		}
		if _, ok := g.members[c.ID]; !ok {
			// New member: open its activity span, backdated to the
			// connection's creation time when known.
			start := now
			if !c.CreatedAt.IsZero() && c.CreatedAt.Before(now) {
				start = c.CreatedAt
			}
			g.spans = append(g.spans, ConnSpan{ID: c.ID, Start: start})
			if len(g.spans) > groupSpansMax {
				g.spans = g.spans[len(g.spans)-groupSpansMax:]
			}
			g.members[c.ID] = connTotals{}
		}
		g.members[c.ID] = connTotals{up: c.UplinkTotal, down: c.DownlinkTotal, at: now}
		g.lastSeen = now
	}

	for key, g := range t.groups {
		for id, totals := range g.members {
			if _, ok := current[id]; ok {
				continue
			}
			// The member vanished from the snapshot: it closed.
			g.closedUp += totals.up
			g.closedDown += totals.down
			delete(g.members, id)
			for i := range g.spans {
				if g.spans[i].ID == id && g.spans[i].End.IsZero() {
					g.spans[i].End = now
				}
			}
		}
		if len(g.members) == 0 && now.Sub(g.lastSeen) > retention {
			delete(t.groups, key)
			continue
		}
		if g.ip4 > 0 || g.ip6 > 0 {
			g.lastIp4, g.lastIp6 = g.ip4, g.ip6
		}
	}

	for len(t.groups) > connGroupsMax {
		var oldestKey string
		var oldestAt time.Time
		for key, g := range t.groups {
			if len(g.members) > 0 {
				continue
			}
			if oldestKey == "" || g.lastSeen.Before(oldestAt) {
				oldestKey = key
				oldestAt = g.lastSeen
			}
		}
		if oldestKey == "" {
			break
		}
		delete(t.groups, oldestKey)
	}
}

// snapshot returns the tracked groups in UI-facing form, sorted by sortMode
// ("date", "traffic" or "total"); active groups always come first. rates
// carries the per-second rate samples of the current connections.
func (t *groupTracker) snapshot(rates map[string]TrafficPoint, now time.Time, sortMode string) []ConnectionGroup {
	out := make([]ConnectionGroup, 0, len(t.groups))
	for key, g := range t.groups {
		cg := ConnectionGroup{
			Key:       key,
			Target:    g.target,
			Network:   g.network,
			FirstSeen: g.firstSeen,
			LastSeen:  g.lastSeen,
			Active:    len(g.members) > 0,
			ConnIDs:   make([]string, 0, len(g.members)),
			UpTotal:   g.closedUp,
			DownTotal: g.closedDown,
			Spans:     make([]ConnSpan, len(g.spans)),
			IPv4:      g.ip4,
			IPv6:      g.ip6,
		}
		if cg.IPv4 == 0 && cg.IPv6 == 0 {
			cg.IPv4, cg.IPv6 = g.lastIp4, g.lastIp6
		}
		copy(cg.Spans, g.spans)
		for id, totals := range g.members {
			cg.ConnIDs = append(cg.ConnIDs, id)
			cg.UpTotal += totals.up
			cg.DownTotal += totals.down
			if r, ok := rates[id]; ok {
				cg.UpRate += r.Up
				cg.DownRate += r.Down
			}
		}
		sort.Strings(cg.ConnIDs)
		cg.ConnCount = len(cg.ConnIDs)
		out = append(out, cg)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Active != out[j].Active {
			return out[i].Active
		}
		switch sortMode {
		case "traffic":
			if d := (out[j].UpRate + out[j].DownRate) - (out[i].UpRate + out[i].DownRate); d != 0 {
				return d < 0
			}
		case "total":
			if d := (out[j].UpTotal + out[j].DownTotal) - (out[i].UpTotal + out[i].DownTotal); d != 0 {
				return d < 0
			}
		}
		// "date" order (and the tie-break for the other modes): active
		// groups by first appearance, inactive by last activity — stable
		// across polls, with the key as the final deterministic tie-break.
		if out[i].Active {
			if !out[i].FirstSeen.Equal(out[j].FirstSeen) {
				return out[i].FirstSeen.Before(out[j].FirstSeen)
			}
		} else if !out[i].LastSeen.Equal(out[j].LastSeen) {
			return out[i].LastSeen.After(out[j].LastSeen)
		}
		return out[i].Key < out[j].Key
	})
	return out
}
