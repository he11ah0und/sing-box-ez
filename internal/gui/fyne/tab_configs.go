package fynegui

import (
	"fmt"
	"sing-box-ez/internal/config"

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

	colWidths := g.computeConfigColumnWidths(headers)

	g.configTable = widget.NewTableWithHeaders(
		func() (int, int) { return len(g.configData), cols },
		func() fyne.CanvasObject { return newCell() },
		g.updateConfigTableCell,
	)

	for i, w := range colWidths {
		g.configTable.SetColumnWidth(i, w)
	}

	g.configTable.CreateHeader = func() fyne.CanvasObject {
		return widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}
	g.configTable.UpdateHeader = func(id widget.TableCellID, obj fyne.CanvasObject) {
		if id.Row == -1 && id.Col >= 0 && id.Col < len(headers) {
			fyne.Do(func() { obj.(*widget.Label).SetText(headers[id.Col]) })
		}
	}

	g.configTable.OnSelected = func(id widget.TableCellID) {
		if id.Row >= 0 && id.Row < len(g.configData) {
			g.configSelected = id.Row
		}
	}

	btnRow := container.NewHBox(
		widget.NewButton(g.t("configs.btn.add"), g.onAddConfig),
		widget.NewButton(g.t("configs.btn.edit"), g.onEditConfig),
		widget.NewButton(g.t("configs.btn.delete"), g.onDeleteConfig),
		widget.NewButton(g.t("configs.btn.activate"), g.onActivateConfig),
		widget.NewButton(g.t("configs.btn.update_all"), g.onUpdateAllConfigs),
	)

	totalWidth := float32(0)
	for _, w := range colWidths {
		totalWidth += w
	}
	tableScroll := container.NewScroll(g.configTable)
	tableScroll.SetMinSize(fyne.NewSize(totalWidth, 400))

	content := container.NewBorder(btnRow, nil, nil, nil, tableScroll)
	return container.NewTabItem(g.t("tab.configs"), content)
}

func (g *GUI) computeConfigColumnWidths(headers []string) []float32 {
	padding := float32(24)
	colWidths := make([]float32, len(headers))
	for i, h := range headers {
		colWidths[i] = measureTextWidth(h, true) + padding
	}
	for _, rec := range g.configData {
		colWidths[0] = maxColWidth(colWidths[0], g.configNameWidth(rec), padding)
		colWidths[1] = maxColWidth(colWidths[1], g.configSourceWidth(rec), padding)
		colWidths[2] = maxColWidth(colWidths[2], g.configLastUpdateWidth(rec), padding)
		colWidths[3] = maxColWidth(colWidths[3], g.configNextUpdateWidth(rec), padding)
		colWidths[4] = maxColWidth(colWidths[4], measureTextWidth(fmt.Sprintf("%dh", rec.UpdateIntervalHours), false), padding)
		colWidths[5] = maxColWidth(colWidths[5], g.configCachedWidth(rec), padding)
	}
	return colWidths
}

func maxColWidth(current, content, padding float32) float32 {
	if w := content + padding; w > current {
		return w
	}
	return current
}

func (g *GUI) configNameWidth(rec config.ConfigRecord) float32 {
	name := rec.Name
	if rec.Name == g.cfg.GetActiveName() {
		name = "> " + name
	}
	return measureTextWidth(name, false)
}

func (g *GUI) configSourceWidth(rec config.ConfigRecord) float32 {
	src := rec.Parent
	if src == "" || src == "user" {
		src = g.t("configs.table.user")
	} else if len(src) > 3 && src[:3] == "pl-" {
		src = src[3:]
	}
	return measureTextWidth(src, false)
}

func (g *GUI) configLastUpdateWidth(rec config.ConfigRecord) float32 {
	if rec.LastUpdate.IsZero() {
		return measureTextWidth(g.t("configs.table.never"), false)
	}
	return measureTextWidth(rec.LastUpdate.Format(timeLayout), false)
}

func (g *GUI) configNextUpdateWidth(rec config.ConfigRecord) float32 {
	next := rec.NextUpdate()
	if next.IsZero() {
		return measureTextWidth(g.t("configs.table.now"), false)
	}
	return measureTextWidth(next.Format(timeLayout), false)
}

func (g *GUI) configCachedWidth(rec config.ConfigRecord) float32 {
	if g.ctrl.HasCachedConfig(rec.Name) {
		return measureTextWidth(g.t("configs.table.yes"), false)
	}
	return measureTextWidth(g.t("configs.table.no"), false)
}

