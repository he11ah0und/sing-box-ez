package gui

import (
	"fmt"
	"image/color"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/i18n"
	"sing-box-ez/internal/updater"
	"sing-box-ez/internal/version"
)

// ---------------------------------------------------------------------------
// Progress / loading dialogs
// ---------------------------------------------------------------------------

func (g *GUI) showProgressDialog(title string) (dialog.Dialog, *widget.ProgressBar) {
	var d dialog.Dialog
	var progress *widget.ProgressBar
	fyne.DoAndWait(func() {
		progress = widget.NewProgressBar()
		cancelBtn := widget.NewButton(i18n.T("dialog.btn.cancel"), func() {
			d.Hide()
		})
		content := container.NewVBox(widget.NewLabel(title), progress, cancelBtn)
		d = dialog.NewCustomWithoutButtons(title, content, g.window)
		d.Show()
	})
	return d, progress
}

func (g *GUI) showInfiniteDialog(title string) dialog.Dialog {
	var d dialog.Dialog
	fyne.DoAndWait(func() {
		progress := widget.NewProgressBarInfinite()
		cancelBtn := widget.NewButton(i18n.T("dialog.btn.cancel"), func() {
			d.Hide()
		})
		content := container.NewVBox(widget.NewLabel(title), progress, cancelBtn)
		d = dialog.NewCustomWithoutButtons(title, content, g.window)
		d.Show()
	})
	return d
}

// ---------------------------------------------------------------------------
// Core update dialogs
// ---------------------------------------------------------------------------

func (g *GUI) showUpdatePrompt(latest, current string) {
	fyne.DoAndWait(func() {
		msg := fmt.Sprintf(i18n.T("dialog.core_update.msg"), current, latest)
		confirm := dialog.NewConfirm(i18n.T("dialog.core_update.title"), msg, func(update bool) {
			if update {
				go g.onDownloadCore()
			}
		}, g.window)
		confirm.SetConfirmText(i18n.T("dialog.btn.update"))
		confirm.SetDismissText(i18n.T("dialog.btn.ignore"))
		confirm.Show()
	})
}

func (g *GUI) showVersionInfoDialog(latest string) {
	fyne.DoAndWait(func() {
		currentVer, err := core.GetCoreVersion(core.GetCorePath())
		var content *fyne.Container
		if err != nil || currentVer == "" {
			content = container.NewVBox(
				widget.NewLabel(i18n.T("dialog.version_check.core_not_installed")),
				widget.NewLabel(i18n.T("dialog.version_check.latest") + latest),
			)
		} else {
			currentLbl := widget.NewLabel(i18n.T("dialog.version_check.current") + currentVer)
			latestLbl := widget.NewLabel(i18n.T("dialog.version_check.latest") + latest)
			if currentVer == latest {
				status := canvas.NewText(i18n.T("dialog.version_check.latest_installed"), colGreen)
				content = container.NewVBox(currentLbl, latestLbl, status)
			} else {
				status := canvas.NewText(i18n.T("dialog.version_check.update_available"), colOrange)
				content = container.NewVBox(currentLbl, latestLbl, status)
			}
		}
		d := dialog.NewCustom(i18n.T("dialog.version_check.title"), i18n.T("about.dialog.close"), content, g.window)
		d.Show()
	})
}

func (g *GUI) showDownloadCompleteDialog(ver, path string) {
	fyne.DoAndWait(func() {
		content := container.NewVBox(
			widget.NewLabel(fmt.Sprintf(i18n.T("dialog.download_complete.msg"), ver, path)),
		)
		d := dialog.NewCustom(i18n.T("dialog.download_complete.title"), i18n.T("about.dialog.close"), content, g.window)
		d.Show()
	})
}

// ---------------------------------------------------------------------------
// Self-update dialogs
// ---------------------------------------------------------------------------

func (g *GUI) showSelfUpdateDialog(info *updater.UpdateInfo) {
	currentText := version.Commit
	if currentText == "unknown" || currentText == "" {
		currentText = info.Current
	}

	var currentDateStr, latestDateStr, diffStr string
	if dt, err := version.BuildDateTime(); err == nil {
		currentDateStr = dt.Format("2006-01-02 15:04:05") + " (" + version.HumanDuration(dt) + ")"
	} else {
		currentDateStr = i18n.T("dialog.unknown")
	}
	if !info.LatestDate.IsZero() {
		lt := info.LatestDate.Local()
		latestDateStr = lt.Format("2006-01-02 15:04:05") + " (" + version.HumanDuration(info.LatestDate) + ")"
		if dt, err := version.BuildDateTime(); err == nil {
			diff := info.LatestDate.Sub(dt)
			if diff < 0 {
				diff = -diff
			}
			diffStr = fmt.Sprintf(i18n.T("dialog.self_update.behind"), humanDuration(diff))
		}
	} else {
		latestDateStr = i18n.T("dialog.unknown")
	}

	currentLbl := widget.NewLabel(i18n.T("dialog.self_update.current") + currentText + "\n  " + currentDateStr)
	latestLbl := widget.NewLabel(i18n.T("dialog.self_update.latest") + info.Latest + "\n  " + latestDateStr)

	changelog := widget.NewRichTextFromMarkdown(info.LatestBody)
	changelog.Wrapping = fyne.TextWrapWord

	scroll := container.NewScroll(changelog)
	scroll.SetMinSize(fyne.NewSize(480, 280))

	whiteSep := canvas.NewRectangle(color.White)
	sepLine := container.New(layout.NewGridWrapLayout(fyne.NewSize(480, 1)), whiteSep)

	items := []fyne.CanvasObject{currentLbl, latestLbl}
	if diffStr != "" {
		items = append(items, widget.NewLabel(diffStr))
	}
	items = append(items, sepLine, widget.NewLabel(i18n.T("dialog.self_update.changelog")), scroll)

	content := container.NewVBox(items...)

	confirm := dialog.NewCustomConfirm(i18n.T("dialog.self_update.title"), i18n.T("dialog.btn.update"), i18n.T("dialog.btn.ignore"), content, func(update bool) {
		if update {
			go g.doSelfUpdate(info.AssetURL)
		}
	}, g.window)
	confirm.Show()
}

