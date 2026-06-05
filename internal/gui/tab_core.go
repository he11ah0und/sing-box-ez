package gui

import (
	"fmt"
	"image/color"
	"runtime"

	"sing-box-ez/internal/core"
	"sing-box-ez/internal/i18n"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (g *GUI) buildCoreTab() *container.TabItem {
	// --- Core block ---
	g.versionText = canvas.NewText(i18n.T("core.version.not_installed"), color.Black)
	g.versionText.TextSize = theme.TextSize()
	g.latestText = canvas.NewText(i18n.T("core.latest.checking"), color.Black)
	g.latestText.TextSize = theme.TextSize()

	downloadBtn := widget.NewButton(i18n.T("core.btn.download"), func() {
		go g.onDownloadCore()
	})

	checkBtn := widget.NewButton(i18n.T("core.btn.check"), func() {
		go func() {
			modal := g.showInfiniteDialog(i18n.T("progress.checking_version"))
			ver, err := core.GetLatestVersion()
			fyne.Do(func() { modal.Hide() })
			if err != nil {
				g.log("Check failed: " + err.Error())
				return
			}
			g.latestVersion = ver
			fyne.Do(func() {
				g.latestText.Text = i18n.T("core.latest.prefix") + ver
				g.latestText.Color = colGreen
				g.latestText.Refresh()
			})
			g.showVersionInfoDialog(ver)
		}()
	})

	g.coreAutoRestartCheck = widget.NewCheck(i18n.T("core.auto_restart"), func(checked bool) {
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
			adminStatus.Text = i18n.T("core.privileges.admin")
			adminStatus.Color = colGreen
		} else {
			adminStatus.Text = i18n.T("core.privileges.user")
			adminStatus.Color = colYellow
		}
		restartBtn := widget.NewButton(i18n.T("core.btn.restart_admin"), func() {
			go g.restartAsAdmin()
		})
		if core.IsAdmin() {
			restartBtn.Disable()
		}
		privilegesContent = container.NewVBox(adminStatus, restartBtn)
	} else {
		// Linux / macOS
		adminLabel := i18n.T("core.admin.label")
		if runtime.GOOS == "linux" {
			if core.HasNetAdminCapability(core.GetCorePath()) {
				adminLabel = i18n.T("core.admin.label_root_setcap")
			} else {
				adminLabel = i18n.T("core.admin.label_root_pkexec")
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
			setcapBtn := widget.NewButton(i18n.T("core.btn.apply_setcap"), func() {
				go func() {
					modal := g.showInfiniteDialog(i18n.T("progress.applying_setcap"))
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
			setcapRow = widget.NewLabel(i18n.T("core.setcap.unavailable"))
		}
		privilegesContent = container.NewVBox(g.adminCheck, g.privilegeText, setcapRow)
	}

	content := container.NewVBox(
		widget.NewLabelWithStyle(i18n.T("tab.core"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(g.versionText, g.latestText),
		downloadBtn,
		checkBtn,
		g.coreAutoRestartCheck,
		widget.NewSeparator(),

		widget.NewLabelWithStyle(i18n.T("core.privileges.title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		privilegesContent,
	)

	return container.NewTabItem(i18n.T("tab.core"), container.NewScroll(content))
}
