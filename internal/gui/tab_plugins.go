package gui

import (
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"sing-box-ez/internal/paths"
	"sing-box-ez/internal/plugins"
)

// pluginListItem holds the data needed to render one row.
type pluginListItem struct {
	name          string
	version       string
	latestVersion string
	enabled       bool
	sourceType    string // "folder" or "package"
	relations     []string
}

// pluginCell is a list row that supports double-click (toggle) and right-click (info).
type pluginCell struct {
	widget.BaseWidget
	indicator *canvas.Text
	label     *widget.Label
	onDblTap  func()
	onRight   func()
}

func newPluginCell() *pluginCell {
	c := &pluginCell{
		indicator: canvas.NewText("●", nil),
		label:     widget.NewLabel(""),
	}
	c.ExtendBaseWidget(c)
	return c
}

func (c *pluginCell) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewHBox(c.indicator, c.label))
}

func (c *pluginCell) DoubleTapped(_ *fyne.PointEvent) {
	if c.onDblTap != nil {
		c.onDblTap()
	}
}

func (c *pluginCell) TappedSecondary(_ *fyne.PointEvent) {
	if c.onRight != nil {
		c.onRight()
	}
}

func (g *GUI) buildPluginsTab() *container.TabItem {
	refreshBtn := widget.NewButton("Refresh", func() {
		g.pluginManager.Close()
		if err := g.pluginManager.Discover(); err != nil {
			g.log("[plugins] discover error: " + err.Error())
		}
		g.refreshPluginsList()
	})

	checkUpdatesBtn := widget.NewButton("Check All Updates", func() {
		go g.pluginManager.CheckAllUpdates()
	})

	installBtn := widget.NewButton("Install from URL", func() {
		g.showInstallPluginDialog()
	})

	genDocsBtn := widget.NewButton("Generate API Docs", func() {
		outDir := paths.PluginDocsDir()
		if err := plugins.GenerateDocs(outDir); err != nil {
			g.log("[plugins] docs generation failed: " + err.Error())
		} else {
			g.log("[plugins] API docs generated: " + outDir)
		}
	})

	genDefsBtn := widget.NewButton("Generate VS Code Defs", func() {
		outDir := paths.PluginDefsDir()
		if err := plugins.GenerateLuaDefs(outDir); err != nil {
			g.log("[plugins] defs generation failed: " + err.Error())
		} else {
			g.log("[plugins] VS Code Lua defs generated: " + outDir)
		}
	})

	genTmplBtn := widget.NewButton("Generate Template", func() {
		g.showGenerateTemplateDialog()
	})

	btnRow := container.NewHBox(refreshBtn, checkUpdatesBtn, installBtn, genDocsBtn, genDefsBtn, genTmplBtn)

	g.pluginsList = widget.NewList(
		func() int {
			g.mu.Lock()
			defer g.mu.Unlock()
			return len(g.pluginItems)
		},
		func() fyne.CanvasObject {
			return newPluginCell()
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			g.mu.Lock()
			defer g.mu.Unlock()
			if i < 0 || i >= len(g.pluginItems) {
				return
			}
			item := g.pluginItems[i]
			c := o.(*pluginCell)

			if item.enabled {
				c.indicator.Text = "●"
				c.indicator.Color = colGreen
			} else {
				c.indicator.Text = "○"
				c.indicator.Color = nil
			}
			c.indicator.Refresh()

			src := "📁"
			if item.sourceType == "package" {
				src = "📦"
			}
			text := fmt.Sprintf("%s %s v%s", src, item.name, item.version)
			if item.latestVersion != "" && item.latestVersion != item.version {
				text += fmt.Sprintf(" (latest: v%s)", item.latestVersion)
			}
			if len(item.relations) > 0 {
				text += " [" + strings.Join(item.relations, ",") + "]"
			}
			c.label.SetText(text)

			c.onDblTap = func() {
				g.togglePlugin(item.name)
			}
			c.onRight = func() {
				g.showPluginInfo(item.name)
			}
		},
	)

	content := container.NewBorder(btnRow, nil, nil, nil, g.pluginsList)
	return container.NewTabItem("Plugins", content)
}

func (g *GUI) togglePlugin(name string) {
	if err := g.pluginManager.Toggle(name); err != nil {
		g.log("[plugins] toggle failed: " + err.Error())
	} else {
		g.log("[plugins] toggled: " + name)
	}
	g.refreshPluginsList()
}

func (g *GUI) refreshPluginsList() {
	names := g.pluginManager.List()
	items := make([]pluginListItem, 0, len(names))
	for _, name := range names {
		mf := g.pluginManager.GetManifest(name)
		if mf == nil {
			continue
		}
		items = append(items, pluginListItem{
			name:          mf.Name,
			version:       mf.Version,
			latestVersion: mf.LatestVersion,
			enabled:       mf.Enabled,
			sourceType:    mf.SourceType,
			relations:     []string(mf.Relations),
		})
	}
	g.mu.Lock()
	g.pluginItems = items
	g.mu.Unlock()
	g.pluginsList.Refresh()
}

