package pages

import (
	"image/color"
	"sync"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/i18n"
)

// MainPage renders the main control screen with start/stop/restart.
type MainPage struct {
	th   *material.Theme
	ctrl *core.InteractiveController

	configBtn  widget.Clickable
	startBtn   widget.Clickable
	stopBtn    widget.Clickable
	restartBtn widget.Clickable

	// Config picker overlay state.
	configPickerActive  bool
	configPickerConfigs []config.ConfigRecord
	configPickerBtns    []widget.Clickable
	configPickerCancel  widget.Clickable
	configPickerMu      sync.Mutex
}

// NewMainPage creates a new main page.
func NewMainPage(th *material.Theme, ctrl *core.InteractiveController) *MainPage {
	return &MainPage{
		th:   th,
		ctrl: ctrl,
	}
}

// Tag returns the page tag.
func (p *MainPage) Tag() string { return "main" }

// Name returns the page name.
func (p *MainPage) Name() string { return "Main" }

// Layout draws the main page.
func (p *MainPage) Layout(gtx layout.Context) layout.Dimensions {
	if p.configBtn.Clicked(gtx) {
		p.openConfigPicker()
	}
	if p.startBtn.Clicked(gtx) {
		p.onStart()
	}
	if p.stopBtn.Clicked(gtx) {
		p.onStop()
	}
	if p.restartBtn.Clicked(gtx) {
		p.onRestart()
	}

	p.configPickerMu.Lock()
	active := p.configPickerActive
	if active {
		for i := range p.configPickerBtns {
			if p.configPickerBtns[i].Clicked(gtx) {
				name := p.configPickerConfigs[i].Name
				p.configPickerActive = false
				go func() {
					_ = p.ctrl.ActivateConfigWithLog(name)
				}()
			}
		}
		if p.configPickerCancel.Clicked(gtx) {
			p.configPickerActive = false
		}
	}
	p.configPickerMu.Unlock()

	dims := p.layoutMainContent(gtx)

	if active {
		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				return dims
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return p.layoutConfigPicker(gtx)
			}),
		)
	}
	return dims
}

func (p *MainPage) layoutMainContent(gtx layout.Context) layout.Dimensions {
	running := p.ctrl.IsRunning()
	active := p.ctrl.Config().GetActiveConfig()

	configText := i18n.T("main.active.none")
	if active != nil {
		configText = i18n.T("main.active.prefix") + active.Name
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			status := i18n.T("main.status.stopped")
			col := color.NRGBA{R: 0xE0, G: 0x40, B: 0x40, A: 0xFF}
			if running {
				status = i18n.T("main.status.running")
				col = color.NRGBA{R: 0x40, G: 0xC0, B: 0x40, A: 0xFF}
			}
			lbl := material.Body1(p.th, status)
			lbl.Color = col
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Button(p.th, &p.configBtn, configText).Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceStart}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Button(p.th, &p.startBtn, i18n.T("main.btn.start")).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Button(p.th, &p.stopBtn, i18n.T("main.btn.stop")).Layout(gtx)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Button(p.th, &p.restartBtn, i18n.T("main.btn.restart")).Layout(gtx)
					})
				}),
			)
		}),
	)
}

func (p *MainPage) layoutConfigPicker(gtx layout.Context) layout.Dimensions {
	maxWidth := gtx.Dp(unit.Dp(360))
	if gtx.Constraints.Max.X < maxWidth {
		maxWidth = gtx.Constraints.Max.X - gtx.Dp(unit.Dp(32))
	}
	cardGtx := gtx
	cardGtx.Constraints.Min.X = maxWidth
	cardGtx.Constraints.Max.X = maxWidth
	cardGtx.Constraints.Min.Y = 0

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return component.Surface(p.th).Layout(cardGtx, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.H6(p.th, i18n.T("main.active.prefix")).Layout(gtx)
					}),
				}
				p.configPickerMu.Lock()
				for i, cfg := range p.configPickerConfigs {
					idx := i
					label := cfg.Name
					active := p.ctrl.Config().GetActiveConfig()
					if active != nil && cfg.Name == active.Name {
						label = "> " + cfg.Name
					}
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return material.Button(p.th, &p.configPickerBtns[idx], label).Layout(gtx)
						})
					}))
				}
				p.configPickerMu.Unlock()
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.Button(p.th, &p.configPickerCancel, i18n.T("dialog.btn.cancel")).Layout(gtx)
					})
				}))
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		})
	})
}

func (p *MainPage) openConfigPicker() {
	p.configPickerMu.Lock()
	p.configPickerActive = true
	configs := p.ctrl.Config().GetConfigs()
	p.configPickerConfigs = configs
	p.configPickerBtns = make([]widget.Clickable, len(configs))
	p.configPickerMu.Unlock()
}

func (p *MainPage) onStart() {
	if _, err := p.ctrl.PrepareConfig(); err != nil {
		p.ctrl.Log(err.Error())
		return
	}
	if err := p.ctrl.Start(); err != nil {
		p.ctrl.Log("Failed to start: " + err.Error())
		return
	}
}

func (p *MainPage) onStop() {
	if err := p.ctrl.Stop(); err != nil {
		p.ctrl.Log("Failed to stop: " + err.Error())
		return
	}
}

func (p *MainPage) onRestart() {
	if err := p.ctrl.Restart(); err != nil {
		p.ctrl.Log("Failed to restart: " + err.Error())
		return
	}
}
