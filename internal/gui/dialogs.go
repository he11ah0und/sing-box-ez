package gui

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
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
		cancelBtn := widget.NewButton("Cancel", func() {
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
		cancelBtn := widget.NewButton("Cancel", func() {
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
		msg := fmt.Sprintf("A new version of sing-box core is available.\n\nCurrent: v%s\nLatest: v%s\n\nUpdate now?", current, latest)
		confirm := dialog.NewConfirm("Core Update Available", msg, func(update bool) {
			if update {
				go g.onDownloadCore()
			}
		}, g.window)
		confirm.SetConfirmText("Update")
		confirm.SetDismissText("Ignore")
		confirm.Show()
	})
}

func (g *GUI) showVersionInfoDialog(latest string) {
	fyne.DoAndWait(func() {
		currentVer, err := core.GetCoreVersion(core.GetCorePath())
		var content *fyne.Container
		if err != nil || currentVer == "" {
			content = container.NewVBox(
				widget.NewLabel("Core is not installed."),
				widget.NewLabel("Latest version: v"+latest),
			)
		} else {
			currentLbl := widget.NewLabel("Current: v" + currentVer)
			latestLbl := widget.NewLabel("Latest: v" + latest)
			if currentVer == latest {
				status := canvas.NewText("✓ Latest version installed", colGreen)
				content = container.NewVBox(currentLbl, latestLbl, status)
			} else {
				status := canvas.NewText("✗ Update available", colOrange)
				content = container.NewVBox(currentLbl, latestLbl, status)
			}
		}
		d := dialog.NewCustom("Version Check", "Close", content, g.window)
		d.Show()
	})
}

func (g *GUI) showDownloadCompleteDialog(ver, path string) {
	fyne.DoAndWait(func() {
		content := container.NewVBox(
			widget.NewLabel("Sing-box core v"+ver+" downloaded successfully."),
			widget.NewLabel("Location: "+path),
		)
		d := dialog.NewCustom("Download Complete", "Close", content, g.window)
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
		currentDateStr = "unknown"
	}
	if !info.LatestDate.IsZero() {
		lt := info.LatestDate.Local()
		latestDateStr = lt.Format("2006-01-02 15:04:05") + " (" + version.HumanDuration(info.LatestDate) + ")"
		if dt, err := version.BuildDateTime(); err == nil {
			diff := info.LatestDate.Sub(dt)
			if diff < 0 {
				diff = -diff
			}
			diffStr = fmt.Sprintf("Behind by: %s", humanDuration(diff))
		}
	} else {
		latestDateStr = "unknown"
	}

	currentLbl := widget.NewLabel("Current: " + currentText + "\n  " + currentDateStr)
	latestLbl := widget.NewLabel("Latest: " + info.Latest + "\n  " + latestDateStr)

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
	items = append(items, sepLine, widget.NewLabel("Changelog:"), scroll)

	content := container.NewVBox(items...)

	confirm := dialog.NewCustomConfirm("Update Available", "Update", "Ignore", content, func(update bool) {
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
		dialog.ShowError(fmt.Errorf("no matching asset found"), g.window)
		return
	}
	progressModal, progress := g.showProgressDialog("Downloading update...")
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
	done := make(chan struct{})
	var d dialog.Dialog

	coreInstalled := core.CoreExists()
	statusText := widget.NewLabel("Welcome to Sing-box EZ!")
	if coreInstalled {
		statusText.SetText("✓ sing-box core is already installed.")
	} else {
		statusText.SetText("sing-box core is not installed.")
	}

	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("https://example.com/config.json")

	downloadBtn := widget.NewButton("Download sing-box core", func() {
		go g.onDownloadCore()
	})
	if coreInstalled {
		downloadBtn.Disable()
	}

	addBtn := widget.NewButton("Add config", func() {
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
		close(done)
	})

	content := container.NewVBox(
		widget.NewLabelWithStyle("Welcome", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("To get started, download sing-box core and add your first config."),
		widget.NewSeparator(),
		statusText,
		downloadBtn,
		widget.NewSeparator(),
		widget.NewLabel("Config URL"),
		urlEntry,
		addBtn,
	)

	d = dialog.NewCustomWithoutButtons("First Run", content, g.window)
	d.Resize(fyne.NewSize(480, 320))
	d.Show()
	<-done
}
