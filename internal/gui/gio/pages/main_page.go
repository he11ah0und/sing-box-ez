package pages

import (
	"image"
	"image/color"
	"time"

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
)

// MainPage renders the main control screen with start/stop/restart.
type MainPage struct {
	th   *material.Theme
	ctrl *core.InteractiveController

	configBtn  widget.Clickable
	mainBtn    widget.Clickable
	restartBtn widget.Clickable

	processing bool
	spinAngle  float32
	spinTime   time.Time

	// Dialog provider is supplied by the shell.
	dialog DialogProvider
}

// NewMainPage creates a new main page.
func NewMainPage(th *material.Theme, ctrl *core.InteractiveController, dialog DialogProvider) *MainPage {
	return &MainPage{
		th:     th,
		ctrl:   ctrl,
		dialog: dialog,
	}
}

// Tag returns the page tag.
func (p *MainPage) Tag() string { return "main" }

// Name returns the page name.
func (p *MainPage) Name() string { return localengine.T("tab", "main") }

// Layout draws the main page.
func (p *MainPage) Layout(gtx layout.Context) layout.Dimensions {
	p.handleInteractions(gtx)
	return p.layoutMainContent(gtx)
}

func (p *MainPage) handleInteractions(gtx layout.Context) {
	if p.processing {
		return
	}
	if p.configBtn.Clicked(gtx) {
		p.openConfigPicker()
	}
	if p.mainBtn.Clicked(gtx) {
		if p.ctrl.IsRunning() {
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
	running := p.ctrl.IsRunning()
	mainLabel := localengine.T("main", "btn", "start")
	if running {
		mainLabel = localengine.T("main", "btn", "stop")
	}

	configText := localengine.T("main", "active", "none")
	if active := p.ctrl.Config().GetActiveConfig(); active != nil {
		configText = localengine.T("main", "active", "prefix") + active.Name
	}

	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Button(p.th, &p.configBtn, configText).Layout(gtx)
				}),
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
			)
		}),
	)
}

func (p *MainPage) openConfigPicker() {
	configs := p.ctrl.Config().GetConfigs()
	btns := make([]widget.Clickable, len(configs))

	p.dialog.ShowCustom(localengine.T("main", "active", "prefix"), func(gtx layout.Context) layout.Dimensions {
		for i := range configs {
			if btns[i].Clicked(gtx) {
				p.dialog.HideCustom()
				go func(name string) {
					_ = p.ctrl.ActivateConfigWithLog(name)
				}(configs[i].Name)
			}
		}

		children := []layout.FlexChild{}
		for i, cfg := range configs {
			idx := i
			label := cfg.Name
			active := p.ctrl.Config().GetActiveConfig()
			if active != nil && cfg.Name == active.Name {
				label = "> " + cfg.Name
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return material.Button(p.th, &btns[idx], label).Layout(gtx)
				})
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
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
		if _, err := p.ctrl.PrepareConfig(); err != nil {
			p.ctrl.LogTag("core", err.Error())
			return
		}
		if err := p.ctrl.Start(); err != nil {
			p.ctrl.LogTag("core", "Failed to start: "+err.Error())
			return
		}
	}()
}

func (p *MainPage) onStop() {
	p.processing = true
	go func() {
		defer func() { p.processing = false }()
		if err := p.ctrl.Stop(); err != nil {
			p.ctrl.LogTag("core", "Failed to stop: "+err.Error())
			return
		}
	}()
}

func (p *MainPage) onRestart() {
	p.processing = true
	go func() {
		defer func() { p.processing = false }()
		if err := p.ctrl.Restart(); err != nil {
			p.ctrl.LogTag("core", "Failed to restart: "+err.Error())
			return
		}
	}()
}
