//go:build noplugins

package fynegui

import (
	"fmt"

	"sing-box-ez/internal/i18n"

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
			g.ctrl.SetLogLimitWithLog(v)
		}
	}
	logLimitRow := container.NewBorder(nil, nil, widget.NewLabel(i18n.T("settings.log_limit.label")), widget.NewButton(i18n.T("settings.btn.save"), func() {
		g.logLimitEntry.OnSubmitted(g.logLimitEntry.Text)
	}), g.logLimitEntry)

	g.showLogsCheck = widget.NewCheck(i18n.T("settings.show_logs"), func(checked bool) {
		g.cfg.SetShowLogs(checked)
		_ = g.cfg.Save()
	})
	g.showLogsCheck.SetChecked(g.cfg.GetShowLogs())

	g.desktopNotificationsCheck = widget.NewCheck(i18n.T("settings.desktop_notifications"), func(checked bool) {
		g.cfg.SetDesktopNotifications(checked)
		_ = g.cfg.Save()
	})
	g.desktopNotificationsCheck.SetChecked(g.cfg.GetDesktopNotifications())

	// --- Language block ---
	langs := i18n.AvailableLanguages()
	langNames := make([]string, len(langs))
	langMap := make(map[string]string) // native name -> code
	for i, code := range langs {
		name := i18n.LanguageName(code)
		langNames[i] = name
		langMap[name] = code
	}
	langSelect := widget.NewSelect(langNames, nil)
	currentLang := g.cfg.GetLanguage()
	if currentLang == "" {
		currentLang = "en"
	}
	langSelect.SetSelected(i18n.LanguageName(currentLang))
	langSelect.OnChanged = func(selected string) {
		code := langMap[selected]
		g.cfg.SetLanguage(code)
		_ = g.cfg.Save()
		i18n.SetLanguage(code)
		g.rebuildUI()
	}

	// --- Config block ---
	g.defaultIntervalEntry = widget.NewEntry()
	g.defaultIntervalEntry.SetText(fmt.Sprintf("%d", g.cfg.UpdateIntervalHours))
	g.defaultIntervalEntry.OnSubmitted = func(s string) {
		var h int
		if _, err := fmt.Sscanf(s, "%d", &h); err == nil && h > 0 {
			g.ctrl.SetDefaultIntervalWithLog(h)
		}
	}
	intervalRow := container.NewBorder(nil, nil, widget.NewLabel(i18n.T("settings.default_interval.label")), widget.NewButton(i18n.T("settings.btn.save"), func() {
		g.defaultIntervalEntry.OnSubmitted(g.defaultIntervalEntry.Text)
	}), g.defaultIntervalEntry)

	// --- Assemble content ---
	content := container.NewVBox(
		widget.NewLabelWithStyle(i18n.T("settings.logging.title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		logLimitRow,
		g.showLogsCheck,
		g.desktopNotificationsCheck,
		widget.NewSeparator(),

		widget.NewLabelWithStyle(i18n.T("settings.config.title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		intervalRow,
		widget.NewSeparator(),

		widget.NewLabelWithStyle(i18n.T("settings.language.title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		langSelect,
		widget.NewSeparator(),

		widget.NewLabelWithStyle(i18n.T("settings.reload_ui.title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewButton(i18n.T("settings.reload_ui.btn"), func() {
			g.rebuildUI()
		}),
	)

	return container.NewTabItem(i18n.T("tab.settings"), container.NewScroll(content))
}
