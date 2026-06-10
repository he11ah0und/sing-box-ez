package fynegui

import (
	"image/color"

	"sing-box-ez/internal/i18n"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (g *GUI) buildCoreTab() *container.TabItem {
	// --- Core block ---
	g.versionText = canvas.NewText(i18n.T("core.version.not_installed"), color.Black)
	g.versionText.TextSize = theme.TextSize()

	downloadBtn := widget.NewButton(i18n.T("core.btn.download"), func() {
		go g.onDownloadCore()
	})

	checkBtn := widget.NewButton(i18n.T("core.btn.check"), func() {
		go func() {
			modal := g.showInfiniteDialog(i18n.T("progress.checking_version"))
			ver, err := g.ctrl.GetLatestCoreVersionWithLog()
			fyne.Do(func() { modal.Hide() })
			if err != nil {
				return
			}
			g.latestVersion = ver
			g.showVersionInfoDialog(ver)
		}()
	})

	g.coreAutoRestartCheck = widget.NewCheck(i18n.T("core.auto_restart"), func(checked bool) {
		g.cfg.SetCoreAutoRestart(checked)
		_ = g.cfg.Save()
	})
	g.coreAutoRestartCheck.SetChecked(g.cfg.GetCoreAutoRestart())

	g.showCoreLogsCheck = widget.NewCheck(i18n.T("core.watch_core_logs"), func(checked bool) {
		g.cfg.SetWatchCoreLogs(checked)
		_ = g.cfg.Save()
	})
	g.showCoreLogsCheck.SetChecked(g.cfg.GetWatchCoreLogs())

	// --- Privileges block ---
	state := g.ctrl.GetPrivilegeTabState()
	var privilegesContent fyne.CanvasObject
	if state.Mode == "windows" {
		adminStatus := canvas.NewText(state.AdminStatusText, color.Black)
		adminStatus.TextSize = theme.TextSize()
		if state.AdminStatusColor == "green" {
			adminStatus.Color = colGreen
		} else {
			adminStatus.Color = colYellow
		}
		restartBtn := widget.NewButton(i18n.T("core.btn.restart_admin"), func() {
			go func() {
				if err := g.ctrl.RestartAsAdminWithLog(); err != nil {
					return
				}
				fyne.Do(func() { g.window.Close() })
			}()
		})
		if !state.ShowRestartAdminBtn {
			restartBtn.Disable()
		}
		privilegesContent = container.NewVBox(adminStatus, restartBtn)
	} else {
		modeOptions := []string{i18n.T("core.mode.admin"), i18n.T("core.mode.setcap")}
		modeSelect := widget.NewSelect(modeOptions, nil)

		currentMode := i18n.T("core.mode.admin")
		if state.HasSetcap {
			currentMode = i18n.T("core.mode.setcap")
		}
		modeSelect.SetSelected(currentMode)

		modeSelect.OnChanged = func(selected string) {
			if selected == i18n.T("core.mode.admin") {
				_ = g.ctrl.SetRunAsAdminWithLog(true)
				return
			}
			// Switching to setcap
			if state.HasSetcap {
				_ = g.ctrl.SetRunAsAdminWithLog(false)
				return
			}
			dialog.ShowConfirm(i18n.T("core.btn.apply_setcap"), i18n.T("core.mode.setcap_prompt"), func(apply bool) {
				if !apply {
					modeSelect.SetSelected(i18n.T("core.mode.admin"))
					return
				}
				go func() {
					modal := g.showInfiniteDialog(i18n.T("progress.applying_setcap"))
					err := g.ctrl.ApplySetcapWithLog()
					fyne.Do(func() { modal.Hide() })
					if err == nil {
						_ = g.ctrl.SetRunAsAdminWithLog(false)
					} else {
						fyne.Do(func() {
							modeSelect.SetSelected(i18n.T("core.mode.admin"))
						})
					}
				}()
			}, g.window)
		}

		privilegesContent = container.NewVBox(modeSelect)
	}

	content := container.NewVBox(
		widget.NewLabelWithStyle(i18n.T("tab.core"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		g.versionText,
		downloadBtn,
		checkBtn,
		g.coreAutoRestartCheck,
		g.showCoreLogsCheck,
		widget.NewSeparator(),

		widget.NewLabelWithStyle(i18n.T("core.privileges.title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		privilegesContent,
	)

	return container.NewTabItem(i18n.T("tab.core"), container.NewScroll(content))
}
