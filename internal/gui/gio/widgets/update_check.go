package widgets

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"sync"
	"time"

	"gio.tools/icons"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// UpdateCheckInfo holds the result of an update check.
type UpdateCheckInfo struct {
	Current    string
	Latest     string
	HasUpdate  bool
	IsDevBuild bool      // current build is newer than the latest release
	Body       string    // release notes / commit description
	Date       time.Time // release date
}

// UpdateCheck is a reusable widget that checks for updates and installs them.
//
// Layout renders a horizontal row: a fill-width main button on the left that
// shows the current version / update status, and a smaller icon button on the
// right that triggers a new update check.
//
// The main button is active only when an update is available. The check button
// is inactive only while a check is already running.
type UpdateCheck struct {
	th     *material.Theme
	dialog DialogProvider

	checkFn  func(ctx context.Context) (UpdateCheckInfo, error)
	updateFn func(ctx context.Context, onProgress func(downloaded, total int64)) error

	currentVersion          string
	currentVersionFormatter func(current string) string
	updateLabel             string
	checkingLabel           string
	updatingLabel           string
	upToDateLabel           string
	availableFormatter       func(latest string) string
	devBuildFormatter        func(current, latest string) string
	devBuildConfirmFormatter func(current, latest string) (title, body string)
	detailsTitle             string
	detailsFormatter        func(info UpdateCheckInfo) string
	checkIcon               *widget.Icon

	mainBtn  widget.Clickable
	checkBtn widget.Clickable

	checked    bool
	checking   bool
	updating   bool
	hasUpdate  bool
	isDevBuild bool
	lastInfo   UpdateCheckInfo

	progressMu sync.Mutex
	progress   float32
}

// NewUpdateCheck creates an update-check widget.
//
// currentVersion is shown on the main button before any check. updateLabel is
// used for dialog titles during the update process.
func NewUpdateCheck(
	th *material.Theme,
	dialog DialogProvider,
	currentVersion, updateLabel string,
	checkFn func(ctx context.Context) (UpdateCheckInfo, error),
	updateFn func(ctx context.Context, onProgress func(downloaded, total int64)) error,
) *UpdateCheck {
	return &UpdateCheck{
		th:                      th,
		dialog:                  dialog,
		currentVersion:          currentVersion,
		currentVersionFormatter: func(current string) string { return current },
		updateLabel:             updateLabel,
		checkingLabel:           "Checking update",
		updatingLabel:           "Updating...",
		upToDateLabel:           "Already latest version",
		devBuildFormatter: func(current, latest string) string {
			return fmt.Sprintf("Dev build: current %s, remote %s", current, latest)
		},
		devBuildConfirmFormatter: func(current, latest string) (string, string) {
			return "Dev build update",
				fmt.Sprintf("You are on a dev build (%s) newer than the latest release (%s). Updating will downgrade. Continue?", current, latest)
		},
		availableFormatter: func(latest string) string {
			if latest == "" {
				return "Update"
			}
			return "Update to " + latest
		},
		detailsTitle: "Update Available",
		checkIcon:    icons.ActionCached,
		checkFn:      checkFn,
		updateFn:     updateFn,
	}
}

// SetCheckingLabel overrides the text shown while checking.
func (u *UpdateCheck) SetCheckingLabel(label string) {
	u.checkingLabel = label
}

// SetUpdatingLabel overrides the text shown while updating.
func (u *UpdateCheck) SetUpdatingLabel(label string) {
	u.updatingLabel = label
}

// SetUpToDateLabel overrides the text shown when no update is available.
// If set to an empty string, the current version formatter is used instead.
func (u *UpdateCheck) SetUpToDateLabel(label string) {
	u.upToDateLabel = label
}

// SetCurrentVersionFormatter sets the formatter used for the current version
// label before any check and when no update is available (if upToDateLabel is
// empty).
func (u *UpdateCheck) SetCurrentVersionFormatter(fn func(current string) string) {
	u.currentVersionFormatter = fn
}

// SetAvailableFormatter overrides the main button label builder when an update
// is available. The argument is the latest version string.
func (u *UpdateCheck) SetAvailableFormatter(fn func(latest string) string) {
	u.availableFormatter = fn
}

// SetDevBuildFormatter overrides the label shown when the current build is
// newer than the latest release.
func (u *UpdateCheck) SetDevBuildFormatter(fn func(current, latest string) string) {
	u.devBuildFormatter = fn
}

// SetDevBuildConfirmFormatter overrides the title/body of the confirmation
// dialog shown before updating from a dev build to an older release.
func (u *UpdateCheck) SetDevBuildConfirmFormatter(fn func(current, latest string) (title, body string)) {
	u.devBuildConfirmFormatter = fn
}

// SetDetailsTitle sets the title of the dialog shown when an update is found.
func (u *UpdateCheck) SetDetailsTitle(title string) {
	u.detailsTitle = title
}

// SetDetailsFormatter sets a custom formatter for the update-details dialog.
// The formatter receives the update info and should return markdown text.
// If nil, a default format is used.
func (u *UpdateCheck) SetDetailsFormatter(fn func(info UpdateCheckInfo) string) {
	u.detailsFormatter = fn
}

