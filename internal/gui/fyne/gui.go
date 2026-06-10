package fynegui

import (
	"image/color"
	"strings"
	"sync"
	"time"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/framework/updater"
	"sing-box-ez/internal/plugins"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const timeLayout = "2006-01-02 15:04"

var (
	colGreen  = color.RGBA{0x40, 0xC0, 0x40, 0xFF}
	colRed    = color.RGBA{0xE0, 0x40, 0x40, 0xFF}
	colYellow = color.RGBA{0xE0, 0xC0, 0x20, 0xFF}
	colOrange = color.RGBA{0xE0, 0x80, 0x20, 0xFF}
)

type GUI struct {
	app    fyne.App
	window fyne.Window
	cfg    *config.AppConfig
	ctrl   *core.InteractiveController

	// Main tab widgets
	statusText   *canvas.Text
	configSelect *widget.Select
	startBtn     *widget.Button
	restartBtn   *widget.Button
	adminCheck   *widget.Check

	// Configs tab widgets
	configTable    *widget.Table
	configData     []config.ConfigRecord
	configSelected int

	// Tools tab widgets
	defaultIntervalEntry      *widget.Entry
	logLimitEntry             *widget.Entry
	showLogsCheck             *widget.Check
	showCoreLogsCheck         *widget.Check
	coreAutoRestartCheck      *widget.Check
	desktopNotificationsCheck *widget.Check
	pluginsEnabledCheck       *widget.Check
	pluginsDeveloperCheck     *widget.Check
	versionText               *canvas.Text
	latestText                *canvas.Text
	privilegeText             *canvas.Text

	// Log tab widgets
	logEntry *widget.Entry

	// Plugin manager
	pluginManager *plugins.Manager
	pluginsList   *widget.List
	pluginItems   []pluginListItem

	// Tabs container
	tabs *container.AppTabs

	// Version tracking
	latestVersion string

	// Lifecycle
	stopped         bool
	stopMu          sync.Mutex
	selectingConfig bool

	// Common
	mu sync.Mutex
}

func New(cfg *config.AppConfig) *GUI {
	a := app.New()
	w := a.NewWindow(localengine.T("app.title"))
	w.Resize(fyne.NewSize(800, 600))

	g := &GUI{
		app:    a,
		window: w,
		cfg:    cfg,
		ctrl:   core.NewInteractiveController(cfg),
	}

	// Wire UI callbacks from core
	g.ctrl.OnAutoRestart = func() {
		g.sendNotification(localengine.T("notify.core_crashed.title"), localengine.T("notify.core_crashed.body"))
	}
	g.ctrl.OnStatusChange = func(running bool) {
		g.updateButtons()
	}
	g.ctrl.OnConfigUpdate = func() {
		g.refreshConfigData()
		if g.configTable != nil {
			g.configTable.Refresh()
		}
	}
	g.ctrl.OnVersionChange = func(ver string) {
		if g.versionText != nil {
			g.versionText.Text = g.t("core.version.installed") + ver
			g.versionText.Color = colGreen
			g.versionText.Refresh()
		}
	}
	g.ctrl.OnPrivilegeChange = func(active bool) {
		if g.privilegeText == nil {
			return
		}
		if active {
			g.privilegeText.Text = g.t("core.privileges.setcap_active")
			g.privilegeText.Color = colGreen
		} else {
			g.privilegeText.Text = g.t("core.privileges.root_required")
			g.privilegeText.Color = colYellow
		}
		g.privilegeText.Refresh()
	}
	g.ctrl.OnLatestVersion = func(ver string) {
		g.latestVersion = ver
		if g.latestText != nil {
			g.latestText.Text = g.t("core.latest.prefix") + ver
			g.latestText.Color = colGreen
			g.latestText.Refresh()
		}
	}
	g.ctrl.OnNotification = func(title, body string) {
		g.sendNotification(title, body)
	}
	g.ctrl.OnFirstRun = func() {
		fyne.DoAndWait(func() { g.showFirstRunDialog() })
	}
	g.ctrl.OnSelfUpdateAvailable = func(info *updater.UpdateInfo) {
		fyne.Do(func() { g.showSelfUpdateDialog(info) })
	}

	w.SetOnClosed(g.onWindowClosed)

	g.buildUI()
	if g.cfg.GetPluginsEnabled() {
		g.initPlugins()
	}
	g.updateButtons()
	g.refreshCoreVersionUI()
	g.refreshPrivilegeStatusUI()

	return g
}

func (g *GUI) t(id string, data ...map[string]any) string {
	return localengine.T(id, data...)
}

func (g *GUI) buildUI() {
	mainTab := g.buildMainTab()
	configsTab := g.buildConfigsTab()
	coreTab := g.buildCoreTab()
	settingsTab := g.buildSettingsTab()
	logTab := g.buildLogTab()
	pluginsTab := g.buildPluginsTab()
	aboutTab := g.buildAboutTab()

	items := []*container.TabItem{mainTab, configsTab, coreTab, settingsTab}
	if logTab != nil {
		items = append(items, logTab)
	}
	if pluginsTab != nil {
		items = append(items, pluginsTab)
	}
	items = append(items, aboutTab)
	g.tabs = container.NewAppTabs(items...)
	g.window.SetContent(g.tabs)
}

// rebuildUI recreates the entire UI after a language change.
func (g *GUI) rebuildUI() {
	g.buildUI()
	g.updateButtons()
	g.refreshCoreVersionUI()
	g.refreshPrivilegeStatusUI()
	g.refreshConfigData()
	if g.cfg.GetPluginsEnabled() {
		g.refreshPluginsList()
	}
}

func (g *GUI) onWindowClosed() {
	g.stopMu.Lock()
	g.stopped = true
	g.stopMu.Unlock()
	if g.ctrl != nil {
		g.ctrl.Close()
	}
}

func (g *GUI) refreshActiveLabel() {
	configs := g.cfg.GetConfigs()
	names := make([]string, 0, len(configs))
	for _, c := range configs {
		names = append(names, c.Name)
	}
	if g.configSelect != nil {
		g.configSelect.SetOptions(names)
		active := g.cfg.GetActiveConfig()
		if active != nil {
			g.selectingConfig = true
			g.configSelect.SetSelected(active.Name)
			g.selectingConfig = false
		} else {
			g.configSelect.ClearSelected()
			g.configSelect.PlaceHolder = g.t("main.active.none")
		}
	}
}

func (g *GUI) logFlushLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		g.stopMu.Lock()
		stopped := g.stopped
		g.stopMu.Unlock()
		if stopped {
			return
		}
		if g.logEntry == nil {
			continue
		}
		lines := g.ctrl.GetLogLines()
		if len(lines) == 0 {
			continue
		}
		text := strings.Join(lines, "\n")
		fyne.Do(func() {
			g.logEntry.SetText(text)
			g.logEntry.CursorRow = 999999
		})
	}
}

