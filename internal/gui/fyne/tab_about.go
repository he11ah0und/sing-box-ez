package fynegui

import (
	"net/url"
	"sync"

	"sing-box-ez/internal/i18n"
	"sing-box-ez/internal/util/githuburl"
	"sing-box-ez/internal/version"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func buildInfoText() string {
	return version.BuildFlags()
}

func (g *GUI) buildAboutTab() *container.TabItem {
	infoLbl := widget.NewLabel(buildInfoText())
	buildLink := g.buildAboutBuildLink()
	repoLink := widget.NewHyperlink(githuburl.DefaultProject().Slug(), g.mustParseURL(githuburl.DefaultProject().RepoURL()))

	notesBtn := widget.NewButton(i18n.T("about.btn.release_notes"), g.showReleaseNotesHandler())
	openDataBtn := widget.NewButton(i18n.T("about.btn.open_data"), func() {
		_ = g.ctrl.OpenDataFolderWithLog()
	})

	var selectedBranch string
	var branchesMu sync.Mutex
	selectedBranch = version.Branch

	switchBranchBtn := widget.NewButton(i18n.T("about.btn.switch_branch"), func() {
		go func() {
			modal := g.showInfiniteDialog(i18n.T("progress.checking_updates"))
			b, err := g.ctrl.GetBranches()
			fyne.Do(func() { modal.Hide() })
			if err != nil {
				fyne.Do(func() {
					dialog.ShowError(err, g.window)
				})
				return
			}
			fyne.Do(func() {
				branchBox := container.NewVBox()
				for _, br := range b {
					name := br.Name
					btn := widget.NewButton(name, func() {
						branchesMu.Lock()
						selectedBranch = name
						branchesMu.Unlock()
					})
					branchBox.Add(btn)
				}
				content := container.NewVBox(
					widget.NewLabel(i18n.T("about.btn.switch_branch")),
					branchBox,
				)
				d := dialog.NewCustom(i18n.T("about.btn.switch_branch"), i18n.T("dialog.btn.cancel"), content, g.window)
				d.Show()
			})
		}()
	})

	selfUpdateBtn := widget.NewButton(i18n.T("about.btn.check_updates"), func() {
		go func() {
			branchesMu.Lock()
			branch := selectedBranch
			branchesMu.Unlock()
			modal := g.showInfiniteDialog(i18n.T("progress.checking_updates"))
			info, err := g.ctrl.CheckSelfUpdateForBranch(branch)
			fyne.Do(func() { modal.Hide() })
			if err != nil || info == nil {
				return
			}
			fyne.Do(func() {
				g.showSelfUpdateDialog(info)
			})
		}()
	})

	content := container.NewVBox(
		widget.NewLabelWithStyle(i18n.T("about.system.title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		infoLbl,
		buildLink,
		repoLink,
		widget.NewSeparator(),
		notesBtn,
		openDataBtn,
		widget.NewSeparator(),
		switchBranchBtn,
		selfUpdateBtn,
	)

	return container.NewTabItem(i18n.T("tab.about"), container.NewScroll(content))
}

func (g *GUI) mustParseURL(raw string) *url.URL {
	u, _ := url.Parse(raw)
	return u
}

func (g *GUI) buildAboutBuildLink() *widget.Hyperlink {
	buildText := version.Branch
	if version.Commit != "unknown" && version.Commit != "" {
		buildText += ", " + version.Commit
	}
	if dt, err := version.BuildDateTime(); err == nil {
		buildText += ", " + dt.Format("2006-01-02 15:04:05") + " (" + version.HumanDuration(dt) + ")"
	}
	var buildURL *url.URL
	if version.Commit != "unknown" && version.Commit != "" {
		buildURL, _ = url.Parse(githuburl.DefaultProject().WebReleaseURL(version.Commit))
	} else {
		buildURL, _ = url.Parse(githuburl.DefaultProject().RepoURL())
	}
	return widget.NewHyperlink(i18n.T("about.build.prefix")+buildText, buildURL)
}

func (g *GUI) showReleaseNotesHandler() func() {
	return func() {
		go func() {
			modal := g.showInfiniteDialog(i18n.T("progress.fetching_notes"))
			release, err := g.ctrl.FetchReleaseNotesWithLog(version.Commit)
			fyne.Do(func() { modal.Hide() })
			if err != nil {
				return
			}
			if release.TagName == "" {
				fyne.Do(func() {
					d := dialog.NewInformation(i18n.T("about.release_notes.title"),
						i18n.T("about.release_notes.not_found"),
						g.window)
					d.Show()
				})
				return
			}
			fyne.Do(func() {
				dateStr := release.PublishedAt.Local().Format("2006-01-02 15:04:05")
				ago := version.HumanDuration(release.PublishedAt)
				header := widget.NewLabel(i18n.T("about.release_notes.released") + dateStr + " (" + ago + ")")
				header.TextStyle = fyne.TextStyle{Bold: true}

				notesRich := widget.NewRichTextFromMarkdown(release.Body)
				scroll := container.NewScroll(notesRich)
				scroll.SetMinSize(fyne.NewSize(500, 400))

				content := container.NewVBox(header, widget.NewSeparator(), scroll)
				d := dialog.NewCustom(i18n.T("about.release_notes.title")+": "+release.TagName, i18n.T("about.dialog.close"), content, g.window)
				d.Show()
			})
		}()
	}
}
