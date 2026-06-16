//go:build !nogui

// Package tray provides a small cross-platform system tray wrapper around
// github.com/gogpu/systray for the Gio GUI build.
package tray

import (
	_ "embed"
	"sync"

	"sing-box-ez/internal/framework/logger"

	systray "github.com/gogpu/systray"
)

//go:embed assets/icon.png
var iconPNG []byte

// Tray wraps a platform system tray icon and exposes show/minimize/quit
// callbacks to the host GUI.
type Tray struct {
	mu      sync.Mutex
	tray    *systray.SystemTray
	started bool
	done    chan struct{}

	log *logger.LogTerminal

	show     func()
	minimize func()
	quit     func()
}

// New creates a tray icon controller. The supplied callbacks are invoked from
// the tray's internal goroutine and must be safe to call concurrently.
// The parent logger is used to allocate a "tray" sub-terminal.
func New(parent *logger.LogTerminal, show, minimize, quit func()) *Tray {
	return &Tray{
		log:      parent.Allocate("tray"),
		show:     show,
		minimize: minimize,
		quit:     quit,
	}
}

// Start creates the tray icon and runs its message loop in a background
// goroutine. It is safe to call multiple times; subsequent calls are no-ops.
func (t *Tray) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.started {
		return nil
	}
	t.started = true
	t.done = make(chan struct{})

	go t.run()
	return nil
}

// Stop removes the tray icon and waits for the message loop to exit.
func (t *Tray) Stop() {
	t.mu.Lock()
	started := t.started
	t.started = false
	tray := t.tray
	t.mu.Unlock()

	if !started {
		return
	}
	if tray != nil {
		tray.Remove()
	}
	waitForDone(t)
}

func (t *Tray) run() {
	defer close(t.done)

	t.checkTrayEnvironment()

	tray := systray.New()
	if tray == nil {
		t.log.Warnf("failed to create system tray icon")
		return
	}

	t.mu.Lock()
	t.tray = tray
	t.mu.Unlock()

	menu := systray.NewMenu()
	menu.Add("Show", t.show).
		Add("Minimize", t.minimize).
		AddSeparator().
		Add("Quit", t.quit)

	tray.SetIcon(iconPNG).
		SetTooltip("sing-box-ez").
		OnClick(t.show).
		OnDoubleClick(t.toggle).
		SetMenu(menu).
		Show()

	t.log.Infof("icon shown")
	t.refreshTray(tray)

	_ = tray.Run()
	t.log.Infof("message loop exited")
}

func (t *Tray) toggle() {
	// A simple double-click toggle is not reliably supported by all platforms,
	// so fall back to showing the window.
	t.show()
}
