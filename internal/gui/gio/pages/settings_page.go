package pages

import (
	"fmt"
	"strconv"

	"gio.tools/icons"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/gui/gio/theme"
	"sing-box-ez/internal/gui/gio/widgets"
)

// SettingsPage renders the application settings screen.
type SettingsPage struct {
	th   *material.Theme
	ctrl *core.InteractiveController

	// Dialog provider for dropdown pickers.
	dialog widgets.DialogProvider

	OnLanguageChange func()
	// OnResetRequested is called when the user presses the reset button.
	OnResetRequested func()
	// OnShowLogsChange is called whenever the "Show logs" toggle changes.
	OnShowLogsChange func(show bool)

	// Dirty tracking – original saved values.
	origLogLimit                      string
	origLogLevel                      string
	origInterval                      string
	origAutoUpdateConfigsInterval     string
	origBackgroundUpdateCheckInterval string
	origShowLogs                      bool
	origDesktopNotif                  bool
	origAutoCheckSelf                 bool
	origAutoCheckCore                 bool
	origAutoUpdateConfigs             bool
	origAutoRestartOnConfigUpdate     bool
	origLang                          string
	origTheme                         string
	origThemeMode                     string

	// Pending (unsaved) selections.
	pendingLang      string
	pendingLogLevel  string
	pendingTheme     string
	pendingThemeMode string

	logLimitEditor                      widget.Editor
	intervalEditor                      widget.Editor
	autoUpdateConfigsIntervalEditor     widget.Editor
	backgroundUpdateCheckIntervalEditor widget.Editor
	showLogs                            widget.Bool
	desktopNotif                        widget.Bool
	autoCheckSelf                       widget.Bool
	autoCheckCore                       widget.Bool
	autoUpdateConfigs                   widget.Bool
	autoRestartOnConfigUpdate           widget.Bool
	logLevelDropdown                    *widgets.Dropdown
	langDropdown                        *widgets.Dropdown
	themeDropdown                       *widgets.Dropdown
	themeModeDropdown                   *widgets.Dropdown
	saveBtn                             widget.Clickable
	resetBtn                            widget.Clickable
}

