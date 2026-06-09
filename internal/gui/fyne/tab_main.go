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
	g.activeLbl = widget.NewLabel(i18n.T("main.active.none"))
	g.refreshActiveLabel()

	g.startBtn = widget.NewButtonWithIcon(i18n.T("main.btn.start"), theme.MediaPlayIcon(), g.onStart)
	g.stopBtn = widget.NewButtonWithIcon(i18n.T("main.btn.stop"), theme.MediaStopIcon(), g.onStop)
	g.restartBtn = widget.NewButtonWithIcon(i18n.T("main.btn.restart"), theme.ViewRefreshIcon(), g.onRestart)
	controlRow := container.NewHBox(g.startBtn, g.stopBtn, g.restartBtn)

	content := container.NewVBox(
		g.activeLbl,
		widget.NewSeparator(),
		g.statusText,
		widget.NewSeparator(),
		controlRow,
	)

	return container.NewTabItem(i18n.T("tab.main"), container.NewScroll(content))
}
