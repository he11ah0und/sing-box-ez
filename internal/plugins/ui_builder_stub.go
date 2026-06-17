//go:build !noplugins

package plugins

// UIBuilder is a no-op stub; custom plugin UI is not implemented for the Gio backend.
type UIBuilder struct{}

// NewUIBuilder creates a no-op UI builder (arguments are ignored).
func NewUIBuilder(_ any, _ any) *UIBuilder {
	return &UIBuilder{}
}
