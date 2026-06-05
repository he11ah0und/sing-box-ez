package gui

import (
	"fmt"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// cell is a table cell that supports double-tap and right-click.
type cell struct {
	widget.BaseWidget
	lbl         *widget.Label
	onDblTap    func()
	onRightTap  func()
}

func newCell() *cell {
	c := &cell{lbl: widget.NewLabel("")}
	c.ExtendBaseWidget(c)
	return c
}

func (c *cell) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.lbl)
}

func (c *cell) SetText(text string) {
	c.lbl.SetText(text)
}

func (c *cell) DoubleTapped(_ *fyne.PointEvent) {
	if c.onDblTap != nil {
		c.onDblTap()
	}
}

func (c *cell) TappedSecondary(_ *fyne.PointEvent) {
	if c.onRightTap != nil {
		c.onRightTap()
	}
}

func (g *GUI) buildConfigsTab() *container.TabItem {
	g.refreshConfigData()

	headers := []string{"Name", "Source", "Last Update", "Next Update", "Period", "Cached"}
	colWidths := []float32{140, 100, 120, 120, 60, 60}
	cols := len(headers)

	g.configTable = widget.NewTableWithHeaders(
		func() (int, int) { return len(g.configData), cols },
		func() fyne.CanvasObject {
			return newCell()
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			c := obj.(*cell)
			if id.Row < 0 || id.Row >= len(g.configData) {
				c.SetText("")
				c.onDblTap = nil
				return
			}
			rec := g.configData[id.Row]
			switch id.Col {
			case 0:
				name := rec.Name
				if rec.Name == g.cfg.ActiveName {
					name = "> " + name
				}
				c.SetText(name)
			case 1:
				src := rec.Parent
				if src == "" || src == "user" {
					src = "user"
				} else if len(src) > 3 && src[:3] == "pl-" {
					src = src[3:]
				}
				c.SetText(src)
			case 2:
				if rec.LastUpdate.IsZero() {
					c.SetText("never")
				} else {
					c.SetText(rec.LastUpdate.Format(timeLayout))
				}
			case 3:
				next := rec.NextUpdate()
				if next.IsZero() {
					c.SetText("now")
				} else {
					c.SetText(next.Format(timeLayout))
				}
			case 4:
				c.SetText(fmt.Sprintf("%dh", rec.UpdateIntervalHours))
			case 5:
				if core.HasCachedConfig(rec.Name) {
					c.SetText("yes")
				} else {
					c.SetText("no")
				}
			}
			// Double-click activates the config; right-click opens edit dialog.
			c.onDblTap = func() {
				g.configSelected = id.Row
				g.onActivateConfig()
			}
			c.onRightTap = func() {
				g.configSelected = id.Row
				g.onEditConfig()
			}
		},
	)

	for i, w := range colWidths {
		g.configTable.SetColumnWidth(i, w)
	}

	// Set header texts
	g.configTable.CreateHeader = func() fyne.CanvasObject {
		return widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}
	g.configTable.UpdateHeader = func(id widget.TableCellID, obj fyne.CanvasObject) {
		if id.Row == -1 && id.Col >= 0 && id.Col < len(headers) {
			obj.(*widget.Label).SetText(headers[id.Col])
		}
	}

	// Allow selecting any cell in a row; track the row index.
	g.configTable.OnSelected = func(id widget.TableCellID) {
		if id.Row >= 0 && id.Row < len(g.configData) {
			g.configSelected = id.Row
		}
	}

	g.addBtn = widget.NewButton("Add", g.onAddConfig)
	g.editBtn = widget.NewButton("Edit", g.onEditConfig)
	g.delBtn = widget.NewButton("Delete", g.onDeleteConfig)
	g.activateBtn = widget.NewButton("Activate", g.onActivateConfig)
	g.updateAllBtn = widget.NewButton("Update all", g.onUpdateAllConfigs)
	btnRow := container.NewHBox(g.addBtn, g.editBtn, g.delBtn, g.activateBtn, g.updateAllBtn)

	content := container.NewBorder(
		btnRow,
		nil, nil, nil,
		g.configTable,
	)

	return container.NewTabItem("Configs", container.NewScroll(content))
}

func (g *GUI) refreshConfigData() {
	g.mu.Lock()
	g.configData = make([]config.ConfigRecord, len(g.cfg.Configs))
	copy(g.configData, g.cfg.Configs)
	g.mu.Unlock()
}

