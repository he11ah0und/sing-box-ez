package giogui

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	apppkg "sing-box-ez/internal/app"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/framework/logger"
	"sing-box-ez/internal/framework/rpc"
	"sing-box-ez/internal/framework/svcman"
	"sing-box-ez/internal/framework/updater"
	"sing-box-ez/internal/framework/version"
	"sing-box-ez/internal/gui/gio/pages"
	"sing-box-ez/internal/gui/gio/startup"
	"sing-box-ez/internal/gui/gio/theme"
	"sing-box-ez/internal/gui/gio/widgets"
	"sing-box-ez/internal/gui/tray"

	"gio.tools/icons"
	gioapp "gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
)

type startupServiceActionResult struct {
	optID  string
	online bool
	err    error
}

// GUI holds the new Gio-based adaptive UI.
type GUI struct {
	app *apppkg.App
	cfg *config.AppConfig

	// Theme
	th *material.Theme

	// Navigation state
	shell *Shell

	// Window reference
	win *gioapp.Window

	// Logger terminal for the GUI subsystem.
	log *logger.LogTerminal

	// System tray integration
	tray *tray.Tray

	// Interactive controller (GUI adapter around the core backend)
	ctrl *core.InteractiveController

	// Operating mode: "embed", "service", or "remote".
	mode string

	// serviceManager holds the selected system service manager (service mode).
	serviceManager svcman.Manager

	// remoteAddr holds the TCP address for remote mode.
	remoteAddr string

	// Dialog reference for startup sequences
	dialog *Dialog

	// startupDone is closed when the user has chosen the startup mode.
	startupDone chan struct{}

	// Page references used for programmatic navigation/highlighting.
	corePage    *pages.CorePage
	configsPage *pages.ConfigsPage

	// Secondary page references so the list can be rebuilt at runtime.
	settingsPage *pages.SettingsPage
	logPage      pages.Page
	pluginsPage  pages.Page
	aboutPage    pages.Page

	// restart is set to true when the app should be restarted after the GUI loop exits.
	restart bool
}

// New creates a new Gio GUI instance.
func New(app *apppkg.App) *GUI {
	cfg := app.Config.(*config.AppConfig)
	th := material.NewTheme()

	if err := theme.Init(th, cfg.DataDir, apppkg.ThemesFS); err != nil {
		// Log and continue with the material defaults if theme loading fails.
		app.Logger.Root.Warnf("failed to load themes: %v", err)
	} else {
		themeName := cfg.MustGet("ui", "theme").String()
		if themeName == "" {
			themeName = "default"
			_ = cfg.MustGet("ui", "theme").Update(themeName)
		}
		mode := theme.Mode(cfg.MustGet("ui", "theme_mode").String())
		if mode == "" {
			mode = theme.ModeSystem
			_ = cfg.MustGet("ui", "theme_mode").Update(string(mode))
		}
		if err := theme.M.Apply(themeName, mode); err != nil {
			app.Logger.Root.Warnf("failed to apply theme %q: %v", themeName, err)
		} else {
			_ = cfg.Save()
		}
	}

	g := &GUI{
		app:         app,
		cfg:         cfg,
		th:          th,
		log:         app.Logger.Root.Allocate("gui"),
		mode:        "embed",
		startupDone: make(chan struct{}),
	}
	g.log.Infof("initialized")

	// Build a font collection with Go font as base + optional emoji support.
	collection := gofont.Collection()
	if emoji := g.tryLoadEmojiFont(); len(emoji) > 0 {
		collection = append(collection, emoji...)
	}
	th.Shaper = text.NewShaper(text.WithCollection(collection))

	dialog := NewDialog()
	g.dialog = dialog

	return g
}

