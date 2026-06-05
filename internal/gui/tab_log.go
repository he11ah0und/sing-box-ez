package gui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (g *GUI) buildLogTab() *container.TabItem {
	g.logEntry = widget.NewMultiLineEntry()
	g.logEntry.Wrapping = fyne.TextWrapBreak
	g.logEntry.OnChanged = func(s string) {
		if !g.updatingLog {
			g.updatingLog = true
			g.logEntry.SetText(strings.Join(g.logLines, "\n"))
			g.updatingLog = false
		}
	}

	copyBtn := widget.NewButton("Copy all", func() {
		g.window.Clipboard().SetContent(g.logEntry.Text)
	})
	clearBtn := widget.NewButton("Clear", func() {
		g.logLines = []string{}
		g.logEntry.SetText("")
	})

	toolbar := container.NewHBox(copyBtn, clearBtn)

	return container.NewTabItem("Log",
		container.NewScroll(container.NewBorder(toolbar, nil, nil, nil, g.logEntry)),
	)
}
