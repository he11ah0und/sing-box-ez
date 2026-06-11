package giogui

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
	"gio.tools/icons"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/gui/gio/pages"
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
	// Length = len(primary) + 1, last item is the Menu button.
	navBtns []widget.Clickable

	// Static nav (desktop/tablet persistent rail)
	staticNav     *component.NavDrawer
	staticAnim    component.VisibilityAnimation
	showStaticNav bool

	// Collapsible sidebar state (desktop only).
	collapsed bool
	toggleBtn widget.Clickable

	// Collapsed nav clickables (desktop icon-only rail)
	collapsedClicks []widget.Clickable

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
		collapsedClicks: make([]widget.Clickable, len(primary)+len(secondary)),
		secClicks:       make([]widget.Clickable, len(secondary)),
		navBtns:         make([]widget.Clickable, len(primary)+1),
	}

	// Static navigation rail/drawer (used on wide screens)
	staticNav := component.NewNav("", "")
	s.staticNav = &staticNav

	for _, p := range primary {
		s.staticNav.AddNavItem(component.NavItem{Tag: p.Tag(), Name: p.Name(), Icon: p.Icon()})
	}
	for _, p := range secondary {
		s.staticNav.AddNavItem(component.NavItem{Tag: p.Tag(), Name: p.Name(), Icon: p.Icon()})
	}

	return s
}

// RebuildNav recreates the static navigation drawer with updated labels.
func (s *Shell) RebuildNav() {
	selectedTag := s.staticNav.CurrentNavDestination()
	staticNav := component.NewNav("", "")
	s.staticNav = &staticNav
	for _, p := range s.primary {
		s.staticNav.AddNavItem(component.NavItem{Tag: p.Tag(), Name: p.Name(), Icon: p.Icon()})
	}
	for _, p := range s.secondary {
		s.staticNav.AddNavItem(component.NavItem{Tag: p.Tag(), Name: p.Name(), Icon: p.Icon()})
	}
	if tag, ok := selectedTag.(string); ok && tag != "" {
		s.staticNav.SetNavDestination(tag)
	}
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
	if s.showStaticNav && s.staticNav.NavDestinationChanged() {
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
			// Toggle sidebar collapsed state.
			if s.toggleBtn.Clicked(gtx) {
				s.collapsed = !s.collapsed
			}
			if s.collapsed {
				gtx.Constraints.Max.X = gtx.Dp(unit.Dp(56))
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return s.layoutCollapsedNav(gtx)
			}
			// Persistent rail: 240dp wide on desktop.
			gtx.Constraints.Max.X = gtx.Dp(unit.Dp(240))
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return s.layoutExpandedNav(gtx)
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
			s.currentPage = len(s.primary)
			return
		}
	}
}

// layoutCollapsedNav renders the icon-only rail for collapsed desktop sidebar.
func (s *Shell) layoutExpandedNav(gtx layout.Context) layout.Dimensions {
	// Handle clicks before layout.
	allPages := append(s.primary, s.secondary...)
	for i := range s.collapsedClicks {
		if i < len(allPages) && s.collapsedClicks[i].Clicked(gtx) {
			s.handleNavDestination(allPages[i].Tag())
		}
	}

	// Fill sidebar background.
	paint.FillShape(gtx.Ops, s.th.Palette.Bg, clip.Rect{Max: gtx.Constraints.Max}.Op())

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// Toggle button at the top, aligned like other nav items.
			gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(56))
			gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
			return material.Clickable(gtx, &s.toggleBtn, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: gtx.Constraints.Min}
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(16), Top: unit.Dp(12), Right: unit.Dp(12), Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									gtx.Constraints.Min = image.Pt(0, 0) // reset so icon uses default 24dp size
									return icons.NavigationMenu.Layout(gtx, s.th.Palette.Fg)
								})
							}),
						)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: gtx.Constraints.Min}
					}),
				)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, len(allPages))
			for i, p := range allPages {
				idx := i
				page := p
				active := false
				if idx < len(s.primary) {
					active = s.currentPage == idx
				} else {
					active = s.secondaryTag == page.Tag() && s.currentPage == len(s.primary)
				}
				children[i] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return s.expandedNavItem(gtx, page, &s.collapsedClicks[idx], active)
				})
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}),
	)
}

// expandedNavItem renders a single icon+label row in the expanded rail.
func (s *Shell) expandedNavItem(gtx layout.Context, page pages.Page, btn *widget.Clickable, active bool) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(56))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	col := s.th.Palette.Fg
	bg := color.NRGBA{}
	if active {
		col = s.th.Palette.ContrastFg
		bg = s.th.Palette.ContrastBg
	}
	return material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
		if bg.A > 0 {
			paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
		}
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(16), Top: unit.Dp(12), Right: unit.Dp(12), Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min = image.Pt(0, 0) // reset so icon uses default 24dp size
							icon := page.Icon()
							if icon == nil {
								return layout.Dimensions{}
							}
							return icon.Layout(gtx, col)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body1(s.th, page.Name())
						lbl.Color = col
						return lbl.Layout(gtx)
					}),
				)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
		)
	})
}

