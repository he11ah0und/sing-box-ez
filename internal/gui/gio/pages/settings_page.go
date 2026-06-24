package pages

import (
	"fmt"
	"strconv"

	"gio.tools/icons"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/gui/gio/startup"
	"sing-box-ez/internal/gui/gio/theme"
	"sing-box-ez/internal/gui/gio/widgets"
)

const (
	settingsTabInterface = "interface"
	settingsTabConfigs   = "configs"
	settingsTabCore      = "core"
	settingsTabSystem    = "system"
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
	origAutoUpdateOnHashMismatch      bool
	origAutoRestartOnConfigUpdate     bool
	origStartupMode                   string
	origShowStartupDialog             bool
	origLang                          string
	origTheme                         string
	origThemeMode                     string

	startupOptions []startup.Option

	// Pending (unsaved) selections.
	pendingLang        string
	pendingLogLevel    string
	pendingTheme       string
	pendingThemeMode   string
	pendingStartupMode string

	selectedTab string

	tabInterface widget.Clickable
	tabConfigs   widget.Clickable
	tabCore      widget.Clickable
	tabSystem    widget.Clickable

	corePage *CorePage

	logLimitEditor                      widget.Editor
	intervalEditor                      widget.Editor
	autoUpdateConfigsIntervalEditor     widget.Editor
	backgroundUpdateCheckIntervalEditor widget.Editor
	showLogs                            widget.Bool
	desktopNotif                        widget.Bool
	autoCheckSelf                       widget.Bool
	autoCheckCore                       widget.Bool
	autoUpdateConfigs                   widget.Bool
	autoUpdateOnHashMismatch            widget.Bool
	autoRestartOnConfigUpdate           widget.Bool
	showStartupDialog                   widget.Bool
	logLevelDropdown                    *widgets.Dropdown
	startupModeDropdown                 *widgets.Dropdown
	langDropdown                        *widgets.Dropdown
	themeDropdown                       *widgets.Dropdown
	themeModeDropdown                   *widgets.Dropdown
	saveBtn                             widget.Clickable
	resetBtn                            widget.Clickable
}