// buildSecondaryPages assembles the secondary navigation list based on the
// current settings (log visibility and plugin support).
func (g *GUI) buildSecondaryPages(showLogs bool) []pages.Page {
	secondary := []pages.Page{
		g.corePage,
		g.settingsPage,
	}
	if showLogs {
		secondary = append(secondary, g.logPage)
	}
	if g.cfg.MustGet("plugins", "enabled").Bool() {
		secondary = append(secondary, g.pluginsPage)
	}
	secondary = append(secondary, g.aboutPage)
	return secondary
}

// rebuildSecondaryPages updates the secondary navigation in real time.
func (g *GUI) rebuildSecondaryPages(showLogs bool) {
	g.shell.SetSecondaryPages(g.buildSecondaryPages(showLogs))
}

// runWithDialogPump runs f and pumps window events until f calls the provided
// done callback. It only renders the dialog overlay, so it can be used before
// the main UI is built (e.g. for pre-startup update checks).
func (g *GUI) runWithDialogPump(w *gioapp.Window, f func(done func())) {
	doneCh := make(chan struct{})
	f(func() { close(doneCh) })

	// Ensure the first frame is rendered even if no user input occurs.
	w.Invalidate()

	var ops op.Ops
	for {
		select {
		case <-doneCh:
			return
		default:
		}

		switch e := w.Event().(type) {
		case gioapp.DestroyEvent:
			g.log.Infof("destroy event received during dialog pump, shutting down")
			os.Exit(0)
		case gioapp.FrameEvent:
			gtx := gioapp.NewContext(&ops, e)
			paint.Fill(gtx.Ops, theme.Current().Colors().Bg)
			g.dialog.Layout(gtx, g.th)
			e.Frame(gtx.Ops)
			w.Invalidate()
		}
	}
}

// Run starts the Gio event loop.
func (g *GUI) Run() {
	g.log.Infof("starting event loop")
	go func() {
		w := new(gioapp.Window)
		windowTitle := localengine.T("app", "title")
		w.Option(
			gioapp.Title(windowTitle),
			gioapp.Size(unit.Dp(800), unit.Dp(600)),
		)
		mainWindowTitle = windowTitle
		g.win = w

		// Run startup update checks before the mode selection dialog so the user
		// sees available updates first. The dialog pump renders loading and
		// confirmation dialogs while the checks run asynchronously.
		g.runWithDialogPump(w, func(pumpDone func()) {
			g.runStartupUpdateChecks(pumpDone)
		})

		if g.cfg.MustGet("remote", "remember_connection_mode").Bool() {
			g.restoreStartupMode()
			close(g.startupDone)
		} else {
			g.showStartupDialog(w)
		}

		g.buildModeUI(w)

		var ops op.Ops
		for {
			switch e := w.Event().(type) {
			case gioapp.DestroyEvent:
				g.log.Infof("destroy event received, shutting down")
				if g.tray != nil {
					g.tray.Stop()
				}
				if g.ctrl != nil {
					g.ctrl.Close()
				}
				if g.restart {
					return
				}
				os.Exit(0)
			case gioapp.FrameEvent:
				gtx := gioapp.NewContext(&ops, e)
				g.shell.Layout(gtx)
				e.Frame(gtx.Ops)
			}
		}
	}()

	g.log.Infof("entering Gio main loop")
	gioapp.Main()

	if g.restart {
		g.doRestart()
	}
}

