package gui

import (
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"runtime"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/i18n"
	"sing-box-ez/internal/plugins"
	"sing-box-ez/internal/updater"
	"sing-box-ez/internal/version"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
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
	app     fyne.App
	window  fyne.Window
	cfg     *config.AppConfig
	manager *core.Manager

	// Main tab widgets
	statusText *canvas.Text
	activeLbl  *widget.Label
	startBtn   *widget.Button
	stopBtn    *widget.Button
	restartBtn *widget.Button
	adminCheck *widget.Check

	// Configs tab widgets
	configTable    *widget.Table
	configData     []config.ConfigRecord
	configSelected int
	addBtn         *widget.Button
	editBtn        *widget.Button
	delBtn         *widget.Button
	activateBtn    *widget.Button
	updateAllBtn   *widget.Button

	// Tools tab widgets
	defaultIntervalEntry     *widget.Entry
	logLimitEntry            *widget.Entry
	showLogsCheck            *widget.Check
	showCoreLogsCheck        *widget.Check
	coreAutoRestartCheck     *widget.Check
	desktopNotificationsCheck *widget.Check
	pluginsEnabledCheck      *widget.Check
	pluginsDeveloperCheck    *widget.Check
	versionText           *canvas.Text
	latestText            *canvas.Text
	privilegeText         *canvas.Text

	// Log tab widgets
	logEntry    *widget.Entry
	updatingLog bool
	logLines    []string
	logMu       sync.Mutex

	// Core log writer
	logWriter *core.CoreLogWriter

	// Plugin manager
	pluginManager *plugins.Manager
	pluginsList   *widget.List
	pluginItems   []pluginListItem

	// Tabs container
	tabs *container.AppTabs

	// Version tracking
	latestVersion string

	// Lifecycle
	stopped bool
	stopMu  sync.Mutex

	// Auto-restart rate limiter
	lastAutoRestart time.Time
	autoRestartMu   sync.Mutex

	// Common
	mu sync.Mutex
}

func New(cfg *config.AppConfig) *GUI {
	a := app.New()

	// Initialize language: detect system locale on first run, otherwise use saved.
	if !cfg.GetFirstRunDone() {
		lang := i18n.DetectSystemLanguage()
		cfg.SetLanguage(lang)
		_ = cfg.Save()
		i18n.SetLanguage(lang)
	} else if lang := cfg.GetLanguage(); lang != "" {
		i18n.SetLanguage(lang)
	}

	w := a.NewWindow(i18n.T("app.title"))
	w.Resize(fyne.NewSize(800, 600))

	g := &GUI{
		app:      a,
		window:   w,
		cfg:      cfg,
		logLines: []string{},
	}
	active := cfg.GetActiveConfig()
	url := ""
	if active != nil {
		url = active.URL
		g.manager = core.NewManager(url)
		g.manager.SetConfigName(active.Name)
	} else {
		g.manager = core.NewManager("")
	}
	g.manager.SetElevated(cfg.RunAsAdmin)

	g.logWriter = core.NewCoreLogWriter(g.logCore)
	g.manager.SetLogOutput(g.logWriter)
	go g.logReader()

	w.SetOnClosed(g.onWindowClosed)

	g.buildUI()
	if g.cfg.GetPluginsEnabled() {
		g.initPlugins()
	}
	g.updateButtons()
	g.refreshCoreVersion()
	go g.updateChecker()
	go g.statusChecker()

	return g
}

func (g *GUI) t(id string, data ...map[string]interface{}) string {
	return i18n.T(id, data...)
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
	g.refreshCoreVersion()
	g.refreshPrivilegeStatus()
	g.refreshConfigData()
	if g.cfg.GetPluginsEnabled() {
		g.refreshPluginsList()
	}
}

func (g *GUI) onWindowClosed() {
	g.stopMu.Lock()
	g.stopped = true
	g.stopMu.Unlock()
	if g.logWriter != nil {
		g.logWriter.Close()
	}
	if g.manager.IsRunning() {
		if err := g.manager.Stop(); err != nil {
			// Silently ignore stop errors during shutdown
			_ = err
		}
	}
}

