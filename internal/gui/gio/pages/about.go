package pages

import (
	"fmt"
	"sync"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"

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

	releaseNotesBtn widget.Clickable
	openDataBtn     widget.Clickable
	checkUpdatesBtn widget.Clickable
	switchBranchBtn widget.Clickable
	repoLink        widget.Clickable
	buildLink       widget.Clickable

	selectedBranch string

	// Branch picker overlay state.
	pickerActive   bool
	pickerBranches []updater.Branch
	pickerBtns     []widget.Clickable
	pickerCancel   widget.Clickable
	pickerMu       sync.Mutex

	// showDialog callback is provided by the shell to show modal dialogs.
	showDialog         func(title, body string)
	showDialogMarkdown func(title, body string)
	showDialogLoading  func(title string)
}

// NewAboutPage creates a new about page.
func NewAboutPage(th *material.Theme, ctrl *core.InteractiveController, showDialog func(title, body string), showDialogMarkdown func(title, body string), showDialogLoading func(title string)) *AboutPage {
	return &AboutPage{
		th:                 th,
		ctrl:               ctrl,
		showDialog:         showDialog,
		showDialogMarkdown: showDialogMarkdown,
		showDialogLoading:  showDialogLoading,
		selectedBranch:     version.Branch,
	}
}

// Tag returns the page tag.
func (p *AboutPage) Tag() string { return "about" }

// Name returns the page name.
func (p *AboutPage) Name() string { return "About" }

// Layout draws the about page.
func (p *AboutPage) Layout(gtx layout.Context) layout.Dimensions {
	p.handleInteractions(gtx)

	dims := p.layoutMainContent(gtx)

	p.pickerMu.Lock()
	active := p.pickerActive
	p.pickerMu.Unlock()
	if active {
		// Render picker as a centered card on top.
		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				return dims
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return p.layoutBranchPicker(gtx)
			}),
		)
	}
	return dims
}

func (p *AboutPage) handleInteractions(gtx layout.Context) {
	if p.releaseNotesBtn.Clicked(gtx) {
		go p.fetchReleaseNotes()
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
	if p.repoLink.Clicked(gtx) {
		_ = openurl.OpenURL(githuburl.DefaultProject().RepoURL())
	}
	if p.buildLink.Clicked(gtx) {
		var urlStr string
		if version.Commit != "unknown" && version.Commit != "" {
			urlStr = githuburl.DefaultProject().WebReleaseURL(version.Commit)
		} else {
			urlStr = githuburl.DefaultProject().RepoURL()
		}
		_ = openurl.OpenURL(urlStr)
	}

	p.pickerMu.Lock()
	for i := range p.pickerBtns {
		if p.pickerBtns[i].Clicked(gtx) {
			p.selectedBranch = p.pickerBranches[i].Name
			p.pickerActive = false
		}
	}
	if p.pickerCancel.Clicked(gtx) {
		p.pickerActive = false
	}
	p.pickerMu.Unlock()
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
			return p.hyperlink(gtx, p.buildLinkText(), &p.buildLink)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.hyperlink(gtx, githuburl.DefaultProject().Slug(), &p.repoLink)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Button(p.th, &p.releaseNotesBtn, i18n.T("about.btn.release_notes")).Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Button(p.th, &p.openDataBtn, i18n.T("about.btn.open_data")).Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Button(p.th, &p.switchBranchBtn, i18n.T("about.btn.switch_branch")).Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Button(p.th, &p.checkUpdatesBtn, i18n.T("about.btn.check_updates")).Layout(gtx)
			})
		}),
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (p *AboutPage) layoutBranchPicker(gtx layout.Context) layout.Dimensions {
	maxWidth := gtx.Dp(unit.Dp(360))
	if gtx.Constraints.Max.X < maxWidth {
		maxWidth = gtx.Constraints.Max.X - gtx.Dp(unit.Dp(32))
	}
	cardGtx := gtx
	cardGtx.Constraints.Min.X = maxWidth
	cardGtx.Constraints.Max.X = maxWidth
	cardGtx.Constraints.Min.Y = 0

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return component.Surface(p.th).Layout(cardGtx, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.H6(p.th, i18n.T("about.btn.switch_branch")).Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return material.Body2(p.th, i18n.T("about.branch.label")+p.selectedBranch).Layout(gtx)
						})
					}),
				}
				p.pickerMu.Lock()
				for i, b := range p.pickerBranches {
					idx := i
					label := b.Name
					if b.Name == p.selectedBranch {
						label = "> " + b.Name
					}
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return material.Button(p.th, &p.pickerBtns[idx], label).Layout(gtx)
						})
					}))
				}
				p.pickerMu.Unlock()
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Button(p.th, &p.pickerCancel, i18n.T("dialog.btn.cancel")).Layout(gtx)
					})
				}))
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		})
	})
}

