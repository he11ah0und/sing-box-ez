package pages

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"sing-box-ez/internal/core"
	"sing-box-ez/internal/util"
	"sing-box-ez/internal/i18n"
	"sing-box-ez/internal/paths"
	"sing-box-ez/internal/updater"
	"sing-box-ez/internal/version"
)

// AboutPage renders the about / system info screen.
type AboutPage struct {
	th   *material.Theme
	ctrl *core.Controller

	releaseNotesBtn widget.Clickable
	openDataBtn     widget.Clickable
	checkUpdatesBtn widget.Clickable
	repoLink        widget.Clickable
	buildLink       widget.Clickable

	// showDialog callback is provided by the shell to show modal dialogs.
	showDialog         func(title, body string)
	showDialogMarkdown func(title, body string)
	showDialogLoading  func(title string)
}

// NewAboutPage creates a new about page.
func NewAboutPage(th *material.Theme, ctrl *core.Controller, showDialog func(title, body string), showDialogMarkdown func(title, body string), showDialogLoading func(title string)) *AboutPage {
	return &AboutPage{
		th:                 th,
		ctrl:               ctrl,
		showDialog:         showDialog,
		showDialogMarkdown: showDialogMarkdown,
		showDialogLoading:  showDialogLoading,
	}
}

// Tag returns the page tag.
func (p *AboutPage) Tag() string { return "about" }

// Name returns the page name.
func (p *AboutPage) Name() string { return "About" }

// Layout draws the about page.
func (p *AboutPage) Layout(gtx layout.Context) layout.Dimensions {
	// Handle interactions.
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
	if p.repoLink.Clicked(gtx) {
		_ = util.OpenURL(util.DefaultProject().RepoURL())
	}
	if p.buildLink.Clicked(gtx) {
		var urlStr string
		if version.Commit != "unknown" && version.Commit != "" {
			urlStr = util.DefaultProject().WebReleaseURL(version.Commit)
		} else {
			urlStr = util.DefaultProject().RepoURL()
		}
		_ = util.OpenURL(urlStr)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
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
			return p.hyperlink(gtx, util.DefaultProject().Slug(), &p.repoLink)
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
				return material.Button(p.th, &p.checkUpdatesBtn, i18n.T("about.btn.check_updates")).Layout(gtx)
			})
		}),
	)
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
	info, err := updater.CheckUpdate(version.Branch)
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


