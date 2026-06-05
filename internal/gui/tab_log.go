package gui

import (
	"strings"

	"sing-box-ez/internal/i18n"

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
		g.logMu.Lock()
		updating := g.updatingLog
		g.logMu.Unlock()
		if !updating {
			g.logMu.Lock()
			text := strings.Join(g.logLines, "\n")
			g.logMu.Unlock()
			g.logEntry.SetText(text)
		}
	}

	copyBtn := widget.NewButton(i18n.T("log.btn.copy"), func() {
		g.app.Clipboard().SetContent(g.logEntry.Text)
	})
	clearBtn := widget.NewButton(i18n.T("log.btn.clear"), func() {
		g.logMu.Lock()
		g.logLines = []string{}
		g.logMu.Unlock()
		g.logEntry.SetText("")
	})

	toolbar := container.NewHBox(copyBtn, clearBtn)

	return container.NewTabItem(i18n.T("tab.log"),
		container.NewScroll(container.NewBorder(toolbar, nil, nil, nil, g.logEntry)),
	)
}