// NewSettingsPage creates a new settings page.
func NewSettingsPage(th *material.Theme, ctrl *core.InteractiveController, dialog widgets.DialogProvider, corePage *CorePage) *SettingsPage {
	p := &SettingsPage{
		th:          th,
		ctrl:        ctrl,
		dialog:      dialog,
		selectedTab: settingsTabInterface,
		corePage:    corePage,
	}

	p.origLogLimit = fmt.Sprintf("%d", ctrl.Backend().Config().MustGet("log", "limit").Int())
	p.logLimitEditor.SingleLine = true
	p.logLimitEditor.Filter = "0123456789"
	p.logLimitEditor.SetText(p.origLogLimit)

	p.origInterval = fmt.Sprintf("%d", ctrl.Backend().Config().MustGet("updates", "default_interval_hours").Int())
	p.intervalEditor.SingleLine = true
	p.intervalEditor.Filter = "0123456789"
	p.intervalEditor.SetText(p.origInterval)

	p.origAutoUpdateConfigsInterval = fmt.Sprintf("%d", ctrl.Backend().Config().MustGet("updates", "auto_update_configs_interval_hours").Int())
	p.autoUpdateConfigsIntervalEditor.SingleLine = true
	p.autoUpdateConfigsIntervalEditor.Filter = "0123456789"
	p.autoUpdateConfigsIntervalEditor.SetText(p.origAutoUpdateConfigsInterval)

	p.origBackgroundUpdateCheckInterval = fmt.Sprintf("%d", ctrl.Backend().Config().MustGet("updates", "background_update_check_interval_hours").Int())
	p.backgroundUpdateCheckIntervalEditor.SingleLine = true
	p.backgroundUpdateCheckIntervalEditor.Filter = "0123456789"
	p.backgroundUpdateCheckIntervalEditor.SetText(p.origBackgroundUpdateCheckInterval)

	p.origShowLogs = ctrl.Backend().Config().MustGet("ui", "show_logs").Bool()
	p.showLogs.Value = p.origShowLogs

	p.origDesktopNotif = ctrl.Backend().Config().MustGet("ui", "desktop_notifications").Bool()
	p.desktopNotif.Value = p.origDesktopNotif

	p.origAutoCheckSelf = ctrl.Backend().Config().MustGet("updates", "auto_check_self").Bool()
	p.autoCheckSelf.Value = p.origAutoCheckSelf

	p.origAutoCheckCore = ctrl.Backend().Config().MustGet("updates", "auto_check_core").Bool()
	p.autoCheckCore.Value = p.origAutoCheckCore

	p.origAutoUpdateConfigs = ctrl.Backend().Config().MustGet("updates", "auto_update_configs").Bool()
	p.autoUpdateConfigs.Value = p.origAutoUpdateConfigs

	p.origAutoUpdateOnHashMismatch = ctrl.Backend().Config().MustGet("updates", "auto_update_on_hash_mismatch").Bool()
	p.autoUpdateOnHashMismatch.Value = p.origAutoUpdateOnHashMismatch

	p.origAutoRestartOnConfigUpdate = ctrl.Backend().Config().MustGet("updates", "auto_restart_on_config_update").Bool()
	p.autoRestartOnConfigUpdate.Value = p.origAutoRestartOnConfigUpdate

	p.startupOptions = startup.Discover(ctrl.Backend().Config())
	p.origStartupMode = ctrl.Backend().Config().MustGet("remote", "last_connection_mode").String()
	if p.origStartupMode == "" || p.origStartupMode == "embedded" {
		p.origStartupMode = "embed"
	}
	if !p.startupOptionExists(p.origStartupMode) {
		p.origStartupMode = "embed"
	}
	p.pendingStartupMode = p.origStartupMode

	p.origShowStartupDialog = !ctrl.Backend().Config().MustGet("remote", "remember_connection_mode").Bool()
	p.showStartupDialog.Value = p.origShowStartupDialog

	p.origLang = ctrl.Backend().Config().MustGet("ui", "language").String()
	p.pendingLang = p.origLang

	p.origLogLevel = ctrl.Backend().Config().MustGet("log", "level").String()
	p.pendingLogLevel = p.origLogLevel

	p.origTheme = ctrl.Backend().Config().MustGet("ui", "theme").String()
	if p.origTheme == "" {
		p.origTheme = "default"
	}
	p.pendingTheme = p.origTheme

	p.origThemeMode = ctrl.Backend().Config().MustGet("ui", "theme_mode").String()
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

	p.startupModeDropdown = widgets.NewDropdown(
		th, dialog,
		localengine.T("settings", "startup", "mode_label"),
		p.pendingStartupMode,
		p.startupOptionIDs(),
		func(id string) string { return p.formatStartupOption(id) },
		func(id string) { p.pendingStartupMode = id },
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
func (p *SettingsPage) startupOptionExists(id string) bool {
	for _, opt := range p.startupOptions {
		if opt.ID == id {
			return true
		}
	}
	return false
}

func (p *SettingsPage) startupOptionIDs() []string {
	ids := make([]string, len(p.startupOptions))
	for i, opt := range p.startupOptions {
		ids[i] = opt.ID
	}
	return ids
}

func (p *SettingsPage) formatStartupOption(id string) string {
	for i := range p.startupOptions {
		if p.startupOptions[i].ID == id {
			opt := &p.startupOptions[i]
			statusKey := "status_online"
			if !opt.Online {
				statusKey = "status_offline"
			}
			status := localengine.T("startup", statusKey)
			switch opt.Type {
			case "embed":
				return localengine.T("settings", "startup", "mode_embed") + " (" + status + ")"
			case "remote":
				name := localengine.T("settings", "startup", "mode_remote")
				if opt.Address != "" {
					name += " " + opt.Address
				}
				return name + " (" + status + ")"
			default:
				name := opt.Type
				if n := localengine.T("settings", "startup", "mode_"+opt.Type); n != "" && n != "mode_"+opt.Type {
					name = n
				}
				return name + " (" + status + ")"
			}
		}
	}
	return id
}

func (p *SettingsPage) Tag() string { return "settings" }

// Name returns the page name.
func (p *SettingsPage) Name() string { return localengine.T("tab", "settings") }

// Icon returns the page icon.
func (p *SettingsPage) Icon() *widget.Icon { return icons.ActionSettings }

// settingsDirty tracks per-field unsaved changes.
type settingsDirty struct {
	logLimit                      bool
	logLevel                      bool
	interval                      bool
	autoUpdateConfigsInterval     bool
	backgroundUpdateCheckInterval bool
	showLogs                      bool
	desktopNotif                  bool
	autoCheckSelf                 bool
	autoCheckCore                 bool
	autoUpdateConfigs             bool
	autoUpdateOnHashMismatch      bool
	autoRestart                   bool
	startupMode                   bool
	showStartupDialog             bool
	lang                          bool
	theme                         bool
	themeMode                     bool
}

func (d settingsDirty) any() bool {
	return d.numericDirty() || d.boolDirty() || d.choiceDirty()
}

func (d settingsDirty) numericDirty() bool {
	return d.logLimit || d.interval || d.autoUpdateConfigsInterval || d.backgroundUpdateCheckInterval
}

func (d settingsDirty) boolDirty() bool {
	return d.showLogs || d.desktopNotif || d.autoCheckSelf || d.autoCheckCore || d.autoUpdateConfigs || d.autoUpdateOnHashMismatch || d.autoRestart || d.showStartupDialog
}

func (d settingsDirty) choiceDirty() bool {
	return d.logLevel || d.startupMode || d.lang || d.theme || d.themeMode
}

func (p *SettingsPage) dirtyFlags() settingsDirty {
	return settingsDirty{
		logLimit:                      p.logLimitEditor.Text() != p.origLogLimit,
		logLevel:                      p.pendingLogLevel != p.origLogLevel,
		interval:                      p.intervalEditor.Text() != p.origInterval,
		autoUpdateConfigsInterval:     p.autoUpdateConfigsIntervalEditor.Text() != p.origAutoUpdateConfigsInterval,
		backgroundUpdateCheckInterval: p.backgroundUpdateCheckIntervalEditor.Text() != p.origBackgroundUpdateCheckInterval,
		showLogs:                      p.showLogs.Value != p.origShowLogs,
		desktopNotif:                  p.desktopNotif.Value != p.origDesktopNotif,
		autoCheckSelf:                 p.autoCheckSelf.Value != p.origAutoCheckSelf,
		autoCheckCore:                 p.autoCheckCore.Value != p.origAutoCheckCore,
		autoUpdateConfigs:             p.autoUpdateConfigs.Value != p.origAutoUpdateConfigs,
		autoUpdateOnHashMismatch:      p.autoUpdateOnHashMismatch.Value != p.origAutoUpdateOnHashMismatch,
		autoRestart:                   p.autoRestartOnConfigUpdate.Value != p.origAutoRestartOnConfigUpdate,
		startupMode:                   p.pendingStartupMode != p.origStartupMode,
		showStartupDialog:             p.showStartupDialog.Value != p.origShowStartupDialog,
		lang:                          p.pendingLang != p.origLang,
		theme:                         p.pendingTheme != p.origTheme,
		themeMode:                     p.pendingThemeMode != p.origThemeMode,
	}
}

func (p *SettingsPage) updateDropdowns() {
	p.logLevelDropdown.SetValue(p.pendingLogLevel)
	p.logLevelDropdown.SetLabel(localengine.T("settings", "log_level", "label"))
	if p.startupModeDropdown != nil {
		p.startupModeDropdown.SetValue(p.pendingStartupMode)
		p.startupModeDropdown.SetLabel(localengine.T("settings", "startup", "mode_label"))
	}
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
}

func (p *SettingsPage) applySettings(d *settingsDirty) {
	if p.isEnabled("log", "limit") {
		p.applyIntField(d.logLimit, &p.logLimitEditor, &p.origLogLimit, 0, func(v int) { p.ctrl.Backend().SetLogLimit(v) })
	}
	if p.isEnabled("updates", "default_interval_hours") {
		p.applyIntField(d.interval, &p.intervalEditor, &p.origInterval, 1, func(v int) { p.ctrl.Backend().SetDefaultInterval(v) })
	}
	if p.isEnabled("updates", "background_update_check_interval_hours") {
		p.applyIntField(d.backgroundUpdateCheckInterval, &p.backgroundUpdateCheckIntervalEditor, &p.origBackgroundUpdateCheckInterval, 1, func(v int) {
			p.ctrl.Backend().Config().MustGet("updates", "background_update_check_interval_hours").Update(v)
		})
	}
	if p.isEnabled("updates", "auto_update_configs_interval_hours") {
		p.applyIntField(d.autoUpdateConfigsInterval, &p.autoUpdateConfigsIntervalEditor, &p.origAutoUpdateConfigsInterval, 1, func(v int) {
			p.ctrl.Backend().Config().MustGet("updates", "auto_update_configs_interval_hours").Update(v)
		})
	}

	if p.isEnabled("ui", "show_logs") {
		p.ctrl.Backend().Config().MustGet("ui", "show_logs").Update(p.showLogs.Value)
		if p.OnShowLogsChange != nil {
			p.OnShowLogsChange(p.showLogs.Value)
		}
		p.origShowLogs = p.showLogs.Value
	}

	if p.isEnabled("log", "level") {
		p.ctrl.Backend().Config().MustGet("log", "level").Update(p.pendingLogLevel)
		p.origLogLevel = p.pendingLogLevel
	}

	if p.isEnabled("ui", "desktop_notifications") {
		p.ctrl.Backend().Config().MustGet("ui", "desktop_notifications").Update(p.desktopNotif.Value)
		p.origDesktopNotif = p.desktopNotif.Value
	}

	if p.isEnabled("updates", "auto_check_self") {
		p.ctrl.Backend().Config().MustGet("updates", "auto_check_self").Update(p.autoCheckSelf.Value)
		p.origAutoCheckSelf = p.autoCheckSelf.Value
	}

	if p.isEnabled("updates", "auto_check_core") {
		p.ctrl.Backend().Config().MustGet("updates", "auto_check_core").Update(p.autoCheckCore.Value)
		p.origAutoCheckCore = p.autoCheckCore.Value
	}

	if p.isEnabled("updates", "auto_update_configs") {
		p.ctrl.Backend().Config().MustGet("updates", "auto_update_configs").Update(p.autoUpdateConfigs.Value)
		p.origAutoUpdateConfigs = p.autoUpdateConfigs.Value
	}

	if p.isEnabled("updates", "auto_update_on_hash_mismatch") {
		p.ctrl.Backend().Config().MustGet("updates", "auto_update_on_hash_mismatch").Update(p.autoUpdateOnHashMismatch.Value)
		p.origAutoUpdateOnHashMismatch = p.autoUpdateOnHashMismatch.Value
	}

	if p.isEnabled("updates", "auto_restart_on_config_update") {
		p.ctrl.Backend().Config().MustGet("updates", "auto_restart_on_config_update").Update(p.autoRestartOnConfigUpdate.Value)
		p.origAutoRestartOnConfigUpdate = p.autoRestartOnConfigUpdate.Value
	}

	if p.isEnabled("remote", "last_connection_mode") {
		p.ctrl.Backend().Config().MustGet("remote", "last_connection_mode").Update(p.pendingStartupMode)
		p.origStartupMode = p.pendingStartupMode
	}

	if p.isEnabled("remote", "remember_connection_mode") {
		p.ctrl.Backend().Config().MustGet("remote", "remember_connection_mode").Update(!p.showStartupDialog.Value)
		p.origShowStartupDialog = p.showStartupDialog.Value
	}

	if d.lang && p.isEnabled("ui", "language") {
		p.ctrl.Backend().Config().MustGet("ui", "language").Update(p.pendingLang)
		localengine.SetLanguage(p.pendingLang)
		p.origLang = p.pendingLang
		if p.OnLanguageChange != nil {
			p.OnLanguageChange()
		}
	}

	if (d.theme || d.themeMode) && p.isEnabled("ui", "theme") && p.isEnabled("ui", "theme_mode") {
		if theme.M != nil {
			mode := theme.Mode(p.pendingThemeMode)
			if mode == "" {
				mode = theme.ModeSystem
			}
			if err := theme.M.Apply(p.pendingTheme, mode); err == nil {
				_ = p.ctrl.Backend().Config().MustGet("ui", "theme").Update(p.pendingTheme)
				_ = p.ctrl.Backend().Config().MustGet("ui", "theme_mode").Update(string(mode))
				p.origTheme = p.pendingTheme
				p.origThemeMode = p.pendingThemeMode
			}
		}
	}

	_ = p.ctrl.Backend().Config().Save()
}

func (p *SettingsPage) applyIntField(dirty bool, ed *widget.Editor, orig *string, min int, apply func(int)) {
	if !dirty {
		return
	}
	if v, err := strconv.Atoi(ed.Text()); err == nil && v >= min {
		apply(v)
		*orig = fmt.Sprintf("%d", v)
	}
}

// Layout draws the settings page.
func (p *SettingsPage) Layout(gtx layout.Context) layout.Dimensions {
	return widgets.SpacedList(gtx, p.Children(gtx)...)
}

// Children returns the page widgets; the shell adds standard vertical spacing.
func (p *SettingsPage) Children(gtx layout.Context) []layout.FlexChild {
	d := p.dirtyFlags()
	if d.any() && p.saveBtn.Clicked(gtx) {
		p.applySettings(&d)
	}
	p.updateDropdowns()
	if p.resetBtn.Clicked(gtx) && p.OnResetRequested != nil {
		p.OnResetRequested()
	}

	if p.tabInterface.Clicked(gtx) && p.selectedTab != settingsTabInterface {
		p.selectedTab = settingsTabInterface
	}
	if p.tabConfigs.Clicked(gtx) && p.selectedTab != settingsTabConfigs {
		p.selectedTab = settingsTabConfigs
	}
	if p.tabCore.Clicked(gtx) && p.selectedTab != settingsTabCore {
		p.selectedTab = settingsTabCore
	}
	if p.tabSystem.Clicked(gtx) && p.selectedTab != settingsTabSystem {
		p.selectedTab = settingsTabSystem
	}

	children := []layout.FlexChild{
		layout.Rigid(p.layoutTabBar(&d)),
	}
	children = append(children, p.tabFields(gtx, &d)...)
	return children
}

func (p *SettingsPage) layoutTabBar(d *settingsDirty) func(layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceStart, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return p.tabButton(gtx, settingsTabInterface, localengine.T("settings", "tab", "interface"), &p.tabInterface)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widgets.HSpace(gtx, unit.Dp(4))
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return p.tabButton(gtx, settingsTabConfigs, localengine.T("settings", "tab", "configs"), &p.tabConfigs)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widgets.HSpace(gtx, unit.Dp(4))
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return p.tabButton(gtx, settingsTabCore, localengine.T("settings", "tab", "core"), &p.tabCore)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widgets.HSpace(gtx, unit.Dp(4))
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return p.tabButton(gtx, settingsTabSystem, localengine.T("settings", "tab", "system"), &p.tabSystem)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !d.any() {
					return layout.Dimensions{}
				}
				return material.Button(p.th, &p.saveBtn, localengine.T("settings", "btn", "save")).Layout(gtx)
			}),
		)
	}
}