func (g *GUI) showStartupDialog(w *gioapp.Window) {
	options, updates := startup.DiscoverAsync(g.cfg)
	selected := g.startupOptionIndex(options, g.cfg.MustGet("remote", "last_connection_mode").String())
	managers := startup.ServiceManagers(options)

	go func() {
		for opt := range updates {
			for i := range options {
				if options[i].ID == opt.ID {
					options[i].Online = opt.Online
					break
				}
			}
			w.Invalidate()
		}
	}()

	var ops op.Ops
	var dropdownBtn widget.Clickable
	var continueBtn widget.Clickable
	var rememberCh widget.Bool
	var serviceActionBtn widget.Clickable
	var remoteAddr string
	if g.cfg.MustGet("remote", "last_tcp_address").String() != "" {
		remoteAddr = g.cfg.MustGet("remote", "last_tcp_address").String()
	}

	var actionMu sync.Mutex
	var actionPending bool
	var actionResult startupServiceActionResult

	for {
		switch e := w.Event().(type) {
		case gioapp.DestroyEvent:
			os.Exit(0)
		case gioapp.FrameEvent:
			gtx := gioapp.NewContext(&ops, e)

			if g.handleStartupFrameEvents(gtx, options, &selected, &remoteAddr, &rememberCh, &dropdownBtn, &continueBtn, &serviceActionBtn, &actionMu, &actionPending, &actionResult, managers, w) {
				return
			}

			colors := theme.Current().Colors()
			paint.Fill(gtx.Ops, colors.Bg)

			g.layoutStartupCard(options, selected, &remoteAddr, &rememberCh, &dropdownBtn, &continueBtn, &serviceActionBtn, actionPending, colors)(gtx)

			g.dialog.Layout(gtx, g.th)

			e.Frame(gtx.Ops)
		}
	}
}

func (g *GUI) handleStartupFrameEvents(gtx layout.Context, options []startup.Option, selected *int, remoteAddr *string, rememberCh *widget.Bool, dropdownBtn, continueBtn, serviceActionBtn *widget.Clickable, actionMu *sync.Mutex, actionPending *bool, actionResult *startupServiceActionResult, managers map[string]svcman.Manager, w *gioapp.Window) bool {
	if dropdownBtn.Clicked(gtx) && !g.dialog.Visible() {
		g.showStartupOptionsDialog(options, selected)
	}
	if continueBtn.Clicked(gtx) {
		opt := options[*selected]
		if opt.Kind == startup.KindRemote {
			opt.Address = strings.TrimSpace(*remoteAddr)
		}
		g.applyStartupChoice(opt, rememberCh.Value, managers)
		close(g.startupDone)
		return true
	}

	actionMu.Lock()
	if *actionPending {
		*actionPending = false
		for i := range options {
			if options[i].ID == actionResult.optID {
				options[i].Online = actionResult.online
			}
		}
		if actionResult.err != nil {
			g.dialog.Show("Error", actionResult.err.Error())
		}
	}
	actionMu.Unlock()

	if serviceActionBtn.Clicked(gtx) {
		go g.handleStartupServiceAction(&options[*selected], actionMu, actionPending, actionResult, w)
	}
	return false
}

func (g *GUI) layoutStartupCard(options []startup.Option, selected int, remoteAddr *string, rememberCh *widget.Bool, dropdownBtn, continueBtn, serviceActionBtn *widget.Clickable, actionPending bool, colors theme.Palette) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			maxWidth := gtx.Dp(unit.Dp(420))
			if gtx.Constraints.Max.X < maxWidth {
				maxWidth = gtx.Constraints.Max.X - gtx.Dp(unit.Dp(32))
			}
			cardGtx := gtx
			cardGtx.Constraints.Min.X = 0
			cardGtx.Constraints.Max.X = maxWidth
			return component.Surface(g.th).Layout(cardGtx, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					children := []layout.FlexChild{
						layout.Rigid(material.H5(g.th, localengine.T("startup", "title")).Layout),
						layout.Rigid(layout.Spacer{Height: 16}.Layout),
						layout.Rigid(material.Body1(g.th, localengine.T("startup", "subtitle")).Layout),
						layout.Rigid(layout.Spacer{Height: 24}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return g.layoutStartupDropdownTrigger(gtx, options, selected, dropdownBtn, colors)
						}),
					}
					if options[selected].Kind == startup.KindRemote {
						children = append(children,
							layout.Rigid(layout.Spacer{Height: 16}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return g.layoutRemoteAddressInput(gtx, remoteAddr, colors)
							}),
						)
					}
					if options[selected].Kind == startup.KindService {
						children = append(children,
							layout.Rigid(layout.Spacer{Height: 16}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return g.layoutStartupServiceAction(gtx, &options[selected], serviceActionBtn, actionPending, colors)
							}),
						)
					}
					children = append(children,
						layout.Rigid(layout.Spacer{Height: 16}.Layout),
						layout.Rigid(material.CheckBox(g.th, rememberCh, localengine.T("startup", "remember")).Layout),
						layout.Rigid(layout.Spacer{Height: 24}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return material.Button(g.th, continueBtn, localengine.T("startup", "continue")).Layout(gtx)
						}),
					)
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, children...)
				})
			})
		})
	}
}

