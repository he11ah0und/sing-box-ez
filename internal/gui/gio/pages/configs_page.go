package pages

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"strings"
	"time"

	"gio.tools/icons"
	"gioui.org/font"
	"gioui.org/io/clipboard"
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
	"sing-box-ez/internal/singboxconfig"
)

// dialogMaxBodyHeight limits the height of scrollable dialog content.
const dialogMaxBodyHeight = unit.Dp(420)

// ConfigsPage renders the configs management screen as vertical cards.
type ConfigsPage struct {
	th     *material.Theme
	ctrl   *core.InteractiveController
	dialog widgets.DialogProvider

	configs    []config.ConfigRecord
	cardClicks map[string]*widget.Clickable

	addBtn       widget.Clickable
	updateAllBtn widget.Clickable

	list           widget.List
	validationList widget.List

	hashMismatches map[string]bool
	hashCheckTime  time.Time
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
	p.configs = p.ctrl.Backend().GetConfigs()
	if time.Since(p.hashCheckTime) > 2*time.Second {
		p.refreshHashMismatches()
		p.hashCheckTime = time.Now()
	}
}

func (p *ConfigsPage) refreshHashMismatches() {
	p.hashMismatches = make(map[string]bool, len(p.configs))
	for _, rec := range p.configs {
		if rec.IsLocal() {
			continue
		}
		p.hashMismatches[rec.Name] = p.ctrl.Backend().IsConfigHashMismatch(rec.Name)
	}
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

	// Handle card clicks — single click opens edit dialog.
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
						return widgets.HSpace(gtx, unit.Dp(8))
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
				return p.layoutConfigCard(gtx, index)
			})
		}),
	}
}

func (p *ConfigsPage) cardBackground(isCached, autoUpdate bool, colors theme.Palette) color.NRGBA {
	switch {
	case autoUpdate && isCached:
		return colors.CardCached
	case autoUpdate && !isCached:
		return colors.CardUncached
	case isCached:
		if colors.CardCachedNoAutoUpdate.A != 0 {
			return colors.CardCachedNoAutoUpdate
		}
		return colors.CardCached
	default:
		if colors.CardUncachedNoAutoUpdate.A != 0 {
			return colors.CardUncachedNoAutoUpdate
		}
		return colors.CardUncached
	}
}

func (p *ConfigsPage) formatConfigSource(rec config.ConfigRecord) string {
	src := rec.Parent
	if src == "" || src == "user" {
		return localengine.T("configs", "table", "user")
	}
	if len(src) > 3 && src[:3] == "pl-" {
		return src[3:]
	}
	return src
}

func (p *ConfigsPage) formatConfigMeta(rec config.ConfigRecord) string {
	src := p.formatConfigSource(rec)
	parts := []string{
		fmt.Sprintf("%s: %s", localengine.T("configs", "table", "last_update"), p.formatLastUpdate(rec.LastUpdate.Time)),
	}
	if !rec.IsLocal() && rec.IsAutoUpdate() {
		parts = append(parts, fmt.Sprintf("%s: %s", localengine.T("configs", "table", "next_update"), p.formatNextUpdate(rec.NextUpdate())))
	}
	parts = append(parts, fmt.Sprintf("%s: %s", localengine.T("configs", "table", "source"), src))
	return strings.Join(parts, "  •  ")
}

func (p *ConfigsPage) layoutConfigCardHeader(gtx layout.Context, rec config.ConfigRecord, nameColor color.NRGBA, isHashMismatch bool) layout.Dimensions {
	isLocal := rec.IsLocal()
	colors := theme.Current().Colors()
	return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceEnd, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body1(p.th, rec.Name)
			lbl.Font.Weight = font.Bold
			lbl.Color = nameColor
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !isLocal {
				return layout.Dimensions{}
			}
			badge := material.Body2(p.th, localengine.T("configs", "badge", "local"))
			badge.Color = colors.Fg
			return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, badge.Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !isHashMismatch {
				return layout.Dimensions{}
			}
			badge := material.Body2(p.th, localengine.T("configs", "badge", "modified"))
			badge.Color = colors.StatusWarning
			return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, badge.Layout)
		}),
	)
}