func (s *Shell) layoutCollapsedNav(gtx layout.Context) layout.Dimensions {
	// Handle clicks before layout.
	allPages := append(s.primary, s.secondary...)
	for i := range s.collapsedClicks {
		if i < len(allPages) && s.collapsedClicks[i].Clicked(gtx) {
			s.handleNavDestination(allPages[i].Tag())
		}
	}

	// Fill sidebar background.
	paint.FillShape(gtx.Ops, s.th.Palette.Bg, clip.Rect{Max: gtx.Constraints.Max}.Op())

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// Toggle button at the top.
			gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(56))
			gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
			return material.Clickable(gtx, &s.toggleBtn, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min = image.Pt(0, 0)
					return icons.NavigationMenu.Layout(gtx, s.th.Palette.Fg)
				})
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, len(allPages))
			for i, p := range allPages {
				idx := i
				page := p
				active := false
				if idx < len(s.primary) {
					active = s.currentPage == idx
				} else {
					active = s.secondaryTag == page.Tag() && s.currentPage == len(s.primary)
				}
				children[i] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(56))
					gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
					return s.collapsedNavItem(gtx, page, &s.collapsedClicks[idx], active)
				})
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}),
	)
}

// collapsedNavItem renders a single icon in the collapsed rail.
func (s *Shell) collapsedNavItem(gtx layout.Context, page pages.Page, btn *widget.Clickable, active bool) layout.Dimensions {
	col := s.th.Palette.Fg
	if active {
		col = s.th.Palette.ContrastFg
	}
	bg := color.NRGBA{}
	if active {
		bg = s.th.Palette.ContrastBg
	}
	return material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
		if bg.A > 0 {
			paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
		}
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Pt(0, 0)
			icon := page.Icon()
			if icon == nil {
				return layout.Dimensions{}
			}
			return icon.Layout(gtx, col)
		})
	})
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
		for i, p := range s.primary {
			if s.currentPage == i {
				return p.Layout(gtx)
			}
		}
		if s.currentPage == len(s.primary) {
			return s.layoutSecondaryPage(gtx)
		}
		if len(s.primary) > 0 {
			return s.primary[0].Layout(gtx)
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

	children := make([]layout.FlexChild, 0, len(s.secondary)*2)
	for i, p := range s.secondary {
		idx := i
		tag := p.Tag()
		name := p.Name()
		icon := p.Icon()
		active := s.secondaryTag == tag
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.secondaryButton(gtx, name, icon, &s.secClicks[idx], active)
		}))
		if i < len(s.secondary)-1 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Point{Y: gtx.Dp(unit.Dp(4))}}
			}))
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// secondaryButton renders a button for the secondary list on mobile.
func (s *Shell) secondaryButton(gtx layout.Context, label string, icon *widget.Icon, btn *widget.Clickable, active bool) layout.Dimensions {
	return material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
		bg := s.th.Palette.Bg
		if active {
			bg = s.th.Palette.ContrastBg
		}
		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())

		// Draw a subtle border around the item.
		borderCol := color.NRGBA{R: 128, G: 128, B: 128, A: 64}
		border := clip.Rect{Max: gtx.Constraints.Max}
		paint.FillShape(gtx.Ops, borderCol, clip.Stroke{
			Path:  border.Path(),
			Width: float32(gtx.Dp(unit.Dp(1))),
		}.Op())

		return layout.Inset{Left: unit.Dp(16), Top: unit.Dp(12), Right: unit.Dp(12), Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			col := s.th.Palette.Fg
			if active {
				col = s.th.Palette.ContrastFg
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if icon == nil {
						return layout.Dimensions{}
					}
					return icon.Layout(gtx, col)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Point{X: gtx.Dp(unit.Dp(12)), Y: 0}}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body1(s.th, label)
					lbl.Color = col
					return lbl.Layout(gtx)
				}),
			)
		})
	})
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
	prevHistory := make([]int, len(s.navBtns))
	for i := range s.navBtns {
		prevHistory[i] = len(s.navBtns[i].History())
	}

	menuIdx := len(s.primary)
	children := make([]layout.FlexChild, len(s.navBtns))
	for i := range s.navBtns {
		idx := i
		label := ""
		var icon *widget.Icon
		if idx < len(s.primary) {
			label = s.primary[idx].Name()
			icon = s.primary[idx].Icon()
		} else {
			label = "Menu"
			icon = icons.NavigationMenu
		}
		active := s.currentPage == idx
		children[i] = layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return s.navButton(gtx, label, icon, &s.navBtns[idx], active)
		})
	}

	dims := layout.Flex{
		Axis:    layout.Horizontal,
		Spacing: layout.SpaceEvenly,
	}.Layout(gtx, children...)

	// Handle bottom-nav interactions after rendering (material.Clickable consumes
	// the pointer event into the button history).
	for i := range s.navBtns {
		if len(s.navBtns[i].History()) > prevHistory[i] {
			s.currentPage = i
			if i == menuIdx {
				// Always reset to show the secondary list when Menu is tapped.
				s.secondaryTag = ""
			} else {
				s.secondaryTag = ""
			}
		}
	}

	return dims
}

// navButton renders a single bottom navigation button (icon + label).
func (s *Shell) navButton(gtx layout.Context, label string, icon *widget.Icon, btn *widget.Clickable, active bool) layout.Dimensions {
	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{
					Top:    unit.Dp(6),
					Bottom: unit.Dp(6),
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					col := s.th.Palette.Fg
					if active {
						col = s.th.Palette.ContrastFg
					}
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if icon == nil {
								return layout.Dimensions{}
							}
							return icon.Layout(gtx, col)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							lbl := material.Body2(s.th, label)
							lbl.Color = col
							lbl.TextSize = unit.Sp(11)
							return lbl.Layout(gtx)
						}),
					)
				})
			})
		}),
	)
}
