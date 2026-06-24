package pages

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"gio.tools/icons"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/gui/gio/theme"
	"sing-box-ez/internal/gui/gio/widgets"
)

// CorePage renders the core management screen.
type CorePage struct {
	th     *material.Theme
	ctrl   *core.InteractiveController
	dialog widgets.DialogProvider

	coreUpdate *widgets.UpdateCheck

	autoRestart widget.Bool

	logLevel   *widgets.Dropdown
	savedLevel string

	highlightEnd time.Time

	restartAdminBtn widget.Clickable

	// privilegeMode is either "admin" or "setcap" (Linux only).
	privilegeMode string
	// privilegePickerBtn opens the mode selector dialog.
	privilegePickerBtn widget.Clickable

	privilegeState core.PrivilegeTabState
}

// NewCorePage creates a new core page.
func NewCorePage(th *material.Theme, ctrl *core.InteractiveController, dialog widgets.DialogProvider) *CorePage {
	p := &CorePage{
		th:     th,
		ctrl:   ctrl,
		dialog: dialog,
	}
	p.autoRestart.Value = ctrl.Backend().Config().MustGet("core", "auto_restart").Bool()
	p.initLogOverride()

	p.privilegeState = ctrl.Backend().GetPrivilegeTabState()

	// Default privilege mode: admin, unless setcap is already detected.
	p.privilegeMode = "admin"
	if p.privilegeState.Mode == "linux" && p.privilegeState.HasSetcap {
		p.privilegeMode = "setcap"
	}

	current := ""
	if ver, err := ctrl.Backend().GetInstalledCoreVersion(); err == nil {
		current = normalizeCoreVersion(ver)
	}

	p.coreUpdate = widgets.NewUpdateCheck(
		th, dialog,
		current,
		localengine.T("core", "btn", "download"),
		func(ctx context.Context) (widgets.UpdateCheckInfo, error) {
			current, err := ctrl.Backend().GetInstalledCoreVersion()
			if err != nil {
				current = ""
			}
			latest, err := ctrl.Backend().GetLatestCoreVersion()
			if err != nil {
				return widgets.UpdateCheckInfo{}, err
			}
			current = normalizeCoreVersion(current)
			latest = normalizeCoreVersion(latest)
			return widgets.UpdateCheckInfo{
				Current:   current,
				Latest:    latest,
				HasUpdate: current != latest && latest != "",
			}, nil
		},
		func(ctx context.Context, onProgress func(downloaded, total int64)) error {
			_, err := ctrl.Backend().DownloadCoreWithProgress(onProgress)
			return err
		},
	)
	p.coreUpdate.SetCheckingLabel(localengine.T("core", "update", "checking"))
	p.coreUpdate.SetUpdatingLabel(localengine.T("core", "update", "downloading"))
	p.coreUpdate.SetUpToDateLabel("")
	p.coreUpdate.SetCurrentVersionFormatter(func(current string) string {
		return localengine.T("core", "update", "current_version") + current
	})
	p.coreUpdate.SetAvailableFormatter(func(latest string) string {
		return fmt.Sprintf(localengine.T("core", "update", "available"), latest)
	})
	p.coreUpdate.SetDetailsTitle(localengine.T("dialog", "core_update", "title"))
	p.coreUpdate.SetDetailsFormatter(func(info widgets.UpdateCheckInfo) string {
		current := localengine.T("dialog", "version_check", "current") + info.Current
		latest := localengine.T("dialog", "version_check", "latest") + info.Latest
		return current + "\n\n" + latest
	})

	return p
}

func (p *CorePage) initLogOverride() {
	cfg := p.ctrl.Backend().Config()
	p.savedLevel = cfg.MustGet("core", "log", "level").String()
	if p.savedLevel == "" {
		p.savedLevel = "error"
	}

	levels := []string{"debug", "info", "warn", "error"}
	p.logLevel = widgets.NewDropdown(
		p.th, p.dialog,
		localengine.T("core", "log", "level"),
		p.savedLevel,
		levels,
		func(level string) string { return localengine.T("settings", "log_level", level) },
		func(level string) { p.saveLogLevel() },
	)
}

func (p *CorePage) saveLogLevel() {
	level := p.logLevel.Value()
	if level == p.savedLevel {
		return
	}
	if err := p.ctrl.Backend().SetCoreLogOverride(core.LogOverride{Level: level}); err != nil {
		return
	}
	p.savedLevel = level
}

func (p *CorePage) layoutHighlightedUpdate(gtx layout.Context) layout.Dimensions {
	dims := p.coreUpdate.Layout(gtx)
	if time.Now().Before(p.highlightEnd) {
		elapsed := time.Since(p.highlightEnd.Add(-3 * time.Second))
		if int(elapsed.Milliseconds()/500)%2 == 0 {
			paint.FillShape(gtx.Ops, theme.Current().Colors().CoreHighlight, clip.Rect{Max: dims.Size}.Op())
		}
		gtx.Execute(op.InvalidateCmd{})
	}
	return dims
}

