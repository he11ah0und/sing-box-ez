package pages

import (
	"context"
	"fmt"
	"sync"

	"gio.tools/icons"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/framework/updater"
	"sing-box-ez/internal/framework/util/openurl"
	"sing-box-ez/internal/framework/version"
	"sing-box-ez/internal/gui/gio/widgets"
)

// AboutPage renders the about / system info screen.
type AboutPage struct {
	th   *material.Theme
	ctrl *core.InteractiveController

	releaseNotesBtn     widget.Clickable
	openReleaseNotesBtn widget.Clickable
	openDataBtn         widget.Clickable
	switchBranchBtn     widget.Clickable
	openRepoBtn         widget.Clickable

	selfUpdate     *widgets.UpdateCheck
	lastSelfUpdate *updater.UpdateInfo

	// Branch picker state (rendered via Dialog).
	pickerBranches []updater.Channel
	pickerBtns     []widget.Clickable
	pickerMu       sync.Mutex

	// currentBranch is the channel used by the self-update widget.
	currentBranch string

	// Dialog provider is supplied by the shell.
	dialog widgets.DialogProvider
}

// NewAboutPage creates a new about page.
func NewAboutPage(th *material.Theme, ctrl *core.InteractiveController, dialog widgets.DialogProvider) *AboutPage {
	p := &AboutPage{
		th:            th,
		ctrl:          ctrl,
		dialog:        dialog,
		currentBranch: version.Branch,
	}

	current := version.Branch
	if version.Commit != "unknown" && version.Commit != "" {
		current = version.Commit
	}

	p.selfUpdate = widgets.NewUpdateCheck(
		th, dialog,
		current,
		localengine.T("dialog", "btn", "update"),
		func(ctx context.Context) (widgets.UpdateCheckInfo, error) {
			info, err := ctrl.CheckSelfUpdateForBranch(p.currentBranch)
			if err != nil {
				return widgets.UpdateCheckInfo{}, err
			}
			p.lastSelfUpdate = info

			hasUpdate := false
			isDevBuild := false
			if info.ReleaseCount > 0 && info.Current != info.Latest {
				currentDate, dateErr := version.CommitDateTime()
				if dateErr != nil {
					hasUpdate = true
				} else {
					switch {
					case currentDate.Before(info.LatestDate):
						hasUpdate = true
					case currentDate.After(info.LatestDate):
						isDevBuild = true
					}
				}
			}

			return widgets.UpdateCheckInfo{
				Current:    info.Current,
				Latest:     info.Latest,
				HasUpdate:  hasUpdate,
				IsDevBuild: isDevBuild,
				Body:       info.LatestBody,
				Date:       info.LatestDate,
			}, nil
		},
		func(ctx context.Context, onProgress func(downloaded, total int64)) error {
			if p.lastSelfUpdate == nil {
				return fmt.Errorf("no update information")
			}
			if p.ctrl.SelfUpdater() == nil {
				return fmt.Errorf("self updater not configured")
			}
			return p.ctrl.SelfUpdater().Install(ctx, p.lastSelfUpdate, onProgress)
		},
	)
	p.selfUpdate.SetCheckingLabel(localengine.T("about", "update", "checking"))
	p.selfUpdate.SetUpdatingLabel(localengine.T("about", "update", "updating"))
	p.selfUpdate.SetUpToDateLabel("")
	p.selfUpdate.SetCurrentVersionFormatter(func(current string) string {
		return localengine.T("about", "update", "current_version") + current
	})
	p.selfUpdate.SetAvailableFormatter(func(latest string) string {
		return fmt.Sprintf(localengine.T("about", "update", "available"), latest)
	})
	p.selfUpdate.SetDevBuildFormatter(func(current, latest string) string {
		return fmt.Sprintf(localengine.T("about", "update", "dev_build"), current, latest)
	})
	p.selfUpdate.SetDevBuildConfirmFormatter(func(current, latest string) (string, string) {
		return localengine.T("about", "update", "dev_build_confirm_title"),
			fmt.Sprintf(localengine.T("about", "update", "dev_build_confirm_body"), current, latest)
	})
	p.selfUpdate.SetDetailsTitle(localengine.T("dialog", "self_update", "title"))
	p.selfUpdate.SetDetailsFormatter(func(info widgets.UpdateCheckInfo) string {
		current := localengine.T("dialog", "self_update", "current") + info.Current
		latest := localengine.T("dialog", "self_update", "latest") + info.Latest
		changelog := localengine.T("dialog", "self_update", "changelog")
		date := ""
		if !info.Date.IsZero() {
			date = fmt.Sprintf("\n\nReleased: %s", info.Date.Local().Format("2006-01-02 15:04:05"))
		}
		return fmt.Sprintf("%s\n\n%s%s\n\n%s\n\n%s", current, latest, date, changelog, info.Body)
	})

	return p
}