func (g *GUI) showPluginInfo(name string) {
	mf := g.pluginManager.GetManifest(name)
	if mf == nil {
		return
	}

	info := container.NewVBox()
	addRow := func(label, value string) {
		info.Add(container.NewHBox(
			widget.NewLabelWithStyle(label+":", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabel(value),
		))
	}

	addRow("Name", mf.Name)
	addRow("Version", mf.Version)
	if mf.LatestVersion != "" {
		addRow("Latest (remote)", mf.LatestVersion)
	}
	addRow("Author", mf.Author)
	addRow("Description", mf.Description)
	addRow("Entrypoint", mf.Entry)
	addRow("Source", mf.SourceType)
	if mf.SourceType == "package" && mf.SourceURL != "" {
		addRow("Package URL", mf.SourceURL)
	}
	if len(mf.Relations) > 0 {
		addRow("Relation", strings.Join([]string(mf.Relations), ", "))
	}
	if mf.UpdateURL != "" {
		addRow("Update URL", mf.UpdateURL)
	}
	addRow("Status", map[bool]string{true: "enabled", false: "disabled"}[mf.Enabled])

	// If it's a folder without update_url, show a note.
	if mf.SourceType == "folder" {
		info.Add(widget.NewLabelWithStyle("Note: folder plugins cannot be auto-updated.", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}))
	}

	checkBtn := widget.NewButton("Check Update", func() {
		go func() {
			hasUpdate, latest, err := g.pluginManager.CheckUpdate(name)
			if err != nil {
				g.log("[plugins] update check failed for " + name + ": " + err.Error())
			} else if hasUpdate {
				g.log("[plugins] update available for " + name + ": v" + latest)
			} else {
				g.log("[plugins] " + name + " is up to date")
			}
			g.refreshPluginsList()
		}()
	})

	toggleLabel := "Enable"
	if mf.Enabled {
		toggleLabel = "Disable"
	}
	toggleBtn := widget.NewButton(toggleLabel, func() {
		g.togglePlugin(name)
	})

	unloadBtn := widget.NewButton("Unload", func() {
		g.pluginManager.Unload(name)
		g.refreshPluginsList()
	})

	footer := container.NewHBox(checkBtn, toggleBtn, unloadBtn)
	full := container.NewBorder(nil, footer, nil, nil, container.NewScroll(info))
	d := dialog.NewCustom("Plugin: "+name, "Close", full, g.window)
	d.Resize(fyne.NewSize(480, 420))
	d.Show()
}

func (g *GUI) showInstallPluginDialog() {
	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("https://example.com/my-plugin.zip")

	content := container.NewVBox(
		widget.NewLabel("Plugin package URL (zip or tar.gz)"),
		urlEntry,
	)

	var d dialog.Dialog
	saveBtn := widget.NewButton("Install", func() {
		url := urlEntry.Text
		if url == "" {
			g.log("[plugins] install: URL is required")
			return
		}
		go func() {
			if err := g.pluginManager.InstallFromURL(url); err != nil {
				g.log("[plugins] install failed: " + err.Error())
			} else {
				g.log("[plugins] installed from: " + url)
			}
			g.refreshPluginsList()
		}()
		d.Hide()
	})

	cancelBtn := widget.NewButton("Cancel", func() { d.Hide() })
	footer := container.NewHBox(saveBtn, cancelBtn)
	full := container.NewBorder(nil, footer, nil, nil, content)
	d = dialog.NewCustom("Install Plugin", "", full, g.window)
	d.Resize(fyne.NewSize(500, 180))
	d.Show()
}

func (g *GUI) showGenerateTemplateDialog() {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("my-plugin")

	relationSelect := widget.NewSelect([]string{"client", "server", "both"}, nil)
	relationSelect.SetSelected("client")

	content := container.NewVBox(
		widget.NewLabel("Plugin name"),
		nameEntry,
		widget.NewLabel("Relation"),
		relationSelect,
	)

	var d dialog.Dialog
	saveBtn := widget.NewButton("Generate", func() {
		name := nameEntry.Text
		if name == "" {
			g.log("[plugins] template: name is required")
			return
		}
		outDir := filepath.Join(plugins.PluginDir(), name)
		rel := relationSelect.Selected
		if rel == "" {
			rel = "client"
		}
		if err := plugins.GeneratePluginTemplate(outDir, name, rel); err != nil {
			g.log("[plugins] template generation failed: " + err.Error())
		} else {
			g.log("[plugins] template generated: " + outDir)
		}
		g.refreshPluginsList()
		d.Hide()
	})

	cancelBtn := widget.NewButton("Cancel", func() { d.Hide() })
	footer := container.NewHBox(saveBtn, cancelBtn)
	full := container.NewBorder(nil, footer, nil, nil, content)
	d = dialog.NewCustom("Generate Plugin Template", "", full, g.window)
	d.Resize(fyne.NewSize(400, 220))
	d.Show()
}

func (g *GUI) initPlugins() {
	g.pluginItems = []pluginListItem{}
	g.pluginManager = plugins.NewManager(g.window, g.tabs, g.cfg, func(line string) {
		g.log(line)
	})
	if err := g.pluginManager.Discover(); err != nil {
		g.log("[plugins] discover error: " + err.Error())
	}
	g.refreshPluginsList()
}
