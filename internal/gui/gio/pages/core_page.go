package pages

import (
	"fmt"
	"image"
	"image/color"

	"gio.tools/icons"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework/localengine"
)

// CorePage renders the core management screen.
type CorePage struct {
	th     *material.Theme
	ctrl   *core.InteractiveController
	dialog DialogProvider

	versionText string
	latestText  string

	downloadBtn widget.Clickable
	checkBtn    widget.Clickable

	autoRestart widget.Bool
	watchLogs   widget.Bool

	restartAdminBtn widget.Clickable

	// privilegeMode is either "admin" or "setcap" (Linux only).
	privilegeMode string
	// privilegePickerBtn opens the mode selector dialog.
	privilegePickerBtn widget.Clickable

	privilegeState core.PrivilegeTabState
}

// NewCorePage creates a new core page.
func NewCorePage(th *material.Theme, ctrl *core.InteractiveController, dialog DialogProvider) *CorePage {
	p := &CorePage{
		th:     th,
		ctrl:   ctrl,
		dialog: dialog,
	}
	p.autoRestart.Value = ctrl.Controller.Config().GetCoreAutoRestart()
	p.watchLogs.Value = ctrl.Controller.Config().GetWatchCoreLogs()

	go p.refreshVersions()
	p.privilegeState = ctrl.Controller.GetPrivilegeTabState()

	// Default privilege mode: admin, unless setcap is already detected.
	p.privilegeMode = "admin"
	if p.privilegeState.Mode == "linux" && p.privilegeState.HasSetcap {
		p.privilegeMode = "setcap"
	}

	return p
}

func (p *CorePage) refreshVersions() {
	ver, err := p.ctrl.Controller.GetInstalledCoreVersion()
	if err != nil || ver == "" {
		p.versionText = localengine.T("core", "version", "not_installed")
	} else {
		p.versionText = localengine.T("core", "version", "installed") + ver
	}
	p.latestText = localengine.T("core", "latest", "checking")
}

// Tag returns the page tag.
func (p *CorePage) Tag() string { return "core" }

// Name returns the page name.
func (p *CorePage) Name() string       { return localengine.T("tab", "core") }
func (p *CorePage) Icon() *widget.Icon { return icons.AVPlayArrow }

// Layout draws the core page.
func (p *CorePage) Layout(gtx layout.Context) layout.Dimensions {
	if p.downloadBtn.Clicked(gtx) {
		go p.onDownloadCore()
	}
	if p.checkBtn.Clicked(gtx) {
		go p.onCheckVersion()
	}
	if p.restartAdminBtn.Clicked(gtx) {
		go func() {
			_ = p.ctrl.Controller.RestartAsAdmin()
		}()
	}
	if p.privilegePickerBtn.Clicked(gtx) {
		p.openPrivilegePicker()
	}

	if changed := p.autoRestart.Update(gtx); changed {
		p.ctrl.Controller.Config().SetCoreAutoRestart(p.autoRestart.Value)
		_ = p.ctrl.Controller.Config().Save()
	}
	if changed := p.watchLogs.Update(gtx); changed {
		p.ctrl.Controller.Config().SetWatchCoreLogs(p.watchLogs.Value)
		_ = p.ctrl.Controller.Config().Save()
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.H6(p.th, localengine.T("tab", "core")).Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(p.th, p.versionText).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Button(p.th, &p.downloadBtn, localengine.T("core", "btn", "download")).Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Button(p.th, &p.checkBtn, localengine.T("core", "btn", "check")).Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.CheckBox(p.th, &p.autoRestart, localengine.T("core", "auto_restart")).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.CheckBox(p.th, &p.watchLogs, localengine.T("core", "watch_core_logs")).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.separator(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.H6(p.th, localengine.T("core", "privileges", "title")).Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutPrivileges(gtx)
		}),
	)
}

func (p *CorePage) separator(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		h := gtx.Dp(unit.Dp(1))
		bounds := image.Rect(0, 0, gtx.Constraints.Max.X, h)
		defer clip.Rect(bounds).Push(gtx.Ops).Pop()
		paint.ColorOp{Color: color.NRGBA{R: 80, G: 80, B: 80, A: 255}}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		return layout.Dimensions{Size: image.Point{X: gtx.Constraints.Max.X, Y: h}}
	})
}

func (p *CorePage) layoutPrivileges(gtx layout.Context) layout.Dimensions {
	if p.privilegeState.Mode == "windows" {
		return p.layoutWindowsPrivileges(gtx)
	}
	return p.layoutLinuxPrivileges(gtx)
}