func (g *GUI) sendNotification(title, content string) {
	if !g.cfg.GetDesktopNotifications() {
		return
	}
	fyne.Do(func() {
		g.app.SendNotification(&fyne.Notification{
			Title:   title,
			Content: content,
		})
	})
}

func (g *GUI) refreshCoreVersionUI() {
	ver, err := g.ctrl.GetInstalledCoreVersion()
	if err == nil && ver != "" {
		if g.versionText != nil {
			g.versionText.Text = g.t("core.version.installed") + ver
			g.versionText.Color = colGreen
			g.versionText.Refresh()
		}
	} else {
		if g.versionText != nil {
			g.versionText.Text = g.t("core.version.not_installed")
			g.versionText.Color = colRed
			g.versionText.Refresh()
		}
	}
}

func (g *GUI) refreshPrivilegeStatusUI() {
	status := g.ctrl.RefreshPrivilegeStatus()
	if g.privilegeText == nil {
		return
	}
	switch status {
	case "active":
		g.privilegeText.Text = g.t("core.privileges.setcap_active")
		g.privilegeText.Color = colGreen
	case "root_required":
		g.privilegeText.Text = g.t("core.privileges.root_required")
		g.privilegeText.Color = colYellow
	}
	g.privilegeText.Refresh()
}

func (g *GUI) onDownloadCore() {
	modal := g.showInfiniteDialog(g.t("progress.checking_version"))
	path, err := g.ctrl.DownloadCoreWithProgressWithLog(func(downloaded, total int64) {
		fyne.Do(func() {
			// progress update handled by dialog if needed
		})
	})
	fyne.Do(func() { modal.Hide() })
	if err != nil {
		return
	}
	g.showDownloadCompleteDialog(g.latestVersion, path)
	g.refreshCoreVersionUI()
}

func (g *GUI) onStart() {
	if !g.ctrl.HasRequiredPrivileges() {
		g.showPrivilegeDialog()
		return
	}
	if err := g.ctrl.StartCore(); err != nil {
		return
	}
	g.sendNotification(localengine.T("notify.core_started.title"), localengine.T("notify.core_started.body"))
	g.updateButtons()
}

func (g *GUI) onStop() {
	if err := g.ctrl.StopCore(); err != nil {
		return
	}
	g.sendNotification(localengine.T("notify.core_stopped.title"), localengine.T("notify.core_stopped.body"))
	g.updateButtons()
}

func (g *GUI) onRestart() {
	_ = g.ctrl.RestartCore()
}

func (g *GUI) updateButtons() {
	fyne.Do(func() {
		running := g.ctrl.IsRunning()
		hasConfig := g.cfg.GetActiveConfig() != nil
		if running {
			g.startBtn.SetText(localengine.T("main.btn.stop"))
			g.startBtn.SetIcon(theme.MediaStopIcon())
			g.startBtn.OnTapped = g.onStop
			g.startBtn.Enable()
			g.restartBtn.Enable()
			g.statusText.Text = g.t("main.status.running")
			g.statusText.Color = colGreen
			g.statusText.Refresh()
		} else {
			g.startBtn.SetText(localengine.T("main.btn.start"))
			g.startBtn.SetIcon(theme.MediaPlayIcon())
			g.startBtn.OnTapped = g.onStart
			if hasConfig {
				g.startBtn.Enable()
			} else {
				g.startBtn.Disable()
			}
			g.restartBtn.Disable()
			g.statusText.Text = g.t("main.status.stopped")
			g.statusText.Color = colRed
			g.statusText.Refresh()
		}
	})
}

// repaintLoop periodically forces a canvas refresh to work around Fyne/GLFW
// not repainting automatically when the window becomes visible after being
// hidden (minimize, workspace switch, or screen idle wake-up on Linux/Windows).
func (g *GUI) repaintLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		g.stopMu.Lock()
		stopped := g.stopped
		g.stopMu.Unlock()
		if stopped {
			return
		}
		fyne.Do(func() {
			if g.window != nil {
				g.window.Canvas().Refresh(g.window.Content())
			}
		})
	}
}

func (g *GUI) Run() {
	g.window.Show()
	go g.ctrl.RunStartupSequence()
	go g.repaintLoop()
	go g.logFlushLoop()
	g.app.Run()
}