// SetCheckIcon overrides the icon used by the check button.
func (u *UpdateCheck) SetCheckIcon(icon *widget.Icon) {
	u.checkIcon = icon
}

// SetCheckFn replaces the update-check function. Used when the active channel
// changes while the widget is already created.
func (u *UpdateCheck) SetCheckFn(fn func(ctx context.Context) (UpdateCheckInfo, error)) {
	u.checkFn = fn
}

// RunCheck starts an update check using the current check function.
func (u *UpdateCheck) RunCheck() {
	u.runCheck()
}

// Layout draws the update-check row.
func (u *UpdateCheck) Layout(gtx layout.Context) layout.Dimensions {
	mainDisabled := u.checking || u.updating || (u.checked && !u.hasUpdate && !u.isDevBuild) || !u.checked
	checkDisabled := u.checking

	if u.mainBtn.Clicked(gtx) && !mainDisabled {
		go u.runUpdate()
	}
	if u.checkBtn.Clicked(gtx) && !checkDisabled {
		go u.runCheck()
	}

	mainLabel := u.currentVersionFormatter(u.currentVersion)
	if mainLabel == "" {
		mainLabel = "Update"
	}
	switch {
	case u.updating:
		mainLabel = u.updatingLabel
	case u.checking:
		mainLabel = u.checkingLabel
	case u.checked && u.hasUpdate:
		mainLabel = u.availableFormatter(u.lastInfo.Latest)
	case u.checked && u.isDevBuild:
		mainLabel = u.devBuildFormatter(u.lastInfo.Current, u.lastInfo.Latest)
	case u.checked && !u.hasUpdate:
		if u.upToDateLabel != "" {
			mainLabel = u.upToDateLabel
		} else {
			mainLabel = u.currentVersionFormatter(u.lastInfo.Current)
		}
	}

	disabledBg := color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	disabledFg := color.NRGBA{R: 220, G: 220, B: 220, A: 255}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			btn := material.Button(u.th, &u.mainBtn, mainLabel)
			if mainDisabled {
				btn.Background = disabledBg
				btn.Color = disabledFg
			}
			return btn.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Point{X: gtx.Dp(unit.Dp(8)), Y: 0}}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.IconButton(u.th, &u.checkBtn, u.checkIcon, "Check for updates")
			if checkDisabled {
				btn.Background = disabledBg
				btn.Color = disabledFg
			}
			return btn.Layout(gtx)
		}),
	)
}

func (u *UpdateCheck) runCheck() {
	u.checking = true
	u.dialog.ShowLoading(u.checkingLabel)
	info, err := u.checkFn(context.Background())
	u.dialog.HideLoading()
	u.checking = false
	u.checked = true
	u.lastInfo = info

	if err != nil {
		u.hasUpdate = false
		u.dialog.Show(u.detailsTitle, "Update check failed: "+err.Error())
		return
	}

	u.hasUpdate = info.HasUpdate
	u.isDevBuild = info.IsDevBuild
	if info.HasUpdate {
		u.showDetails(info)
	} else if info.IsDevBuild {
		u.runUpdate()
	}
}

func (u *UpdateCheck) runUpdate() {
	if u.isDevBuild {
		title, _ := u.devBuildConfirmFormatter(u.lastInfo.Current, u.lastInfo.Latest)
		body := u.formatDetailsBody(u.lastInfo)
		u.dialog.ShowConfirmMarkdown(title, body, func() {
			go u.doUpdate()
		}, nil)
		return
	}
	u.doUpdate()
}

// formatDetailsBody builds the markdown body used for update details and dev-build confirmation.
func (u *UpdateCheck) formatDetailsBody(info UpdateCheckInfo) string {
	var body string
	if u.detailsFormatter != nil {
		body = u.detailsFormatter(info)
	} else {
		if info.Date.IsZero() {
			body = fmt.Sprintf("# %s\n\n%s", info.Latest, info.Body)
		} else {
			body = fmt.Sprintf("# %s\n\nReleased: %s\n\n%s",
				info.Latest, info.Date.Local().Format("2006-01-02 15:04:05"), info.Body)
		}
	}
	if body == "" {
		body = "No release notes available."
	}
	return body
}

func (u *UpdateCheck) doUpdate() {
	u.updating = true
	u.progressMu.Lock()
	u.progress = 0
	u.progressMu.Unlock()
	u.dialog.ShowLoadingWithProgress(u.updateLabel, func() float32 {
		u.progressMu.Lock()
		defer u.progressMu.Unlock()
		return u.progress
	})
	err := u.updateFn(context.Background(), func(downloaded, total int64) {
		u.progressMu.Lock()
		defer u.progressMu.Unlock()
		if total > 0 {
			u.progress = float32(downloaded) / float32(total)
		} else {
			u.progress = 0
		}
	})
	u.dialog.HideLoading()
	u.updating = false

	if err != nil {
		u.dialog.Show(u.updateLabel, "Update failed: "+err.Error())
		return
	}
	u.dialog.Show(u.updateLabel, "Update installed successfully.")
	u.checked = false
	u.hasUpdate = false
}

func (u *UpdateCheck) showDetails(info UpdateCheckInfo) {
	body := u.formatDetailsBody(info)
	u.dialog.ShowMarkdown(u.detailsTitle, body)
}
