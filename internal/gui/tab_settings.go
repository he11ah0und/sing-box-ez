package gui

import (
	"fmt"
	"image/color"
	"net/url"
	"os"
	"runtime"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/version"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func guiProtocol() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return "Wayland"
	}
	if os.Getenv("DISPLAY") != "" {
		return "X11"
	}
	return "unknown"
}

func systemInfoText() string {
	s := runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "linux" {
		proto := guiProtocol()
		if proto != "" {
			s += "-" + proto
		}
	}
	return s
}

func (g *GUI) buildSettingsTab() *container.TabItem {
	// System info
	infoLbl := widget.NewLabel(systemInfoText())
	buildLbl := widget.NewLabel("Build: " + version.Info())
	repoURL, _ := url.Parse(version.RepoURL)
	repoLink := widget.NewHyperlink("GitHub Repository", repoURL)

	// Default update interval
	g.defaultIntervalEntry = widget.NewEntry()
	g.defaultIntervalEntry.SetText(fmt.Sprintf("%d", g.cfg.UpdateIntervalHours))
	g.defaultIntervalEntry.OnSubmitted = func(s string) {
		var h int
		if _, err := fmt.Sscanf(s, "%d", &h); err == nil && h > 0 {
			g.cfg.SetDefaultUpdateInterval(h)
			_ = g.cfg.Save()
			g.log("Default interval set to " + s + "h")
		}
	}
	intervalRow := container.NewBorder(nil, nil, widget.NewLabel("Default update interval (hours):"), widget.NewButton("Save", func() {
		g.defaultIntervalEntry.OnSubmitted(g.defaultIntervalEntry.Text)
	}), g.defaultIntervalEntry)

	// Privileges block — platform specific
	var privilegesContent fyne.CanvasObject
	if runtime.GOOS == "windows" {
		adminStatus := canvas.NewText("", color.Black)
		adminStatus.TextSize = theme.TextSize()
		if core.IsAdmin() {
			adminStatus.Text = "Privileges: running as administrator"
			adminStatus.Color = colGreen
		} else {
			adminStatus.Text = "Privileges: running as user"
			adminStatus.Color = colYellow
		}
		restartBtn := widget.NewButton("Restart as administrator", func() {
			go g.restartAsAdmin()
		})
		if core.IsAdmin() {
			restartBtn.Disable()
		}
		privilegesContent = container.NewVBox(adminStatus, restartBtn)
	} else {
		// Linux / macOS
		adminLabel := "Run as administrator"
		if runtime.GOOS == "linux" {
			if core.HasNetAdminCapability(core.GetCorePath()) {
				adminLabel = "Run as root (setcap active, TUN without root)"
			} else {
				adminLabel = "Run as root (pkexec, for TUN)"
			}
		}
		g.adminCheck = widget.NewCheck(adminLabel, func(checked bool) {
			g.cfg.SetRunAsAdmin(checked)
			g.manager.SetElevated(checked)
			if err := g.cfg.Save(); err != nil {
				g.log("Failed to save admin setting: " + err.Error())
			} else {
				g.log("Admin mode: " + fmt.Sprintf("%v", checked))
			}
		})
		g.adminCheck.SetChecked(g.cfg.RunAsAdmin)

		g.privilegeText = canvas.NewText("", color.Black)
		g.privilegeText.TextSize = theme.TextSize()
		g.refreshPrivilegeStatus()

		var setcapRow fyne.CanvasObject
		if runtime.GOOS == "linux" {
			setcapBtn := widget.NewButton("Apply setcap (TUN without root)", func() {
				go func() {
					modal := g.showInfiniteDialog("Applying setcap...")
					err := core.SetNetAdminCapabilityGUI(core.GetCorePath())
					fyne.Do(func() { modal.Hide() })
					if err != nil {
						g.log("setcap failed: " + err.Error())
						g.log("Tip: run manually: sudo setcap cap_net_admin=+ep ./sing-box")
					} else {
						g.log("setcap applied successfully.")
						g.refreshPrivilegeStatus()
					}
				}()
			})
			setcapRow = setcapBtn
		} else {
			setcapRow = widget.NewLabel("setcap not available on this OS")
		}
		privilegesContent = container.NewVBox(g.adminCheck, g.privilegeText, setcapRow)
	}

	// Log limit
	g.logLimitEntry = widget.NewEntry()
	g.logLimitEntry.SetText(fmt.Sprintf("%d", g.cfg.GetLogLimit()))
	g.logLimitEntry.OnSubmitted = func(s string) {
		var v int
		if _, err := fmt.Sscanf(s, "%d", &v); err == nil && v >= 0 {
			g.cfg.SetLogLimit(v)
			_ = g.cfg.Save()
			g.log("Log limit set to " + s)
		}
	}
	logLimitRow := container.NewBorder(nil, nil, widget.NewLabel("Log limit (lines, 0=unlimited):"), widget.NewButton("Save", func() {
		g.logLimitEntry.OnSubmitted(g.logLimitEntry.Text)
	}), g.logLimitEntry)

	// Show logs toggles
	g.showLogsCheck = widget.NewCheck("Show logs", func(checked bool) {
		g.cfg.SetShowLogs(checked)
		_ = g.cfg.Save()
	})
	g.showLogsCheck.SetChecked(g.cfg.GetShowLogs())

	g.showCoreLogsCheck = widget.NewCheck("Show core logs", func(checked bool) {
		g.cfg.SetShowCoreLogs(checked)
		_ = g.cfg.Save()
	})
	g.showCoreLogsCheck.SetChecked(g.cfg.GetShowCoreLogs())

	// Core management
	g.versionText = canvas.NewText("Core: not installed", color.Black)
	g.versionText.TextSize = theme.TextSize()
	g.latestText = canvas.NewText("Latest: checking...", color.Black)
	g.latestText.TextSize = theme.TextSize()

	downloadBtn := widget.NewButton("Download latest sing-box core", func() {
		go g.onDownloadCore()
	})

	checkBtn := widget.NewButton("Check latest version", func() {
		go func() {
			modal := g.showInfiniteDialog("Checking latest version...")
			ver, err := core.GetLatestVersion()
			fyne.Do(func() { modal.Hide() })
			if err != nil {
				g.log("Check failed: " + err.Error())
				return
			}
			g.latestVersion = ver
			fyne.Do(func() {
				g.latestText.Text = "Latest: v" + ver
				g.latestText.Color = colGreen
				g.latestText.Refresh()
			})
			g.showVersionInfoDialog(ver)
		}()
	})

	content := container.NewVBox(
		widget.NewLabelWithStyle("System", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		infoLbl,
		buildLbl,
		repoLink,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("Updates", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		intervalRow,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("Privileges", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		privilegesContent,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("Logging", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		logLimitRow,
		g.showLogsCheck,
		g.showCoreLogsCheck,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("Core", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(g.versionText, g.latestText),
		downloadBtn,
		checkBtn,
	)

	return container.NewTabItem("Settings", container.NewScroll(content))
}