func (p *SettingsPage) tabButton(gtx layout.Context, tab, label string, btn *widget.Clickable) layout.Dimensions {
	colors := theme.Current().Colors()
	active := p.selectedTab == tab
	bg := colors.Surface
	fg := colors.Fg
	borderColor := colors.Border
	if active {
		bg = colors.SurfaceVariant
		fg = colors.Primary
		borderColor = colors.Primary
	}
	if btn.Hovered() {
		bg = colors.Hover
	}

	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return widgets.BorderedCard(gtx, borderColor, bg, unit.Dp(1), unit.Dp(4), unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(p.th, label)
			lbl.Color = fg
			if active {
				lbl.Font.Weight = 700
			}
			return lbl.Layout(gtx)
		})
	})
}

func (p *SettingsPage) tabFields(gtx layout.Context, d *settingsDirty) []layout.FlexChild {
	switch p.selectedTab {
	case settingsTabConfigs:
		return p.configsTabFields(d)
	case settingsTabCore:
		return p.corePage.Children(gtx)
	case settingsTabSystem:
		return p.systemTabFields(d)
	default:
		return p.interfaceTabFields(d)
	}
}

// SelectCoreTab switches the settings view to the Core tab.
func (p *SettingsPage) SelectCoreTab() {
	p.selectedTab = settingsTabCore
}

