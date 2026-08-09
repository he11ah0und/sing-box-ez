//go:build linux && !nogui

package tray

import (
	"os"
	"time"

	"github.com/godbus/dbus/v5"
	systray "github.com/gogpu/systray"
)

func (t *Tray) checkTrayEnvironment() {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.log.TWarnf("gui.tray.dbus_session_address_unset")
		return
	}

	conn, err := dbus.SessionBus()
	if err != nil {
		t.log.TWarnf("gui.tray.dbus_connect_failed", err)
		return
	}

	var names []string
	if err := conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names); err != nil {
		t.log.TWarnf("gui.tray.dbus_list_names_failed", err)
		return
	}

	for _, n := range names {
		if n == "org.kde.StatusNotifierWatcher" {
			t.log.TInfof("gui.tray.status_notifier_present")
			return
		}
	}

	t.log.TWarnf("gui.tray.status_notifier_missing")
}

// refreshTray re-emits the icon/status signals after a short delay. Some tray
// hosts (e.g. Waybar) create the item proxy asynchronously and may miss the
// initial NewIcon/NewStatus signals if they are emitted before the signal
// handler is connected. Re-emitting gives those hosts a second chance to pick
// up the icon without requiring a host restart.
func (t *Tray) refreshTray(tray *systray.SystemTray) {
	go func() {
		time.Sleep(500 * time.Millisecond)
		tray.SetIcon(iconPNG).Show()
		t.log.TDebugf("gui.tray.icon_refreshed")
		time.Sleep(1 * time.Second)
		tray.SetIcon(iconPNG).Show()
		t.log.TDebugf("gui.tray.icon_refreshed_again")
	}()
}
