package gui

import (
	"fmt"
	"image/color"
	"runtime"

	"sing-box-ez/internal/core"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (g *GUI) buildCoreTab() *container.TabItem {
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

	content := container.NewVBox(
		widget.NewLabelWithStyle("Core", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(g.versionText, g.latestText),
		downloadBtn,
		checkBtn,
		g.coreAutoRestartCheck,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("Privileges", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		privilegesContent,
	)

	return container.NewTabItem("Core", container.NewScroll(content))
}
