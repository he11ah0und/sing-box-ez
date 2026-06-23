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
	"sing-box-ez/internal/framework/updater"
	"sing-box-ez/internal/framework/version"
	"sing-box-ez/internal/gui/gio/pages"
	"sing-box-ez/internal/gui/gio/theme"
	"sing-box-ez/internal/gui/tray"

	gioapp "gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
)

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

	// Interactive controller (GUI adapter around the core controller)
	ctrl *core.InteractiveController

	// Remote page used in remote mode.
	remotePage *pages.RemotePage

	// Operating mode: "embedded" or "remote".
	mode string

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
		mode:        "embedded",
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

		if g.cfg.MustGet("remote", "remember_connection_mode").Bool() {
			g.mode = g.cfg.MustGet("remote", "last_connection_mode").String()
			if g.mode != "embedded" && g.mode != "remote" {
				g.mode = "embedded"
			}
			close(g.startupDone)
		} else {
			g.showStartupDialog(w)
		}

		if g.mode == "remote" {
			g.buildRemoteUI(w)
		} else {
			g.buildEmbeddedUI(w)
		}

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
	var ops op.Ops
	var embeddedBtn, remoteBtn widget.Clickable
	var rememberCh widget.Bool

	for {
		switch e := w.Event().(type) {
		case gioapp.DestroyEvent:
			os.Exit(0)
		case gioapp.FrameEvent:
			gtx := gioapp.NewContext(&ops, e)

			if embeddedBtn.Clicked(gtx) {
				g.mode = "embedded"
				_ = g.cfg.MustGet("remote", "remember_connection_mode").Update(rememberCh.Value)
				_ = g.cfg.MustGet("remote", "last_connection_mode").Update("embedded")
				_ = g.cfg.Save()
				close(g.startupDone)
				return
			}
			if remoteBtn.Clicked(gtx) {
				g.mode = "remote"
				_ = g.cfg.MustGet("remote", "remember_connection_mode").Update(rememberCh.Value)
				_ = g.cfg.MustGet("remote", "last_connection_mode").Update("remote")
				_ = g.cfg.Save()
				close(g.startupDone)
				return
			}

			colors := theme.Current().Colors()
			paint.Fill(gtx.Ops, colors.Bg)

			layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				maxWidth := gtx.Dp(unit.Dp(360))
				if gtx.Constraints.Max.X < maxWidth {
					maxWidth = gtx.Constraints.Max.X - gtx.Dp(unit.Dp(32))
				}
				cardGtx := gtx
				cardGtx.Constraints.Min.X = 0
				cardGtx.Constraints.Max.X = maxWidth
				return component.Surface(g.th).Layout(cardGtx, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(material.H5(g.th, localengine.T("startup", "title")).Layout),
							layout.Rigid(layout.Spacer{Height: 16}.Layout),
							layout.Rigid(material.Body1(g.th, localengine.T("startup", "subtitle")).Layout),
							layout.Rigid(layout.Spacer{Height: 16}.Layout),
							layout.Rigid(material.Button(g.th, &embeddedBtn, localengine.T("startup", "btn", "embedded")).Layout),
							layout.Rigid(layout.Spacer{Height: 8}.Layout),
							layout.Rigid(material.Button(g.th, &remoteBtn, localengine.T("startup", "btn", "remote")).Layout),
							layout.Rigid(layout.Spacer{Height: 16}.Layout),
							layout.Rigid(material.CheckBox(g.th, &rememberCh, localengine.T("startup", "remember")).Layout),
						)
					})
				})
			})

			e.Frame(gtx.Ops)
		}
	}
}

func (g *GUI) buildEmbeddedUI(w *gioapp.Window) {
	g.ctrl = core.NewInteractiveController(g.app.Controller)
	g.ctrl.OnStatusChange = func(running bool) {
		if g.tray != nil {
			g.tray.Refresh()
		}
	}
	g.ctrl.OnUpdateCheckDue = func() {
		g.runStartupUpdateChecks()
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
	g.logPage = pages.NewLogPage(g.th, g.ctrl.Controller)

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
	g.pluginsPage = pages.NewPluginsPage(g.th, g.ctrl.Controller)

	primary := []pages.Page{mainPage, g.configsPage}
	secondary := g.buildSecondaryPages(g.cfg.MustGet("ui", "show_logs").Bool())

	g.shell = NewShell(g.th, g.cfg, g.ctrl.Controller, primary, secondary)
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
		func() bool { return g.ctrl.Controller.IsRunning() },
		func() { go g.ctrl.StartService() },
		func() { go g.ctrl.StopService() },
	)
	if err := g.tray.Start(); err != nil {
		g.log.Warnf("failed to start tray: %v", err)
	} else {
		g.log.Infof("tray started")
		g.tray.Refresh()
	}

	go g.runStartupUpdateChecks()
}

func (g *GUI) buildRemoteUI(w *gioapp.Window) {
	g.remotePage = pages.NewRemotePage(g.th, g.cfg)
	g.remotePage.SetAddress(g.cfg.MustGet("remote", "last_tcp_address").String())

	g.shell = NewShell(g.th, g.cfg, nil, []pages.Page{g.remotePage}, nil)
	g.shell.dialog = g.dialog
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

func (g *GUI) runStartupUpdateChecks() {
	g.checkSelfUpdateAtStartup(func() {
		g.checkCoreUpdateAtStartup(nil)
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
		info, err := g.ctrl.CheckSelfUpdate()
		g.dialog.HideLoading()
		if err != nil {
			g.log.Warnf("startup self-update check failed: %v", err)
			if done != nil {
				done()
			}
			return
		}

		hasUpdate := false
		isDevBuild := false
		if info.ReleaseCount > 0 && info.Current != info.Latest {
			currentDate, dateErr := version.CommitDateTime()
			if dateErr != nil {
				hasUpdate = true
			} else {
				switch {
				case currentDate.Before(info.LatestDate):
					hasUpdate = true
				case currentDate.After(info.LatestDate):
					isDevBuild = true
				}
			}
		}

		if !hasUpdate && !isDevBuild {
			if done != nil {
				done()
			}
			return
		}

		currentText := info.Current
		if version.Commit != "unknown" && version.Commit != "" {
			currentText = version.Commit
		}

		date := ""
		if !info.LatestDate.IsZero() {
			date = fmt.Sprintf("\n\nReleased: %s", info.LatestDate.Local().Format("2006-01-02 15:04:05"))
		}
		body := fmt.Sprintf("%s\n\n%s%s\n\n%s\n\n%s",
			localengine.T("dialog", "self_update", "current")+currentText,
			localengine.T("dialog", "self_update", "latest")+info.Latest,
			date,
			localengine.T("dialog", "self_update", "changelog"),
			info.LatestBody)

		onUpdate := func() {
			g.runSelfUpdateAtStartup(info, done)
		}

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

func (g *GUI) runSelfUpdateAtStartup(info *updater.UpdateInfo, done func()) {
	u := g.ctrl.SelfUpdater()
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

func (g *GUI) checkCoreUpdateAtStartup(done func()) {
	if !g.cfg.MustGet("updates", "auto_check_core").Bool() {
		if done != nil {
			done()
		}
		return
	}

	g.dialog.ShowLoading(localengine.T("core", "update", "checking"))
	go func() {
		current, _ := g.ctrl.Controller.GetInstalledCoreVersion()
		latest, err := g.ctrl.Controller.GetLatestCoreVersion()
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
		_, err := g.ctrl.Controller.DownloadCoreWithProgress(func(downloaded, total int64) {
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

func normalizeCoreVersion(v string) string {
	if v == "" {
		return v
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}
