package core

import (
	"testing"
	"time"

	"sing-box-ez/internal/config"
)

func autoUpdateSchedulerRecord(name string, lastUpdate time.Time, intervalHours int) config.ConfigRecord {
	return config.ConfigRecord{
		Name:                name,
		URL:                 "https://example.com/" + name,
		Type:                "remote",
		UpdateIntervalHours: intervalHours,
		LastUpdate:          config.Timestamp{Time: lastUpdate},
	}
}

func TestNextConfigUpdateDelayAfterSuccessJustAfterNominalTick(t *testing.T) {
	now := time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)
	rec := autoUpdateSchedulerRecord("remote", now.Add(-2*time.Hour+time.Minute), 2)

	got := nextConfigUpdateDelay(now, time.Hour, true, true, []config.ConfigRecord{rec}, nil)
	want := time.Minute + configUpdateWakeEpsilon
	if got != want {
		t.Fatalf("delay = %v, want NextUpdate + epsilon = %v", got, want)
	}
}

func TestNextConfigUpdateDelayUsesEarliestProfile(t *testing.T) {
	now := time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)
	later := autoUpdateSchedulerRecord("later", now, 2)
	earlier := autoUpdateSchedulerRecord("earlier", now.Add(-30*time.Minute), 1)

	got := nextConfigUpdateDelay(now, 3*time.Hour, true, true, []config.ConfigRecord{later, earlier}, nil)
	want := 30*time.Minute + configUpdateWakeEpsilon
	if got != want {
		t.Fatalf("delay = %v, want earliest NextUpdate + epsilon = %v", got, want)
	}
}

func TestNextConfigUpdateDelayDueNowAndFailedBackoff(t *testing.T) {
	now := time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)
	due := autoUpdateSchedulerRecord("due", now.Add(-3*time.Hour), 2)

	t.Run("newly due", func(t *testing.T) {
		got := nextConfigUpdateDelay(now, time.Hour, true, true, []config.ConfigRecord{due}, nil)
		if got != configUpdateWakeEpsilon {
			t.Fatalf("delay = %v, want prompt wake in %v", got, configUpdateWakeEpsilon)
		}
	})

	t.Run("just failed", func(t *testing.T) {
		attempts := map[string]configUpdateAttempt{
			due.Name: {lastUpdate: due.LastUpdate, attemptedAt: now},
		}
		got := nextConfigUpdateDelay(now, time.Hour, true, true, []config.ConfigRecord{due}, attempts)
		if got != time.Hour {
			t.Fatalf("delay = %v, want failure backoff %v", got, time.Hour)
		}
		retryAt, ok := configUpdateRetryAt(&due, attempts, time.Hour)
		if !ok || !retryAt.Equal(now.Add(time.Hour)) {
			t.Fatalf("retryAt = %v, ok = %v; want %v, true", retryAt, ok, now.Add(time.Hour))
		}
	})

	t.Run("zero timestamp just failed", func(t *testing.T) {
		neverUpdated := autoUpdateSchedulerRecord("new", time.Time{}, 2)
		attempts := map[string]configUpdateAttempt{
			neverUpdated.Name: {lastUpdate: neverUpdated.LastUpdate, attemptedAt: now},
		}
		got := nextConfigUpdateDelay(now, time.Hour, true, true, []config.ConfigRecord{neverUpdated}, attempts)
		if got != time.Hour {
			t.Fatalf("delay = %v, want failure backoff %v", got, time.Hour)
		}
	})

	t.Run("success advances timestamp", func(t *testing.T) {
		previous := autoUpdateSchedulerRecord("remote", now.Add(-3*time.Hour), 2)
		refreshed := autoUpdateSchedulerRecord("remote", now, 2)
		attempts := map[string]configUpdateAttempt{
			previous.Name: {lastUpdate: previous.LastUpdate, attemptedAt: now.Add(-time.Minute)},
		}
		got := nextConfigUpdateDelay(now, 3*time.Hour, true, true, []config.ConfigRecord{refreshed}, attempts)
		want := 2*time.Hour + configUpdateWakeEpsilon
		if got != want {
			t.Fatalf("delay = %v, want refreshed NextUpdate + epsilon = %v", got, want)
		}
	})
}

func TestNextConfigUpdateDelayStoppedCoreUsesGlobalInterval(t *testing.T) {
	now := time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)
	due := autoUpdateSchedulerRecord("due", time.Time{}, 2)

	got := nextConfigUpdateDelay(now, time.Hour, false, true, []config.ConfigRecord{due}, nil)
	if got != time.Hour {
		t.Fatalf("delay = %v, want stopped-core fallback %v", got, time.Hour)
	}
}

func TestNextConfigUpdateDelaySkipsLocalAndDisabledAndHandlesZeroTimestamp(t *testing.T) {
	now := time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)
	local := autoUpdateSchedulerRecord("local", time.Time{}, 2)
	local.Type = config.ConfigTypeLocal
	local.URL = ""
	disabled := autoUpdateSchedulerRecord("disabled", time.Time{}, 2)
	autoUpdate := false
	disabled.AutoUpdate = &autoUpdate

	got := nextConfigUpdateDelay(now, time.Hour, true, true, []config.ConfigRecord{local, disabled}, nil)
	if got != time.Hour {
		t.Fatalf("delay with only local/disabled profiles = %v, want %v", got, time.Hour)
	}

	neverUpdated := autoUpdateSchedulerRecord("new", time.Time{}, 2)
	got = nextConfigUpdateDelay(now, time.Hour, true, true, []config.ConfigRecord{local, disabled, neverUpdated}, nil)
	if got != configUpdateWakeEpsilon {
		t.Fatalf("delay with zero LastUpdate = %v, want prompt wake in %v", got, configUpdateWakeEpsilon)
	}
}

func TestNextConfigUpdateDelayHashOnlyUsesGlobalInterval(t *testing.T) {
	now := time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)
	due := autoUpdateSchedulerRecord("due", time.Time{}, 2)

	got := nextConfigUpdateDelay(now, time.Hour, true, false, []config.ConfigRecord{due}, nil)
	if got != time.Hour {
		t.Fatalf("delay = %v, want hash-only fallback %v", got, time.Hour)
	}
}

func TestNextConfigUpdateDelayCapAndInvalidInterval(t *testing.T) {
	now := time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)
	rec := autoUpdateSchedulerRecord("remote", now, 2)

	tests := []struct {
		name     string
		interval time.Duration
		configs  []config.ConfigRecord
		want     time.Duration
	}{
		{name: "global interval caps next update", interval: time.Hour, configs: []config.ConfigRecord{rec}, want: time.Hour},
		{name: "zero interval falls back", interval: 0, want: time.Hour},
		{name: "negative interval falls back", interval: -time.Hour, want: time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextConfigUpdateDelay(now, tt.interval, true, true, tt.configs, nil)
			if got != tt.want {
				t.Fatalf("delay = %v, want %v", got, tt.want)
			}
		})
	}
}
