package tray

import (
	"sync"

	"github.com/gen2brain/beeep"
	"github.com/he11ah0und/localengine"
)

var notificationAppName sync.Once

// SystemNotification delivers a notification through the operating system.
func SystemNotification(title, body string) error {
	notificationAppName.Do(func() {
		beeep.AppName = localengine.T("app", "title")
	})
	return beeep.Notify(title, body, iconPNG)
}