func (p *SettingsPage) interfaceTabFields(d *settingsDirty) []layout.FlexChild {
	var fields []func(layout.Context) layout.Dimensions

	if p.isEnabled("ui", "language") {
		fields = append(fields, p.section(localengine.T("settings", "language", "title")))
		fields = append(fields, func(gtx layout.Context) layout.Dimensions { return p.langDropdown.Layout(gtx, d.lang) })
	}

	if p.isEnabled("ui", "theme") || p.isEnabled("ui", "theme_mode") {
		fields = append(fields, p.section(localengine.T("settings", "theme", "title")))
		if p.isEnabled("ui", "theme") && p.themeDropdown != nil {
			fields = append(fields, func(gtx layout.Context) layout.Dimensions { return p.themeDropdown.Layout(gtx, d.theme) })
		}
		if p.isEnabled("ui", "theme_mode") && p.themeModeDropdown != nil {
			fields = append(fields, func(gtx layout.Context) layout.Dimensions { return p.themeModeDropdown.Layout(gtx, d.themeMode) })
		}
	}

	if p.isEnabled("ui", "desktop_notifications") {
		fields = append(fields, func(gtx layout.Context) layout.Dimensions {
			return p.checkBox(gtx, &p.desktopNotif, localengine.T("settings", "desktop_notifications"), d.desktopNotif)
		})
	}

	if p.isEnabled("ui", "show_logs") {
		fields = append(fields, func(gtx layout.Context) layout.Dimensions {
			return p.checkBox(gtx, &p.showLogs, localengine.T("settings", "show_logs"), d.showLogs)
		})
	}

	children := make([]layout.FlexChild, 0, len(fields))
	for _, f := range fields {
		children = append(children, layout.Rigid(f))
	}
	return children
}

