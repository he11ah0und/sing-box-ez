//go:build !noplugins

package fynegui

import (
	"fmt"
	"path/filepath"
	"strings"

	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/plugins"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
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
	if !g.cfg.GetPluginsEnabled() {
		return nil
	}

	refreshBtn := widget.NewButton(localengine.T("plugins", "btn", "refresh"), func() {
		g.pluginManager.Close()
		_ = g.ctrl.PluginDiscover(g.pluginManager)
		g.refreshPluginsList()
	})

	checkUpdatesBtn := widget.NewButton(localengine.T("plugins", "btn", "check_all_updates"), func() {
		go g.pluginManager.CheckAllUpdates()
	})

	installBtn := widget.NewButton(localengine.T("plugins", "btn", "install_from_url"), func() {
		g.showInstallPluginDialog()
	})

	var btnRow fyne.CanvasObject
	if g.cfg.GetPluginsDeveloper() {
		genDocsBtn := widget.NewButton(localengine.T("plugins", "btn", "generate_api_docs"), func() {
			outDir := filepath.Join(g.cfg.DataDir, plugins.DocsDir())
			_ = g.ctrl.PluginGenerateDocs(plugins.GenerateDocs, outDir)
		})

		genDefsBtn := widget.NewButton(localengine.T("plugins", "btn", "generate_vscode_defs"), func() {
			outDir := filepath.Join(g.cfg.DataDir, plugins.DefsDir())
			_ = g.ctrl.PluginGenerateDefs(plugins.GenerateLuaDefs, outDir)
		})

		genTmplBtn := widget.NewButton(localengine.T("plugins", "btn", "generate_template"), func() {
			g.showGenerateTemplateDialog()
		})

		btnRow = container.NewHBox(refreshBtn, checkUpdatesBtn, installBtn, genDocsBtn, genDefsBtn, genTmplBtn)
	} else {
		btnRow = container.NewHBox(refreshBtn, checkUpdatesBtn, installBtn)
	}

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
	return container.NewTabItem(localengine.T("tab", "plugins"), content)
}

func (g *GUI) togglePlugin(name string) {
	_ = g.ctrl.PluginToggle(g.pluginManager, name)
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

	addRow(localengine.T("plugins", "info", "name"), mf.Name)
	addRow(localengine.T("plugins", "info", "version"), mf.Version)
	if mf.LatestVersion != "" {
		addRow(localengine.T("plugins", "info", "latest_remote"), mf.LatestVersion)
	}
	addRow(localengine.T("plugins", "info", "author"), mf.Author)
	addRow(localengine.T("plugins", "info", "description"), mf.Description)
	addRow(localengine.T("plugins", "info", "entrypoint"), mf.Entry)
	addRow(localengine.T("plugins", "info", "source"), mf.SourceType)
	if mf.SourceType == "package" && mf.SourceURL != "" {
		addRow(localengine.T("plugins", "info", "package_url"), mf.SourceURL)
	}
	if len(mf.Relations) > 0 {
		addRow(localengine.T("plugins", "info", "relation"), strings.Join([]string(mf.Relations), ", "))
	}
	if mf.UpdateURL != "" {
		addRow(localengine.T("plugins", "info", "update_url"), mf.UpdateURL)
	}
	statusKey := "plugins.status.disabled"
	if mf.Enabled {
		statusKey = "plugins.status.enabled"
	}
	addRow(localengine.T("plugins", "info", "status"), localengine.T(strings.Split(statusKey, ".")...))

	// If it's a folder without update_url, show a note.
	if mf.SourceType == "folder" {
		info.Add(widget.NewLabelWithStyle(localengine.T("plugins", "note", "folder_no_update"), fyne.TextAlignLeading, fyne.TextStyle{Italic: true}))
	}

	checkBtn := widget.NewButton(localengine.T("plugins", "btn", "check_update"), func() {
		go func() {
			_, _, _ = g.ctrl.PluginCheckUpdate(g.pluginManager, name)
			fyne.Do(func() { g.refreshPluginsList() })
		}()
	})

	toggleLabel := localengine.T("plugins", "btn", "enable")
	if mf.Enabled {
		toggleLabel = localengine.T("plugins", "btn", "disable")
	}
	toggleBtn := widget.NewButton(toggleLabel, func() {
		g.togglePlugin(name)
	})

	unloadBtn := widget.NewButton(localengine.T("plugins", "btn", "unload"), func() {
		g.pluginManager.Unload(name)
		g.refreshPluginsList()
	})

	footer := container.NewHBox(checkBtn, toggleBtn, unloadBtn)
	full := container.NewBorder(nil, footer, nil, nil, container.NewScroll(info))
	d := dialog.NewCustom(fmt.Sprintf(localengine.T("plugins", "dialog", "title"), name), localengine.T("about", "dialog", "close"), full, g.window)
	d.Resize(fyne.NewSize(480, 420))
	d.Show()
}

func (g *GUI) showInstallPluginDialog() {
	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("https://example.com/my-plugin.zip")

	content := container.NewVBox(
		widget.NewLabel(localengine.T("plugins", "install", "url_label")),
		urlEntry,
	)

	var d dialog.Dialog
	saveBtn := widget.NewButton(localengine.T("plugins", "btn", "install"), func() {
		url := urlEntry.Text
		go func() {
			_ = g.ctrl.PluginInstallFromURL(g.pluginManager, url)
			fyne.Do(func() { g.refreshPluginsList() })
		}()
		d.Hide()
	})

	cancelBtn := widget.NewButton(localengine.T("dialog", "btn", "cancel"), func() { d.Hide() })
	footer := container.NewHBox(saveBtn, cancelBtn)
	full := container.NewBorder(nil, footer, nil, nil, content)
	d = dialog.NewCustom(localengine.T("plugins", "install", "title"), "", full, g.window)
	d.Resize(fyne.NewSize(500, 180))
	d.Show()
}

func (g *GUI) showGenerateTemplateDialog() {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("my-plugin")

	relationSelect := widget.NewSelect([]string{"client", "server", "both"}, nil)
	relationSelect.SetSelected("client")

	content := container.NewVBox(
		widget.NewLabel(localengine.T("plugins", "template", "name")),
		nameEntry,
		widget.NewLabel(localengine.T("plugins", "template", "relation")),
		relationSelect,
	)

	var d dialog.Dialog
	saveBtn := widget.NewButton(localengine.T("plugins", "btn", "generate"), func() {
		name := nameEntry.Text
		outDir := filepath.Join(plugins.PluginDir(), name)
		rel := relationSelect.Selected
		if rel == "" {
			rel = "client"
		}
		_ = g.ctrl.PluginGenerateTemplate(plugins.GeneratePluginTemplate, outDir, name, rel)
		g.refreshPluginsList()
		d.Hide()
	})

	cancelBtn := widget.NewButton(localengine.T("dialog", "btn", "cancel"), func() { d.Hide() })
	footer := container.NewHBox(saveBtn, cancelBtn)
	full := container.NewBorder(nil, footer, nil, nil, content)
	d = dialog.NewCustom(localengine.T("plugins", "template", "title"), "", full, g.window)
	d.Resize(fyne.NewSize(400, 220))
	d.Show()
}

func (g *GUI) initPlugins() {
	g.pluginItems = []pluginListItem{}
	g.pluginManager = plugins.NewManager(plugins.NewUIBuilder(g.window, g.tabs), g.cfg, g.ctrl.PluginManagerLogCallback())
	_ = g.ctrl.PluginDiscover(g.pluginManager)
	g.refreshPluginsList()
}
