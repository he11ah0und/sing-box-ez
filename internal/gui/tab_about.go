package gui

import (
	"fmt"
	"net/url"

	"sing-box-ez/internal/githuburl"
	"sing-box-ez/internal/paths"
	"sing-box-ez/internal/updater"
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
	buildLink := widget.NewHyperlink("Build: "+buildText, buildURL)

	repoURL, _ := url.Parse(githuburl.DefaultProject().RepoURL())
	repoLink := widget.NewHyperlink(githuburl.DefaultProject().Slug(), repoURL)

	notesBtn := widget.NewButton("Show release notes", func() {
		go func() {
			modal := g.showInfiniteDialog("Fetching release notes...")
			release, err := updater.GetReleaseByTag(version.Commit)
			fyne.Do(func() { modal.Hide() })
			if err != nil {
				if release.TagName == "" {
					fyne.Do(func() {
						buildInfo := version.Branch
						if version.Commit != "unknown" && version.Commit != "" {
							buildInfo += ", commit: " + version.Commit
						}
						if dt, err := version.BuildDateTime(); err == nil {
							buildInfo += ", built: " + dt.Format("2006-01-02 15:04:05") + " (" + version.HumanDuration(dt) + ")"
						}
						d := dialog.NewInformation("Release notes",
							"Current build ("+buildInfo+") does not match any official release.\n\n"+
							"This build is not listed among GitHub releases.\n"+
							"Maybe this is a dev build?", g.window)
						d.Show()
					})
					return
				}
				g.log("Failed to fetch release notes: " + err.Error())
				return
			}
			fyne.Do(func() {
				dateStr := release.PublishedAt.Local().Format("2006-01-02 15:04:05")
				ago := version.HumanDuration(release.PublishedAt)
				header := widget.NewLabel("Released: " + dateStr + " (" + ago + ")")
				header.TextStyle = fyne.TextStyle{Bold: true}

				notesRich := widget.NewRichTextFromMarkdown(release.Body)
				scroll := container.NewScroll(notesRich)
				scroll.SetMinSize(fyne.NewSize(500, 400))

				content := container.NewVBox(header, widget.NewSeparator(), scroll)
				d := dialog.NewCustom("Release notes: "+release.TagName, "Close", content, g.window)
				d.Show()
			})
		}()
	})

	openDataBtn := widget.NewButton("Open data folder", func() {
		if err := paths.OpenDataDir(); err != nil {
			g.log("Failed to open data folder: " + err.Error())
		}
	})

	selfUpdateBtn := widget.NewButton("Check for updates", func() {
		go func() {
			modal := g.showInfiniteDialog("Checking for updates...")
			info, err := updater.CheckUpdate(version.Branch)
			fyne.Do(func() { modal.Hide() })
			if err != nil {
				g.log("Update check failed: " + err.Error())
				return
			}
			if info.ReleaseCount == 0 {
				g.log("Already on latest version: " + info.Current)
				return
			}
			g.log(fmt.Sprintf("Update available: %s → %s (%d releases behind)", info.Current, info.Latest, info.ReleaseCount))
			fyne.Do(func() {
				g.showSelfUpdateDialog(info)
			})
		}()
	})

	content := container.NewVBox(
		widget.NewLabelWithStyle("System", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		infoLbl,
		buildLink,
		repoLink,
		notesBtn,
		openDataBtn,
		selfUpdateBtn,
	)

	return container.NewTabItem("About", container.NewScroll(content))
}
