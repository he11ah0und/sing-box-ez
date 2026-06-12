package fynegui

import (
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/framework/util/openurl"
	"sing-box-ez/internal/framework/version"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func (g *GUI) buildAboutTab() *container.TabItem {
	flagsLbl := widget.NewLabel(version.BuildFlags())
	commitInfoLbl := widget.NewLabel(commitInfoText())
	buildInfoLbl := widget.NewLabel(buildInfoText())

	openRepoBtn := widget.NewButton(localengine.T("about", "btn", "open_repo"), func() {
		_ = openurl.OpenURL("https://github.com/he11ah0und/sing-box-ez")
	})

	var notesRow fyne.CanvasObject
	if version.IsDev() {
		notesRow = widget.NewLabel(localengine.T("about", "dev_build", "label"))
	} else {
		notesBtn := widget.NewButton(localengine.T("about", "btn", "release_notes"), g.showReleaseNotesHandler())
		openNotesBtn := widget.NewButton(localengine.T("about", "btn", "open_release_notes"), func() {
			var urlStr string
			if version.Commit != "unknown" && version.Commit != "" {
				urlStr = "https://github.com/he11ah0und/sing-box-ez/releases/tag/" + version.Commit
			} else {
				urlStr = "https://github.com/he11ah0und/sing-box-ez/releases/latest"
			}
			_ = openurl.OpenURL(urlStr)
		})
		notesRow = container.NewGridWithColumns(2, notesBtn, openNotesBtn)
	}

	openDataBtn := widget.NewButton(localengine.T("about", "btn", "open_data"), func() {
		_ = g.ctrl.OpenDataFolder()
	})

	switchBranchBtn := widget.NewButton(localengine.T("about", "btn", "switch_branch"), func() {
		go func() {
			modal := g.showInfiniteDialog(localengine.T("progress", "checking_updates"))
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
					branchName := br.Name
					btn := widget.NewButton(branchName, func() {
						go func() {
							modal := g.showInfiniteDialog(localengine.T("progress", "checking_updates"))
							info, err := g.ctrl.CheckSelfUpdateForBranch(branchName)
							fyne.Do(func() { modal.Hide() })
							if err != nil || info == nil {
								return
							}
							fyne.Do(func() {
								g.showSelfUpdateDialog(info)
							})
						}()
					})
					branchBox.Add(btn)
				}
				content := container.NewVBox(
					widget.NewLabel(localengine.T("about", "btn", "switch_branch")),
					branchBox,
				)
				d := dialog.NewCustom(localengine.T("about", "btn", "switch_branch"), localengine.T("dialog", "btn", "cancel"), content, g.window)
				d.Show()
			})
		}()
	})

	selfUpdateBtn := widget.NewButton(localengine.T("about", "btn", "check_updates"), func() {
		go func() {
			modal := g.showInfiniteDialog(localengine.T("progress", "checking_updates"))
			info, err := g.ctrl.CheckSelfUpdate()
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
		widget.NewLabelWithStyle(localengine.T("about", "system", "title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		flagsLbl,
		commitInfoLbl,
		buildInfoLbl,
		openRepoBtn,
		widget.NewSeparator(),
		notesRow,
		selfUpdateBtn,
		switchBranchBtn,
		openDataBtn,
	)

	return container.NewTabItem(localengine.T("tab", "about"), container.NewScroll(content))
}

func commitInfoText() string {
	txt := version.Branch
	if version.Commit != "unknown" && version.Commit != "" {
		txt += ", " + version.Commit
	}
	if dt, err := version.CommitDateTime(); err == nil {
		txt += ", " + dt.Format("2006-01-02 15:04:05") + " (" + version.HumanDuration(dt) + ")"
	}
	return localengine.T("about", "commit_info", "prefix") + txt
}

func buildInfoText() string {
	txt := ""
	if dt, err := version.BuildDateTime(); err == nil {
		txt = dt.Format("2006-01-02 15:04:05") + " (" + version.HumanDuration(dt) + ")"
	}
	return localengine.T("about", "build_info", "prefix") + txt
}

func (g *GUI) showReleaseNotesHandler() func() {
	return func() {
		go func() {
			modal := g.showInfiniteDialog(localengine.T("progress", "fetching_notes"))
			release, err := g.ctrl.FetchReleaseNotes(version.Commit)
			fyne.Do(func() { modal.Hide() })
			if err != nil {
				return
			}
			if release.TagName == "" {
				fyne.Do(func() {
					d := dialog.NewInformation(localengine.T("about", "release_notes", "title"),
						localengine.T("about", "release_notes", "not_found"),
						g.window)
					d.Show()
				})
				return
			}
			fyne.Do(func() {
				dateStr := release.PublishedAt.Local().Format("2006-01-02 15:04:05")
				ago := version.HumanDuration(release.PublishedAt)
				header := widget.NewLabel(localengine.T("about", "release_notes", "released") + dateStr + " (" + ago + ")")
				header.TextStyle = fyne.TextStyle{Bold: true}

				notesRich := widget.NewRichTextFromMarkdown(release.Body)
				scroll := container.NewScroll(notesRich)
				scroll.SetMinSize(fyne.NewSize(500, 400))

				content := container.NewVBox(header, widget.NewSeparator(), scroll)
				d := dialog.NewCustom(localengine.T("about", "release_notes", "title")+": "+release.TagName, localengine.T("about", "dialog", "close"), content, g.window)
				d.Show()
			})
		}()
	}
}