// Tag returns the page tag.
func (p *AboutPage) Tag() string { return "about" }

// Name returns the page name.
func (p *AboutPage) Name() string       { return localengine.T("tab", "about") }
func (p *AboutPage) Icon() *widget.Icon { return icons.ActionInfo }

func (p *AboutPage) handleInteractions(gtx layout.Context) {
	if p.releaseNotesBtn.Clicked(gtx) {
		go p.fetchReleaseNotes()
	}
	if p.openReleaseNotesBtn.Clicked(gtx) {
		var urlStr string
		if version.Commit != "unknown" && version.Commit != "" {
			urlStr = "https://github.com/he11ah0und/sing-box-ez/releases/tag/" + version.Commit
		} else {
			urlStr = "https://github.com/he11ah0und/sing-box-ez/releases/latest"
		}
		if err := openurl.OpenURL(urlStr); err != nil {
			p.ctrl.Backend().Terminal().Infof("Failed to open release notes: %v", err)
		}
	}
	if p.openDataBtn.Clicked(gtx) {
		if err := p.ctrl.Backend().OpenDataDir(); err != nil {
			p.ctrl.Backend().Terminal().Infof("Failed to open data folder: %v", err)
		}
	}
	if p.switchBranchBtn.Clicked(gtx) {
		go p.openBranchPicker()
	}
	if p.openRepoBtn.Clicked(gtx) {
		if err := openurl.OpenURL("https://github.com/he11ah0und/sing-box-ez"); err != nil {
			p.ctrl.Backend().Terminal().Infof("Failed to open repo: %v", err)
		}
	}
}

// Layout draws the about page.
func (p *AboutPage) Layout(gtx layout.Context) layout.Dimensions {
	p.handleInteractions(gtx)
	return widgets.SpacedList(gtx, p.Children(gtx)...)
}

// Children returns the page widgets; the shell adds standard vertical spacing.
func (p *AboutPage) Children(gtx layout.Context) []layout.FlexChild {
	p.handleInteractions(gtx)
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.H6(p.th, localengine.T("about", "system", "title"))
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(p.th, version.BuildFlags()).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(p.th, p.commitInfoText()).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(p.th, p.buildInfoText()).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Button(p.th, &p.openRepoBtn, localengine.T("about", "btn", "open_repo")).Layout(gtx)
		}),
	}
	if version.IsDev() {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(p.th, localengine.T("about", "dev_build", "label"))
			lbl.Color = p.th.Palette.ContrastBg
			return lbl.Layout(gtx)
		}))
	} else {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return material.Button(p.th, &p.releaseNotesBtn, localengine.T("about", "btn", "release_notes")).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widgets.HSpace(gtx, unit.Dp(8))
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return material.Button(p.th, &p.openReleaseNotesBtn, localengine.T("about", "btn", "open_release_notes")).Layout(gtx)
				}),
			)
		}))
	}
	children = append(children,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(p.th, localengine.T("about", "branch", "label")+p.currentBranch).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.selfUpdate.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Button(p.th, &p.switchBranchBtn, localengine.T("about", "btn", "switch_branch")).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Button(p.th, &p.openDataBtn, localengine.T("about", "btn", "open_data")).Layout(gtx)
		}),
	)
	return children
}

