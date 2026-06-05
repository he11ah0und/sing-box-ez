package gui

import (
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"runtime"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/plugins"
	"sing-box-ez/internal/updater"
	"sing-box-ez/internal/version"
	"strconv"
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
	defaultIntervalEntry *widget.Entry
	logLimitEntry        *widget.Entry
	showLogsCheck        *widget.Check
	showCoreLogsCheck    *widget.Check
	versionText          *canvas.Text
	latestText           *canvas.Text
	privilegeText        *canvas.Text

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

	// Common
	mu sync.Mutex
}

func New(cfg *config.AppConfig) *GUI {
	a := app.New()
	w := a.NewWindow("Sing-box EZ")
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
	g.initPlugins()
	g.updateButtons()
	g.refreshCoreVersion()
	go g.checkStartupUpdate()
	go g.checkSelfUpdate()
	go g.updateChecker()
	go g.statusChecker()

	return g
}

func (g *GUI) buildUI() {
	mainTab := g.buildMainTab()
	configsTab := g.buildConfigsTab()
	settingsTab := g.buildSettingsTab()
	logTab := g.buildLogTab()
	pluginsTab := g.buildPluginsTab()
	g.tabs = container.NewAppTabs(
		mainTab,
		configsTab,
		settingsTab,
		logTab,
		pluginsTab,
	)
	g.window.SetContent(g.tabs)
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
		g.activeLbl.SetText("Active config: " + active.Name)
	} else {
		g.activeLbl.SetText("No config selected — select one in the Configs tab")
	}
}

func (g *GUI) appendLogLines(newLines []string) {
	g.logMu.Lock()
	g.logLines = append(g.logLines, newLines...)
	limit := g.cfg.GetLogLimit()
	if limit > 0 && len(g.logLines) > limit {
		g.logLines = g.logLines[len(g.logLines)-limit:]
	}
	text := strings.Join(g.logLines, "\n")
	g.logMu.Unlock()
	fyne.Do(func() {
		g.logEntry.SetText(text)
		g.logEntry.CursorRow = 999999
	})
}

func (g *GUI) log(msg string) {
	if !g.cfg.GetShowLogs() {
		return
	}
	g.stopMu.Lock()
	stopped := g.stopped
	g.stopMu.Unlock()
	if stopped {
		return
	}
	g.logMu.Lock()
	if g.logEntry == nil {
		g.logMu.Unlock()
		return
	}
	g.logMu.Unlock()
	timestamp := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s", timestamp, msg)
	fyne.Do(func() {
		g.appendLogLines([]string{line})
	})
}

func (g *GUI) logCore(msg string) {
	if !g.cfg.GetShowCoreLogs() {
		return
	}
	g.stopMu.Lock()
	stopped := g.stopped
	g.stopMu.Unlock()
	if stopped {
		return
	}
	g.logMu.Lock()
	if g.logEntry == nil {
		g.logMu.Unlock()
		return
	}
	g.logMu.Unlock()
	lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
	var valid []string
	for _, l := range lines {
		if l != "" {
			valid = append(valid, l)
		}
	}
	if len(valid) == 0 {
		return
	}
	fyne.Do(func() {
		g.appendLogLines(valid)
	})
}