func (g *GUI) updateConfigTableCell(id widget.TableCellID, obj fyne.CanvasObject) {
	c := obj.(*cell)
	if id.Row < 0 || id.Row >= len(g.configData) {
		c.SetText("")
		c.onDblTap = nil
		return
	}
	rec := g.configData[id.Row]
	c.SetText(g.configCellText(rec, id.Col))
	c.onDblTap = func() {
		g.configSelected = id.Row
		g.onActivateConfig()
	}
	c.onRightTap = func() {
		g.configSelected = id.Row
		g.onEditConfig()
	}
}

func (g *GUI) configCellText(rec config.ConfigRecord, col int) string {
	switch col {
	case 0:
		name := rec.Name
		if rec.Name == g.cfg.GetActiveName() {
			name = "> " + name
		}
		return name
	case 1:
		src := rec.Parent
		if src == "" || src == "user" {
			src = g.t("configs.table.user")
		} else if len(src) > 3 && src[:3] == "pl-" {
			src = src[3:]
		}
		return src
	case 2:
		if rec.LastUpdate.IsZero() {
			return g.t("configs.table.never")
		}
		return rec.LastUpdate.Format(timeLayout)
	case 3:
		next := rec.NextUpdate()
		if next.IsZero() {
			return g.t("configs.table.now")
		}
		return next.Format(timeLayout)
	case 4:
		return fmt.Sprintf("%dh", rec.UpdateIntervalHours)
	case 5:
		if g.ctrl.HasCachedConfig(rec.Name) {
			return g.t("configs.table.yes")
		}
		return g.t("configs.table.no")
	}
	return ""
}

func (g *GUI) refreshConfigData() {
	g.mu.Lock()
	configs := g.cfg.GetConfigs()
	g.configData = make([]config.ConfigRecord, len(configs))
	copy(g.configData, configs)
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
		_, _ = fmt.Sscanf(periodEntry.Text, "%d", &hours)
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
			if err := g.ctrl.UpdateConfigNowWithLog(existing.Name, urlEntry.Text); err == nil {
				g.refreshConfigData()
				g.configTable.Refresh()
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
	d = dialog.NewCustomWithoutButtons(g.t("configs.dialog.title"), full, g.window)
	d.Resize(fyne.NewSize(500, 280))
	d.Show()
}

func (g *GUI) onAddConfig() {
	g.showConfigDialog(nil, func(rec config.ConfigRecord) {
		if err := g.ctrl.AddConfigWithLog(rec); err == nil {
			g.refreshActiveLabel()
			g.updateButtons()
			g.refreshConfigData()
			g.configTable.Refresh()
		}
	}, nil)
}

func (g *GUI) onEditConfig() {
	if g.configSelected < 0 || g.configSelected >= len(g.configData) {
		return
	}
	old := g.configData[g.configSelected]
	g.showConfigDialog(&old, func(rec config.ConfigRecord) {
		if err := g.ctrl.EditConfigWithLog(old.Name, rec); err == nil {
			if old.Name == g.cfg.GetActiveName() || rec.Name == g.cfg.GetActiveName() {
				g.refreshActiveLabel()
				g.updateButtons()
			}
			g.refreshConfigData()
			g.configTable.Refresh()
		}
	}, func() {
		g.onDeleteConfig()
	})
}

func (g *GUI) onDeleteConfig() {
	if g.configSelected < 0 || g.configSelected >= len(g.configData) {
		return
	}
	name := g.configData[g.configSelected].Name
	dialog.ShowConfirm(g.t("configs.dialog.delete_title"), g.t("configs.dialog.delete_msg")+" \""+name+"\"?", func(ok bool) {
		if !ok {
			return
		}
		_ = g.ctrl.DeleteConfigWithLog(name)
		g.refreshConfigData()
		g.configTable.Refresh()
		g.refreshActiveLabel()
		g.updateButtons()
	}, g.window)
}

func (g *GUI) onActivateConfig() {
	if g.configSelected < 0 || g.configSelected >= len(g.configData) {
		return
	}
	name := g.configData[g.configSelected].Name
	if err := g.ctrl.ActivateConfigWithLog(name); err == nil {
		g.refreshActiveLabel()
		g.updateButtons()
		g.configTable.Refresh()
	}
}

func (g *GUI) onUpdateAllConfigs() {
	go func() {
		progressModal, progress := g.showProgressDialog(g.t("progress.updating_configs"))
		_, total, err := g.ctrl.UpdateAllConfigsWithLog(func(done, total int) {
			fyne.Do(func() {
				progress.SetValue(float64(done) / float64(total))
			})
		})
		if err == nil && total > 0 {
			g.refreshConfigData()
			fyne.Do(func() {
				progressModal.Hide()
				g.configTable.Refresh()
			})
		}
	}()
}
