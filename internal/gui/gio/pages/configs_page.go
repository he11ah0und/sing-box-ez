package pages

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"gio.tools/icons"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/framework/version"
	"sing-box-ez/internal/gui/gio/theme"
	"sing-box-ez/internal/gui/gio/widgets"
)

// ConfigsPage renders the configs management screen as vertical cards.
type ConfigsPage struct {
	th     *material.Theme
	ctrl   *core.InteractiveController
	dialog widgets.DialogProvider

	configs    []config.ConfigRecord
	cardClicks map[string]*widget.Clickable

	addBtn       widget.Clickable
	updateAllBtn widget.Clickable

	list widget.List
}

// NewConfigsPage creates a new configs page.
func NewConfigsPage(th *material.Theme, ctrl *core.InteractiveController, dialog widgets.DialogProvider) *ConfigsPage {
	p := &ConfigsPage{
		th:         th,
		ctrl:       ctrl,
		dialog:     dialog,
		cardClicks: make(map[string]*widget.Clickable),
	}
	p.refreshConfigs()
	return p
}

func (p *ConfigsPage) refreshConfigs() {
	p.configs = p.ctrl.Controller.Config().GetConfigs()
}

// Tag returns the page tag.
func (p *ConfigsPage) Tag() string { return "configs" }

// Name returns the page name.
func (p *ConfigsPage) Name() string       { return localengine.T("tab", "configs") }
func (p *ConfigsPage) Icon() *widget.Icon { return icons.ActionList }

// NoInset tells the shell not to wrap this page in padding.
func (p *ConfigsPage) NoInset() bool { return true }

// NoShellScroll tells the shell not to wrap this page in a scroller because it
// already scrolls its own config cards list.
func (p *ConfigsPage) NoShellScroll() bool { return true }

// ShowAddDialog opens the "add config" dialog. Used when the app detects that
// no active config exists and the user needs to create one.
func (p *ConfigsPage) ShowAddDialog() {
	p.openAddDialog()
}

// Layout draws the configs page.
func (p *ConfigsPage) Layout(gtx layout.Context) layout.Dimensions {
	return widgets.SpacedList(gtx, p.Children(gtx)...)
}

// Children returns the page widgets; the shell adds standard vertical spacing.
func (p *ConfigsPage) Children(gtx layout.Context) []layout.FlexChild {
	if p.addBtn.Clicked(gtx) {
		p.openAddDialog()
	}
	if p.updateAllBtn.Clicked(gtx) {
		go p.onUpdateAll()
	}

	p.refreshConfigs()

	// Handle card clicks — single click opens edit dialog
	for i := range p.configs {
		rec := p.configs[i]
		click, ok := p.cardClicks[rec.Name]
		if !ok {
			click = new(widget.Clickable)
			p.cardClicks[rec.Name] = click
		}
		if click.Clicked(gtx) {
			p.openEditDialog(i)
		}
	}

	return []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceStart}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Button(p.th, &p.addBtn, localengine.T("configs", "btn", "add")).Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Point{X: gtx.Dp(unit.Dp(8)), Y: 0}}
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Button(p.th, &p.updateAllBtn, localengine.T("configs", "btn", "update_all")).Layout(gtx)
					}),
				)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			p.list.Axis = layout.Vertical
			return p.list.Layout(gtx, len(p.configs), func(gtx layout.Context, index int) layout.Dimensions {
				if index >= len(p.configs) {
					return layout.Dimensions{}
				}
				return p.layoutConfigCard(gtx, p.configs[index])
			})
		}),
	}
}

