//go:build !noplugins

package fynegui

import (
	"fmt"

	"sing-box-ez/internal/framework/localengine"

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
			g.ctrl.SetLogLimit(v)
		}
	}
	logLimitRow := container.NewBorder(nil, nil, widget.NewLabel(localengine.T("settings", "log_limit", "label")), widget.NewButton(localengine.T("settings", "btn", "save"), func() {
		g.logLimitEntry.OnSubmitted(g.logLimitEntry.Text)
	}), g.logLimitEntry)

	g.showLogsCheck = widget.NewCheck(localengine.T("settings", "show_logs"), func(checked bool) {
		g.cfg.SetShowLogs(checked)
		_ = g.cfg.Save()
	})
	g.showLogsCheck.SetChecked(g.cfg.GetShowLogs())

	g.desktopNotificationsCheck = widget.NewCheck(localengine.T("settings", "desktop_notifications"), func(checked bool) {
		g.cfg.SetDesktopNotifications(checked)
		_ = g.cfg.Save()
	})
	g.desktopNotificationsCheck.SetChecked(g.cfg.GetDesktopNotifications())

	// --- Language block ---
	langs := localengine.AvailableLanguages()
	langNames := make([]string, len(langs))
	langMap := make(map[string]string) // native name -> code
	for i, code := range langs {
		name := localengine.LanguageName(code)
		langNames[i] = name
		langMap[name] = code
	}
	langSelect := widget.NewSelect(langNames, nil)
	currentLang := g.cfg.GetLanguage()
	if currentLang == "" {
		currentLang = "en"
	}
	langSelect.SetSelected(localengine.LanguageName(currentLang))
	langSelect.OnChanged = func(selected string) {
		code := langMap[selected]
		g.cfg.SetLanguage(code)
		_ = g.cfg.Save()
		localengine.SetLanguage(code)
		g.rebuildUI()
	}

	// --- Config block ---
	g.defaultIntervalEntry = widget.NewEntry()
	g.defaultIntervalEntry.SetText(fmt.Sprintf("%d", g.cfg.UpdateIntervalHours))
	g.defaultIntervalEntry.OnSubmitted = func(s string) {
		var h int
		if _, err := fmt.Sscanf(s, "%d", &h); err == nil && h > 0 {
			g.ctrl.SetDefaultInterval(h)
		}
	}
	intervalRow := container.NewBorder(nil, nil, widget.NewLabel(localengine.T("settings", "default_interval", "label")), widget.NewButton(localengine.T("settings", "btn", "save"), func() {
		g.defaultIntervalEntry.OnSubmitted(g.defaultIntervalEntry.Text)
	}), g.defaultIntervalEntry)

	// --- Plugins block ---
	g.pluginsEnabledCheck = widget.NewCheck(localengine.T("settings", "plugins", "enabled"), func(checked bool) {
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

	g.pluginsDeveloperCheck = widget.NewCheck(localengine.T("settings", "plugins", "developer"), func(checked bool) {
		g.cfg.SetPluginsDeveloper(checked)
		_ = g.cfg.Save()
	})
	g.pluginsDeveloperCheck.SetChecked(g.cfg.GetPluginsDeveloper())
	if !g.cfg.GetPluginsEnabled() {
		g.pluginsDeveloperCheck.Disable()
	}

	// --- Assemble content ---
	content := container.NewVBox(
		widget.NewLabelWithStyle(localengine.T("settings", "logging", "title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		logLimitRow,
		g.showLogsCheck,
		g.desktopNotificationsCheck,
		widget.NewSeparator(),

		widget.NewLabelWithStyle(localengine.T("settings", "config", "title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		intervalRow,
		widget.NewSeparator(),

		widget.NewLabelWithStyle(localengine.T("settings", "language", "title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		langSelect,
		widget.NewSeparator(),

		widget.NewLabelWithStyle(localengine.T("settings", "reload_ui", "title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewButton(localengine.T("settings", "reload_ui", "btn"), func() {
			g.rebuildUI()
		}),
		widget.NewSeparator(),

		widget.NewLabelWithStyle(localengine.T("settings", "plugins", "title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		g.pluginsEnabledCheck,
		g.pluginsDeveloperCheck,
	)

	return container.NewTabItem(localengine.T("tab", "settings"), container.NewScroll(content))
}