func (p *ConfigsPage) layoutConfigCard(gtx layout.Context, idx int) layout.Dimensions {
	rec := p.configs[idx]
	isCached := p.ctrl.Backend().HasCachedConfig(rec.Name)
	if !rec.IsLocal() {
		isCached = isCached && !rec.LastUpdate.IsZero()
	}
	isActive := rec.Name == p.ctrl.Backend().GetActiveName()

	click, ok := p.cardClicks[rec.Name]
	if !ok {
		click = new(widget.Clickable)
		p.cardClicks[rec.Name] = click
	}

	colors := theme.Current().Colors()
	bg := p.cardBackground(isCached, rec.IsAutoUpdate(), colors)

	nameColor := p.th.Palette.Fg
	if isActive {
		nameColor = colors.Fg
	}

	isHashMismatch := p.hashMismatches[rec.Name]

	return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			macro := op.Record(gtx.Ops)
			dims := layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return p.layoutConfigCardHeader(gtx, rec, nameColor, isHashMismatch)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Body2(p.th, p.formatConfigMeta(rec)).Layout(gtx)
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

func (p *ConfigsPage) layoutTypeSelector(gtx layout.Context, enum *widget.Enum) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(p.th, localengine.T("configs", "dialog", "type")).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Point{Y: gtx.Dp(unit.Dp(4))}}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceStart, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.RadioButton(p.th, enum, config.ConfigTypeRemote, localengine.T("configs", "dialog", "type_remote")).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widgets.HSpace(gtx, unit.Dp(16))
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.RadioButton(p.th, enum, config.ConfigTypeLocal, localengine.T("configs", "dialog", "type_local")).Layout(gtx)
				}),
			)
		}),
	)
}

func (p *ConfigsPage) openAddDialog() {
	var nameEd widget.Editor
	var urlEd widget.Editor
	var periodEd widget.Editor
	var autoUpdate widget.Bool
	var typeSel widget.Enum
	nameEd.SingleLine = true
	urlEd.SingleLine = true
	periodEd.SingleLine = true
	periodEd.SetText(fmt.Sprintf("%d", p.ctrl.Backend().Config().MustGet("updates", "default_interval_hours").Int()))
	autoUpdate.Value = true
	typeSel.Value = config.ConfigTypeRemote

	save := func() {
		hours := p.ctrl.Backend().Config().MustGet("updates", "default_interval_hours").Int()
		fmt.Sscanf(periodEd.Text(), "%d", &hours)
		if hours <= 0 {
			hours = p.ctrl.Backend().Config().MustGet("updates", "default_interval_hours").Int()
		}
		url := urlEd.Text()
		if typeSel.Value == config.ConfigTypeLocal {
			url = ""
		}
		rec := config.ConfigRecord{
			Name:                nameEd.Text(),
			URL:                 url,
			Type:                typeSel.Value,
			UpdateIntervalHours: hours,
			Parent:              "user",
			AutoUpdate:          &autoUpdate.Value,
		}
		go func() {
			if err := p.ctrl.Backend().AddConfig(rec); err == nil {
				p.refreshConfigs()
			}
		}()
	}

	p.dialog.Show(widgets.Custom(localengine.T("configs", "dialog", "title"), func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.layoutTypeSelector(gtx, &typeSel)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widgets.LabeledInput(gtx, p.th, localengine.T("configs", "dialog", "name"), &nameEd, false)
			}),
		}
		if typeSel.Value == config.ConfigTypeRemote {
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widgets.LabeledInput(gtx, p.th, localengine.T("configs", "dialog", "url"), &urlEd, false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widgets.LabeledInput(gtx, p.th, localengine.T("configs", "dialog", "period"), &periodEd, false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.CheckBox(p.th, &autoUpdate, localengine.T("configs", "dialog", "auto_update")).Layout(gtx)
				}),
			)
		}
		return widgets.DialogSpacedList(gtx, children...)
	}), widgets.Confirm(save), widgets.Cancel())
}