func (g *GUI) showConfigDialog(existing *config.ConfigRecord, onSave func(config.ConfigRecord), onDelete func()) {
	nameEntry := widget.NewEntry()
	urlEntry := widget.NewEntry()
	periodEntry := widget.NewEntry()
	periodEntry.SetText(fmt.Sprintf("%d", g.cfg.UpdateIntervalHours))

	if existing != nil {
		nameEntry.SetText(existing.Name)
		urlEntry.SetText(existing.URL)
		periodEntry.SetText(fmt.Sprintf("%d", existing.UpdateIntervalHours))
	}

	content := container.NewVBox(
		widget.NewLabel("Name"),
		nameEntry,
		widget.NewLabel("URL"),
		urlEntry,
		widget.NewLabel("Period (hours)"),
		periodEntry,
	)

	var d dialog.Dialog

	saveBtn := widget.NewButton("Save", func() {
		var hours int
		fmt.Sscanf(periodEntry.Text, "%d", &hours)
		if hours <= 0 {
			hours = g.cfg.UpdateIntervalHours
		}
		rec := config.ConfigRecord{
			Name:                nameEntry.Text,
			URL:                 urlEntry.Text,
			UpdateIntervalHours: hours,
			Parent:              "user",
		}
		if existing != nil {
			rec.LastUpdate = existing.LastUpdate
		}
		onSave(rec)
		d.Hide()
	})

	var footer *fyne.Container
	if existing != nil && onDelete != nil {
		updateBtn := widget.NewButton("Update now", func() {
			d.Hide()
			if err := core.DownloadConfigFor(existing.Name, urlEntry.Text); err != nil {
				g.log("Update failed: " + err.Error())
			} else {
				g.cfg.SetLastUpdateFor(existing.Name, time.Now())
				_ = g.cfg.Save()
				g.refreshConfigData()
				g.configTable.Refresh()
				g.log("Config updated: " + existing.Name)
			}
		})
		delBtn := widget.NewButton("Delete", func() {
			d.Hide()
			onDelete()
		})
		footer = container.NewHBox(saveBtn, updateBtn, delBtn, widget.NewButton("Cancel", func() { d.Hide() }))
	} else {
		footer = container.NewHBox(saveBtn, widget.NewButton("Cancel", func() { d.Hide() }))
	}

	full := container.NewBorder(nil, footer, nil, nil, content)
	d = dialog.NewCustom("Config", "", full, g.window)
	d.Resize(fyne.NewSize(500, 280))
	d.Show()
}

func (g *GUI) onAddConfig() {
	g.showConfigDialog(nil, func(rec config.ConfigRecord) {
		if rec.Name == "" || rec.URL == "" {
			g.log("Name and URL are required")
			return
		}
		if g.cfg.GetConfigByName(rec.Name) != nil {
			g.log("Config with this name already exists")
			return
		}
		g.cfg.AddConfig(rec)
		if g.cfg.ActiveName == "" {
			g.cfg.SetActiveName(rec.Name)
			g.manager.SetConfigURL(rec.URL)
			g.manager.SetConfigName(rec.Name)
			g.refreshActiveLabel()
		}
		_ = g.cfg.Save()
		g.refreshConfigData()
		g.configTable.Refresh()
		g.log("Config added: " + rec.Name)
	}, nil)
}

func (g *GUI) onEditConfig() {
	if g.configSelected < 0 || g.configSelected >= len(g.configData) {
		g.log("No config selected")
		return
	}
	old := g.configData[g.configSelected]
	g.showConfigDialog(&old, func(rec config.ConfigRecord) {
		if rec.Name == "" || rec.URL == "" {
			g.log("Name and URL are required")
			return
		}
		if rec.Name != old.Name {
			if g.cfg.GetConfigByName(rec.Name) != nil {
				g.log("Config with name \"" + rec.Name + "\" already exists")
				return
			}
			g.cfg.RenameConfig(old.Name, rec.Name)
		}
		g.cfg.UpdateConfig(rec.Name, rec)
		_ = g.cfg.Save()
		if old.Name == g.cfg.ActiveName || rec.Name == g.cfg.ActiveName {
			g.manager.SetConfigURL(rec.URL)
			g.refreshActiveLabel()
		}
		g.refreshConfigData()
		g.configTable.Refresh()
		if rec.Name != old.Name {
			g.log("Config renamed: " + old.Name + " -> " + rec.Name)
		} else {
			g.log("Config updated: " + rec.Name)
		}
	}, func() {
		g.onDeleteConfig()
	})
}

func (g *GUI) onDeleteConfig() {
	if g.configSelected < 0 || g.configSelected >= len(g.configData) {
		g.log("No config selected")
		return
	}
	name := g.configData[g.configSelected].Name
	dialog.ShowConfirm("Delete", "Delete config \""+name+"\"?", func(ok bool) {
		if !ok {
			return
		}
		g.cfg.RemoveConfig(name)
		_ = g.cfg.Save()
		g.refreshConfigData()
		g.configTable.Refresh()
		g.refreshActiveLabel()
		g.log("Config deleted: " + name)
	}, g.window)
}

func (g *GUI) onActivateConfig() {
	if g.configSelected < 0 || g.configSelected >= len(g.configData) {
		g.log("No config selected")
		return
	}
	name := g.configData[g.configSelected].Name
	rec := g.cfg.GetConfigByName(name)
	if rec == nil {
		return
	}

	if !core.HasCachedConfig(name) {
		g.log("No cached config for: " + name)
		return
	}

	g.cfg.SetActiveName(name)
	_ = g.cfg.Save()
	g.manager.SetConfigURL(rec.URL)
	g.manager.SetConfigName(name)
	g.refreshActiveLabel()
	g.configTable.Refresh()
	g.log("Activated config: " + name)
}

func (g *GUI) onUpdateAllConfigs() {
	go func() {
		configs := g.cfg.GetConfigs()
		for _, rec := range configs {
			if rec.URL == "" {
				continue
			}
			g.log("Updating config: " + rec.Name + "...")
			if err := core.DownloadConfigFor(rec.Name, rec.URL); err != nil {
				g.log("Failed to update " + rec.Name + ": " + err.Error())
			} else {
				g.cfg.SetLastUpdateFor(rec.Name, time.Now())
				g.log("Config updated: " + rec.Name)
			}
		}
		_ = g.cfg.Save()
		g.refreshConfigData()
		g.configTable.Refresh()
		g.log("Update all finished")
	}()
}