func (p *AboutPage) buildLinkText() string {
	txt := version.Branch
	if version.Commit != "unknown" && version.Commit != "" {
		txt += ", " + version.Commit
	}
	if dt, err := version.BuildDateTime(); err == nil {
		txt += ", " + dt.Format("2006-01-02 15:04:05") + " (" + version.HumanDuration(dt) + ")"
	}
	return i18n.T("about.build.prefix") + txt
}

func (p *AboutPage) hyperlink(gtx layout.Context, text string, btn *widget.Clickable) layout.Dimensions {
	return material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
		lbl := material.Body2(p.th, text)
		lbl.Color = p.th.Palette.ContrastBg
		return lbl.Layout(gtx)
	})
}

func (p *AboutPage) fetchReleaseNotes() {
	p.showDialogLoading(i18n.T("about.release_notes.title"))
	release, err := updater.GetReleaseByTag(version.Commit)
	if err != nil {
		if release.TagName == "" {
			p.showDialog(i18n.T("about.release_notes.title"), i18n.T("about.release_notes.not_found"))
			p.ctrl.Log("Release notes not found")
			return
		}
		p.ctrl.Log("Failed to fetch release notes: " + err.Error())
		p.showDialog(i18n.T("about.release_notes.title"), "Failed to fetch release notes")
		return
	}
	dateStr := release.PublishedAt.Local().Format("2006-01-02 15:04:05")
	ago := version.HumanDuration(release.PublishedAt)
	body := fmt.Sprintf("# %s: %s\n\nReleased: %s (%s)\n\n%s",
		release.TagName, release.Name, dateStr, ago, release.Body)
	p.showDialogMarkdown(i18n.T("about.release_notes.title")+": "+release.TagName, body)
	p.ctrl.Log("Release notes fetched: " + release.TagName)
}

func (p *AboutPage) checkUpdates() {
	p.showDialogLoading(i18n.T("about.btn.check_updates"))
	info, err := updater.CheckUpdateForBranch(p.selectedBranch)
	if err != nil {
		p.ctrl.Log("Update check failed: " + err.Error())
		p.showDialog(i18n.T("about.btn.check_updates"), "Update check failed")
		return
	}
	if info.ReleaseCount == 0 {
		p.ctrl.Log("Already on latest version: " + info.Current)
		p.showDialog(i18n.T("about.btn.check_updates"), "Already on latest version: "+info.Current)
		return
	}
	msg := fmt.Sprintf("Update available: %s → %s (%d releases behind)", info.Current, info.Latest, info.ReleaseCount)
	p.showDialog(i18n.T("about.btn.check_updates"), msg)
	p.ctrl.Log(msg)
}

func (p *AboutPage) openBranchPicker() {
	p.pickerMu.Lock()
	p.pickerActive = true
	p.pickerMu.Unlock()

	branches, err := p.ctrl.GetBranches()
	if err != nil {
		p.ctrl.Log("Failed to load branches: " + err.Error())
		p.showDialog(i18n.T("about.btn.switch_branch"), "Failed to load branches: "+err.Error())
		p.pickerMu.Lock()
		p.pickerActive = false
		p.pickerMu.Unlock()
		return
	}
	p.pickerMu.Lock()
	p.pickerBranches = branches
	p.pickerBtns = make([]widget.Clickable, len(branches))
	p.pickerMu.Unlock()
	p.ctrl.Log(fmt.Sprintf("Loaded %d branches", len(branches)))
}
