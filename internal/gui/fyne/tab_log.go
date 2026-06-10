package fynegui

import (
	"strings"

	"sing-box-ez/internal/framework/localengine"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (g *GUI) buildLogTab() *container.TabItem {
	if !g.cfg.GetShowLogs() {
		g.logEntry = nil
		return nil
	}

	g.logEntry = widget.NewMultiLineEntry()
	g.logEntry.Wrapping = fyne.TextWrapBreak
	g.logEntry.OnChanged = func(s string) {
		lines := g.ctrl.GetLogLines()
		text := strings.Join(lines, "\n")
		g.logEntry.SetText(text)
	}

	copyBtn := widget.NewButton(localengine.T("log", "btn", "copy"), func() {
		g.app.Clipboard().SetContent(g.logEntry.Text)
	})
	clearBtn := widget.NewButton(localengine.T("log", "btn", "clear"), func() {
		g.ctrl.ClearLogs()
		g.logEntry.SetText("")
	})

	toolbar := container.NewHBox(copyBtn, clearBtn)

	return container.NewTabItem(localengine.T("tab", "log"),
		container.NewScroll(container.NewBorder(toolbar, nil, nil, nil, g.logEntry)),
	)
}