func (p *ConfigsPage) layoutConfigCard(gtx layout.Context, rec config.ConfigRecord) layout.Dimensions {
	isCached := p.ctrl.Controller.HasCachedConfig(rec.Name) && !rec.LastUpdate.IsZero()
	isActive := rec.Name == p.ctrl.Controller.Config().GetActiveName()
	autoUpdate := rec.IsAutoUpdate()
	click := p.cardClicks[rec.Name]

	colors := theme.Current().Colors()
	var bg color.NRGBA
	switch {
	case autoUpdate && isCached:
		bg = colors.CardCached
	case autoUpdate && !isCached:
		bg = colors.CardUncached
	case isCached:
		if colors.CardCachedNoAutoUpdate.A != 0 {
			bg = colors.CardCachedNoAutoUpdate
		} else {
			bg = colors.CardCached
		}
	default:
		if colors.CardUncachedNoAutoUpdate.A != 0 {
			bg = colors.CardUncachedNoAutoUpdate
		} else {
			bg = colors.CardUncached
		}
	}

	return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			macro := op.Record(gtx.Ops)
			dims := layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				src := rec.Parent
				if src == "" || src == "user" {
					src = localengine.T("configs", "table", "user")
				} else if len(src) > 3 && src[:3] == "pl-" {
					src = src[3:]
				}

				parts := []string{
					fmt.Sprintf("%s: %s", localengine.T("configs", "table", "last_update"), p.formatLastUpdate(rec.LastUpdate.Time)),
				}
				if autoUpdate {
					parts = append(parts, fmt.Sprintf("%s: %s", localengine.T("configs", "table", "next_update"), p.formatNextUpdate(rec.NextUpdate())))
				}
				parts = append(parts, fmt.Sprintf("%s: %s", localengine.T("configs", "table", "source"), src))

				meta := strings.Join(parts, "  •  ")

				nameColor := p.th.Palette.Fg
				if isActive {
					nameColor = colors.Fg
				}

				return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body1(p.th, rec.Name)
						lbl.Font.Weight = font.Bold
						lbl.Color = nameColor
						return lbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(p.th, meta)
						return lbl.Layout(gtx)
					}),
				)
			})
			call := macro.Stop()

			// Draw background sized to content.
			defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
			paint.ColorOp{Color: bg}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)

			call.Add(gtx.Ops)
			return dims
		})
	})
}

func (p *ConfigsPage) formatLastUpdate(t time.Time) string {
	if t.IsZero() {
		return localengine.T("configs", "table", "never")
	}
	return version.HumanDuration(t)
}

func (p *ConfigsPage) formatNextUpdate(t time.Time) string {
	if t.IsZero() {
		return localengine.T("configs", "table", "now")
	}
	return version.HumanDurationFrom(time.Until(t), true)
}

func (p *ConfigsPage) openAddDialog() {
	var nameEd widget.Editor
	var urlEd widget.Editor
	var periodEd widget.Editor
	var autoUpdate widget.Bool
	nameEd.SingleLine = true
	urlEd.SingleLine = true
	periodEd.SingleLine = true
	periodEd.SetText(fmt.Sprintf("%d", p.ctrl.Controller.Config().MustGet("updates", "default_interval_hours").Int()))
	autoUpdate.Value = true

	var saveBtn widget.Clickable

	p.dialog.ShowCustom(localengine.T("configs", "dialog", "title"), func(gtx layout.Context) layout.Dimensions {
		if saveBtn.Clicked(gtx) {
			p.dialog.HideCustom()
			hours := p.ctrl.Controller.Config().MustGet("updates", "default_interval_hours").Int()
			fmt.Sscanf(periodEd.Text(), "%d", &hours)
			if hours <= 0 {
				hours = p.ctrl.Controller.Config().MustGet("updates", "default_interval_hours").Int()
			}
			rec := config.ConfigRecord{
				Name:                nameEd.Text(),
				URL:                 urlEd.Text(),
				UpdateIntervalHours: hours,
				Parent:              "user",
				AutoUpdate:          &autoUpdate.Value,
			}
			go func() {
				if err := p.ctrl.Controller.AddConfig(rec); err == nil {
					p.refreshConfigs()
				}
			}()
		}

		return widgets.DialogSpacedList(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widgets.LabeledInput(gtx, p.th, localengine.T("configs", "dialog", "name"), &nameEd, false)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widgets.LabeledInput(gtx, p.th, localengine.T("configs", "dialog", "url"), &urlEd, false)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widgets.LabeledInput(gtx, p.th, localengine.T("configs", "dialog", "period"), &periodEd, false)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.CheckBox(p.th, &autoUpdate, localengine.T("configs", "dialog", "auto_update")).Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Button(p.th, &saveBtn, localengine.T("configs", "dialog", "btn", "save")).Layout(gtx)
			}),
		)
	})
}