func (p *SettingsPage) configsTabFields(d *settingsDirty) []layout.FlexChild {
	var fields []func(layout.Context) layout.Dimensions

	if p.isEnabled("updates", "auto_update_configs") || p.isEnabled("updates", "auto_update_on_hash_mismatch") || p.isEnabled("updates", "auto_restart_on_config_update") ||
		p.isEnabled("updates", "auto_update_configs_interval_hours") || p.isEnabled("updates", "background_update_check_interval_hours") {
		fields = append(fields, p.section(localengine.T("settings", "config_update", "title")))
		if p.isEnabled("updates", "auto_update_configs") {
			fields = append(fields, func(gtx layout.Context) layout.Dimensions {
				return p.checkBox(gtx, &p.autoUpdateConfigs, localengine.T("settings", "config_update", "auto"), d.autoUpdateConfigs)
			})
		}
		if p.isEnabled("updates", "auto_update_on_hash_mismatch") {
			fields = append(fields, func(gtx layout.Context) layout.Dimensions {
				return p.checkBox(gtx, &p.autoUpdateOnHashMismatch, localengine.T("settings", "config_update", "hash_mismatch"), d.autoUpdateOnHashMismatch)
			})
		}
		if p.isEnabled("updates", "auto_restart_on_config_update") {
			fields = append(fields, func(gtx layout.Context) layout.Dimensions {
				return p.checkBox(gtx, &p.autoRestartOnConfigUpdate, localengine.T("settings", "config_update", "auto_restart"), d.autoRestart)
			})
		}
		if p.isEnabled("updates", "auto_update_configs_interval_hours") {
			fields = append(fields, func(gtx layout.Context) layout.Dimensions {
				return p.labeledInput(gtx, localengine.T("settings", "config_update", "interval"), &p.autoUpdateConfigsIntervalEditor, d.autoUpdateConfigsInterval)
			})
		}
		if p.isEnabled("updates", "background_update_check_interval_hours") {
			fields = append(fields, func(gtx layout.Context) layout.Dimensions {
				return p.labeledInput(gtx, localengine.T("settings", "config_update", "background_interval"), &p.backgroundUpdateCheckIntervalEditor, d.backgroundUpdateCheckInterval)
			})
		}
	}

	if p.isEnabled("updates", "default_interval_hours") {
		fields = append(fields, p.section(localengine.T("settings", "config", "title")))
		fields = append(fields, func(gtx layout.Context) layout.Dimensions {
			return p.labeledInput(gtx, localengine.T("settings", "default_interval", "label"), &p.intervalEditor, d.interval)
		})
	}

	children := make([]layout.FlexChild, 0, len(fields))
	for _, f := range fields {
		children = append(children, layout.Rigid(f))
	}
	return children
}

