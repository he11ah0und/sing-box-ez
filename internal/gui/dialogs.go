package gui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

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
	currentLbl := widget.NewLabel("Current: " + currentText)
	latestLbl := widget.NewLabel("Latest: " + info.Latest)

	changelog := widget.NewRichTextFromMarkdown(info.LatestBody)
	changelog.Wrapping = fyne.TextWrapWord

	scroll := container.NewScroll(changelog)
	scroll.SetMinSize(fyne.NewSize(480, 280))

	whiteSep := canvas.NewRectangle(color.White)
	sepLine := container.New(layout.NewGridWrapLayout(fyne.NewSize(480, 1)), whiteSep)

	content := container.NewVBox(
		currentLbl,
		latestLbl,
		sepLine,
		widget.NewLabel("Changelog:"),
		scroll,
	)

	confirm := dialog.NewCustomConfirm("Update Available", "Update", "Ignore", content, func(update bool) {
		if update {
			go g.doSelfUpdate(info.AssetURL)
		}
	}, g.window)
	confirm.Show()
}

func (g *GUI) doSelfUpdate(assetURL string) {
	if assetURL == "" {
		g.log("Self-update: no matching asset for this system")
		dialog.ShowError(fmt.Errorf("no matching asset found"), g.window)
		return
	}
	modal := g.showInfiniteDialog("Downloading update...")
	if err := updater.ApplyUpdate(assetURL); err != nil {
		fyne.Do(func() { modal.Hide() })
		g.log("Self-update failed: " + err.Error())
		dialog.ShowError(err, g.window)
	}
}