func normalizeCoreVersion(v string) string {
	if v == "" {
		return v
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

// Tag returns the page tag.
func (p *CorePage) Tag() string { return "core" }

// Name returns the page name.
func (p *CorePage) Name() string       { return localengine.T("tab", "core") }
func (p *CorePage) Icon() *widget.Icon { return icons.AVPlayArrow }

// HighlightUpdateBlock flashes the core update row background 3 times to draw
// attention when the core binary is missing.
func (p *CorePage) HighlightUpdateBlock() {
	p.highlightEnd = time.Now().Add(3 * time.Second)
}

// Layout draws the core page.
func (p *CorePage) Layout(gtx layout.Context) layout.Dimensions {
	return widgets.SpacedList(gtx, p.Children(gtx)...)
}

// Children returns the page widgets; the shell adds standard vertical spacing.
func (p *CorePage) Children(gtx layout.Context) []layout.FlexChild {
	if p.restartAdminBtn.Clicked(gtx) {
		go func() {
			_ = p.ctrl.Backend().RestartAsAdmin()
		}()
	}
	if p.privilegePickerBtn.Clicked(gtx) {
		p.openPrivilegePicker()
	}

	if changed := p.autoRestart.Update(gtx); changed {
		_ = p.ctrl.Backend().SetAutoRestart(p.autoRestart.Value)
	}

	return []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.H6(p.th, localengine.T("tab", "core")).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutHighlightedUpdate(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.CheckBox(p.th, &p.autoRestart, localengine.T("core", "auto_restart")).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.separator(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.H6(p.th, localengine.T("core", "log", "title")).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.logLevel.Layout(gtx, false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.separator(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.H6(p.th, localengine.T("core", "privileges", "title")).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutPrivileges(gtx)
		}),
	}
}

func (p *CorePage) separator(gtx layout.Context) layout.Dimensions {
	h := gtx.Dp(unit.Dp(1))
	bounds := image.Rect(0, 0, gtx.Constraints.Max.X, h)
	defer clip.Rect(bounds).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: theme.Current().Colors().Separator}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Point{X: gtx.Constraints.Max.X, Y: h}}
}

func (p *CorePage) layoutPrivileges(gtx layout.Context) layout.Dimensions {
	if p.privilegeState.Mode == "windows" {
		return p.layoutWindowsPrivileges(gtx)
	}
	return p.layoutLinuxPrivileges(gtx)
}

func (p *CorePage) layoutWindowsPrivileges(gtx layout.Context) layout.Dimensions {
	colors := theme.Current().Colors()
	c := colors.StatusWarning
	if p.privilegeState.AdminStatusColor == "green" {
		c = colors.StatusOK
	}

	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.coloredLabel(gtx, p.privilegeState.AdminStatusText, c)
		}),
	}
	if p.privilegeState.ShowRestartAdminBtn {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Button(p.th, &p.restartAdminBtn, localengine.T("core", "btn", "restart_admin")).Layout(gtx)
		}))
	}
	return widgets.SpacedList(gtx, children...)
}

func (p *CorePage) layoutLinuxPrivileges(gtx layout.Context) layout.Dimensions {
	label := localengine.T("core", "mode", "admin")
	if p.privilegeMode == "setcap" {
		label = localengine.T("core", "mode", "setcap")
	}

	return widgets.SpacedList(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Button(p.th, &p.privilegePickerBtn, label).Layout(gtx)
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
				return material.Button(p.th, &btns[idx], label).Layout(gtx)
			})
		}
		return widgets.DialogSpacedList(gtx, children...)
	})
}

func (p *CorePage) onPrivilegeModeChange(mode string) {
	if mode == p.privilegeMode {
		return
	}

	if mode == "admin" {
		_ = p.ctrl.Backend().SetRunAsAdmin(true)
		p.privilegeMode = "admin"
		p.privilegeState = p.ctrl.Backend().GetPrivilegeTabState()
		return
	}

	// Switching to setcap
	if p.privilegeState.HasSetcap {
		_ = p.ctrl.Backend().SetRunAsAdmin(false)
		p.privilegeMode = "setcap"
		p.privilegeState = p.ctrl.Backend().GetPrivilegeTabState()
		return
	}

	// setcap not applied — show confirmation dialog
	var applyBtn widget.Clickable

	p.dialog.ShowCustom(localengine.T("core", "btn", "apply_setcap"), func(gtx layout.Context) layout.Dimensions {
		if applyBtn.Clicked(gtx) {
			p.dialog.HideCustom()
			go func() {
				p.dialog.ShowLoading(localengine.T("progress", "applying_setcap"))
				err := p.ctrl.Backend().ApplySetcap()
				p.dialog.HideLoading()
				if err == nil {
					_ = p.ctrl.Backend().SetRunAsAdmin(false)
					p.privilegeMode = "setcap"
					p.privilegeState = p.ctrl.Backend().GetPrivilegeTabState()
				}
			}()
		}

		return widgets.DialogSpacedList(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Body2(p.th, localengine.T("core", "mode", "setcap_prompt")).Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Button(p.th, &applyBtn, localengine.T("core", "btn", "apply")).Layout(gtx)
			}),
		)
	})
}