// NewSettingsPage creates a new settings page.
func NewSettingsPage(th *material.Theme, ctrl *core.InteractiveController, dialog widgets.DialogProvider) *SettingsPage {
	p := &SettingsPage{
		th:     th,
		ctrl:   ctrl,
		dialog: dialog,
	}

	p.origLogLimit = fmt.Sprintf("%d", ctrl.Controller.Config().MustGet("log", "limit").Int())
	p.logLimitEditor.SingleLine = true
	p.logLimitEditor.Filter = "0123456789"
	p.logLimitEditor.SetText(p.origLogLimit)

	p.origInterval = fmt.Sprintf("%d", ctrl.Controller.Config().MustGet("updates", "default_interval_hours").Int())
	p.intervalEditor.SingleLine = true
	p.intervalEditor.Filter = "0123456789"
	p.intervalEditor.SetText(p.origInterval)

	p.origAutoUpdateConfigsInterval = fmt.Sprintf("%d", ctrl.Controller.Config().MustGet("updates", "auto_update_configs_interval_hours").Int())
	p.autoUpdateConfigsIntervalEditor.SingleLine = true
	p.autoUpdateConfigsIntervalEditor.Filter = "0123456789"
	p.autoUpdateConfigsIntervalEditor.SetText(p.origAutoUpdateConfigsInterval)

	p.origBackgroundUpdateCheckInterval = fmt.Sprintf("%d", ctrl.Controller.Config().MustGet("updates", "background_update_check_interval_hours").Int())
	p.backgroundUpdateCheckIntervalEditor.SingleLine = true
	p.backgroundUpdateCheckIntervalEditor.Filter = "0123456789"
	p.backgroundUpdateCheckIntervalEditor.SetText(p.origBackgroundUpdateCheckInterval)

	p.origShowLogs = ctrl.Controller.Config().MustGet("ui", "show_logs").Bool()
	p.showLogs.Value = p.origShowLogs

	p.origDesktopNotif = ctrl.Controller.Config().MustGet("ui", "desktop_notifications").Bool()
	p.desktopNotif.Value = p.origDesktopNotif

	p.origAutoCheckSelf = ctrl.Controller.Config().MustGet("updates", "auto_check_self").Bool()
	p.autoCheckSelf.Value = p.origAutoCheckSelf

	p.origAutoCheckCore = ctrl.Controller.Config().MustGet("updates", "auto_check_core").Bool()
	p.autoCheckCore.Value = p.origAutoCheckCore

	p.origAutoUpdateConfigs = ctrl.Controller.Config().MustGet("updates", "auto_update_configs").Bool()
	p.autoUpdateConfigs.Value = p.origAutoUpdateConfigs

	p.origAutoRestartOnConfigUpdate = ctrl.Controller.Config().MustGet("updates", "auto_restart_on_config_update").Bool()
	p.autoRestartOnConfigUpdate.Value = p.origAutoRestartOnConfigUpdate

	p.origLang = ctrl.Controller.Config().MustGet("ui", "language").String()
	p.pendingLang = p.origLang

	p.origLogLevel = ctrl.Controller.Config().MustGet("log", "level").String()
	p.pendingLogLevel = p.origLogLevel

	p.origTheme = ctrl.Controller.Config().MustGet("ui", "theme").String()
	if p.origTheme == "" {
		p.origTheme = "default"
	}
	p.pendingTheme = p.origTheme

	p.origThemeMode = ctrl.Controller.Config().MustGet("ui", "theme_mode").String()
	if p.origThemeMode == "" {
		p.origThemeMode = "system"
	}
	p.pendingThemeMode = p.origThemeMode

	levels := []string{"debug", "info", "warn", "error"}
	p.logLevelDropdown = widgets.NewDropdown(
		th, dialog,
		localengine.T("settings", "log_level", "label"),
		p.pendingLogLevel,
		levels,
		func(level string) string { return localengine.T("settings", "log_level", level) },
		func(level string) { p.pendingLogLevel = level },
	)

	langs := localengine.AvailableLanguages()
	p.langDropdown = widgets.NewDropdown(
		th, dialog,
		localengine.T("settings", "language", "title"),
		p.pendingLang,
		langs,
		func(lang string) string { return localengine.LanguageName(lang) },
		func(lang string) { p.pendingLang = lang },
	)

	if theme.M != nil {
		themeNames := theme.M.Names()
		p.themeDropdown = widgets.NewDropdown(
			th, dialog,
			localengine.T("settings", "theme", "title"),
			p.pendingTheme,
			themeNames,
			func(name string) string { return name },
			func(name string) { p.pendingTheme = name },
		)

		modes := []string{"system", "dark", "light"}
		p.themeModeDropdown = widgets.NewDropdown(
			th, dialog,
			localengine.T("settings", "theme_mode", "title"),
			p.pendingThemeMode,
			modes,
			func(mode string) string { return localengine.T("settings", "theme_mode", mode) },
			func(mode string) { p.pendingThemeMode = mode },
		)
	}

	return p
}

// Tag returns the page tag.
func (p *SettingsPage) Tag() string { return "settings" }

// Name returns the page name.
func (p *SettingsPage) Name() string { return localengine.T("tab", "settings") }

// Icon returns the page icon.
func (p *SettingsPage) Icon() *widget.Icon { return icons.ActionSettings }

func (p *SettingsPage) isDirty() bool {
	return p.logLimitEditor.Text() != p.origLogLimit ||
		p.pendingLogLevel != p.origLogLevel ||
		p.intervalEditor.Text() != p.origInterval ||
		p.autoUpdateConfigsIntervalEditor.Text() != p.origAutoUpdateConfigsInterval ||
		p.backgroundUpdateCheckIntervalEditor.Text() != p.origBackgroundUpdateCheckInterval ||
		p.showLogs.Value != p.origShowLogs ||
		p.desktopNotif.Value != p.origDesktopNotif ||
		p.autoCheckSelf.Value != p.origAutoCheckSelf ||
		p.autoCheckCore.Value != p.origAutoCheckCore ||
		p.autoUpdateConfigs.Value != p.origAutoUpdateConfigs ||
		p.autoRestartOnConfigUpdate.Value != p.origAutoRestartOnConfigUpdate ||
		p.pendingLang != p.origLang ||
		p.pendingTheme != p.origTheme ||
		p.pendingThemeMode != p.origThemeMode
}

// Layout draws the settings page.
func (p *SettingsPage) Layout(gtx layout.Context) layout.Dimensions {
	return widgets.SpacedList(gtx, p.Children(gtx)...)
}

