package fynegui

import (
	"sing-box-ez/internal/i18n"

	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (g *GUI) buildMainTab() *container.TabItem {
	g.statusText = canvas.NewText(i18n.T("main.status.stopped"), colRed)
	g.statusText.TextSize = theme.TextSize()

	configs := g.cfg.GetConfigs()
	names := make([]string, 0, len(configs))
	for _, c := range configs {
		names = append(names, c.Name)
	}
	active := g.cfg.GetActiveConfig()
	selected := ""
	if active != nil {
		selected = active.Name
	}
	g.configSelect = widget.NewSelect(names, nil)
	g.configSelect.SetSelected(selected)
	if selected == "" {
		g.configSelect.PlaceHolder = i18n.T("main.active.none")
	}
	g.configSelect.OnChanged = func(s string) {
		if s == "" || g.selectingConfig {
			return
		}
		g.selectingConfig = true
		if err := g.ctrl.ActivateConfigWithLog(s); err == nil {
			g.refreshActiveLabel()
		}
		g.selectingConfig = false
	}

	g.startBtn = widget.NewButtonWithIcon(i18n.T("main.btn.start"), theme.MediaPlayIcon(), g.onStart)
	g.restartBtn = widget.NewButtonWithIcon(i18n.T("main.btn.restart"), theme.ViewRefreshIcon(), g.onRestart)
	controlRow := container.NewHBox(g.startBtn, g.restartBtn)

	content := container.NewVBox(
		g.configSelect,
		widget.NewSeparator(),
		g.statusText,
		widget.NewSeparator(),
		controlRow,
	)

	return container.NewTabItem(i18n.T("tab.main"), container.NewScroll(content))
}
