package gio

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/gio/pages"
)

// Shell provides an adaptive layout:
//   - Mobile (<600dp): Bottom Nav (Main, Configs, Menu) + Menu page with secondary items.
//   - Tablet/Desktop (>=600dp): Static side rail with all items, no bottom nav.
type Shell struct {
	th   *material.Theme
	cfg  *config.AppConfig
	ctrl *core.Controller

	primary   []pages.Page
	secondary []pages.Page

	// Bottom navigation clickables (mobile only)
	navMain    widget.Clickable
	navConfigs widget.Clickable
	navMenu    widget.Clickable

	// Static nav (desktop/tablet persistent rail)
	staticNav     *component.NavDrawer
	staticAnim    component.VisibilityAnimation
	showStaticNav bool

	// Secondary page clickables (mobile Menu page)
	secClicks []widget.Clickable

	// Navigation state.
	// 0 = primary[0], 1 = primary[1], 2 = secondary list or selected sub-page.
	currentPage int

	// Sub-page tag selected from the secondary list.
	secondaryTag string

	// Modal dialog (rendered at root level so it covers the whole window).
	dialog *Dialog
}

// NewShell creates the adaptive shell.
func NewShell(th *material.Theme, cfg *config.AppConfig, ctrl *core.Controller, primary, secondary []pages.Page) *Shell {
	s := &Shell{
		th:        th,
		cfg:       cfg,
		ctrl:      ctrl,
		primary:   primary,
		secondary: secondary,
		dialog:    NewDialog(),
		secClicks: make([]widget.Clickable, len(secondary)),
	}

	// Static navigation rail/drawer (used on wide screens)
	staticNav := component.NewNav("sing-box-ez", "")
	s.staticNav = &staticNav

	for _, p := range primary {
		s.staticNav.AddNavItem(component.NavItem{Tag: p.Tag(), Name: p.Name()})
	}
	for _, p := range secondary {
		s.staticNav.AddNavItem(component.NavItem{Tag: p.Tag(), Name: p.Name()})
	}

	return s
}

func (s *Shell) showDialog(title, body string) {
	s.dialog.Show(title, body)
}

func (s *Shell) showDialogMarkdown(title, body string) {
	s.dialog.ShowMarkdown(title, body)
}

func (s *Shell) showDialogLoading(title string) {
	s.dialog.ShowLoading(title)
}

// Layout draws the adaptive shell.
func (s *Shell) Layout(gtx layout.Context) layout.Dimensions {
	// Determine available width in dp.
	widthDp := float32(gtx.Constraints.Max.X) / gtx.Metric.PxPerDp
	isWide := widthDp >= 600

	// If we are on a wide screen we keep the static nav visible.
	s.showStaticNav = isWide

	// Use a snappy animation duration for the static rail.
	s.staticAnim.Duration = 200_000_000 // 200ms

	// Update the static nav visibility animation.
	if s.showStaticNav {
		s.staticAnim.Appear(gtx.Now)
	} else {
		s.staticAnim.Disappear(gtx.Now)
	}

	// Handle static nav item selections (desktop only).
	if s.staticNav.NavDestinationChanged() {
		if tag, ok := s.staticNav.CurrentNavDestination().(string); ok {
			s.handleNavDestination(tag)
		}
	}

	dims := layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		// Optional static side rail / drawer.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !s.showStaticNav {
				return layout.Dimensions{}
			}
			// Persistent rail: 240dp wide on desktop.
			gtx.Constraints.Max.X = gtx.Dp(unit.Dp(240))
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return s.staticNav.Layout(gtx, s.th, &s.staticAnim)
		}),
		// Main content area + optional bottom nav.
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				// Content
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return s.layoutContent(gtx)
				}),
				// Bottom navigation bar (only on narrow screens)
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if isWide {
						return layout.Dimensions{}
					}
					return s.layoutBottomNav(gtx)
				}),
			)
		}),
	)

	// Render modal dialog on top of everything.
	s.dialog.Layout(gtx, s.th)

	return dims
}

// handleNavDestination routes a nav drawer tag to the correct page.
func (s *Shell) handleNavDestination(tag string) {
	for i, p := range s.primary {
		if p.Tag() == tag {
			s.currentPage = i
			return
		}
	}
	for _, p := range s.secondary {
		if p.Tag() == tag {
			s.secondaryTag = tag
			s.currentPage = 2
			return
		}
	}
}