func (p *ConfigsPage) editRecordFromEditors(old config.ConfigRecord, nameEd, urlEd, periodEd *widget.Editor, autoUpdate *widget.Bool) config.ConfigRecord {
	hours := old.UpdateIntervalHours
	fmt.Sscanf(periodEd.Text(), "%d", &hours)
	if hours <= 0 {
		hours = p.ctrl.Backend().Config().MustGet("updates", "default_interval_hours").Int()
	}
	return config.ConfigRecord{
		Name:                nameEd.Text(),
		URL:                 urlEd.Text(),
		Type:                old.Type,
		UpdateIntervalHours: hours,
		Parent:              old.Parent,
		LastUpdate:          old.LastUpdate,
		AutoUpdate:          &autoUpdate.Value,
	}
}

func (p *ConfigsPage) layoutEditTypeLabel(typeLabel string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Body2(p.th, localengine.T("configs", "dialog", "type")).Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Point{Y: gtx.Dp(unit.Dp(4))}}
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Body1(p.th, typeLabel).Layout(gtx)
			}),
		)
	}
}

func (p *ConfigsPage) layoutEditRemoteFields(urlEd, periodEd *widget.Editor, autoUpdate *widget.Bool) []layout.FlexChild {
	return []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return widgets.LabeledInput(gtx, p.th, localengine.T("configs", "dialog", "url"), urlEd, false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return widgets.LabeledInput(gtx, p.th, localengine.T("configs", "dialog", "period"), periodEd, false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.CheckBox(p.th, autoUpdate, localengine.T("configs", "dialog", "auto_update")).Layout(gtx)
		}),
	}
}

func (p *ConfigsPage) showValidationResult(result singboxconfig.ValidationResult) {
	var copyBtn widget.Clickable
	p.dialog.Show(widgets.Custom(localengine.T("configs", "dialog", "validation_result"), func(gtx layout.Context) layout.Dimensions {
		if copyBtn.Clicked(gtx) {
			gtx.Execute(clipboard.WriteCmd{
				Type: "text/plain",
				Data: io.NopCloser(strings.NewReader(p.formatValidationResult(result))),
			})
		}
		return p.layoutValidationResult(gtx, result, &copyBtn)
	}), widgets.Close())
}

