package pages

import (
	"fmt"
	"image"
	"sync"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"sing-box-ez/internal/core"
	"sing-box-ez/internal/i18n"
	"sing-box-ez/internal/updater"
	"sing-box-ez/internal/util/githuburl"
	"sing-box-ez/internal/util/openurl"
	"sing-box-ez/internal/util/paths"
	"sing-box-ez/internal/version"
)

// AboutPage renders the about / system info screen.
type AboutPage struct {
	th   *material.Theme
	ctrl *core.InteractiveController

	releaseNotesBtn     widget.Clickable
	openReleaseNotesBtn widget.Clickable
	openDataBtn         widget.Clickable
	checkUpdatesBtn     widget.Clickable
	switchBranchBtn     widget.Clickable
	openRepoBtn         widget.Clickable

	// Branch picker state (rendered via Dialog).
	pickerBranches []updater.Branch
	pickerBtns     []widget.Clickable
	pickerMu       sync.Mutex

	// Dialog provider is supplied by the shell.
	dialog DialogProvider
}

// NewAboutPage creates a new about page.
func NewAboutPage(th *material.Theme, ctrl *core.InteractiveController, dialog DialogProvider) *AboutPage {
	return &AboutPage{
		th:     th,
		ctrl:   ctrl,
		dialog: dialog,
	}
}

// Tag returns the page tag.
func (p *AboutPage) Tag() string { return "about" }

// Name returns the page name.
func (p *AboutPage) Name() string { return i18n.T("tab.about") }

// Layout draws the about page.
func (p *AboutPage) Layout(gtx layout.Context) layout.Dimensions {
	p.handleInteractions(gtx)
	return p.layoutMainContent(gtx)
}

func (p *AboutPage) handleInteractions(gtx layout.Context) {
	if p.releaseNotesBtn.Clicked(gtx) {
		go p.fetchReleaseNotes()
	}
	if p.openReleaseNotesBtn.Clicked(gtx) {
		var urlStr string
		if version.Commit != "unknown" && version.Commit != "" {
			urlStr = githuburl.DefaultProject().WebReleaseURL(version.Commit)
		} else {
			urlStr = githuburl.DefaultProject().WebLatestReleaseURL()
		}
		if err := openurl.OpenURL(urlStr); err != nil {
			p.ctrl.Log("Failed to open release notes: " + err.Error())
		}
	}
	if p.openDataBtn.Clicked(gtx) {
		if err := paths.OpenDataDir(); err != nil {
			p.ctrl.Log("Failed to open data folder: " + err.Error())
		}
	}
	if p.checkUpdatesBtn.Clicked(gtx) {
		go p.checkUpdates()
	}
	if p.switchBranchBtn.Clicked(gtx) {
		go p.openBranchPicker()
	}
	if p.openRepoBtn.Clicked(gtx) {
		if err := openurl.OpenURL(githuburl.DefaultProject().RepoURL()); err != nil {
			p.ctrl.Log("Failed to open repo: " + err.Error())
		}
	}
}

func (p *AboutPage) layoutMainContent(gtx layout.Context) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.H6(p.th, i18n.T("about.system.title"))
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Body2(p.th, version.BuildFlags()).Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(p.th, p.commitInfoText()).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(p.th, p.buildInfoText()).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Button(p.th, &p.openRepoBtn, i18n.T("about.btn.open_repo")).Layout(gtx)
			})
		}),
	}
	if version.IsDev() {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(p.th, i18n.T("about.dev_build.label"))
				lbl.Color = p.th.Palette.ContrastBg
				return lbl.Layout(gtx)
			})
		}))
	} else {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.Button(p.th, &p.releaseNotesBtn, i18n.T("about.btn.release_notes")).Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Point{X: gtx.Dp(unit.Dp(8)), Y: 0}}
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.Button(p.th, &p.openReleaseNotesBtn, i18n.T("about.btn.open_release_notes")).Layout(gtx)
					}),
				)
			})
		}))
	}
	children = append(children,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Button(p.th, &p.checkUpdatesBtn, i18n.T("about.btn.check_updates")).Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Button(p.th, &p.switchBranchBtn, i18n.T("about.btn.switch_branch")).Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Button(p.th, &p.openDataBtn, i18n.T("about.btn.open_data")).Layout(gtx)
			})
		}),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (p *AboutPage) commitInfoText() string {
	txt := version.Branch
	if version.Commit != "unknown" && version.Commit != "" {
		txt += ", " + version.Commit
	}
	if dt, err := version.CommitDateTime(); err == nil {
		txt += ", " + dt.Format("2006-01-02 15:04:05") + " (" + version.HumanDuration(dt) + ")"
	}
	return i18n.T("about.commit_info.prefix") + txt
}

