//go:build !nogui

package wails

import "sync"

type notificationRouter struct {
	enabled     func() bool
	system      func(title, body string) error
	inApp       func(title, body string)
	reportError func(error)
}

func (r notificationRouter) notify(title, body string) {
	if !r.enabled() {
		r.inApp(title, body)
		return
	}
	if err := r.system(title, body); err != nil {
		r.reportError(err)
		r.inApp(title, body)
	}
}

type updateNotificationDedup struct {
	mu       sync.Mutex
	versions map[string]string
}

func (d *updateNotificationDedup) allow(target, current, latest string) bool {
	if current == "" || latest == "" {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if current == latest {
		delete(d.versions, target)
		return false
	}
	if d.versions[target] == latest {
		return false
	}
	if d.versions == nil {
		d.versions = make(map[string]string)
	}
	d.versions[target] = latest
	return true
}

func (d *updateNotificationDedup) clear(target string) {
	d.mu.Lock()
	delete(d.versions, target)
	d.mu.Unlock()
}

func (b *Bindings) notifyUpdate(target, current, latest, title, body string) {
	if b.updateNotificationDedup.allow(target, current, latest) {
		b.notify(title, body)
	}
}
