package pages

import (
	"image"
	"image/color"
	"time"

	"gio.tools/icons"
	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/gui/gio/widgets"
)

// MainPage renders the main control screen with start/stop/restart.
type MainPage struct {
	th   *material.Theme
	ctrl *core.InteractiveController

	mainBtn    widget.Clickable
	restartBtn widget.Clickable

	processing bool
	spinAngle  float32
	spinTime   time.Time

	configDropdown *widgets.Dropdown

	// Dialog provider is supplied by the shell.
	dialog widgets.DialogProvider
}

// NewMainPage creates a new main page.
func NewMainPage(th *material.Theme, ctrl *core.InteractiveController, dialog widgets.DialogProvider) *MainPage {
	p := &MainPage{
		th:     th,
		ctrl:   ctrl,
		dialog: dialog,
	}
	p.configDropdown = widgets.NewDropdown(
		th, dialog,
		localengine.T("main", "active", "label"),
		"",
		[]string{},
		nil,
		func(s string) { go p.activateConfigAndMaybeRestart(s) },
	)
	return p
}

// Tag returns the page tag.
func (p *MainPage) Tag() string { return "main" }

// Name returns the page name.
func (p *MainPage) Name() string       { return localengine.T("tab", "main") }
func (p *MainPage) Icon() *widget.Icon { return icons.ActionHome }

// NoShellScroll tells the shell not to wrap this page in a scroller. The main
// page uses a centered layout that breaks inside an unbounded scroll list.
func (p *MainPage) NoShellScroll() bool { return true }

// Layout draws the main page.
func (p *MainPage) Layout(gtx layout.Context) layout.Dimensions {
	p.handleInteractions(gtx)
	return p.layoutMainContent(gtx)
}

func (p *MainPage) handleInteractions(gtx layout.Context) {
	if p.processing {
		return
	}
	if p.mainBtn.Clicked(gtx) {
		if p.ctrl.Backend().IsRunning() {
			go p.onStop()
		} else {
			go p.onStart()
		}
	}
	if p.restartBtn.Clicked(gtx) {
		go p.onRestart()
	}
}

func (p *MainPage) layoutMainContent(gtx layout.Context) layout.Dimensions {
	running := p.ctrl.Backend().IsRunning()
	mainLabel := localengine.T("main", "btn", "start")
	if running {
		mainLabel = localengine.T("main", "btn", "stop")
	}

	if !p.dialog.Visible() {
		p.syncConfigDropdown()
	}

	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, layout.Spacer{}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						if p.processing {
							return p.spinnerButton(gtx, unit.Dp(120))
						}
						return p.roundButton(gtx, p.th, &p.mainBtn, mainLabel, unit.Dp(120))
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !running || p.processing {
						return layout.Dimensions{}
					}
					return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Button(p.th, &p.restartBtn, localengine.T("main", "btn", "restart")).Layout(gtx)
					})
				}),
				layout.Flexed(1, layout.Spacer{}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return p.configDropdown.Layout(gtx, false)
				}),
			)
		}),
	)
}

func (p *MainPage) syncConfigDropdown() {
	configs := p.ctrl.Backend().GetConfigs()
	names := make([]string, len(configs))
	for i, cfg := range configs {
		names[i] = cfg.Name
	}
	activeName := ""
	if active := p.ctrl.Backend().GetActiveConfig(); active != nil {
		activeName = active.Name
	}
	p.configDropdown.SetOptions(names, nil, nil)
	p.configDropdown.SetValue(activeName)
}

func (p *MainPage) activateConfigAndMaybeRestart(name string) {
	active := p.ctrl.Backend().GetActiveConfig()
	if active != nil && active.Name == name {
		return
	}

	p.processing = true
	defer func() { p.processing = false }()

	if err := p.ctrl.Backend().ActivateConfig(name); err != nil {
		p.ctrl.Backend().Terminal().Infof("Failed to activate config: %v", err)
		return
	}
	if p.ctrl.Backend().IsRunning() {
		if err := p.ctrl.Backend().Restart(); err != nil {
			p.ctrl.Backend().Terminal().Infof("Failed to restart: %v", err)
		}
	}
}

func (p *MainPage) roundButton(gtx layout.Context, th *material.Theme, btn *widget.Clickable, label string, diameter unit.Dp) layout.Dimensions {
	return material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
		d := gtx.Dp(diameter)
		gtx.Constraints.Min = image.Point{X: d, Y: d}
		gtx.Constraints.Max = gtx.Constraints.Min

		bg := th.Palette.ContrastBg
		if btn.Hovered() {
			bg = lighten(bg, 20)
		}

		defer clip.Ellipse{Max: image.Point{X: d, Y: d}}.Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, bg, clip.Ellipse{Max: image.Point{X: d, Y: d}}.Op(gtx.Ops))

		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body1(th, label)
			lbl.Color = th.Palette.ContrastFg
			return lbl.Layout(gtx)
		})
	})
}

func (p *MainPage) spinnerButton(gtx layout.Context, diameter unit.Dp) layout.Dimensions {
	d := gtx.Dp(diameter)
	gtx.Constraints.Min = image.Point{X: d, Y: d}
	gtx.Constraints.Max = gtx.Constraints.Min

	if !p.spinTime.IsZero() {
		dt := float32(gtx.Now.Sub(p.spinTime).Seconds())
		p.spinAngle += dt * 6 // ~1 rotation per second
	}
	p.spinTime = gtx.Now
	gtx.Execute(op.InvalidateCmd{})

	bg := p.th.Palette.ContrastBg
	defer clip.Ellipse{Max: image.Point{X: d, Y: d}}.Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, bg, clip.Ellipse{Max: image.Point{X: d, Y: d}}.Op(gtx.Ops))

	center := f32.Point{X: float32(d) / 2, Y: float32(d) / 2}
	defer op.Affine(f32.Affine2D{}.Rotate(center, p.spinAngle)).Push(gtx.Ops).Pop()

	pc := material.ProgressCircle(p.th, 0.25)
	pc.Color = p.th.Palette.ContrastFg
	return pc.Layout(gtx)
}

func lighten(c color.NRGBA, amount uint8) color.NRGBA {
	if c.R <= 255-amount {
		c.R += amount
	} else {
		c.R = 255
	}
	if c.G <= 255-amount {
		c.G += amount
	} else {
		c.G = 255
	}
	if c.B <= 255-amount {
		c.B += amount
	} else {
		c.B = 255
	}
	return c
}

func (p *MainPage) onStart() {
	p.processing = true
	go func() {
		defer func() { p.processing = false }()
		_ = p.ctrl.StartService()
	}()
}

func (p *MainPage) onStop() {
	p.processing = true
	go func() {
		defer func() { p.processing = false }()
		_ = p.ctrl.StopService()
	}()
}

func (p *MainPage) onRestart() {
	p.processing = true
	go func() {
		defer func() { p.processing = false }()
		if err := p.ctrl.Backend().Restart(); err != nil {
			p.ctrl.Backend().Terminal().Infof("Failed to restart: %v", err)
			return
		}
	}()
}
