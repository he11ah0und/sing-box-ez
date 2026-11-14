//go:build !nogui

package wails

import (
	"errors"
	"slices"
	"testing"
)

func TestNotificationRouter(t *testing.T) {
	systemErr := errors.New("system delivery failed")
	tests := []struct {
		name      string
		enabled   bool
		systemErr error
		events    int
		wantCalls []string
	}{
		{
			name:      "disabled uses in-app only",
			events:    1,
			wantCalls: []string{"in-app:Title|Body"},
		},
		{
			name:      "enabled uses system only on success",
			enabled:   true,
			events:    1,
			wantCalls: []string{"system:Title|Body"},
		},
		{
			name:      "system failure reports once and falls back once",
			enabled:   true,
			systemErr: systemErr,
			events:    1,
			wantCalls: []string{"system:Title|Body", "error:system delivery failed", "in-app:Title|Body"},
		},
		{
			name:      "repeated events route independently",
			enabled:   true,
			systemErr: systemErr,
			events:    2,
			wantCalls: []string{
				"system:Title|Body", "error:system delivery failed", "in-app:Title|Body",
				"system:Title|Body", "error:system delivery failed", "in-app:Title|Body",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			router := notificationRouter{
				enabled: func() bool { return tt.enabled },
				system: func(title, body string) error {
					calls = append(calls, "system:"+title+"|"+body)
					return tt.systemErr
				},
				inApp: func(title, body string) {
					calls = append(calls, "in-app:"+title+"|"+body)
				},
				reportError: func(err error) {
					calls = append(calls, "error:"+err.Error())
				},
			}

			for range tt.events {
				router.notify("Title", "Body")
			}

			if !slices.Equal(calls, tt.wantCalls) {
				t.Fatalf("calls = %v, want %v", calls, tt.wantCalls)
			}
		})
	}
}

func TestUpdateNotificationDedup(t *testing.T) {
	var calls []string
	bindings := &Bindings{notifications: notificationRouter{
		enabled: func() bool { return false },
		inApp: func(title, body string) {
			calls = append(calls, title+"|"+body)
		},
	}}
	notify := func(target, current, latest string) {
		bindings.notifyUpdate(target, current, latest, target+" "+latest, "body")
	}

	notify("app", "v1", "v2")
	notify("app", "v1", "v2")
	notify("core", "v1", "v2")
	notify("app", "v1", "v3")
	notify("app", "v1", "")
	notify("app", "", "v4")
	notify("core", "v2", "v2")
	notify("core", "v1", "v2")

	want := []string{"app v2|body", "core v2|body", "app v3|body", "core v2|body"}
	if !slices.Equal(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}
