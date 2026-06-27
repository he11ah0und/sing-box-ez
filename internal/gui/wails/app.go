//go:build !nogui

package wails

import (
	"context"
	"embed"
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"

	apppkg "sing-box-ez/internal/app"
)

// Bindings exposes Go methods that can be called from the frontend.
type Bindings struct {
	ctx context.Context
	app *apppkg.App
}

// ServiceName returns the service name used by Wails.
func (b *Bindings) ServiceName() string {
	return "App"
}

// ServiceStartup is called by Wails when the application starts.
func (b *Bindings) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	b.ctx = ctx
	return nil
}

// Greet is a temporary binding used to verify the Wails bridge.
func (b *Bindings) Greet(name string) string {
	return fmt.Sprintf("Hello %s, Wails v3 is working!", name)
}

// WailsApp is the Wails v3 binding layer for the sing-box-ez GUI.
type WailsApp struct {
	app    *apppkg.App
	assets embed.FS
}

// New creates a new Wails GUI instance.
func New(app *apppkg.App, assets embed.FS) *WailsApp {
	return &WailsApp{
		app:    app,
		assets: assets,
	}
}

// Run starts the Wails event loop.
func (w *WailsApp) Run() error {
	bindings := &Bindings{app: w.app}

	wailsApp := application.New(application.Options{
		Name:        "sing-box-ez",
		Description: "sing-box-ez graphical interface",
		Services: []application.Service{
			application.NewService(bindings),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(w.assets),
		},
	})

	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "sing-box-ez",
		Width:            1024,
		Height:           768,
		BackgroundColour: application.NewRGB(15, 23, 42),
		URL:              "/",
	})

	return wailsApp.Run()
}