func (g *GUI) showStartupOptionsDialog(options []startup.Option, selected *int) {
	btns := make([]widget.Clickable, len(options))
	g.dialog.ShowCustomNoCancel(localengine.T("startup", "subtitle"), func(gtx layout.Context) layout.Dimensions {
		colors := theme.Current().Colors()
		for i := range options {
			if btns[i].Clicked(gtx) {
				*selected = i
				g.dialog.HideCustom()
			}
		}

		children := make([]layout.FlexChild, len(options))
		for i := range options {
			idx := i
			children[i] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return btns[idx].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					bg := colors.Surface
					if idx == *selected {
						bg = colors.SurfaceVariant
					}
					if btns[idx].Hovered() || btns[idx].Pressed() {
						bg = colors.Hover
					}
					defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
					paint.Fill(gtx.Ops, bg)
					return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Body2(g.th, g.formatStartupOption(&options[idx])).Layout(gtx)
					})
				})
			})
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (g *GUI) startupOptionIndex(options []startup.Option, id string) int {
	for i, opt := range options {
		if opt.ID == id {
			return i
		}
	}
	return 0
}

func (g *GUI) restoreStartupMode() {
	mode := g.cfg.MustGet("remote", "last_connection_mode").String()
	options := startup.Discover(g.cfg)
	managers := startup.ServiceManagers(options)
	for _, opt := range options {
		if opt.ID != mode {
			continue
		}
		g.applyStartupChoice(opt, true, managers)
		return
	}
	g.mode = "embed"
}

func (g *GUI) applyStartupChoice(opt startup.Option, remember bool, managers map[string]svcman.Manager) {
	switch opt.Kind {
	case startup.KindEmbed:
		g.mode = "embed"
		g.serviceManager = nil
	case startup.KindService:
		g.mode = "service"
		if m, ok := managers[opt.ID]; ok {
			g.serviceManager = m
		}
	case startup.KindRemote:
		g.mode = "remote"
		g.serviceManager = nil
		if opt.Address != "" {
			g.remoteAddr = opt.Address
			_ = g.cfg.MustGet("remote", "last_tcp_address").Update(opt.Address)
		}
	}
	_ = g.cfg.MustGet("remote", "remember_connection_mode").Update(remember)
	_ = g.cfg.MustGet("remote", "last_connection_mode").Update(opt.ID)
	_ = g.cfg.Save()
}

func (g *GUI) layoutStartupDropdownTrigger(gtx layout.Context, options []startup.Option, selected int, dropdownBtn *widget.Clickable, colors theme.Palette) layout.Dimensions {
	label := g.formatStartupOption(&options[selected])
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return material.Clickable(gtx, dropdownBtn, func(gtx layout.Context) layout.Dimensions {
		return widgets.BorderedCard(gtx, colors.InputBorder, colors.InputBg, unit.Dp(1), unit.Dp(4), unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return material.Body1(g.th, label).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return icons.NavigationArrowDropDown.Layout(gtx, colors.Fg)
				}),
			)
		})
	})
}

