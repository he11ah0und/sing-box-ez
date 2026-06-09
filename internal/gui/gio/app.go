package giogui

import (
	"image/color"
	"os"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/gui/gio/pages"
)

// GUI holds the new Gio-based adaptive UI.
type GUI struct {
	cfg *config.AppConfig

	// Theme
	th *material.Theme

	// Navigation state
	shell *Shell

	// Window reference
	win *app.Window

	// Core controller (business logic)
	ctrl *core.InteractiveController
}

// New creates a new Gio GUI instance.
func New(cfg *config.AppConfig) *GUI {
	th := material.NewTheme()
	// Dark theme by default
	th.Palette.Bg = color.NRGBA{R: 18, G: 18, B: 18, A: 255}
	th.Palette.Fg = color.NRGBA{R: 255, G: 255, B: 255, A: 255}

	g := &GUI{
		cfg: cfg,
		th:  th,
	}

	// Initialize core controller (encapsulates manager + logger + i18n)
	g.ctrl = core.NewInteractiveController(cfg)

	dialog := NewDialog()

	aboutPage := pages.NewAboutPage(th, g.ctrl, dialog.Show, dialog.ShowMarkdown, dialog.ShowLoading)
	mainPage := pages.NewMainPage(th, g.ctrl.Controller)
	logPage := pages.NewLogPage(th, g.ctrl.Controller)

	primary := []pages.Page{mainPage, pages.NewConfigsPage(th, g.ctrl.Controller)}
	secondary := []pages.Page{
		pages.NewCorePage(th, g.ctrl.Controller),
		pages.NewSettingsPage(th, g.ctrl.Controller),
		logPage,
	}
	if cfg.GetPluginsEnabled() {
		secondary = append(secondary, pages.NewPluginsPage(th, g.ctrl.Controller))
	}
	secondary = append(secondary, aboutPage)

	g.shell = NewShell(th, cfg, g.ctrl.Controller, primary, secondary)
	g.shell.dialog = dialog

	return g
}

// Run starts the Gio event loop.
func (g *GUI) Run() {
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("sing-box-ez"),
			app.Size(unit.Dp(800), unit.Dp(600)),
		)
		g.win = w

		var ops op.Ops
		for {
			switch e := w.Event().(type) {
			case app.DestroyEvent:
				if g.ctrl != nil {
					g.ctrl.Close()
				}
				os.Exit(0)
			case app.FrameEvent:
				gtx := app.NewContext(&ops, e)
				g.shell.Layout(gtx)
				e.Frame(gtx.Ops)
			}
		}
	}()
	app.Main()
}
