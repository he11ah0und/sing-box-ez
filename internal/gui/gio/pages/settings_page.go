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
)

// SettingsPage renders the application settings screen.
type SettingsPage struct {
	th   *material.Theme
	ctrl *core.InteractiveController

	// Dialog provider for language picker dropdown.
	dialog DialogProvider

	OnLanguageChange func()

	// Dirty tracking – original saved values.
	origLogLimit     string
	origInterval     string
	origShowLogs     bool
	origDesktopNotif bool
	origLang         string

	// Pending (unsaved) language selection.
	pendingLang string

	logLimitEditor widget.Editor
	intervalEditor widget.Editor
	showLogs       widget.Bool
	desktopNotif   widget.Bool
	langBtn        widget.Clickable
	saveBtn        widget.Clickable
}

// NewSettingsPage creates a new settings page.
func NewSettingsPage(th *material.Theme, ctrl *core.InteractiveController, dialog DialogProvider) *SettingsPage {
	p := &SettingsPage{
		th:     th,
		ctrl:   ctrl,
		dialog: dialog,
	}

	p.origLogLimit = fmt.Sprintf("%d", ctrl.Controller.Config().GetLogLimit())
	p.logLimitEditor.SingleLine = true
	p.logLimitEditor.Filter = "0123456789"
	p.logLimitEditor.SetText(p.origLogLimit)

	p.origInterval = fmt.Sprintf("%d", ctrl.Controller.Config().UpdateIntervalHours)
	p.intervalEditor.SingleLine = true
	p.intervalEditor.Filter = "0123456789"
	p.intervalEditor.SetText(p.origInterval)

	p.origShowLogs = ctrl.Controller.Config().GetShowLogs()
	p.showLogs.Value = p.origShowLogs

	p.origDesktopNotif = ctrl.Controller.Config().GetDesktopNotifications()
	p.desktopNotif.Value = p.origDesktopNotif

	p.origLang = ctrl.Controller.Config().GetLanguage()
	p.pendingLang = p.origLang

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
		p.intervalEditor.Text() != p.origInterval ||
		p.showLogs.Value != p.origShowLogs ||
		p.desktopNotif.Value != p.origDesktopNotif ||
		p.pendingLang != p.origLang
}

// Layout draws the settings page.
func (p *SettingsPage) Layout(gtx layout.Context) layout.Dimensions {
	if p.isDirty() && p.saveBtn.Clicked(gtx) {
		if v, err := strconv.Atoi(p.logLimitEditor.Text()); err == nil && v >= 0 {
			p.ctrl.Controller.SetLogLimit(v)
			p.origLogLimit = fmt.Sprintf("%d", v)
		}
		if h, err := strconv.Atoi(p.intervalEditor.Text()); err == nil && h > 0 {
			p.ctrl.Controller.SetDefaultInterval(h)
			p.origInterval = fmt.Sprintf("%d", h)
		}
		p.ctrl.Controller.Config().SetShowLogs(p.showLogs.Value)
		p.origShowLogs = p.showLogs.Value
		p.ctrl.Controller.Config().SetDesktopNotifications(p.desktopNotif.Value)
		p.origDesktopNotif = p.desktopNotif.Value
		if p.pendingLang != p.origLang {
			p.ctrl.Controller.Config().SetLanguage(p.pendingLang)
			localengine.SetLanguage(p.pendingLang)
			p.origLang = p.pendingLang
			if p.OnLanguageChange != nil {
				p.OnLanguageChange()
			}
		}
		_ = p.ctrl.Controller.Config().Save()
	}

	if p.langBtn.Clicked(gtx) {
		p.openLangPicker()
	}

	logLimitDirty := p.logLimitEditor.Text() != p.origLogLimit
	intervalDirty := p.intervalEditor.Text() != p.origInterval
	showLogsDirty := p.showLogs.Value != p.origShowLogs
	desktopNotifDirty := p.desktopNotif.Value != p.origDesktopNotif
	langDirty := p.pendingLang != p.origLang
	dirty := p.isDirty()

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Header row with Save button on the right.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.H6(p.th, localengine.T("settings", "logging", "title")).Layout(gtx)
					})
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
			label := localengine.T("settings", "log_limit", "label")
			if logLimitDirty {
				label += " *"
			}
			return LabeledInput(gtx, p.th, label, &p.logLimitEditor, logLimitDirty)
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
			return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.H6(p.th, localengine.T("settings", "config", "title")).Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := localengine.T("settings", "default_interval", "label")
			if intervalDirty {
				label += " *"
			}
			return LabeledInput(gtx, p.th, label, &p.intervalEditor, intervalDirty)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.H6(p.th, localengine.T("settings", "language", "title")).Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lang := p.pendingLang
			if lang == "" {
				lang = "en"
			}
			btnLabel := localengine.LanguageName(lang)
			if langDirty {
				btnLabel += " *"
			}
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Button(p.th, &p.langBtn, btnLabel).Layout(gtx)
			})
		}),
	)
}

func (p *SettingsPage) openLangPicker() {
	langs := localengine.AvailableLanguages()
	btns := make([]widget.Clickable, len(langs))

	p.dialog.ShowCustom(localengine.T("settings", "language", "title"), func(gtx layout.Context) layout.Dimensions {
		for i := range langs {
			if btns[i].Clicked(gtx) {
				p.dialog.HideCustom()
				p.pendingLang = langs[i]
			}
		}

		children := make([]layout.FlexChild, len(langs))
		for i, l := range langs {
			idx := i
			lang := l
			label := localengine.LanguageName(lang)
			if lang == p.pendingLang {
				label = "> " + label
			}
			children[i] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return material.Button(p.th, &btns[idx], label).Layout(gtx)
				})
			})
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}
