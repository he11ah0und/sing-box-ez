package giogui

import (
	"context"
	_ "embed"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"sing-box-ez/internal/app"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/framework/logger"
	"sing-box-ez/internal/framework/updater"
	"sing-box-ez/internal/framework/version"
	"sing-box-ez/internal/gui/gio/pages"
	"sing-box-ez/internal/gui/tray"

	gioapp "gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/io/system"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// GUI holds the new Gio-based adaptive UI.
type GUI struct {
	app *app.App
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

	// Dialog reference for startup sequences
	dialog *Dialog

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
func New(app *app.App) *GUI {
	cfg := app.Config
	th := material.NewTheme()
	// Dark theme by default
	th.Palette.Bg = color.NRGBA{R: 18, G: 18, B: 18, A: 255}
	th.Palette.Fg = color.NRGBA{R: 255, G: 255, B: 255, A: 255}

	g := &GUI{
		app: app,
		cfg: cfg,
		th:  th,
		log: app.Logger.Root.Allocate("gui"),
	}
	g.log.Infof("initialized")

	// Build a font collection with Go font as base + optional emoji support.
	collection := gofont.Collection()
	if emoji := g.tryLoadEmojiFont(); len(emoji) > 0 {
		collection = append(collection, emoji...)
	}
	th.Shaper = text.NewShaper(text.WithCollection(collection))

	// Initialize interactive controller wrapping the shared core controller.
	g.ctrl = core.NewInteractiveController(app.Controller)
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

	dialog := NewDialog()
	g.dialog = dialog

	g.aboutPage = pages.NewAboutPage(th, g.ctrl, dialog)
	mainPage := pages.NewMainPage(th, g.ctrl, dialog)
	g.logPage = pages.NewLogPage(th, g.ctrl.Controller)

	g.settingsPage = pages.NewSettingsPage(th, g.ctrl, dialog)
	g.settingsPage.OnLanguageChange = func() {
		g.shell.RebuildNav()
	}
	g.settingsPage.OnResetRequested = func() {
		g.showResetConfirm()
	}
	g.settingsPage.OnShowLogsChange = func(show bool) {
		g.rebuildSecondaryPages(show)
	}

	g.configsPage = pages.NewConfigsPage(th, g.ctrl, dialog)
	g.corePage = pages.NewCorePage(th, g.ctrl, dialog)
	g.pluginsPage = pages.NewPluginsPage(th, g.ctrl.Controller)

	primary := []pages.Page{mainPage, g.configsPage}
	secondary := g.buildSecondaryPages(cfg.GetShowLogs())

	g.shell = NewShell(th, cfg, g.ctrl.Controller, primary, secondary)
	g.shell.dialog = dialog

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
	if g.cfg.GetPluginsEnabled() {
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
	go g.runStartupUpdateChecks()
	gioapp.Main()

	if g.restart {
		g.doRestart()
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

// resetDataAndRestart removes config.json, profiles.json and the configs folder,
// shows a confirmation dialog, and then requests a restart.
func (g *GUI) resetDataAndRestart() {
	dataDir := g.cfg.DataDir
	paths := []string{
		filepath.Join(dataDir, "config.json"),
		filepath.Join(dataDir, "profiles.json"),
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
	if !g.cfg.GetAutoCheckSelfUpdates() {
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
	if !g.cfg.GetAutoCheckCoreUpdates() {
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