func (g *GUI) logReader() {
	for line := range g.logWriter.Ch {
		g.stopMu.Lock()
		stopped := g.stopped
		g.stopMu.Unlock()
		if stopped {
			return
		}
		g.logCore("[core] " + line)
	}
}

func (g *GUI) refreshActiveLabel() {
	active := g.cfg.GetActiveConfig()
	if active != nil {
		g.activeLbl.SetText(g.t("main.active.prefix") + active.Name)
	} else {
		g.activeLbl.SetText(g.t("main.no.config"))
	}
}

func (g *GUI) appendLogLines(newLines []string) {
	g.logMu.Lock()
	g.logLines = append(g.logLines, newLines...)
	limit := g.cfg.GetLogLimit()
	if limit > 0 && len(g.logLines) > limit {
		g.logLines = g.logLines[len(g.logLines)-limit:]
	}
	g.logMu.Unlock()
}

func (g *GUI) logFlushLoop() {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		g.stopMu.Lock()
		stopped := g.stopped
		g.stopMu.Unlock()
		if stopped {
			return
		}
		g.logMu.Lock()
		if g.logEntry == nil || len(g.logLines) == 0 {
			g.logMu.Unlock()
			continue
		}
		text := strings.Join(g.logLines, "\n")
		g.logMu.Unlock()
		fyne.Do(func() {
			g.logEntry.SetText(text)
			g.logEntry.CursorRow = 999999
		})
	}
}

func (g *GUI) log(msg string) {
	g.stopMu.Lock()
	stopped := g.stopped
	g.stopMu.Unlock()
	if stopped {
		return
	}
	timestamp := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s", timestamp, msg)
	g.appendLogLines([]string{line})
}

func (g *GUI) sendNotification(title, content string) {
	if !g.cfg.GetDesktopNotifications() {
		return
	}
	g.app.SendNotification(&fyne.Notification{
		Title:   title,
		Content: content,
	})
}

func isCoreFatalError(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "fatal[") ||
		strings.Contains(lower, "panic:") ||
		strings.Contains(lower, "fetch rule-set") ||
		strings.Contains(lower, "initial rule-set:") ||
		strings.Contains(lower, "save rule-set")
}

func (g *GUI) logCore(msg string) {
	g.stopMu.Lock()
	stopped := g.stopped
	g.stopMu.Unlock()
	if stopped {
		return
	}

	lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
	var valid []string
	for _, l := range lines {
		if l != "" {
			valid = append(valid, l)
		}
	}

	if g.cfg.GetCoreAutoRestart() && len(valid) > 0 {
		for _, l := range valid {
			if isCoreFatalError(l) {
				g.autoRestartMu.Lock()
				if time.Since(g.lastAutoRestart) > 30*time.Second {
					g.lastAutoRestart = time.Now()
					g.autoRestartMu.Unlock()
					g.log("Detected core fatal error, auto-restarting...")
					g.sendNotification(i18n.T("notify.core_crashed.title"), i18n.T("notify.core_crashed.body"))
					go func() {
						if err := g.manager.Restart(); err != nil {
							g.log("Auto-restart failed: " + err.Error())
						}
					}()
				} else {
					g.autoRestartMu.Unlock()
				}
				break
			}
		}
	}

	if !g.cfg.GetWatchCoreLogs() {
		return
	}
	if len(valid) == 0 {
		return
	}
	g.appendLogLines(valid)
}

func (g *GUI) refreshCoreVersion() {
	ver, err := core.GetCoreVersion(core.GetCorePath())
	if err == nil && ver != "" {
		fyne.Do(func() {
			g.versionText.Text = g.t("core.version.installed") + ver
			g.versionText.Color = colGreen
			g.versionText.Refresh()
		})
	} else {
		fyne.Do(func() {
			g.versionText.Text = g.t("core.version.not_installed")
			g.versionText.Color = colRed
			g.versionText.Refresh()
		})
	}
}