// humanDuration formats a time.Duration into a human-readable string.
func humanDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days < 30 {
		return fmt.Sprintf("%dd", days)
	}
	months := days / 30
	if months < 12 {
		return fmt.Sprintf("%dmo", months)
	}
	return fmt.Sprintf("%dy", months/12)
}

func (g *GUI) doSelfUpdate(assetURL string) {
	if assetURL == "" {
		g.log("Self-update: no matching asset for this system")
		dialog.ShowError(fmt.Errorf("%s", i18n.T("dialog.error.no_matching_asset")), g.window)
		return
	}
	progressModal, progress := g.showProgressDialog(i18n.T("progress.downloading_update"))
	if err := updater.ApplyUpdate(assetURL, func(d, t int64) {
		fyne.Do(func() {
			progress.SetValue(float64(d) / float64(t))
		})
	}); err != nil {
		fyne.Do(func() { progressModal.Hide() })
		g.log("Self-update failed: " + err.Error())
		dialog.ShowError(err, g.window)
	}
}

// ---------------------------------------------------------------------------
// First-run wizard
// ---------------------------------------------------------------------------

func (g *GUI) showFirstRunDialog() {
	var d dialog.Dialog

	coreInstalled := core.CoreExists()
	statusText := widget.NewLabel(i18n.T("first_run.welcome_title"))
	var versionText *widget.Label
	if coreInstalled {
		statusText.SetText(i18n.T("first_run.core.installed"))
		if ver, err := core.GetCoreVersion(core.GetCorePath()); err == nil && ver != "" {
			versionText = widget.NewLabel(fmt.Sprintf(i18n.T("first_run.core.version"), ver))
		}
	} else {
		statusText.SetText(i18n.T("first_run.core.not_installed"))
	}

	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("https://example.com/config.json")

	downloadBtn := widget.NewButton(i18n.T("first_run.btn.download_core"), func() {
		go g.onDownloadCore()
	})
	if coreInstalled {
		downloadBtn.Disable()
	}

	addBtn := widget.NewButton(i18n.T("first_run.btn.add_config"), func() {
		url := urlEntry.Text
		if url == "" {
			g.log("First run: empty config URL")
			return
		}
		name := "default"
		rec := config.ConfigRecord{
			Name:                name,
			URL:                 url,
			UpdateIntervalHours: g.cfg.UpdateIntervalHours,
			Parent:              "user",
		}
		g.cfg.AddConfig(rec)
		g.cfg.SetActiveName(name)
		g.cfg.SetFirstRunDone(true)
		_ = g.cfg.Save()
		g.manager.SetConfigURL(url)
		g.manager.SetConfigName(name)
		g.refreshConfigData()
		fyne.Do(func() {
			g.configTable.Refresh()
			g.refreshActiveLabel()
			g.updateButtons()
		})
		g.log("First config added: " + name)
		fyne.Do(func() { d.Hide() })
	})

	items := []fyne.CanvasObject{
		widget.NewLabelWithStyle(i18n.T("first_run.welcome"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(i18n.T("first_run.description")),
		widget.NewSeparator(),
		statusText,
	}
	if versionText != nil {
		items = append(items, versionText)
	}
	items = append(items,
		downloadBtn,
		widget.NewSeparator(),
		widget.NewLabel(i18n.T("first_run.config_url")),
		urlEntry,
		addBtn,
	)
	content := container.NewVBox(items...)

	d = dialog.NewCustomWithoutButtons(i18n.T("first_run.title"), content, g.window)
	d.Resize(fyne.NewSize(480, 320))
	d.Show()
}

func (g *GUI) showPrivilegeDialog() {
	if runtime.GOOS == "darwin" {
		return // macOS handles elevation via osascript at runtime
	}

	var d dialog.Dialog
	content := container.NewVBox()

	if runtime.GOOS == "windows" {
		content.Add(widget.NewLabel(i18n.T("dialog.privileges.msg_windows")))
		btn := widget.NewButton(i18n.T("dialog.privileges.btn_restart_admin"), func() {
			d.Hide()
			g.restartAsAdmin()
		})
		content.Add(btn)
	} else if runtime.GOOS == "linux" {
		content.Add(widget.NewLabel(i18n.T("dialog.privileges.msg_linux")))
		setcapBtn := widget.NewButton(i18n.T("dialog.privileges.btn_setcap"), func() {
			d.Hide()
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
		adminBtn := widget.NewButton(i18n.T("dialog.privileges.btn_run_as_admin"), func() {
			d.Hide()
			g.cfg.SetRunAsAdmin(true)
			g.manager.SetElevated(true)
			if err := g.cfg.Save(); err != nil {
				g.log("Failed to save admin setting: " + err.Error())
			} else {
				g.log("Run as admin enabled. Please click Start again.")
				g.refreshPrivilegeStatus()
			}
		})
		content.Add(setcapBtn)
		content.Add(adminBtn)
	}

	cancelBtn := widget.NewButton(i18n.T("dialog.btn.cancel"), func() {
		d.Hide()
	})

	d = dialog.NewCustom(i18n.T("dialog.privileges.title"), "", container.NewVBox(content, cancelBtn), g.window)
	d.Resize(fyne.NewSize(400, 0))
	fyne.Do(func() { d.Show() })
}
