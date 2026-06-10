package pages

import (
	"fmt"
	"image"
	"strconv"

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

	logLimitEditor widget.Editor
	logLimitSave   widget.Clickable

	showLogs     widget.Bool
	desktopNotif widget.Bool

	intervalEditor widget.Editor
	intervalSave   widget.Clickable

	langBtn widget.Clickable
}

// NewSettingsPage creates a new settings page.
func NewSettingsPage(th *material.Theme, ctrl *core.InteractiveController, dialog DialogProvider) *SettingsPage {
	p := &SettingsPage{
		th:     th,
		ctrl:   ctrl,
		dialog: dialog,
	}

	p.logLimitEditor.SingleLine = true
	p.logLimitEditor.SetText(fmt.Sprintf("%d", ctrl.Config().GetLogLimit()))

	p.intervalEditor.SingleLine = true
	p.intervalEditor.SetText(fmt.Sprintf("%d", ctrl.Config().UpdateIntervalHours))

	p.showLogs.Value = ctrl.Config().GetShowLogs()
	p.desktopNotif.Value = ctrl.Config().GetDesktopNotifications()

	return p
}

// Tag returns the page tag.
func (p *SettingsPage) Tag() string { return "settings" }

// Name returns the page name.
func (p *SettingsPage) Name() string { return localengine.T("tab.settings") }

// Layout draws the settings page.
func (p *SettingsPage) Layout(gtx layout.Context) layout.Dimensions {
	if p.logLimitSave.Clicked(gtx) {
		if v, err := strconv.Atoi(p.logLimitEditor.Text()); err == nil && v >= 0 {
			p.ctrl.SetLogLimitWithLog(v)
		}
	}
	if p.intervalSave.Clicked(gtx) {
		if h, err := strconv.Atoi(p.intervalEditor.Text()); err == nil && h > 0 {
			p.ctrl.SetDefaultIntervalWithLog(h)
		}
	}
	if changed := p.showLogs.Update(gtx); changed {
		p.ctrl.Config().SetShowLogs(p.showLogs.Value)
		_ = p.ctrl.Config().Save()
	}
	if changed := p.desktopNotif.Update(gtx); changed {
		p.ctrl.Config().SetDesktopNotifications(p.desktopNotif.Value)
		_ = p.ctrl.Config().Save()
	}

	if p.langBtn.Clicked(gtx) {
		p.openLangPicker()
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.H6(p.th, localengine.T("settings.logging.title")).Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.inputWithLabelAndSave(gtx, localengine.T("settings.log_limit.label"), &p.logLimitEditor, &p.logLimitSave)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.CheckBox(p.th, &p.showLogs, localengine.T("settings.show_logs")).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.CheckBox(p.th, &p.desktopNotif, localengine.T("settings.desktop_notifications")).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.H6(p.th, localengine.T("settings.config.title")).Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.inputWithLabelAndSave(gtx, localengine.T("settings.default_interval.label"), &p.intervalEditor, &p.intervalSave)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.H6(p.th, localengine.T("settings.language.title")).Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lang := p.ctrl.Config().GetLanguage()
				if lang == "" {
					lang = "en"
				}
				return material.Button(p.th, &p.langBtn, localengine.LanguageName(lang)).Layout(gtx)
			})
		}),
	)
}

func (p *SettingsPage) openLangPicker() {
	langs := localengine.AvailableLanguages()
	btns := make([]widget.Clickable, len(langs))

	p.dialog.ShowCustom(localengine.T("settings.language.title"), func(gtx layout.Context) layout.Dimensions {
		for i := range langs {
			if btns[i].Clicked(gtx) {
				p.dialog.HideCustom()
				p.ctrl.Config().SetLanguage(langs[i])
				_ = p.ctrl.Config().Save()
				localengine.SetLanguage(langs[i])
				if p.OnLanguageChange != nil {
					p.OnLanguageChange()
				}
			}
		}

		children := make([]layout.FlexChild, len(langs))
		for i, l := range langs {
			idx := i
			lang := l
			label := localengine.LanguageName(lang)
			if lang == p.ctrl.Config().GetLanguage() {
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

func (p *SettingsPage) inputWithLabelAndSave(gtx layout.Context, label string, ed *widget.Editor, saveBtn *widget.Clickable) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(p.th, label).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Point{X: gtx.Dp(unit.Dp(8)), Y: 0}}
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.Editor(p.th, ed, "").Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Point{X: gtx.Dp(unit.Dp(8)), Y: 0}}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Button(p.th, saveBtn, localengine.T("settings.btn.save")).Layout(gtx)
		}),
	)
}