func (g *GUI) layoutRemoteAddressInput(gtx layout.Context, addr *string, colors theme.Palette) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	tv := &widget.Editor{}
	tv.SetText(*addr)
	tv.SingleLine = true
	// Update the backing string when the editor changes.
	*addr = tv.Text()
	return widgets.BorderedCard(gtx, colors.InputBorder, colors.InputBg, unit.Dp(1), unit.Dp(4), unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
		return material.Editor(g.th, tv, localengine.T("startup", "remote_address_placeholder")).Layout(gtx)
	})
}

func (g *GUI) layoutStartupServiceAction(gtx layout.Context, opt *startup.Option, btn *widget.Clickable, pending bool, colors theme.Palette) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(g.th, g.formatStartupOption(opt)).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if pending {
				gtx.Constraints.Min.X = gtx.Dp(48)
				gtx.Constraints.Min.Y = gtx.Dp(48)
				return material.Loader(g.th).Layout(gtx)
			}
			label := localengine.T("startup", "service_start")
			if opt.Online {
				label = localengine.T("startup", "service_stop")
			}
			return material.Button(g.th, btn, label).Layout(gtx)
		}),
	)
}

func (g *GUI) handleStartupServiceAction(opt *startup.Option, actionMu *sync.Mutex, actionPending *bool, actionResult *startupServiceActionResult, w *gioapp.Window) {
	m := opt.Manager
	if m == nil {
		return
	}
	var err error
	if opt.Online {
		err = m.Stop()
	} else {
		err = m.Start()
	}
	online := opt.Online
	if err == nil {
		if st, e := m.Status(); e == nil && st == svcman.StatusRunning {
			online = true
		} else {
			online = false
		}
	}
	actionMu.Lock()
	*actionPending = true
	*actionResult = startupServiceActionResult{optID: opt.ID, online: online, err: err}
	actionMu.Unlock()
	w.Invalidate()
}

func (g *GUI) formatStartupOption(opt *startup.Option) string {
	statusKey := "status_online"
	if !opt.Online {
		statusKey = "status_offline"
	}
	status := localengine.T("startup", statusKey)

	switch opt.Type {
	case "embed":
		return localengine.T("startup", "mode_embed") + " (" + status + ")"
	case "remote":
		return localengine.T("startup", "mode_remote") + " (" + status + ")"
	default:
		name := opt.Type
		if n := localengine.T("startup", "mode_"+opt.Type); n != "" && n != "mode_"+opt.Type {
			name = n
		}
		return name + " (" + status + ")"
	}
}

func (g *GUI) buildModeUI(w *gioapp.Window) {
	switch g.mode {
	case "service":
		if g.serviceManager == nil {
			g.log.Warnf("no service manager selected; falling back to embed")
			g.buildEmbedUI(w)
			return
		}
		g.buildServiceUI(w)
	case "remote":
		g.buildRemoteUI(w)
	default:
		g.buildEmbedUI(w)
	}
}

func (g *GUI) buildEmbedUI(w *gioapp.Window) {
	g.ctrl = core.NewInteractiveController(g.app.Controller)
	g.finishBuildUI(w)
}

func (g *GUI) buildServiceUI(w *gioapp.Window) {
	g.ctrl = core.NewInteractiveControllerWithManager(g.app.Controller, g.serviceManager)
	g.finishBuildUI(w)
}

func (g *GUI) buildRemoteUI(w *gioapp.Window) {
	addr := g.remoteAddr
	if addr == "" {
		addr = g.cfg.MustGet("remote", "last_tcp_address").String()
	}

	var backend core.Backend
	if addr != "" {
		transport, err := rpc.ParseAddress(addr)
		if err != nil {
			g.log.Warnf("invalid remote address %q: %v", addr, err)
		} else {
			backend = core.NewRemoteController(rpc.NewRemoteBackend(transport), g.cfg, g.app.Logger.Root.Allocate("remote"))
		}
	}
	if backend == nil {
		// Fall back to the local controller so the UI still loads; operations will
		// behave like local embed mode if no remote address was supplied.
		backend = g.app.Controller
	}

	g.ctrl = core.NewInteractiveController(backend)
	g.finishBuildUI(w)
}