func (p *ConfigsPage) openEditDialog(idx int) {
	if idx < 0 || idx >= len(p.configs) {
		return
	}
	old := p.configs[idx]

	var nameEd widget.Editor
	var urlEd widget.Editor
	var periodEd widget.Editor
	var autoUpdate widget.Bool
	nameEd.SingleLine = true
	urlEd.SingleLine = true
	periodEd.SingleLine = true
	nameEd.SetText(old.Name)
	urlEd.SetText(old.URL)
	periodEd.SetText(fmt.Sprintf("%d", old.UpdateIntervalHours))
	autoUpdate.Value = old.IsAutoUpdate()

	var saveBtn widget.Clickable
	var deleteBtn widget.Clickable
	var updateNowBtn widget.Clickable

	p.dialog.ShowCustom(localengine.T("configs", "dialog", "title"), func(gtx layout.Context) layout.Dimensions {
		if saveBtn.Clicked(gtx) {
			p.dialog.HideCustom()
			hours := old.UpdateIntervalHours
			fmt.Sscanf(periodEd.Text(), "%d", &hours)
			if hours <= 0 {
				hours = p.ctrl.Controller.Config().MustGet("updates", "default_interval_hours").Int()
			}
			rec := config.ConfigRecord{
				Name:                nameEd.Text(),
				URL:                 urlEd.Text(),
				UpdateIntervalHours: hours,
				Parent:              old.Parent,
				LastUpdate:          old.LastUpdate,
				AutoUpdate:          &autoUpdate.Value,
			}
			go func() {
				if err := p.ctrl.Controller.EditConfig(old.Name, rec); err == nil {
					p.refreshConfigs()
				}
			}()
		}
		if deleteBtn.Clicked(gtx) {
			p.dialog.HideCustom()
			p.onDelete(old.Name)
		}
		if updateNowBtn.Clicked(gtx) {
			p.dialog.HideCustom()
			go func() {
				p.dialog.ShowLoading(localengine.T("progress", "updating_configs"))
				_ = p.ctrl.Controller.UpdateConfigNow(old.Name, urlEd.Text())
				p.dialog.HideLoading()
				p.refreshConfigs()
			}()
		}

		return widgets.DialogSpacedList(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widgets.LabeledInput(gtx, p.th, localengine.T("configs", "dialog", "name"), &nameEd, false)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widgets.LabeledInput(gtx, p.th, localengine.T("configs", "dialog", "url"), &urlEd, false)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widgets.LabeledInput(gtx, p.th, localengine.T("configs", "dialog", "period"), &periodEd, false)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.CheckBox(p.th, &autoUpdate, localengine.T("configs", "dialog", "auto_update")).Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceStart, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Button(p.th, &saveBtn, localengine.T("configs", "dialog", "btn", "save")).Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Point{X: gtx.Dp(unit.Dp(8)), Y: 0}}
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Button(p.th, &updateNowBtn, localengine.T("configs", "dialog", "btn", "update_now")).Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Point{X: gtx.Dp(unit.Dp(8)), Y: 0}}
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Button(p.th, &deleteBtn, localengine.T("configs", "dialog", "btn", "delete")).Layout(gtx)
					}),
				)
			}),
		)
	})
}

func (p *ConfigsPage) onDelete(name string) {
	var confirmBtn widget.Clickable
	var cancelBtn widget.Clickable

	p.dialog.ShowCustom(localengine.T("configs", "dialog", "delete_title"), func(gtx layout.Context) layout.Dimensions {
		if confirmBtn.Clicked(gtx) {
			p.dialog.HideCustom()
			go func() {
				_ = p.ctrl.Controller.DeleteConfig(name)
				p.refreshConfigs()
			}()
		}
		if cancelBtn.Clicked(gtx) {
			p.dialog.HideCustom()
		}

		return widgets.DialogSpacedList(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Body2(p.th, localengine.T("configs", "dialog", "delete_msg")+" \""+name+"\"?").Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Button(p.th, &confirmBtn, localengine.T("configs", "dialog", "btn", "delete")).Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return material.Button(p.th, &cancelBtn, localengine.T("configs", "dialog", "btn", "cancel")).Layout(gtx)
						})
					}),
				)
			}),
		)
	})
}

func (p *ConfigsPage) onUpdateAll() {
	p.dialog.ShowLoading(localengine.T("progress", "updating_configs"))
	_, _, err := p.ctrl.Controller.UpdateAllConfigs(nil)
	p.dialog.HideLoading()
	if err == nil {
		p.refreshConfigs()
	}
}
