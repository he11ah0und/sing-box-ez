package gui

import (
	"fmt"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"

	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// cell is a table cell that supports double-tap and right-click.
type cell struct {
	widget.BaseWidget
	lbl        *widget.Label
	onDblTap   func()
	onRightTap func()
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
	fyne.Do(func() { c.lbl.SetText(text) })
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

// measureTextWidth returns the rendered width of a text string in pixels.
func measureTextWidth(text string, bold bool) float32 {
	style := fyne.TextStyle{Bold: bold}
	return fyne.MeasureText(text, theme.TextSize(), style).Width
}

func (g *GUI) buildConfigsTab() *container.TabItem {
	g.refreshConfigData()

	headers := []string{g.t("configs.table.name"), g.t("configs.table.source"), g.t("configs.table.last_update"), g.t("configs.table.next_update"), g.t("configs.table.period"), g.t("configs.table.cached")}
	cols := len(headers)

	// Compute dynamic column widths based on header + content text.
	padding := float32(24) // internal cell padding
	colWidths := make([]float32, cols)
	for i, h := range headers {
		colWidths[i] = measureTextWidth(h, true) + padding
	}
	for _, rec := range g.configData {
		// Col 0: name
		name := rec.Name
		if rec.Name == g.cfg.ActiveName {
			name = "> " + name
		}
		if w := measureTextWidth(name, false) + padding; w > colWidths[0] {
			colWidths[0] = w
		}
		// Col 1: source
		src := rec.Parent
		if src == "" || src == "user" {
			src = g.t("configs.table.user")
		} else if len(src) > 3 && src[:3] == "pl-" {
			src = src[3:]
		}
		if w := measureTextWidth(src, false) + padding; w > colWidths[1] {
			colWidths[1] = w
		}
		// Col 2: last update
		var lu string
		if rec.LastUpdate.IsZero() {
			lu = g.t("configs.table.never")
		} else {
			lu = rec.LastUpdate.Format(timeLayout)
		}
		if w := measureTextWidth(lu, false) + padding; w > colWidths[2] {
			colWidths[2] = w
		}
		// Col 3: next update
		var nu string
		next := rec.NextUpdate()
		if next.IsZero() {
			nu = g.t("configs.table.now")
		} else {
			nu = next.Format(timeLayout)
		}
		if w := measureTextWidth(nu, false) + padding; w > colWidths[3] {
			colWidths[3] = w
		}
		// Col 4: period
		p := fmt.Sprintf("%dh", rec.UpdateIntervalHours)
		if w := measureTextWidth(p, false) + padding; w > colWidths[4] {
			colWidths[4] = w
		}
		// Col 5: cached
		var cached string
		if core.HasCachedConfig(rec.Name) {
			cached = g.t("configs.table.yes")
		} else {
			cached = g.t("configs.table.no")
		}
		if w := measureTextWidth(cached, false) + padding; w > colWidths[5] {
			colWidths[5] = w
		}
	}

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
					src = g.t("configs.table.user")
				} else if len(src) > 3 && src[:3] == "pl-" {
					src = src[3:]
				}
				c.SetText(src)
			case 2:
				if rec.LastUpdate.IsZero() {
					c.SetText(g.t("configs.table.never"))
				} else {
					c.SetText(rec.LastUpdate.Format(timeLayout))
				}
			case 3:
				next := rec.NextUpdate()
				if next.IsZero() {
					c.SetText(g.t("configs.table.now"))
				} else {
					c.SetText(next.Format(timeLayout))
				}
			case 4:
				c.SetText(fmt.Sprintf("%dh", rec.UpdateIntervalHours))
			case 5:
				if core.HasCachedConfig(rec.Name) {
					c.SetText(g.t("configs.table.yes"))
				} else {
					c.SetText(g.t("configs.table.no"))
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
			fyne.Do(func() { obj.(*widget.Label).SetText(headers[id.Col]) })
		}
	}

	// Allow selecting any cell in a row; track the row index.
	g.configTable.OnSelected = func(id widget.TableCellID) {
		if id.Row >= 0 && id.Row < len(g.configData) {
			g.configSelected = id.Row
		}
	}

	g.addBtn = widget.NewButton(g.t("configs.btn.add"), g.onAddConfig)
	g.editBtn = widget.NewButton(g.t("configs.btn.edit"), g.onEditConfig)
	g.delBtn = widget.NewButton(g.t("configs.btn.delete"), g.onDeleteConfig)
	g.activateBtn = widget.NewButton(g.t("configs.btn.activate"), g.onActivateConfig)
	g.updateAllBtn = widget.NewButton(g.t("configs.btn.update_all"), g.onUpdateAllConfigs)
	btnRow := container.NewHBox(g.addBtn, g.editBtn, g.delBtn, g.activateBtn, g.updateAllBtn)

	totalWidth := float32(0)
	for _, w := range colWidths {
		totalWidth += w
	}
	tableScroll := container.NewScroll(g.configTable)
	tableScroll.SetMinSize(fyne.NewSize(totalWidth, 400))

	content := container.NewBorder(
		btnRow,
		nil, nil, nil,
		tableScroll,
	)

	return container.NewTabItem(g.t("tab.configs"), content)
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
		widget.NewLabel(g.t("configs.dialog.name")),
		nameEntry,
		widget.NewLabel(g.t("configs.dialog.url")),
		urlEntry,
		widget.NewLabel(g.t("configs.dialog.period")),
		periodEntry,
	)

	var d dialog.Dialog

	saveBtn := widget.NewButton(g.t("configs.dialog.btn.save"), func() {
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
		updateBtn := widget.NewButton(g.t("configs.dialog.btn.update_now"), func() {
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
		delBtn := widget.NewButton(g.t("configs.dialog.btn.delete"), func() {
			d.Hide()
			onDelete()
		})
		footer = container.NewHBox(saveBtn, updateBtn, delBtn, widget.NewButton(g.t("configs.dialog.btn.cancel"), func() { d.Hide() }))
	} else {
		footer = container.NewHBox(saveBtn, widget.NewButton(g.t("configs.dialog.btn.cancel"), func() { d.Hide() }))
	}

	full := container.NewBorder(nil, footer, nil, nil, content)
	d = dialog.NewCustom(g.t("configs.dialog.title"), g.t("configs.dialog.btn.cancel"), full, g.window)
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
			g.updateButtons()
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
			g.updateButtons()
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
	dialog.ShowConfirm(g.t("configs.dialog.delete_title"), g.t("configs.dialog.delete_msg")+" \""+name+"\"?", func(ok bool) {
		if !ok {
			return
		}
		g.cfg.RemoveConfig(name)
		_ = g.cfg.Save()
		g.refreshConfigData()
		g.configTable.Refresh()
		g.refreshActiveLabel()
		g.updateButtons()
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
	g.updateButtons()
	g.configTable.Refresh()
	g.log("Activated config: " + name)
}

func (g *GUI) onUpdateAllConfigs() {
	go func() {
		configs := g.cfg.GetConfigs()
		total := 0
		for _, rec := range configs {
			if rec.URL != "" {
				total++
			}
		}
		if total == 0 {
			g.log("No configs to update")
			return
		}

		progressModal, progress := g.showProgressDialog(g.t("progress.updating_configs"))
		updated := 0
		for _, rec := range configs {
			if rec.URL == "" {
				continue
			}
			g.log("Updating config: " + rec.Name + "...")
			if err := core.DownloadConfigFor(rec.Name, rec.URL); err != nil {
				g.log("Failed to update " + rec.Name + ": " + err.Error())
			} else {
				g.cfg.SetLastUpdateFor(rec.Name, time.Now())
				updated++
				g.log("Config updated: " + rec.Name)
			}
			fyne.Do(func() {
				progress.SetValue(float64(updated) / float64(total))
			})
		}
		_ = g.cfg.Save()
		g.refreshConfigData()
		fyne.Do(func() {
			progressModal.Hide()
			g.configTable.Refresh()
		})
		g.log(fmt.Sprintf("Update all finished (%d/%d)", updated, total))
	}()
}
