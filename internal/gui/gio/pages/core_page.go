package pages

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"io"
	"strconv"
	"strings"
	"time"

	"gio.tools/icons"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
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

	logLevel             *widgets.Dropdown
	savedLevel           string
	graphHistoryDropdown *widgets.Dropdown

	urlTestURLEditor widget.Editor
	savedURLTestURL  string

	highlightEnd time.Time

	restartAdminBtn widget.Clickable

	// privilegeMode is either "admin" or "setcap" (Linux only).
	privilegeMode     string
	privilegeDropdown *widgets.Dropdown

	privilegeState core.PrivilegeTabState

	apiCopyBtn widget.Clickable
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
	p.initGraphHistoryDropdown()
	p.initURLTestURL()

	p.privilegeState = ctrl.Backend().GetPrivilegeTabState()

	// Default privilege mode: admin, unless setcap is already detected.
	p.privilegeMode = "admin"
	if p.privilegeState.Mode == "linux" && p.privilegeState.HasSetcap {
		p.privilegeMode = "setcap"
	}

	p.initPrivilegeDropdown()

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

func (p *CorePage) initGraphHistoryDropdown() {
	current := p.ctrl.Backend().Config().Int("core", "traffic_graph_history")
	if current < 2 {
		current = 60
	}
	options := []string{"30", "60", "120", "300"}
	p.graphHistoryDropdown = widgets.NewDropdown(
		p.th, p.dialog,
		localengine.T("core", "graph_history", "label"),
		strconv.Itoa(current),
		options,
		nil,
		func(v string) {
			n, err := strconv.Atoi(v)
			if err != nil || n < 2 {
				return
			}
			_ = p.ctrl.Backend().Config().Set([]string{"core", "traffic_graph_history"}, n)
		},
	)
}

func (p *CorePage) initURLTestURL() {
	p.savedURLTestURL = p.ctrl.Backend().Config().String("core", "url_test_url")
	p.urlTestURLEditor.SingleLine = true
	p.urlTestURLEditor.Submit = true
	p.urlTestURLEditor.SetText(p.savedURLTestURL)
}

func (p *CorePage) saveURLTestURL() {
	v := strings.TrimSpace(p.urlTestURLEditor.Text())
	if v == p.savedURLTestURL {
		return
	}
	cfg := p.ctrl.Backend().Config()
	if err := cfg.MustGet("core", "url_test_url").Update(v); err == nil {
		_ = cfg.Save()
		p.savedURLTestURL = v
	}
}

func (p *CorePage) layoutURLTestURL(gtx layout.Context) layout.Dimensions {
	dirty := p.urlTestURLEditor.Text() != p.savedURLTestURL
	return widgets.LabeledInput(gtx, p.th, localengine.T("core", "url_test_url", "label"), &p.urlTestURLEditor, dirty)
}

func (p *CorePage) initPrivilegeDropdown() {
	if p.privilegeState.Mode != "linux" {
		return
	}
	modes := []string{"admin", "setcap"}
	p.privilegeDropdown = widgets.NewDropdown(
		p.th, p.dialog,
		localengine.T("core", "privileges", "title"),
		p.privilegeMode,
		modes,
		func(mode string) string { return localengine.T("core", "mode", mode) },
		func(mode string) { p.onPrivilegeModeChange(mode) },
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
	if p.privilegeDropdown != nil {
		p.privilegeDropdown.SetValue(p.privilegeMode)
	}

	if changed := p.autoRestart.Update(gtx); changed {
		_ = p.ctrl.Backend().SetAutoRestart(p.autoRestart.Value)
	}

	for {
		ev, ok := p.urlTestURLEditor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			p.saveURLTestURL()
		}
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
			return p.graphHistoryDropdown.Layout(gtx, false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.separator(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.H6(p.th, localengine.T("core", "url_test_url", "title")).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutURLTestURL(gtx)
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
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.separator(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.H6(p.th, localengine.T("core", "api", "title")).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutAPIInfo(gtx)
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
	if p.privilegeDropdown == nil {
		return layout.Dimensions{}
	}
	return p.privilegeDropdown.Layout(gtx, false)
}

func (p *CorePage) coloredLabel(gtx layout.Context, text string, c color.NRGBA) layout.Dimensions {
	lbl := material.Body2(p.th, text)
	lbl.Color = c
	return lbl.Layout(gtx)
}

func (p *CorePage) layoutAPIInfo(gtx layout.Context) layout.Dimensions {
	info := p.ctrl.Backend().APIInfo()
	if info == nil || !p.ctrl.Backend().IsRunning() {
		lbl := material.Body2(p.th, localengine.T("core", "api", "not_running"))
		lbl.Color = theme.Current().Colors().DisabledFg
		return lbl.Layout(gtx)
	}

	if p.apiCopyBtn.Clicked(gtx) {
		text := fmt.Sprintf("%s: %s\n%s: %s\n%s: %s",
			localengine.T("core", "api", "type"), info.Backend,
			localengine.T("core", "api", "address"), info.Addr(),
			localengine.T("core", "api", "secret"), info.Secret,
		)
		gtx.Execute(clipboard.WriteCmd{
			Type: "text/plain",
			Data: io.NopCloser(strings.NewReader(text)),
		})
	}

	colors := theme.Current().Colors()
	return widgets.BorderedCard(gtx, colors.Separator, colors.Surface, unit.Dp(1), unit.Dp(8), unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
		rows := []layout.FlexChild{
			p.apiInfoRow(gtx, localengine.T("core", "api", "type"), string(info.Backend)),
			p.apiInfoRow(gtx, localengine.T("core", "api", "address"), info.Addr()),
			p.apiInfoRow(gtx, localengine.T("core", "api", "secret"), info.Secret),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return material.Button(p.th, &p.apiCopyBtn, localengine.T("core", "api", "copy")).Layout(gtx)
				})
			}),
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
}

func (p *CorePage) apiInfoRow(gtx layout.Context, label, value string) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start, Spacing: layout.SpaceBetween}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(p.th, label)
					lbl.Color = theme.Current().Colors().DisabledFg
					return lbl.Layout(gtx)
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(p.th, value)
				lbl.Alignment = text.End
				return lbl.Layout(gtx)
			}),
		)
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
	p.dialog.Show(widgets.Custom(localengine.T("core", "btn", "apply_setcap"), func(gtx layout.Context) layout.Dimensions {
		return widgets.DialogSpacedList(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Body2(p.th, localengine.T("core", "mode", "setcap_prompt")).Layout(gtx)
			}),
		)
	}), widgets.Action(localengine.T("core", "btn", "apply"), func() {
		go func() {
			p.dialog.Show(widgets.Loading(localengine.T("progress", "applying_setcap")))
			err := p.ctrl.Backend().ApplySetcap()
			p.dialog.Hide()
			if err == nil {
				_ = p.ctrl.Backend().SetRunAsAdmin(false)
				p.privilegeMode = "setcap"
				p.privilegeState = p.ctrl.Backend().GetPrivilegeTabState()
			}
		}()
	}))
}