func (p *CorePage) layoutWindowsPrivileges(gtx layout.Context) layout.Dimensions {
	c := color.NRGBA{R: 255, G: 255, B: 0, A: 255} // yellow
	if p.privilegeState.AdminStatusColor == "green" {
		c = color.NRGBA{R: 0, G: 255, B: 0, A: 255}
	}

	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.coloredLabel(gtx, p.privilegeState.AdminStatusText, c)
		}),
	}
	if p.privilegeState.ShowRestartAdminBtn {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Button(p.th, &p.restartAdminBtn, localengine.T("core", "btn", "restart_admin")).Layout(gtx)
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (p *CorePage) layoutLinuxPrivileges(gtx layout.Context) layout.Dimensions {
	label := localengine.T("core", "mode", "admin")
	if p.privilegeMode == "setcap" {
		label = localengine.T("core", "mode", "setcap")
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Button(p.th, &p.privilegePickerBtn, label).Layout(gtx)
			})
		}),
	)
}

func (p *CorePage) coloredLabel(gtx layout.Context, text string, c color.NRGBA) layout.Dimensions {
	lbl := material.Body2(p.th, text)
	lbl.Color = c
	return lbl.Layout(gtx)
}

func (p *CorePage) openPrivilegePicker() {
	modes := []string{"admin", "setcap"}
	btns := make([]widget.Clickable, len(modes))

	p.dialog.ShowCustom(localengine.T("core", "privileges", "title"), func(gtx layout.Context) layout.Dimensions {
		for i := range modes {
			if btns[i].Clicked(gtx) {
				p.dialog.HideCustom()
				p.onPrivilegeModeChange(modes[i])
			}
		}

		children := make([]layout.FlexChild, len(modes))
		for i, m := range modes {
			idx := i
			mode := m
			label := localengine.T("core", "mode", mode)
			if mode == p.privilegeMode {
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

func (p *CorePage) onPrivilegeModeChange(mode string) {
	if mode == p.privilegeMode {
		return
	}

	if mode == "admin" {
		_ = p.ctrl.Controller.SetRunAsAdmin(true)
		p.privilegeMode = "admin"
		p.privilegeState = p.ctrl.Controller.GetPrivilegeTabState()
		return
	}

	// Switching to setcap
	if p.privilegeState.HasSetcap {
		_ = p.ctrl.Controller.SetRunAsAdmin(false)
		p.privilegeMode = "setcap"
		p.privilegeState = p.ctrl.Controller.GetPrivilegeTabState()
		return
	}

	// setcap not applied — show confirmation dialog
	var applyBtn widget.Clickable
	var cancelBtn widget.Clickable

	p.dialog.ShowCustom(localengine.T("core", "btn", "apply_setcap"), func(gtx layout.Context) layout.Dimensions {
		if applyBtn.Clicked(gtx) {
			p.dialog.HideCustom()
			go func() {
				p.dialog.ShowLoading(localengine.T("progress", "applying_setcap"))
				err := p.ctrl.Controller.ApplySetcap()
				p.dialog.HideLoading()
				if err == nil {
					_ = p.ctrl.Controller.SetRunAsAdmin(false)
					p.privilegeMode = "setcap"
					p.privilegeState = p.ctrl.Controller.GetPrivilegeTabState()
				}
			}()
		}
		if cancelBtn.Clicked(gtx) {
			p.dialog.HideCustom()
		}

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Body2(p.th, localengine.T("core", "mode", "setcap_prompt")).Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return material.Button(p.th, &applyBtn, localengine.T("core", "btn", "apply")).Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return material.Button(p.th, &cancelBtn, localengine.T("dialog", "btn", "cancel")).Layout(gtx)
							})
						}),
					)
				})
			}),
		)
	})
}

func (p *CorePage) onDownloadCore() {
	p.dialog.ShowLoading(localengine.T("progress", "checking_version"))
	path, err := p.ctrl.Controller.DownloadCoreWithProgress(nil)
	p.dialog.HideLoading()
	if err != nil {
		return
	}
	ver, _ := p.ctrl.Controller.GetInstalledCoreVersion()
	p.dialog.Show(localengine.T("core", "btn", "download"), fmt.Sprintf(localengine.T("dialog", "download_complete", "msg"), ver, path))
	go p.refreshVersions()
}

func (p *CorePage) onCheckVersion() {
	p.dialog.ShowLoading(localengine.T("progress", "checking_version"))
	ver, err := p.ctrl.Controller.GetLatestCoreVersion()
	p.dialog.HideLoading()
	if err != nil {
		return
	}
	p.latestText = localengine.T("core", "latest", "prefix") + ver
	p.showVersionInfoDialog(ver)
}

func (p *CorePage) showVersionInfoDialog(latest string) {
	currentVer, err := p.ctrl.Controller.GetInstalledCoreVersion()
	var body string
	if err != nil || currentVer == "" {
		body = localengine.T("dialog", "version_check", "core_not_installed") + "\n" +
			localengine.T("dialog", "version_check", "latest") + latest
	} else {
		body = localengine.T("dialog", "version_check", "current") + currentVer + "\n" +
			localengine.T("dialog", "version_check", "latest") + latest + "\n"
		if currentVer == latest {
			body += localengine.T("dialog", "version_check", "latest_installed")
		} else {
			body += localengine.T("dialog", "version_check", "update_available")
		}
	}
	p.dialog.Show(localengine.T("dialog", "version_check", "title"), body)
}