func (p *AboutPage) commitInfoText() string {
	txt := version.Branch
	if version.Commit != "unknown" && version.Commit != "" {
		txt += ", " + version.Commit
	}
	if dt, err := version.CommitDateTime(); err == nil {
		txt += ", " + dt.Format("2006-01-02 15:04:05") + " (" + version.HumanDuration(dt) + ")"
	}
	return localengine.T("about", "commit_info", "prefix") + txt
}

func (p *AboutPage) buildInfoText() string {
	txt := ""
	if dt, err := version.BuildDateTime(); err == nil {
		txt = dt.Format("2006-01-02 15:04:05") + " (" + version.HumanDuration(dt) + ")"
	}
	return localengine.T("about", "build_info", "prefix") + txt
}

func (p *AboutPage) fetchReleaseNotes() {
	p.dialog.Show(widgets.Loading(localengine.T("about", "release_notes", "title")))
	release, err := updater.GetReleaseByVersion(version.Commit)
	if err != nil {
		if release.Version == "" {
			p.dialog.Show(widgets.Text(localengine.T("about", "release_notes", "title"), localengine.T("about", "release_notes", "not_found")))
			p.logUpdater("Release notes not found")
			return
		}
		p.logUpdater("Failed to fetch release notes: %v", err)
		p.dialog.Show(widgets.Text(localengine.T("about", "release_notes", "title"), localengine.T("about", "release_notes", "error")))
		return
	}
	dateStr := release.PublishedAt.Local().Format("2006-01-02 15:04:05")
	ago := version.HumanDuration(release.PublishedAt)
	body := fmt.Sprintf("# %s: %s\n\nReleased: %s (%s)\n\n%s",
		release.Version, release.Name, dateStr, ago, release.Body)
	p.dialog.Show(widgets.Markdown(localengine.T("about", "release_notes", "title")+": "+release.Version, body))
	p.logUpdater("Release notes fetched: %s", release.Version)
}

func (p *AboutPage) logUpdater(format string, args ...interface{}) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	if u := p.ctrl.SelfUpdater(); u != nil {
		u.Log.Infof(msg)
	} else {
		p.ctrl.Backend().Terminal().Infof(msg)
	}
}

func (p *AboutPage) openBranchPicker() {
	p.dialog.Show(widgets.Loading(localengine.T("progress", "checking_updates")))

	branches, err := p.ctrl.GetBranches()
	if err != nil {
		p.dialog.Hide()
		p.logUpdater("Failed to load branches: %v", err)
		p.dialog.Show(widgets.Text(localengine.T("about", "btn", "switch_branch"), localengine.T("about", "branch", "load_error")+err.Error()))
		return
	}

	p.pickerMu.Lock()
	p.pickerBranches = branches
	p.pickerBtns = make([]widget.Clickable, len(branches))
	p.pickerMu.Unlock()

	p.dialog.Hide()

	p.dialog.Show(widgets.CustomNoButtons(localengine.T("about", "btn", "switch_branch"), func(gtx layout.Context) layout.Dimensions {
		p.pickerMu.Lock()
		branches := p.pickerBranches
		btns := p.pickerBtns
		p.pickerMu.Unlock()

		for i := range branches {
			if btns[i].Clicked(gtx) {
				p.dialog.Hide()
				p.currentBranch = branches[i].Name
				go p.selfUpdate.RunCheck()
			}
		}

		children := []layout.FlexChild{}
		for i, b := range branches {
			idx := i
			label := b.Name
			if b.Name == p.currentBranch {
				label = "> " + b.Name
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Button(p.th, &btns[idx], label).Layout(gtx)
			}))
		}
		return widgets.DialogSpacedList(gtx, children...)
	}))

	p.logUpdater("Loaded %d branches", len(branches))
}