func (g *GUI) finishBuildUI(w *gioapp.Window) {
	g.ctrl.OnStatusChange = func(running bool) {
		if g.tray != nil {
			g.tray.Refresh()
		}
	}
	g.ctrl.OnUpdateCheckDue = func() {
		g.runStartupUpdateChecks(nil)
	}
	g.ctrl.OnCoreMissing = func() {
		g.shell.NavigateTo("core")
		g.corePage.HighlightUpdateBlock()
	}
	g.ctrl.OnConfigMissing = func() {
		g.shell.NavigateTo("configs")
		g.configsPage.ShowAddDialog()
	}

	g.aboutPage = pages.NewAboutPage(g.th, g.ctrl, g.dialog)
	mainPage := pages.NewMainPage(g.th, g.ctrl, g.dialog)
	g.logPage = pages.NewLogPage(g.th, g.ctrl.Backend())

	g.settingsPage = pages.NewSettingsPage(g.th, g.ctrl, g.dialog)
	g.settingsPage.OnLanguageChange = func() {
		g.shell.RebuildNav()
	}
	g.settingsPage.OnResetRequested = func() {
		g.showResetConfirm()
	}
	g.settingsPage.OnShowLogsChange = func(show bool) {
		g.rebuildSecondaryPages(show)
	}

	g.configsPage = pages.NewConfigsPage(g.th, g.ctrl, g.dialog)
	g.corePage = pages.NewCorePage(g.th, g.ctrl, g.dialog)
	g.pluginsPage = pages.NewPluginsPage(g.th, g.ctrl.Backend())

	primary := []pages.Page{mainPage, g.configsPage}
	secondary := g.buildSecondaryPages(g.cfg.MustGet("ui", "show_logs").Bool())

	g.shell = NewShell(g.th, g.cfg, primary, secondary)
	g.shell.dialog = g.dialog

	g.tray = tray.New(
		g.log,
		func() {
			g.log.Debugf("tray: show requested")
			if err := showMainWindow(w); err != nil {
				g.log.Warnf("tray show failed: %v", err)
			}
		},
		func() {
			g.log.Debugf("tray: minimize requested")
			if err := hideMainWindow(w); err != nil {
				g.log.Warnf("tray hide failed: %v", err)
			}
		},
		func() {
			g.log.Debugf("tray: quit requested")
			w.Perform(system.ActionClose)
		},
		func() bool { return g.ctrl.Backend().IsRunning() },
		func() { go g.ctrl.StartService() },
		func() { go g.ctrl.StopService() },
	)
	if err := g.tray.Start(); err != nil {
		g.log.Warnf("failed to start tray: %v", err)
	} else {
		g.log.Infof("tray started")
		g.tray.Refresh()
	}

}

// RequestRestart stops the current app services and replaces the process with
// a fresh instance. It does not return on success.
func (g *GUI) RequestRestart() {
	g.restart = true
	if g.tray != nil {
		g.tray.Stop()
	}
	if g.ctrl != nil {
		g.ctrl.Close()
	}
	g.doRestart()
}

// doRestart replaces the current process with a fresh instance of the app.
func (g *GUI) doRestart() {
	exe, err := os.Executable()
	if err != nil {
		g.log.Errorf("failed to locate executable for restart: %v", err)
		return
	}
	g.log.Infof("restarting application")
	if err := restartProcess(exe, os.Args, restartEnv()); err != nil {
		g.log.Errorf("failed to restart application: %v", err)
	}
}

// showResetConfirm asks the user to confirm deleting all local data.
func (g *GUI) showResetConfirm() {
	title := localengine.T("settings", "reset", "confirm_title")
	body := localengine.T("settings", "reset", "confirm_msg")
	g.dialog.ShowConfirm(title, body, func() {
		g.resetDataAndRestart()
	}, nil)
}

