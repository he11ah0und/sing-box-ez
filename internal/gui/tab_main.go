package gui

import (
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (g *GUI) buildMainTab() *container.TabItem {
	g.statusText = canvas.NewText("Status: stopped", colRed)
	g.statusText.TextSize = theme.TextSize()
	g.activeLbl = widget.NewLabel("Active config: (none)")
	g.refreshActiveLabel()

	g.startBtn = widget.NewButtonWithIcon("Start", theme.MediaPlayIcon(), g.onStart)
	g.stopBtn = widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), g.onStop)
	g.restartBtn = widget.NewButtonWithIcon("Restart", theme.ViewRefreshIcon(), g.onRestart)
	controlRow := container.NewHBox(g.startBtn, g.stopBtn, g.restartBtn)

	content := container.NewVBox(
		g.activeLbl,
		widget.NewSeparator(),
		g.statusText,
		widget.NewSeparator(),
		controlRow,
	)

	return container.NewTabItem("Main", container.NewScroll(content))
}