func (p *ConfigsPage) formatValidationResult(result singboxconfig.ValidationResult) string {
	var sb strings.Builder
	if len(result.Errors) > 0 {
		sb.WriteString(fmt.Sprintf(localengine.T("validation", "errors_title"), len(result.Errors)))
		sb.WriteString("\n")
		for _, e := range result.Errors {
			sb.WriteString("- ")
			sb.WriteString(p.formatDeprecatedField(e, true))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	if len(result.Warnings) > 0 {
		sb.WriteString(fmt.Sprintf(localengine.T("validation", "warnings_title"), len(result.Warnings)))
		sb.WriteString("\n")
		for _, w := range result.Warnings {
			sb.WriteString("- ")
			sb.WriteString(p.formatDeprecatedField(w, false))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	if len(result.Errors) == 0 && len(result.Warnings) == 0 {
		sb.WriteString(localengine.T("validation", "ok"))
	}
	return sb.String()
}

func (p *ConfigsPage) formatDeprecatedField(f singboxconfig.DeprecatedField, isError bool) string {
	if isError {
		return fmt.Sprintf(localengine.T("validation", "item", "removed"), f.Removed, f.Path, f.Replacement)
	}
	if f.Removed != "" {
		return fmt.Sprintf(localengine.T("validation", "item", "deprecated"), f.Deprecated, f.Removed, f.Path, f.Replacement)
	}
	return fmt.Sprintf(localengine.T("validation", "item", "deprecated_no_removal"), f.Deprecated, f.Path, f.Replacement)
}

func (p *ConfigsPage) layoutValidationResult(gtx layout.Context, result singboxconfig.ValidationResult, copyBtn *widget.Clickable) layout.Dimensions {
	colors := theme.Current().Colors()
	children := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return p.layoutValidationSummary(gtx, result, colors)
		},
	}
	if len(result.Errors) > 0 {
		title := fmt.Sprintf(localengine.T("validation", "errors_title"), len(result.Errors))
		children = append(children, p.layoutValidationSection(title, result.Errors, colors.Error, true)...)
	}
	if len(result.Warnings) > 0 {
		title := fmt.Sprintf(localengine.T("validation", "warnings_title"), len(result.Warnings))
		children = append(children, p.layoutValidationSection(title, result.Warnings, colors.Warning, false)...)
	}
	if len(result.Errors) == 0 && len(result.Warnings) == 0 {
		children = append(children, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body1(p.th, localengine.T("validation", "ok"))
			lbl.Color = colors.Success
			return lbl.Layout(gtx)
		})
	}
	children = append(children, func(gtx layout.Context) layout.Dimensions {
		return material.Button(p.th, copyBtn, localengine.T("configs", "dialog", "btn", "copy")).Layout(gtx)
	})

	p.validationList.Axis = layout.Vertical
	list := material.List(p.th, &p.validationList)
	list.AnchorStrategy = material.Overlay

	maxH := gtx.Dp(dialogMaxBodyHeight)
	if gtx.Constraints.Max.Y > maxH {
		gtx.Constraints.Max.Y = maxH
	}
	if gtx.Constraints.Min.Y > gtx.Constraints.Max.Y {
		gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
	}
	return list.Layout(gtx, len(children), func(gtx layout.Context, i int) layout.Dimensions {
		return children[i](gtx)
	})
}

func (p *ConfigsPage) layoutValidationSummary(gtx layout.Context, result singboxconfig.ValidationResult, colors theme.Palette) layout.Dimensions {
	var msg string
	var c color.NRGBA
	switch {
	case len(result.Errors) > 0:
		msg = fmt.Sprintf(localengine.T("validation", "summary_errors"), len(result.Errors), len(result.Warnings))
		c = colors.Error
	case len(result.Warnings) > 0:
		msg = fmt.Sprintf(localengine.T("validation", "summary_warnings"), len(result.Warnings))
		c = colors.Warning
	default:
		msg = localengine.T("validation", "ok")
		c = colors.Success
	}
	lbl := material.Body1(p.th, msg)
	lbl.Color = c
	lbl.Font.Weight = font.Bold
	return lbl.Layout(gtx)
}

func (p *ConfigsPage) layoutValidationSection(title string, items []singboxconfig.DeprecatedField, titleColor color.NRGBA, isError bool) []layout.Widget {
	children := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body1(p.th, title)
			lbl.Color = titleColor
			lbl.Font.Weight = font.Bold
			return lbl.Layout(gtx)
		},
	}
	for _, it := range items {
		it := it
		children = append(children, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(p.th, "• "+p.formatDeprecatedField(it, isError))
			return lbl.Layout(gtx)
		})
	}
	return children
}