func (g *GUI) checkLatestVersion() {
	ver, err := core.GetLatestVersion()
	if err != nil {
		fyne.Do(func() {
			g.latestText.Text = g.t("core.latest.error")
			g.latestText.Color = colRed
			g.latestText.Refresh()
		})
		return
	}
	g.latestVersion = ver
	fyne.Do(func() {
		g.latestText.Text = g.t("core.latest.prefix") + ver
		g.latestText.Color = colGreen
		g.latestText.Refresh()
	})
}

func (g *GUI) checkUpdatesOnStartup() {
	modal := g.showInfiniteDialog(g.t("progress.checking_updates"))

	// Check self-update first
	info, err := updater.CheckUpdate(version.Branch)
	if err == nil && info.ReleaseCount > 0 {
		fyne.Do(func() { modal.Hide() })
		fyne.Do(func() { g.showSelfUpdateDialog(info) })
		return
	}

	// Check core update only if core is installed.
	currentVer, err := core.GetCoreVersion(core.GetCorePath())
	if err != nil || currentVer == "" {
		fyne.Do(func() { modal.Hide() })
		return
	}

	ver, err := core.GetLatestVersion()
	if err != nil {
		fyne.Do(func() { modal.Hide() })
		return
	}
	g.latestVersion = ver

	if currentVer == ver {
		fyne.Do(func() { modal.Hide() })
		return
	}

	fyne.Do(func() { modal.Hide() })
	fyne.Do(func() {
		g.showUpdatePrompt(ver, currentVer)
	})
}

func (g *GUI) refreshPrivilegeStatus() {
	if runtime.GOOS != "linux" {
		return
	}
	if g.privilegeText == nil {
		return
	}
	active := core.HasNetAdminCapability(core.GetCorePath())
	fyne.Do(func() {
		if active {
			g.privilegeText.Text = g.t("core.privileges.setcap_active")
			g.privilegeText.Color = colGreen
		} else {
			g.privilegeText.Text = g.t("core.privileges.root_required")
			g.privilegeText.Color = colYellow
		}
		g.privilegeText.Refresh()
	})
}

func (g *GUI) onDownloadCore() {
	modal := g.showInfiniteDialog(g.t("progress.checking_version"))
	ver, err := core.GetLatestVersion()
	fyne.Do(func() { modal.Hide() })
	if err != nil {
		g.log("Failed to check latest version: " + err.Error())
		return
	}
	g.log("Latest version: v" + ver)

	progressModal, progress := g.showProgressDialog(fmt.Sprintf(g.t("progress.downloading_core"), ver))
	defer fyne.Do(func() { progressModal.Hide() })
	var lastLog time.Time
	corePath, err := core.DownloadCore("", func(downloaded, total int64) {
		fyne.Do(func() {
			progress.SetValue(float64(downloaded) / float64(total))
		})
		now := time.Now()
		if now.Sub(lastLog) < 500*time.Millisecond {
			return
		}
		lastLog = now
		percent := float64(downloaded) / float64(total) * 100
		g.log(fmt.Sprintf("Download progress: %.1f%% (%d / %d bytes)", percent, downloaded, total))
	})
	if err != nil {
		g.log("Failed to download core: " + err.Error())
		return
	}
	g.log("Core downloaded to: " + corePath)
	g.showDownloadCompleteDialog(ver, corePath)
	g.refreshCoreVersion()
}

func (g *GUI) onStart() {
	if !core.CoreExists() {
		g.log("Core not found. Please download it first.")
		return
	}

	active := g.cfg.GetActiveConfig()
	if active == nil {
		g.log("No active config. Please add and activate a config in the Configs tab.")
		return
	}
	g.manager.SetConfigURL(active.URL)
	g.manager.SetConfigName(active.Name)

	if active.ShouldUpdate() || !core.HasCachedConfig(active.Name) {
		g.log("Updating config...")
		if err := g.manager.UpdateConfig(); err != nil {
			g.log("Config download issue: " + err.Error())
			if core.HasCachedConfig(active.Name) {
				g.log("Using existing config")
			} else {
				g.log("No config available!")
				return
			}
		} else {
			g.cfg.SetLastUpdateFor(active.Name, time.Now())
			_ = g.cfg.Save()
			g.refreshConfigData()
			g.configTable.Refresh()
			g.log("Config updated")
		}
	}

	if !g.hasRequiredPrivileges() {
		g.showPrivilegeDialog()
		return
	}

	if err := g.manager.Start(); err != nil {
		g.log("Failed to start: " + err.Error())
		return
	}
	g.log("Sing-box started")
	g.sendNotification(i18n.T("notify.core_started.title"), i18n.T("notify.core_started.body"))
	g.updateButtons()
}