func (p *SettingsPage) systemTabFields(d *settingsDirty) []layout.FlexChild {
	var fields []func(layout.Context) layout.Dimensions

	if p.isEnabled("updates", "auto_check_self") || p.isEnabled("updates", "auto_check_core") {
		fields = append(fields, p.section(localengine.T("settings", "update_check", "title")))
		if p.isEnabled("updates", "auto_check_self") {
			fields = append(fields, func(gtx layout.Context) layout.Dimensions {
				return p.checkBox(gtx, &p.autoCheckSelf, localengine.T("settings", "update_check", "self"), d.autoCheckSelf)
			})
		}
		if p.isEnabled("updates", "auto_check_core") {
			fields = append(fields, func(gtx layout.Context) layout.Dimensions {
				return p.checkBox(gtx, &p.autoCheckCore, localengine.T("settings", "update_check", "core"), d.autoCheckCore)
			})
		}
	}

	if p.isEnabled("remote", "last_connection_mode") || p.isEnabled("remote", "remember_connection_mode") {
		fields = append(fields, p.section(localengine.T("settings", "startup", "title")))
		if p.isEnabled("remote", "last_connection_mode") && p.startupModeDropdown != nil {
			fields = append(fields, func(gtx layout.Context) layout.Dimensions {
				return p.startupModeDropdown.Layout(gtx, d.startupMode)
			})
		}
		if p.isEnabled("remote", "remember_connection_mode") {
			fields = append(fields, func(gtx layout.Context) layout.Dimensions {
				return p.checkBox(gtx, &p.showStartupDialog, localengine.T("settings", "startup", "show_dialog"), d.showStartupDialog)
			})
		}
	}

	if p.isEnabled("log", "limit") || p.isEnabled("log", "level") {
		fields = append(fields, p.section(localengine.T("settings", "logging", "title")))
		if p.isEnabled("log", "limit") {
			fields = append(fields, func(gtx layout.Context) layout.Dimensions {
				return p.labeledInput(gtx, localengine.T("settings", "log_limit", "label"), &p.logLimitEditor, d.logLimit)
			})
		}
		if p.isEnabled("log", "level") {
			fields = append(fields, func(gtx layout.Context) layout.Dimensions { return p.logLevelDropdown.Layout(gtx, d.logLevel) })
		}
	}

	fields = append(fields, p.section(localengine.T("settings", "reset", "title")))
	fields = append(fields, func(gtx layout.Context) layout.Dimensions {
		return material.Button(p.th, &p.resetBtn, localengine.T("settings", "reset", "btn")).Layout(gtx)
	})

	children := make([]layout.FlexChild, 0, len(fields))
	for _, f := range fields {
		children = append(children, layout.Rigid(f))
	}
	return children
}

func (p *SettingsPage) isEnabled(path ...string) bool {
	cell, err := p.ctrl.Backend().Config().Get(path...)
	return err == nil && !cell.IsDisabled()
}

func (p *SettingsPage) section(title string) func(layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		return material.H6(p.th, title).Layout(gtx)
	}
}

func (p *SettingsPage) checkBox(gtx layout.Context, btn *widget.Bool, label string, dirty bool) layout.Dimensions {
	if dirty {
		label += " *"
	}
	return material.CheckBox(p.th, btn, label).Layout(gtx)
}

func (p *SettingsPage) labeledInput(gtx layout.Context, label string, ed *widget.Editor, dirty bool) layout.Dimensions {
	if dirty {
		label += " *"
	}
	return widgets.LabeledInput(gtx, p.th, label, ed, dirty)
}