func parseSemver(v string) (maj, min, pat int) {
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	parts := strings.Split(v, ".")
	if len(parts) >= 1 {
		maj, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		min, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 {
		pat, _ = strconv.Atoi(parts[2])
	}
	return
}

func (g *GUI) refreshCoreVersion() {
	ver, err := core.GetCoreVersion(core.GetCorePath())
	if err == nil && ver != "" {
		fyne.Do(func() {
			g.versionText.Text = "Core: v" + ver
			g.versionText.Refresh()
			g.compareVersions(ver)
		})
	} else {
		fyne.Do(func() {
			g.versionText.Text = "Core: not installed"
			g.versionText.Color = colRed
			g.versionText.Refresh()
		})
	}
}

func (g *GUI) compareVersions(current string) {
	if g.latestVersion == "" || current == "" {
		return
	}
	cMaj, cMin, cPat := parseSemver(current)
	lMaj, lMin, lPat := parseSemver(g.latestVersion)

	var col color.Color
	switch {
	case cMaj != lMaj:
		col = colRed
	case cMin != lMin:
		col = colOrange
	case cPat != lPat:
		col = colYellow
	default:
		col = colGreen
	}
	g.versionText.Color = col
	g.versionText.Refresh()
}

func (g *GUI) checkLatestVersion() {
	ver, err := core.GetLatestVersion()
	if err != nil {
		fyne.Do(func() {
			g.latestText.Text = "Latest: error"
			g.latestText.Color = colRed
			g.latestText.Refresh()
		})
		return
	}
	g.latestVersion = ver
	fyne.Do(func() {
		g.latestText.Text = "Latest: v" + ver
		g.latestText.Color = colGreen
		g.latestText.Refresh()
		// Re-evaluate core version color if known
		if cur, ok := strings.CutPrefix(g.versionText.Text, "Core: v"); ok {
			g.compareVersions(cur)
		}
	})
}

func (g *GUI) checkStartupUpdate() {
	ver, err := core.GetLatestVersion()
	if err != nil {
		fyne.Do(func() {
			g.latestText.Text = "Latest: error"
			g.latestText.Color = colRed
			g.latestText.Refresh()
		})
		return
	}
	g.latestVersion = ver
	fyne.Do(func() {
		g.latestText.Text = "Latest: v" + ver
		g.latestText.Color = colGreen
		g.latestText.Refresh()
		if cur, ok := strings.CutPrefix(g.versionText.Text, "Core: v"); ok {
			g.compareVersions(cur)
		}
	})

	currentVer, err := core.GetCoreVersion(core.GetCorePath())
	if err != nil || currentVer == "" {
		return
	}

	cMaj, cMin, cPat := parseSemver(currentVer)
	lMaj, lMin, lPat := parseSemver(ver)
	if cMaj == lMaj && cMin == lMin && cPat == lPat {
		return
	}

	fyne.Do(func() {
		g.showUpdatePrompt(ver, currentVer)
	})
}

func (g *GUI) checkSelfUpdate() {
	info, err := updater.CheckUpdate(version.Version)
	if err != nil || info.ReleaseCount == 0 {
		return
	}
	fyne.Do(func() {
		g.showSelfUpdateDialog(info)
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
			g.privilegeText.Text = "Privileges: setcap active (TUN without root)"
			g.privilegeText.Color = colGreen
		} else {
			g.privilegeText.Text = "Privileges: root required for TUN"
			g.privilegeText.Color = colYellow
		}
		g.privilegeText.Refresh()
	})
}

func (g *GUI) onDownloadCore() {
	modal := g.showInfiniteDialog("Checking latest version...")
	ver, err := core.GetLatestVersion()
	fyne.Do(func() { modal.Hide() })
	if err != nil {
		g.log("Failed to check latest version: " + err.Error())
		return
	}
	g.log("Latest version: v" + ver)

	progressModal, progress := g.showProgressDialog("Downloading sing-box core v" + ver + "...")
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

	if err := g.manager.Start(); err != nil {
		g.log("Failed to start: " + err.Error())
		return
	}
	g.log("Sing-box started")
	g.updateButtons()
}

func (g *GUI) onStop() {
	if err := g.manager.Stop(); err != nil {
		g.log("Failed to stop: " + err.Error())
		return
	}
	g.log("Sing-box stopped")
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
			g.statusText.Text = "Status: running"
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
			g.statusText.Text = "Status: stopped"
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
	ticker := time.NewTicker(1 * time.Second)
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

func (g *GUI) Run() {
	g.window.ShowAndRun()
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