func (g *GUI) onStop() {
	if err := g.manager.Stop(); err != nil {
		g.log("Failed to stop: " + err.Error())
		return
	}
	g.log("Sing-box stopped")
	g.sendNotification(i18n.T("notify.core_stopped.title"), i18n.T("notify.core_stopped.body"))
	g.updateButtons()
}

func (g *GUI) onRestart() {
	g.log("Restarting...")
	if err := g.manager.Restart(); err != nil {
		g.log("Failed to restart: " + err.Error())
		return
	}
	g.log("Sing-box restarted")
}

func (g *GUI) updateButtons() {
	fyne.Do(func() {
		running := g.manager.IsRunning()
		hasConfig := g.cfg.GetActiveConfig() != nil
		if running {
			g.startBtn.Disable()
			g.stopBtn.Enable()
			g.restartBtn.Enable()
			g.statusText.Text = g.t("main.status.running")
			g.statusText.Color = colGreen
			g.statusText.Refresh()
		} else {
			if hasConfig {
				g.startBtn.Enable()
			} else {
				g.startBtn.Disable()
			}
			g.stopBtn.Disable()
			g.restartBtn.Disable()
			g.statusText.Text = g.t("main.status.stopped")
			g.statusText.Color = colRed
			g.statusText.Refresh()
		}
	})
}

func (g *GUI) updateChecker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		g.stopMu.Lock()
		stopped := g.stopped
		g.stopMu.Unlock()
		if stopped {
			return
		}
		active := g.cfg.GetActiveConfig()
		if active != nil && active.ShouldUpdate() && g.manager.IsRunning() {
			g.log("Auto-updating config...")
			g.manager.SetConfigName(active.Name)
			if err := g.manager.UpdateConfig(); err != nil {
				g.log("Auto-update failed: " + err.Error())
			} else {
				g.cfg.SetLastUpdateFor(active.Name, time.Now())
				_ = g.cfg.Save()
				g.refreshConfigData()
				g.configTable.Refresh()
				g.log("Config auto-updated, restarting core...")
				if err := g.manager.Restart(); err != nil {
					g.log("Auto-restart failed: " + err.Error())
				}
			}
		}
	}
}

func (g *GUI) statusChecker() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var lastRunning bool
	for range ticker.C {
		g.stopMu.Lock()
		stopped := g.stopped
		g.stopMu.Unlock()
		if stopped {
			return
		}
		running := g.manager.IsRunning()
		if running != lastRunning {
			lastRunning = running
			g.updateButtons()
		}
	}
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
	go func() {
		if !g.cfg.GetFirstRunDone() {
			g.showFirstRunDialog()
		}
		g.checkUpdatesOnStartup()
	}()
	go g.repaintLoop()
	go g.logFlushLoop()
	g.app.Run()
}

func (g *GUI) hasRequiredPrivileges() bool {
	switch runtime.GOOS {
	case "linux":
		if core.HasNetAdminCapability(core.GetCorePath()) {
			return true
		}
		return g.cfg.RunAsAdmin
	case "windows":
		return core.IsAdmin()
	default:
		return true
	}
}

func (g *GUI) restartAsAdmin() {
	if runtime.GOOS != "windows" {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		g.log("Failed to get executable path: " + err.Error())
		return
	}
	cwd, err := os.Getwd()
	if err != nil {
		g.log("Failed to get working directory: " + err.Error())
		return
	}
	cmd := exec.Command("powershell", "-WindowStyle", "hidden", "-Command",
		"Start-Process", "-FilePath", exe, "-Verb", "runAs", "-WorkingDirectory", cwd)
	if err := cmd.Start(); err != nil {
		g.log("Failed to restart as admin: " + err.Error())
		return
	}
	g.window.Close()
}
