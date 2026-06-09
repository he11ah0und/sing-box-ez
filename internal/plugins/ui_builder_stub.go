//go:build !fyne && !noplugins

package plugins

// UIBuilder is a no-op stub when the fyne backend is not used.
type UIBuilder struct{}

// NewUIBuilder creates a no-op UI builder (arguments are ignored on non-fyne backends).
func NewUIBuilder(_ any, _ any) *UIBuilder {
	return &UIBuilder{}
}