// resetDataAndRestart removes config.yaml, profiles.yaml and the configs folder,
// shows a confirmation dialog, and then requests a restart.
func (g *GUI) resetDataAndRestart() {
	dataDir := g.cfg.DataDir
	paths := []string{
		filepath.Join(dataDir, "config.yaml"),
		filepath.Join(dataDir, "profiles.yaml"),
		filepath.Join(dataDir, "configs"),
	}
	var errs []string
	for _, p := range paths {
		if err := os.RemoveAll(p); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p, err))
		}
	}
	if len(errs) > 0 {
		g.dialog.Show(localengine.T("settings", "reset", "title"), strings.Join(errs, "\n"))
		return
	}

	successTitle := localengine.T("settings", "reset", "success_title")
	successBody := localengine.T("settings", "reset", "success_msg")
	g.dialog.ShowConfirm(successTitle, successBody, func() {
		g.RequestRestart()
	}, func() {
		g.RequestRestart()
	})
}

//go:embed assets/NotoEmoji.ttf
var notoEmojiFontData []byte

func (g *GUI) tryLoadEmojiFont() []font.FontFace {
	face, err := opentype.Parse(notoEmojiFontData)
	if err != nil {
		g.log.Warnf("failed to parse emoji font: %v", err)
		return nil
	}
	return []font.FontFace{{Font: face.Font(), Face: face}}
}

func (g *GUI) runStartupUpdateChecks(done func()) {
	g.checkSelfUpdateAtStartup(func() {
		g.checkCoreUpdateAtStartup(done)
	})
}

func (g *GUI) checkSelfUpdateAtStartup(done func()) {
	if !g.cfg.MustGet("updates", "auto_check_self").Bool() {
		if done != nil {
			done()
		}
		return
	}

	g.dialog.ShowLoading(localengine.T("about", "update", "checking"))
	go func() {
		info, err := updater.CheckUpdate(version.Branch)
		g.dialog.HideLoading()
		if err != nil {
			g.log.Warnf("startup self-update check failed: %v", err)
			if done != nil {
				done()
			}
			return
		}

		hasUpdate, isDevBuild := g.startupUpdateStatus(info)
		if !hasUpdate && !isDevBuild {
			if done != nil {
				done()
			}
			return
		}

		body := g.selfUpdateBody(info)
		onUpdate := func() { g.runSelfUpdateAtStartup(info, done) }
		onDismiss := func() {
			if done != nil {
				done()
			}
		}
		title := localengine.T("dialog", "self_update", "title")
		if isDevBuild {
			title = localengine.T("about", "update", "dev_build_confirm_title")
		}
		g.dialog.ShowConfirmMarkdown(title, body, onUpdate, onDismiss)
	}()
}

func (g *GUI) startupUpdateStatus(info *updater.UpdateInfo) (hasUpdate, isDevBuild bool) {
	if info.ReleaseCount == 0 || info.Current == info.Latest {
		return false, false
	}
	currentDate, err := version.CommitDateTime()
	if err != nil {
		return true, false
	}
	switch {
	case currentDate.Before(info.LatestDate):
		return true, false
	case currentDate.After(info.LatestDate):
		return false, true
	}
	return false, false
}

func (g *GUI) selfUpdateBody(info *updater.UpdateInfo) string {
	currentText := info.Current
	if version.Commit != "unknown" && version.Commit != "" {
		currentText = version.Commit
	}

	date := ""
	if !info.LatestDate.IsZero() {
		date = fmt.Sprintf("\n\nReleased: %s", info.LatestDate.Local().Format("2006-01-02 15:04:05"))
	}
	return fmt.Sprintf("%s\n\n%s%s\n\n%s\n\n%s",
		localengine.T("dialog", "self_update", "current")+currentText,
		localengine.T("dialog", "self_update", "latest")+info.Latest,
		date,
		localengine.T("dialog", "self_update", "changelog"),
		info.LatestBody)
}