func (p *ConfigsPage) openEditDialog(idx int) {
	if idx < 0 || idx >= len(p.configs) {
		return
	}
	old := p.configs[idx]
	isLocal := old.IsLocal()

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

	typeLabel := localengine.T("configs", "dialog", "type_remote")
	if isLocal {
		typeLabel = localengine.T("configs", "dialog", "type_local")
	}

	hasCache := p.ctrl.Backend().HasCachedConfig(old.Name)

	save := func() {
		rec := p.editRecordFromEditors(old, &nameEd, &urlEd, &periodEd, &autoUpdate)
		go func() {
			if err := p.ctrl.Backend().EditConfig(old.Name, rec); err == nil {
				p.refreshConfigs()
			}
		}()
	}
	openDir := func() {
		go func() { _ = p.ctrl.Backend().OpenConfigDir(old.Name) }()
	}
	validate := func() {
		go func() {
			res, err := p.ctrl.Backend().ValidateConfig(old.Name)
			if err != nil {
				p.dialog.Show(widgets.Text(localengine.T("configs", "dialog", "validation_error"), err.Error()))
				return
			}
			p.showValidationResult(res)
		}()
	}

	buttons := []widgets.ButtonSpec{
		widgets.Confirm(save),
		widgets.Action(localengine.T("configs", "dialog", "btn", "open_dir"), openDir),
		widgets.Danger(localengine.T("configs", "dialog", "btn", "delete"), func() {
			p.onDelete(old.Name)
		}),
	}
	if isLocal {
		if hasCache {
			buttons = append(buttons,
				widgets.Action(localengine.T("configs", "dialog", "btn", "open"), func() {
					go func() { _ = p.ctrl.Backend().OpenConfigFile(old.Name) }()
				}),
				widgets.Action(localengine.T("configs", "dialog", "btn", "validate"), validate),
			)
		} else {
			buttons = append(buttons,
				widgets.Action(localengine.T("configs", "dialog", "btn", "create"), func() {
					go func() {
						if err := p.ctrl.Backend().RecreateLocalConfig(old.Name); err == nil {
							p.refreshConfigs()
						}
					}()
				}),
			)
		}
	} else {
		buttons = append(buttons,
			widgets.Action(localengine.T("configs", "dialog", "btn", "update_now"), func() {
				go func() {
					p.dialog.Show(widgets.Loading(localengine.T("progress", "updating_configs")))
					_ = p.ctrl.Backend().UpdateConfigNow(old.Name, urlEd.Text())
					p.dialog.Hide()
					p.refreshConfigs()
				}()
			}),
		)
		if hasCache {
			buttons = append(buttons, widgets.Action(localengine.T("configs", "dialog", "btn", "validate"), validate))
		}
	}
	buttons = append(buttons, widgets.Cancel())

	p.dialog.Show(widgets.Custom(localengine.T("configs", "dialog", "title"), func(gtx layout.Context) layout.Dimensions {
		children := p.editDialogChildren(&nameEd, &urlEd, &periodEd, &autoUpdate,
			typeLabel, isLocal, hasCache)
		return widgets.DialogSpacedList(gtx, children...)
	}), buttons...)
}

func (p *ConfigsPage) editDialogChildren(nameEd, urlEd, periodEd *widget.Editor, autoUpdate *widget.Bool,
	typeLabel string, isLocal, hasCache bool) []layout.FlexChild {
	children := []layout.FlexChild{
		layout.Rigid(p.layoutEditTypeLabel(typeLabel)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return widgets.LabeledInput(gtx, p.th, localengine.T("configs", "dialog", "name"), nameEd, false)
		}),
	}
	if !isLocal {
		children = append(children, p.layoutEditRemoteFields(urlEd, periodEd, autoUpdate)...)
	}
	return children
}

func (p *ConfigsPage) onDelete(name string) {
	p.dialog.Show(widgets.Text(localengine.T("configs", "dialog", "delete_title"),
		localengine.T("configs", "dialog", "delete_msg")+" \""+name+"\"?"),
		widgets.Cancel(),
		widgets.Danger(localengine.T("configs", "dialog", "btn", "delete"), func() {
			go func() {
				_ = p.ctrl.Backend().DeleteConfig(name)
				p.refreshConfigs()
			}()
		}),
	)
}

func (p *ConfigsPage) onUpdateAll() {
	p.dialog.Show(widgets.Loading(localengine.T("progress", "updating_configs")))
	_, _, err := p.ctrl.Backend().UpdateAllConfigs(nil)
	p.dialog.Hide()
	if err == nil {
		p.refreshConfigs()
	}
}