func (p *AboutPage) buildInfoText() string {
	txt := ""
	if dt, err := version.BuildDateTime(); err == nil {
		txt = dt.Format("2006-01-02 15:04:05") + " (" + version.HumanDuration(dt) + ")"
	}
	return i18n.T("about.build_info.prefix") + txt
}

func (p *AboutPage) fetchReleaseNotes() {
	p.dialog.ShowLoading(i18n.T("about.release_notes.title"))
	release, err := updater.GetReleaseByTag(version.Commit)
	if err != nil {
		if release.TagName == "" {
			p.dialog.Show(i18n.T("about.release_notes.title"), i18n.T("about.release_notes.not_found"))
			p.ctrl.Log("Release notes not found")
			return
		}
		p.ctrl.Log("Failed to fetch release notes: " + err.Error())
		p.dialog.Show(i18n.T("about.release_notes.title"), "Failed to fetch release notes")
		return
	}
	dateStr := release.PublishedAt.Local().Format("2006-01-02 15:04:05")
	ago := version.HumanDuration(release.PublishedAt)
	body := fmt.Sprintf("# %s: %s\n\nReleased: %s (%s)\n\n%s",
		release.TagName, release.Name, dateStr, ago, release.Body)
	p.dialog.ShowMarkdown(i18n.T("about.release_notes.title")+": "+release.TagName, body)
	p.ctrl.Log("Release notes fetched: " + release.TagName)
}

func (p *AboutPage) checkUpdates() {
	p.dialog.ShowLoading(i18n.T("about.btn.check_updates"))
	info, err := p.ctrl.CheckSelfUpdateWithLog()
	if err != nil {
		p.dialog.Show(i18n.T("about.btn.check_updates"), "Update check failed")
		return
	}
	if info == nil {
		p.dialog.Show(i18n.T("about.btn.check_updates"), "Already on latest version: "+version.Branch)
		return
	}
	msg := fmt.Sprintf("Update available: %s → %s (%d releases behind)", info.Current, info.Latest, info.ReleaseCount)
	p.dialog.Show(i18n.T("about.btn.check_updates"), msg)
}

func (p *AboutPage) checkUpdatesForBranch(branch string) {
	p.dialog.ShowLoading(i18n.T("about.btn.check_updates"))
	info, err := p.ctrl.CheckSelfUpdateForBranch(branch)
	if err != nil {
		p.dialog.Show(i18n.T("about.btn.check_updates"), "Update check failed")
		return
	}
	if info.ReleaseCount == 0 {
		p.dialog.Show(i18n.T("about.btn.check_updates"), "Already on latest version on "+branch)
		return
	}
	msg := fmt.Sprintf("Update available on %s: %s → %s (%d releases behind)", branch, info.Current, info.Latest, info.ReleaseCount)
	p.dialog.Show(i18n.T("about.btn.check_updates"), msg)
}

func (p *AboutPage) openBranchPicker() {
	p.dialog.ShowLoading(i18n.T("progress.checking_updates"))

	branches, err := p.ctrl.GetBranches()
	if err != nil {
		p.dialog.HideLoading()
		p.ctrl.Log("Failed to load branches: " + err.Error())
		p.dialog.Show(i18n.T("about.btn.switch_branch"), "Failed to load branches: "+err.Error())
		return
	}

	p.pickerMu.Lock()
	p.pickerBranches = branches
	p.pickerBtns = make([]widget.Clickable, len(branches))
	p.pickerMu.Unlock()

	p.dialog.HideLoading()

	p.dialog.ShowCustom(i18n.T("about.btn.switch_branch"), func(gtx layout.Context) layout.Dimensions {
		p.pickerMu.Lock()
		branches := p.pickerBranches
		btns := p.pickerBtns
		p.pickerMu.Unlock()

		for i := range branches {
			if btns[i].Clicked(gtx) {
				p.dialog.HideCustom()
				go p.checkUpdatesForBranch(branches[i].Name)
			}
		}

		children := []layout.FlexChild{}
		for i, b := range branches {
			idx := i
			label := b.Name
			if b.Name == version.Branch {
				label = "> " + b.Name
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return material.Button(p.th, &btns[idx], label).Layout(gtx)
				})
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})

	p.ctrl.Log(fmt.Sprintf("Loaded %d branches", len(branches)))
}
