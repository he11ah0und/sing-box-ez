package gui

import (
	"fmt"
	"image/color"
	"net/url"
	"os"
	"runtime"

	"sing-box-ez/internal/core"
	"sing-box-ez/internal/paths"
	"sing-box-ez/internal/updater"
	"sing-box-ez/internal/version"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
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

func buildInfoText() string {
	return version.BuildFlags()
}

func (g *GUI) buildSettingsTab() *container.TabItem {
	// --- Logging block ---
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

	// --- Core block ---
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

	g.coreAutoRestartCheck = widget.NewCheck("Auto-restart core on fatal errors", func(checked bool) {
		g.cfg.SetCoreAutoRestart(checked)
		_ = g.cfg.Save()
	})
	g.coreAutoRestartCheck.SetChecked(g.cfg.GetCoreAutoRestart())

	// --- Config block ---
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

	// --- Plugins block ---
	g.pluginsEnabledCheck = widget.NewCheck("Plugins feature", func(checked bool) {
		g.cfg.SetPluginsEnabled(checked)
		_ = g.cfg.Save()
		if !checked {
			g.pluginsDeveloperCheck.SetChecked(false)
			g.cfg.SetPluginsDeveloper(false)
			g.pluginsDeveloperCheck.Disable()
		} else {
			g.pluginsDeveloperCheck.Enable()
		}
	})
	g.pluginsEnabledCheck.SetChecked(g.cfg.GetPluginsEnabled())

	g.pluginsDeveloperCheck = widget.NewCheck("Plugins developer", func(checked bool) {
		g.cfg.SetPluginsDeveloper(checked)
		_ = g.cfg.Save()
	})
	g.pluginsDeveloperCheck.SetChecked(g.cfg.GetPluginsDeveloper())
	if !g.cfg.GetPluginsEnabled() {
		g.pluginsDeveloperCheck.Disable()
	}

	// --- Privileges block ---
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

	// --- System block ---
	infoLbl := widget.NewLabel(buildInfoText())

	buildURLStr := version.RepoURL + "/releases"
	if version.Version != "dev" && version.Version != "" {
		buildURLStr = version.RepoURL + "/releases/tag/" + version.Version
	}
	buildURL, _ := url.Parse(buildURLStr)
	buildLink := widget.NewHyperlink("Build: "+version.Info(), buildURL)

	repoURL, _ := url.Parse(version.RepoURL)
	repoLink := widget.NewHyperlink("he11ah0und/sing-box-ez", repoURL)

	notesBtn := widget.NewButton("Show release notes", func() {
		go func() {
			modal := g.showInfiniteDialog("Fetching release notes...")
			releases, err := updater.GetReleases()
			fyne.Do(func() { modal.Hide() })
			if err != nil {
				g.log("Failed to fetch release notes: " + err.Error())
				return
			}
			var latest *updater.Release
			for i := range releases {
				if !releases[i].Prerelease {
					latest = &releases[i]
					break
				}
			}
			if latest == nil {
				g.log("No stable releases found")
				return
			}
			fyne.Do(func() {
				notesLabel := widget.NewLabel(latest.Body)
				notesLabel.Wrapping = fyne.TextWrapWord
				scroll := container.NewScroll(notesLabel)
				scroll.SetMinSize(fyne.NewSize(500, 400))
				d := dialog.NewCustom("Release notes: "+latest.TagName, "Close", scroll, g.window)
				d.Show()
			})
		}()
	})

	openDataBtn := widget.NewButton("Open data folder", func() {
		if err := paths.OpenDataDir(); err != nil {
			g.log("Failed to open data folder: " + err.Error())
		}
	})

	selfUpdateBtn := widget.NewButton("Check for updates", func() {
		go func() {
			modal := g.showInfiniteDialog("Checking for updates...")
			info, err := updater.CheckUpdate(version.Version)
			fyne.Do(func() { modal.Hide() })
			if err != nil {
				g.log("Update check failed: " + err.Error())
				return
			}
			if info.ReleaseCount == 0 {
				g.log("Already on latest version: " + info.Current)
				return
			}
			g.log(fmt.Sprintf("Update available: %s → %s (%d releases behind)", info.Current, info.Latest, info.ReleaseCount))
			fyne.Do(func() {
				g.showSelfUpdateDialog(info)
			})
		}()
	})

	// --- Assemble content ---
	content := container.NewVBox(
		widget.NewLabelWithStyle("Logging", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		logLimitRow,
		g.showLogsCheck,
		g.showCoreLogsCheck,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("Core", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(g.versionText, g.latestText),
		downloadBtn,
		checkBtn,
		g.coreAutoRestartCheck,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("Config", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		intervalRow,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("Plugins", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		g.pluginsEnabledCheck,
		g.pluginsDeveloperCheck,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("Privileges", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		privilegesContent,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("System", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		infoLbl,
		buildLink,
		repoLink,
		notesBtn,
		openDataBtn,
		selfUpdateBtn,
	)

	return container.NewTabItem("Settings", container.NewScroll(content))
}