// Children returns the page widgets; the shell adds standard vertical spacing.
func (p *SettingsPage) Children(gtx layout.Context) []layout.FlexChild {
	if p.isDirty() && p.saveBtn.Clicked(gtx) {
		if v, err := strconv.Atoi(p.logLimitEditor.Text()); err == nil && v >= 0 {
			p.ctrl.Controller.SetLogLimit(v)
			p.origLogLimit = fmt.Sprintf("%d", v)
		}
		if h, err := strconv.Atoi(p.intervalEditor.Text()); err == nil && h > 0 {
			p.ctrl.Controller.SetDefaultInterval(h)
			p.origInterval = fmt.Sprintf("%d", h)
		}
		if h, err := strconv.Atoi(p.backgroundUpdateCheckIntervalEditor.Text()); err == nil && h > 0 {
			p.ctrl.Controller.Config().MustGet("updates", "background_update_check_interval_hours").Update(h)
			p.origBackgroundUpdateCheckInterval = fmt.Sprintf("%d", h)
		}
		if h, err := strconv.Atoi(p.autoUpdateConfigsIntervalEditor.Text()); err == nil && h > 0 {
			p.ctrl.Controller.Config().MustGet("updates", "auto_update_configs_interval_hours").Update(h)
			p.origAutoUpdateConfigsInterval = fmt.Sprintf("%d", h)
		}
		p.ctrl.Controller.Config().MustGet("ui", "show_logs").Update(p.showLogs.Value)
		if p.OnShowLogsChange != nil {
			p.OnShowLogsChange(p.showLogs.Value)
		}
		p.origShowLogs = p.showLogs.Value
		p.ctrl.Controller.Config().MustGet("log", "level").Update(p.pendingLogLevel)
		p.origLogLevel = p.pendingLogLevel
		p.ctrl.Controller.Config().MustGet("ui", "desktop_notifications").Update(p.desktopNotif.Value)
		p.origDesktopNotif = p.desktopNotif.Value
		p.ctrl.Controller.Config().MustGet("updates", "auto_check_self").Update(p.autoCheckSelf.Value)
		p.origAutoCheckSelf = p.autoCheckSelf.Value
		p.ctrl.Controller.Config().MustGet("updates", "auto_check_core").Update(p.autoCheckCore.Value)
		p.origAutoCheckCore = p.autoCheckCore.Value
		p.ctrl.Controller.Config().MustGet("updates", "auto_update_configs").Update(p.autoUpdateConfigs.Value)
		p.origAutoUpdateConfigs = p.autoUpdateConfigs.Value
		p.ctrl.Controller.Config().MustGet("updates", "auto_restart_on_config_update").Update(p.autoRestartOnConfigUpdate.Value)
		p.origAutoRestartOnConfigUpdate = p.autoRestartOnConfigUpdate.Value
		if p.pendingLang != p.origLang {
			p.ctrl.Controller.Config().MustGet("ui", "language").Update(p.pendingLang)
			localengine.SetLanguage(p.pendingLang)
			p.origLang = p.pendingLang
			if p.OnLanguageChange != nil {
				p.OnLanguageChange()
			}
		}
		if p.pendingTheme != p.origTheme || p.pendingThemeMode != p.origThemeMode {
			if theme.M != nil {
				mode := theme.Mode(p.pendingThemeMode)
				if mode == "" {
					mode = theme.ModeSystem
				}
				if err := theme.M.Apply(p.pendingTheme, mode); err == nil {
					_ = p.ctrl.Controller.Config().MustGet("ui", "theme").Update(p.pendingTheme)
					_ = p.ctrl.Controller.Config().MustGet("ui", "theme_mode").Update(string(mode))
					p.origTheme = p.pendingTheme
					p.origThemeMode = p.pendingThemeMode
				}
			}
		}
		_ = p.ctrl.Controller.Config().Save()
	}

	logLimitDirty := p.logLimitEditor.Text() != p.origLogLimit
	logLevelDirty := p.pendingLogLevel != p.origLogLevel
	intervalDirty := p.intervalEditor.Text() != p.origInterval
	autoUpdateConfigsIntervalDirty := p.autoUpdateConfigsIntervalEditor.Text() != p.origAutoUpdateConfigsInterval
	backgroundUpdateCheckIntervalDirty := p.backgroundUpdateCheckIntervalEditor.Text() != p.origBackgroundUpdateCheckInterval
	showLogsDirty := p.showLogs.Value != p.origShowLogs
	desktopNotifDirty := p.desktopNotif.Value != p.origDesktopNotif
	autoCheckSelfDirty := p.autoCheckSelf.Value != p.origAutoCheckSelf
	autoCheckCoreDirty := p.autoCheckCore.Value != p.origAutoCheckCore
	autoUpdateConfigsDirty := p.autoUpdateConfigs.Value != p.origAutoUpdateConfigs
	autoRestartDirty := p.autoRestartOnConfigUpdate.Value != p.origAutoRestartOnConfigUpdate
	langDirty := p.pendingLang != p.origLang
	themeDirty := p.pendingTheme != p.origTheme
	themeModeDirty := p.pendingThemeMode != p.origThemeMode
	dirty := p.isDirty()

	p.logLevelDropdown.SetValue(p.pendingLogLevel)
	p.logLevelDropdown.SetLabel(localengine.T("settings", "log_level", "label"))
	p.langDropdown.SetValue(p.pendingLang)
	p.langDropdown.SetLabel(localengine.T("settings", "language", "title"))
	if p.themeDropdown != nil {
		p.themeDropdown.SetValue(p.pendingTheme)
		p.themeDropdown.SetLabel(localengine.T("settings", "theme", "title"))
	}
	if p.themeModeDropdown != nil {
		p.themeModeDropdown.SetValue(p.pendingThemeMode)
		p.themeModeDropdown.SetLabel(localengine.T("settings", "theme_mode", "title"))
	}

	if p.resetBtn.Clicked(gtx) && p.OnResetRequested != nil {
		p.OnResetRequested()
	}

	return []layout.FlexChild{
		// Header row with Save button on the right.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return material.H6(p.th, localengine.T("settings", "language", "title")).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !dirty {
						return layout.Dimensions{}
					}
					return material.Button(p.th, &p.saveBtn, localengine.T("settings", "btn", "save")).Layout(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.langDropdown.Layout(gtx, langDirty)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if p.themeDropdown == nil {
				return layout.Dimensions{}
			}
			return material.H6(p.th, localengine.T("settings", "theme", "title")).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if p.themeDropdown == nil {
				return layout.Dimensions{}
			}
			return p.themeDropdown.Layout(gtx, themeDirty)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if p.themeModeDropdown == nil {
				return layout.Dimensions{}
			}
			return p.themeModeDropdown.Layout(gtx, themeModeDirty)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.H6(p.th, localengine.T("settings", "logging", "title")).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := localengine.T("settings", "log_limit", "label")
			if logLimitDirty {
				label += " *"
			}
			return widgets.LabeledInput(gtx, p.th, label, &p.logLimitEditor, logLimitDirty)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.logLevelDropdown.Layout(gtx, logLevelDirty)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := localengine.T("settings", "show_logs")
			if showLogsDirty {
				label += " *"
			}
			return material.CheckBox(p.th, &p.showLogs, label).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := localengine.T("settings", "desktop_notifications")
			if desktopNotifDirty {
				label += " *"
			}
			return material.CheckBox(p.th, &p.desktopNotif, label).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.H6(p.th, localengine.T("settings", "update_check", "title")).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := localengine.T("settings", "update_check", "self")
			if autoCheckSelfDirty {
				label += " *"
			}
			return material.CheckBox(p.th, &p.autoCheckSelf, label).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := localengine.T("settings", "update_check", "core")
			if autoCheckCoreDirty {
				label += " *"
			}
			return material.CheckBox(p.th, &p.autoCheckCore, label).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := localengine.T("settings", "update_check", "background_interval")
			if backgroundUpdateCheckIntervalDirty {
				label += " *"
			}
			return widgets.LabeledInput(gtx, p.th, label, &p.backgroundUpdateCheckIntervalEditor, backgroundUpdateCheckIntervalDirty)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.H6(p.th, localengine.T("settings", "config_update", "title")).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := localengine.T("settings", "config_update", "auto")
			if autoUpdateConfigsDirty {
				label += " *"
			}
			return material.CheckBox(p.th, &p.autoUpdateConfigs, label).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := localengine.T("settings", "config_update", "auto_restart")
			if autoRestartDirty {
				label += " *"
			}
			return material.CheckBox(p.th, &p.autoRestartOnConfigUpdate, label).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := localengine.T("settings", "config_update", "interval")
			if autoUpdateConfigsIntervalDirty {
				label += " *"
			}
			return widgets.LabeledInput(gtx, p.th, label, &p.autoUpdateConfigsIntervalEditor, autoUpdateConfigsIntervalDirty)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.H6(p.th, localengine.T("settings", "config", "title")).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := localengine.T("settings", "default_interval", "label")
			if intervalDirty {
				label += " *"
			}
			return widgets.LabeledInput(gtx, p.th, label, &p.intervalEditor, intervalDirty)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.H6(p.th, localengine.T("settings", "reset", "title")).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Button(p.th, &p.resetBtn, localengine.T("settings", "reset", "btn")).Layout(gtx)
		}),
	}
}