// layoutContent renders the current page with a background color.
func (s *Shell) layoutContent(gtx layout.Context) layout.Dimensions {
	// Fill background.
	bg := s.th.Palette.Bg
	if bg == (color.NRGBA{}) {
		bg = color.NRGBA{R: 18, G: 18, B: 18, A: 255}
	}
	paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())

	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		switch s.currentPage {
		case 0:
			if len(s.primary) > 0 {
				return s.primary[0].Layout(gtx)
			}
		case 1:
			if len(s.primary) > 1 {
				return s.primary[1].Layout(gtx)
			}
		case 2:
			return s.layoutSecondaryPage(gtx)
		default:
			if len(s.primary) > 0 {
				return s.primary[0].Layout(gtx)
			}
		}
		return material.Body1(s.th, "No pages").Layout(gtx)
	})
}

// layoutSecondaryPage shows either the selected sub-page (on desktop via side rail)
// or the list of secondary items (on mobile via Menu tab).
func (s *Shell) layoutSecondaryPage(gtx layout.Context) layout.Dimensions {
	if s.showStaticNav {
		// Desktop: side rail handles navigation, just show the selected sub-page.
		return s.layoutSubPage(gtx)
	}
	// Mobile: if a secondary page is selected, show it; otherwise show the list.
	if s.secondaryTag != "" {
		return s.layoutSubPage(gtx)
	}
	return s.layoutSecondaryList(gtx)
}

// layoutSubPage renders the actual sub-page content.
func (s *Shell) layoutSubPage(gtx layout.Context) layout.Dimensions {
	for _, p := range s.secondary {
		if p.Tag() == s.secondaryTag {
			return p.Layout(gtx)
		}
	}
	return material.Body1(s.th, "Select an item from the menu").Layout(gtx)
}

// layoutSecondaryList renders a list of buttons for secondary pages on mobile.
func (s *Shell) layoutSecondaryList(gtx layout.Context) layout.Dimensions {
	// Handle clicks before layout.
	for i, p := range s.secondary {
		if s.secClicks[i].Clicked(gtx) {
			s.secondaryTag = p.Tag()
		}
	}

	children := make([]layout.FlexChild, len(s.secondary))
	for i, p := range s.secondary {
		idx := i
		tag := p.Tag()
		name := p.Name()
		active := s.secondaryTag == tag
		children[i] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.secondaryButton(gtx, name, &s.secClicks[idx], active)
		})
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// secondaryButton renders a button for the secondary list on mobile.
func (s *Shell) secondaryButton(gtx layout.Context, label string, btn *widget.Clickable, active bool) layout.Dimensions {
	return material.Button(s.th, btn, label).Layout(gtx)
}

// layoutBottomNav renders the bottom navigation bar.
func (s *Shell) layoutBottomNav(gtx layout.Context) layout.Dimensions {
	// Background for the bottom bar.
	barHeight := gtx.Dp(unit.Dp(56))
	paint.FillShape(gtx.Ops, s.th.Palette.Bg, clip.Rect{
		Max: image.Point{X: gtx.Constraints.Max.X, Y: barHeight},
	}.Op())

	gtx.Constraints.Min.Y = barHeight
	gtx.Constraints.Max.Y = barHeight

	// Track click history lengths before rendering so we can detect new clicks.
	prevMain := len(s.navMain.History())
	prevConfigs := len(s.navConfigs.History())
	prevMenu := len(s.navMenu.History())

	primary0Name := "Main"
	if len(s.primary) > 0 {
		primary0Name = s.primary[0].Name()
	}
	primary1Name := "Configs"
	if len(s.primary) > 1 {
		primary1Name = s.primary[1].Name()
	}

	dims := layout.Flex{
		Axis:    layout.Horizontal,
		Spacing: layout.SpaceEvenly,
	}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return s.navButton(gtx, primary0Name, &s.navMain, s.currentPage == 0)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return s.navButton(gtx, primary1Name, &s.navConfigs, s.currentPage == 1)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return s.navButton(gtx, "Menu", &s.navMenu, s.currentPage == 2)
		}),
	)

	// Handle bottom-nav interactions after rendering (material.Clickable consumes
	// the pointer event into the button history).
	if len(s.navMain.History()) > prevMain {
		s.currentPage = 0
		s.secondaryTag = ""
	}
	if len(s.navConfigs.History()) > prevConfigs {
		s.currentPage = 1
		s.secondaryTag = ""
	}
	if len(s.navMenu.History()) > prevMenu {
		s.secondaryTag = ""
		s.currentPage = 2
	}

	return dims
}

// navButton renders a single bottom navigation button.
func (s *Shell) navButton(gtx layout.Context, label string, btn *widget.Clickable, active bool) layout.Dimensions {
	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{
					Top:    unit.Dp(8),
					Bottom: unit.Dp(8),
					Left:   unit.Dp(16),
					Right:  unit.Dp(16),
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					col := s.th.Palette.Fg
					if active {
						col = s.th.Palette.ContrastFg
					}
					lbl := material.Body2(s.th, label)
					lbl.Color = col
					return lbl.Layout(gtx)
				})
			})
		}),
	)
}