func (g *GUI) runSelfUpdateAtStartup(info *updater.UpdateInfo, done func()) {
	u := g.selfUpdater()
	if u == nil {
		g.dialog.Show(localengine.T("dialog", "self_update", "title"), "Self updater not configured")
		if done != nil {
			done()
		}
		return
	}

	var progress float32
	var mu sync.Mutex
	g.dialog.ShowLoadingWithProgress(localengine.T("about", "update", "updating"), func() float32 {
		mu.Lock()
		defer mu.Unlock()
		return progress
	})
	go func() {
		err := u.Install(context.Background(), info, func(downloaded, total int64) {
			mu.Lock()
			defer mu.Unlock()
			if total > 0 {
				progress = float32(downloaded) / float32(total)
			} else {
				progress = 0
			}
		})
		g.dialog.HideLoading()
		if err != nil {
			g.log.Warnf("startup self-update failed: %v", err)
			g.dialog.Show(localengine.T("dialog", "self_update", "title"), "Update failed: "+err.Error())
		} else {
			g.dialog.Show(localengine.T("dialog", "self_update", "title"), "Update complete. Please restart.")
		}
		if done != nil {
			done()
		}
	}()
}

// coreBackend returns the active core backend, preferring the interactive controller
// when it has already been built.
func (g *GUI) coreBackend() core.Backend {
	if g.ctrl != nil {
		return g.ctrl.Backend()
	}
	return g.app.Controller
}

func (g *GUI) checkCoreUpdateAtStartup(done func()) {
	if !g.cfg.MustGet("updates", "auto_check_core").Bool() {
		if done != nil {
			done()
		}
		return
	}

	g.dialog.ShowLoading(localengine.T("core", "update", "checking"))
	go func() {
		backend := g.coreBackend()
		current, _ := backend.GetInstalledCoreVersion()
		latest, err := backend.GetLatestCoreVersion()
		g.dialog.HideLoading()
		if err != nil {
			g.log.Warnf("startup core-update check failed: %v", err)
			if done != nil {
				done()
			}
			return
		}

		current = normalizeCoreVersion(current)
		latest = normalizeCoreVersion(latest)
		if current == latest || latest == "" {
			if done != nil {
				done()
			}
			return
		}

		body := localengine.T("dialog", "version_check", "current") + current + "\n\n" +
			localengine.T("dialog", "version_check", "latest") + latest
		onDismiss := func() {
			if done != nil {
				done()
			}
		}
		g.dialog.ShowConfirm(localengine.T("dialog", "core_update", "title"), body, func() {
			g.runCoreUpdateAtStartup(done)
		}, onDismiss)
	}()
}

func (g *GUI) runCoreUpdateAtStartup(done func()) {
	var progress float32
	var mu sync.Mutex
	g.dialog.ShowLoadingWithProgress(localengine.T("core", "update", "downloading"), func() float32 {
		mu.Lock()
		defer mu.Unlock()
		return progress
	})
	go func() {
		_, err := g.coreBackend().DownloadCoreWithProgress(func(downloaded, total int64) {
			mu.Lock()
			defer mu.Unlock()
			if total > 0 {
				progress = float32(downloaded) / float32(total)
			} else {
				progress = 0
			}
		})
		g.dialog.HideLoading()
		if err != nil {
			g.log.Warnf("startup core download failed: %v", err)
			g.dialog.Show(localengine.T("dialog", "core_update", "title"), "Download failed: "+err.Error())
		} else {
			g.dialog.Show(localengine.T("dialog", "core_update", "title"), "Core downloaded successfully.")
		}
		if done != nil {
			done()
		}
	}()
}

func (g *GUI) selfUpdater() *updater.Manager {
	for _, m := range g.app.Updaters {
		if m.Name == "updater" {
			return m
		}
	}
	return nil
}

func normalizeCoreVersion(v string) string {
	if v == "" {
		return v
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}
