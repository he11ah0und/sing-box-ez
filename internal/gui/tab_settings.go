package gui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (g *GUI) buildSettingsTab() *container.TabItem {
	// --- Logging block ---
	g.logLimitEntry = widget.NewEntry()
	g.logLimitEntry.SetText(fmt.Sprintf("%d", g.cfg.GetLogLimit()))
	g.logLimitEntry.OnSubmitted = func(s string) {
		var v int
		if _, err := fmt.Sscanf(s, "%d", &v); err == nil && v >= 0 {
			g.cfg.SetLogLimit(v)
			_ = g.cfg.Save()
			g.log("Log limit set to " + s)
		}
	}
	logLimitRow := container.NewBorder(nil, nil, widget.NewLabel("Log limit (lines, 0=unlimited):"), widget.NewButton("Save", func() {
		g.logLimitEntry.OnSubmitted(g.logLimitEntry.Text)
	}), g.logLimitEntry)

	g.showLogsCheck = widget.NewCheck("Show logs", func(checked bool) {
		g.cfg.SetShowLogs(checked)
		_ = g.cfg.Save()
	})
	g.showLogsCheck.SetChecked(g.cfg.GetShowLogs())

	g.showCoreLogsCheck = widget.NewCheck("Show core logs", func(checked bool) {
		g.cfg.SetShowCoreLogs(checked)
		_ = g.cfg.Save()
	})
	g.showCoreLogsCheck.SetChecked(g.cfg.GetShowCoreLogs())

	// --- Config block ---
	g.defaultIntervalEntry = widget.NewEntry()
	g.defaultIntervalEntry.SetText(fmt.Sprintf("%d", g.cfg.UpdateIntervalHours))
	g.defaultIntervalEntry.OnSubmitted = func(s string) {
		var h int
		if _, err := fmt.Sscanf(s, "%d", &h); err == nil && h > 0 {
			g.cfg.SetDefaultUpdateInterval(h)
			_ = g.cfg.Save()
			g.log("Default interval set to " + s + "h")
		}
	}
	intervalRow := container.NewBorder(nil, nil, widget.NewLabel("Default update interval (hours):"), widget.NewButton("Save", func() {
		g.defaultIntervalEntry.OnSubmitted(g.defaultIntervalEntry.Text)
	}), g.defaultIntervalEntry)

	// --- Plugins block ---
	g.pluginsEnabledCheck = widget.NewCheck("Plugins feature", func(checked bool) {
		g.cfg.SetPluginsEnabled(checked)
		_ = g.cfg.Save()
		if !checked {
			g.pluginsDeveloperCheck.SetChecked(false)
			g.cfg.SetPluginsDeveloper(false)
			g.pluginsDeveloperCheck.Disable()
		} else {
			g.pluginsDeveloperCheck.Enable()
		}
	})
	g.pluginsEnabledCheck.SetChecked(g.cfg.GetPluginsEnabled())

	g.pluginsDeveloperCheck = widget.NewCheck("Plugins developer", func(checked bool) {
		g.cfg.SetPluginsDeveloper(checked)
		_ = g.cfg.Save()
	})
	g.pluginsDeveloperCheck.SetChecked(g.cfg.GetPluginsDeveloper())
	if !g.cfg.GetPluginsEnabled() {
		g.pluginsDeveloperCheck.Disable()
	}

	// --- Assemble content ---
	content := container.NewVBox(
		widget.NewLabelWithStyle("Logging", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		logLimitRow,
		g.showLogsCheck,
		g.showCoreLogsCheck,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("Config", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		intervalRow,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("Plugins", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		g.pluginsEnabledCheck,
		g.pluginsDeveloperCheck,
	)

	return container.NewTabItem("Settings", container.NewScroll(content))
}
