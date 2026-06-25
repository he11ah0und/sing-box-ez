package pages

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"gio.tools/icons"
	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	coreapi "sing-box-ez/internal/core/api"
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/gui/gio/theme"
	"sing-box-ez/internal/gui/gio/widgets"
)

const (
	defaultURLTestURL = "http://www.gstatic.com/generate_204"
	maxGraphPoints    = 60
)

type mainTab int

const (
	tabOverview mainTab = iota
	tabGroups
	tabConnections
)

// MainPage renders the main control screen with start/stop/restart and live
// core API controls (mode, groups, connections).
type MainPage struct {
	th   *material.Theme
	ctrl *core.InteractiveController

	invalidate func()
	dialog     widgets.DialogProvider

	mainBtn    widget.Clickable
	restartBtn widget.Clickable

	processing bool
	spinAngle  float32
	spinTime   time.Time

	activeTab   mainTab
	tabBtns     []widget.Clickable
	contentList widget.List

	configDropdown *widgets.Dropdown
	modeDropdown   *widgets.Dropdown

	// API state, protected by apiMu.
	apiMu       sync.Mutex
	apiClient   coreapi.CoreAPIClient
	apiStatus   coreapi.Status
	apiMode     string
	apiGroups   []coreapi.Group
	apiConns    []coreapi.Connection
	apiErr      string
	lastRefresh time.Time

	// Groups tab state, protected by groupMu.
	groupMu        sync.Mutex
	groupDelays    map[string]map[string]int
	expandedGroups map[string]bool
	groupTesting   map[string]bool
	groupTestMsg   map[string]string

	// Connections tab state.
	connMu          sync.RWMutex
	connRows        map[string]*widget.Clickable
	closeDetailsBtn widget.Clickable
	closeConnsBtn   widget.Clickable

	// Traffic graph state, protected by trafficMu.
	trafficMu     sync.Mutex
	upSpark       *widgets.Sparkline
	downSpark     *widgets.Sparkline
	trafficErr    string
	lastUpTotal   int64
	lastDownTotal int64
	lastTraffic   time.Time
	currentUpRate string
	currentDnRate string

	lastConnUpTotal   int64
	lastConnDownTotal int64
	lastConnTraffic   time.Time

	lastInvalidate time.Time

	// Cached dropdown state to avoid rebuilding widgets every frame.
	lastConfigNames      []string
	lastActiveConfigName string
	lastModeValue        string
}

// NewMainPage creates a new main page.
func NewMainPage(th *material.Theme, ctrl *core.InteractiveController, dialog widgets.DialogProvider, invalidate func()) *MainPage {
	p := &MainPage{
		th:             th,
		ctrl:           ctrl,
		dialog:         dialog,
		invalidate:     invalidate,
		connRows:       make(map[string]*widget.Clickable),
		groupDelays:    make(map[string]map[string]int),
		expandedGroups: make(map[string]bool),
		groupTesting:   make(map[string]bool),
		groupTestMsg:   make(map[string]string),
	}

	colors := theme.Current().Colors()
	p.upSpark = widgets.NewSparkline(colors.Success, maxGraphPoints)
	p.downSpark = widgets.NewSparkline(colors.Info, maxGraphPoints)

	p.configDropdown = widgets.NewDropdown(
		th, dialog,
		localengine.T("main", "active", "label"),
		"",
		[]string{},
		nil,
		func(s string) { go p.activateConfigAndMaybeRestart(s) },
	)

	p.modeDropdown = widgets.NewDropdown(
		th, dialog,
		localengine.T("main", "api", "mode"),
		"rule",
		[]string{"rule", "global", "direct"},
		p.formatMode,
		func(s string) { go p.setMode(s) },
	)

	p.tabBtns = make([]widget.Clickable, 3)
	p.contentList.Axis = layout.Vertical

	go p.refreshLoop()
	go p.trafficLoop()
	return p
}

func (p *MainPage) Tag() string  { return "main" }
func (p *MainPage) Name() string { return localengine.T("tab", "main") }
func (p *MainPage) Icon() *widget.Icon {
	return icons.ActionHome
}

// NoShellScroll tells the shell not to wrap this page in its own scroller;
// the page manages a fixed sidebar and a scrollable content area itself.
func (p *MainPage) NoShellScroll() bool { return true }

func (p *MainPage) Layout(gtx layout.Context) layout.Dimensions {
	p.handleClicks(gtx)

	if !p.dialog.Visible() {
		p.syncConfigDropdown()
		p.syncUIFromData()
	}

	if !p.ctrl.Backend().IsRunning() {
		return p.stoppedLayout(gtx)
	}
	return p.runningLayout(gtx)
}

func (p *MainPage) stoppedLayout(gtx layout.Context) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return widgets.SpacedList(gtx, p.stoppedChildren(gtx)...)
	})
}

func (p *MainPage) runningLayout(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(p.layoutTabBar),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(p.th, &p.contentList).Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
				return widgets.SpacedList(gtx, p.contentChildren(gtx)...)
			})
		}),
	)
}

func (p *MainPage) contentChildren(gtx layout.Context) []layout.FlexChild {
	switch p.activeTab {
	case tabGroups:
		return p.groupsChildren()
	case tabConnections:
		return p.connectionsChildren()
	default:
		return p.overviewChildren(gtx)
	}
}

func (p *MainPage) stoppedChildren(gtx layout.Context) []layout.FlexChild {
	return []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if p.processing {
					return p.spinnerButton(gtx, unit.Dp(120))
				}
				return p.roundButton(gtx, p.th, &p.mainBtn, localengine.T("main", "btn", "start"), unit.Dp(120))
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.configDropdown.Layout(gtx, false)
		}),
	}
}

func (p *MainPage) overviewChildren(gtx layout.Context) []layout.FlexChild {
	return []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutStatusHeader(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutProfileCard(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.modeDropdown.Layout(gtx, false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutGraphs(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutBottomControls(gtx)
		}),
	}
}

func (p *MainPage) groupsChildren() []layout.FlexChild {
	p.apiMu.Lock()
	groups := p.apiGroups
	errMsg := p.apiErr
	p.apiMu.Unlock()

	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.H6(p.th, localengine.T("main", "groups", "title")).Layout(gtx)
				}),
			)
		}),
	}
	if errMsg != "" {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(p.th, errMsg)
			lbl.Color = theme.Current().Colors().Error
			return lbl.Layout(gtx)
		}))
	}
	if len(groups) == 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(p.th, localengine.T("main", "groups", "empty")).Layout(gtx)
		}))
		return children
	}

	sorted := make([]coreapi.Group, len(groups))
	copy(sorted, groups)
	sort.Slice(sorted, func(i, j int) bool {
		return strings.ToLower(sorted[i].Tag) < strings.ToLower(sorted[j].Tag)
	})

	for i := range sorted {
		g := sorted[i]
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutGroupCard(gtx, g)
		}))
	}
	return children
}

func (p *MainPage) connectionsChildren() []layout.FlexChild {
	p.apiMu.Lock()
	conns := p.apiConns
	errMsg := p.apiErr
	p.apiMu.Unlock()

	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.H6(p.th, fmt.Sprintf(localengine.T("main", "api", "connections"), len(conns))).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Button(p.th, &p.closeConnsBtn, localengine.T("main", "api", "close_connections")).Layout(gtx)
				}),
			)
		}),
	}
	if errMsg != "" {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(p.th, errMsg)
			lbl.Color = theme.Current().Colors().Error
			return lbl.Layout(gtx)
		}))
	}
	if len(conns) == 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(p.th, localengine.T("main", "connections", "empty")).Layout(gtx)
		}))
		return children
	}
	for _, conn := range conns {
		conn := conn
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutConnectionRow(gtx, conn)
		}))
	}
	return children
}

// ---------- tab bar ----------

func (p *MainPage) layoutTabBar(gtx layout.Context) layout.Dimensions {
	tabs := []struct {
		label string
		btn   *widget.Clickable
	}{
		{localengine.T("main", "tabs", "overview"), &p.tabBtns[tabOverview]},
		{localengine.T("main", "tabs", "groups"), &p.tabBtns[tabGroups]},
		{localengine.T("main", "tabs", "connections"), &p.tabBtns[tabConnections]},
	}

	children := make([]layout.FlexChild, 0, len(tabs)*2-1)
	for i, t := range tabs {
		idx := mainTab(i)
		if i > 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widgets.HSpace(gtx, unit.Dp(4))
			}))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.tabButton(gtx, idx, t.label, t.btn)
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceStart}.Layout(gtx, children...)
}

func (p *MainPage) tabButton(gtx layout.Context, tab mainTab, label string, btn *widget.Clickable) layout.Dimensions {
	colors := theme.Current().Colors()
	active := p.activeTab == tab
	bg := colors.Surface
	fg := colors.Fg
	borderColor := colors.Border
	if active {
		bg = colors.SurfaceVariant
		fg = colors.Primary
		borderColor = colors.Primary
	}
	if btn.Hovered() {
		bg = colors.Hover
	}

	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return widgets.BorderedCard(gtx, borderColor, bg, unit.Dp(1), unit.Dp(4), unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(p.th, label)
			lbl.Color = fg
			if active {
				lbl.Font.Weight = 700
			}
			return lbl.Layout(gtx)
		})
	})
}

// ---------- overview widgets ----------

func (p *MainPage) layoutStatusHeader(gtx layout.Context) layout.Dimensions {
	p.apiMu.Lock()
	status := p.apiStatus
	errMsg := p.apiErr
	p.apiMu.Unlock()

	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.H6(p.th, localengine.T("main", "api", "title")).Layout(gtx)
		}),
	}

	if info := p.ctrl.Backend().APIInfo(); info != nil {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(p.th, localengine.T("main", "api", "backend")+string(info.Backend)).Layout(gtx)
		}))
	}

	if status.Version != "" {
		uptime := status.Uptime.Round(time.Second).String()
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(p.th, fmt.Sprintf(localengine.T("main", "api", "status"), status.Version, uptime)).Layout(gtx)
		}))
	}
	if errMsg != "" {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(p.th, errMsg)
			lbl.Color = theme.Current().Colors().Error
			return lbl.Layout(gtx)
		}))
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (p *MainPage) layoutProfileCard(gtx layout.Context) layout.Dimensions {
	colors := theme.Current().Colors()
	active := p.ctrl.Backend().GetActiveConfig()
	name := localengine.T("main", "active", "none")
	badge := ""
	updated := ""
	if active != nil {
		name = active.Name
		if active.Type == config.ConfigTypeRemote {
			badge = localengine.T("configs", "badge", "remote")
		} else {
			badge = localengine.T("configs", "badge", "local")
		}
		if !active.LastUpdate.IsZero() {
			updated = formatRelative(active.LastUpdate.Time)
		}
	}

	return widgets.BorderedCard(gtx, colors.Separator, colors.Surface, unit.Dp(1), unit.Dp(8), unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.Body2(p.th, localengine.T("main", "dashboard", "profile")).Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if badge == "" {
							return layout.Dimensions{}
						}
						return p.badgeChip(gtx, badge, colors)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widgets.VSpace(gtx, unit.Dp(8))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Body2(p.th, name).Layout(gtx)
			}),
		}
		if updated != "" {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Caption(p.th, updated)
					lbl.Color = colors.DisabledFg
					return lbl.Layout(gtx)
				})
			}))
		}
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return p.configDropdown.Layout(gtx, false)
				})
			}),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (p *MainPage) badgeChip(gtx layout.Context, text string, colors theme.Palette) layout.Dimensions {
	return widgets.BorderedCard(gtx, colors.Separator, colors.SurfaceVariant, unit.Dp(1), unit.Dp(4), unit.Dp(4), func(gtx layout.Context) layout.Dimensions {
		lbl := material.Caption(p.th, text)
		lbl.Color = colors.Fg
		return lbl.Layout(gtx)
	})
}

func (p *MainPage) layoutGraphs(gtx layout.Context) layout.Dimensions {
	colors := theme.Current().Colors()
	p.trafficMu.Lock()
	upSpark := p.upSpark
	dnSpark := p.downSpark
	upRate := p.currentUpRate
	dnRate := p.currentDnRate
	trafficErr := p.trafficErr
	p.trafficMu.Unlock()

	return widgets.BorderedCard(gtx, colors.Separator, colors.Surface, unit.Dp(1), unit.Dp(8), unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.graphCard(gtx, localengine.T("main", "dashboard", "upload"), upRate, upSpark)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widgets.VSpace(gtx, unit.Dp(12))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.graphCard(gtx, localengine.T("main", "dashboard", "download"), dnRate, dnSpark)
			}),
		}
		if trafficErr != "" {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(p.th, trafficErr)
				lbl.Color = colors.Error
				return lbl.Layout(gtx)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (p *MainPage) graphCard(gtx layout.Context, title, rate string, spark *widgets.Sparkline) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return material.Body2(p.th, title).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if rate == "" {
						return layout.Dimensions{}
					}
					lbl := material.Body2(p.th, rate)
					lbl.Color = theme.Current().Colors().DisabledFg
					return lbl.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				w := unit.Dp(float32(gtx.Constraints.Max.X) / gtx.Metric.PxPerDp)
				return spark.Layout(gtx, w, unit.Dp(80))
			})
		}),
	)
}

func (p *MainPage) layoutBottomControls(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.processing {
					return layout.Dimensions{}
				}
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return material.Button(p.th, &p.mainBtn, localengine.T("main", "btn", "stop")).Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widgets.VSpace(gtx, unit.Dp(12))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.processing {
					return layout.Dimensions{}
				}
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return material.Button(p.th, &p.restartBtn, localengine.T("main", "btn", "restart")).Layout(gtx)
			}),
		)
	})
}

// ---------- groups widgets ----------

func (p *MainPage) layoutGroupCard(gtx layout.Context, g coreapi.Group) layout.Dimensions {
	colors := theme.Current().Colors()

	p.groupMu.Lock()
	delays := p.groupDelays[g.Tag]
	expanded, ok := p.expandedGroups[g.Tag]
	if !ok {
		expanded = true
		p.expandedGroups[g.Tag] = true
	}
	testing := p.groupTesting[g.Tag]
	msg := p.groupTestMsg[g.Tag]
	p.groupMu.Unlock()

	headerBtn := p.groupHeaderBtn(g.Tag)
	if headerBtn.Clicked(gtx) {
		p.groupMu.Lock()
		p.expandedGroups[g.Tag] = !expanded
		expanded = !expanded
		p.groupMu.Unlock()
	}

	testBtn := p.groupTestBtn(g.Tag)
	if testBtn.Clicked(gtx) {
		go p.testGroup(g.Tag)
	}

	return widgets.BorderedCard(gtx, colors.Separator, colors.Surface, unit.Dp(1), unit.Dp(8), unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.Clickable(gtx, headerBtn, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									lbl := material.Body2(p.th, g.Tag)
									lbl.Font.Weight = 700
									return lbl.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									selected := g.Selected
									if selected == "" {
										selected = "-"
									}
									lbl := material.Caption(p.th, selected)
									lbl.Color = colors.DisabledFg
									return lbl.Layout(gtx)
								}),
							)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return p.badgeChip(gtx, g.Type, colors)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return widgets.HSpace(gtx, unit.Dp(8))
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if g.Type == "URLTest" || g.Tag == "GLOBAL" {
									if g.DelayValid {
										return p.badgeChip(gtx, fmt.Sprintf("%d ms", g.Delay), colors)
									}
									return layout.Dimensions{}
								}
								label := localengine.T("main", "api", "url_test")
								if testing {
									label = localengine.T("main", "api", "url_test_testing")
								}
								return material.Button(p.th, testBtn, label).Layout(gtx)
							}),
						)
					}),
				)
			}),
		}

		if msg != "" {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Caption(p.th, msg)
					lbl.Color = colors.DisabledFg
					return lbl.Layout(gtx)
				})
			}))
		}

		if expanded {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widgets.VSpace(gtx, unit.Dp(8))
			}))
			for _, n := range g.Nodes {
				n := n
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return p.layoutNodeRow(gtx, g.Tag, n, g.Selected, delays)
				}))
			}
		}

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (p *MainPage) layoutNodeRow(gtx layout.Context, groupTag string, n coreapi.Node, selected string, delays map[string]int) layout.Dimensions {
	colors := theme.Current().Colors()
	isSelected := n.Tag == selected

	btn := p.nodeSelectBtn(groupTag, n.Tag)
	if btn.Clicked(gtx) {
		go p.selectNode(groupTag, n.Tag)
	}

	bg := colors.Surface
	fg := colors.Fg
	if isSelected {
		bg = colors.Primary
		fg = colors.OnPrimary
	} else if btn.Hovered() || btn.Pressed() {
		bg = colors.Hover
	}

	return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
			defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
			paint.Fill(gtx.Ops, bg)

			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(p.th, n.Tag)
						lbl.Color = fg
						return lbl.Layout(gtx)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(p.th, p.formatNodeDelay(n, delays))
						lbl.Color = p.delayColor(n, delays, fg)
						return lbl.Layout(gtx)
					})
				}),
			)
		})
	})
}

func (p *MainPage) formatNodeDelay(n coreapi.Node, delays map[string]int) string {
	if n.DelayValid {
		return fmt.Sprintf("%d ms", n.Delay)
	}
	if d, ok := delays[n.Tag]; ok {
		return fmt.Sprintf("%d ms", d)
	}
	return "-"
}

func (p *MainPage) delayColor(n coreapi.Node, delays map[string]int, defaultFg color.NRGBA) color.NRGBA {
	d := -1
	if n.DelayValid {
		d = n.Delay
	} else if v, ok := delays[n.Tag]; ok {
		d = v
	}
	if d < 0 {
		return defaultFg
	}
	colors := theme.Current().Colors()
	switch {
	case d < 300:
		return colors.Success
	case d < 800:
		return colors.Warning
	default:
		return colors.Error
	}
}

func (p *MainPage) groupHeaderBtn(tag string) *widget.Clickable {
	key := "h:" + tag
	p.connMu.RLock()
	btn, ok := p.connRows[key]
	p.connMu.RUnlock()
	if ok {
		return btn
	}
	p.connMu.Lock()
	defer p.connMu.Unlock()
	btn, ok = p.connRows[key]
	if !ok {
		btn = &widget.Clickable{}
		p.connRows[key] = btn
	}
	return btn
}

func (p *MainPage) groupTestBtn(tag string) *widget.Clickable {
	key := "t:" + tag
	p.connMu.RLock()
	btn, ok := p.connRows[key]
	p.connMu.RUnlock()
	if ok {
		return btn
	}
	p.connMu.Lock()
	defer p.connMu.Unlock()
	btn, ok = p.connRows[key]
	if !ok {
		btn = &widget.Clickable{}
		p.connRows[key] = btn
	}
	return btn
}

func (p *MainPage) nodeSelectBtn(group, node string) *widget.Clickable {
	key := group + "/" + node
	p.connMu.RLock()
	btn, ok := p.connRows[key]
	p.connMu.RUnlock()
	if ok {
		return btn
	}
	p.connMu.Lock()
	defer p.connMu.Unlock()
	btn, ok = p.connRows[key]
	if !ok {
		btn = &widget.Clickable{}
		p.connRows[key] = btn
	}
	return btn
}

// ---------- connection widgets ----------

func (p *MainPage) layoutConnectionRow(gtx layout.Context, conn coreapi.Connection) layout.Dimensions {
	colors := theme.Current().Colors()
	target := p.formatConnectionTarget(conn)
	outbound := conn.Outbound
	if outbound == "" && len(conn.Chain) > 0 {
		outbound = conn.Chain[len(conn.Chain)-1]
	}

	sub := fmt.Sprintf("↑%s ↓%s", formatBytes(conn.UplinkTotal), formatBytes(conn.DownlinkTotal))
	if outbound != "" {
		sub += " · " + outbound
	}

	btn := p.connRowBtn(conn.ID)
	if btn.Clicked(gtx) {
		p.showConnectionDetails(conn)
	}

	return widgets.BorderedCard(gtx, colors.Separator, colors.Surface, unit.Dp(1), unit.Dp(4), unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return material.Clickable(gtx, btn, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return material.Body2(p.th, target).Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							ipType := p.ipVersionLabel(conn.Destination)
							if ipType == "" {
								return layout.Dimensions{}
							}
							return p.badgeChip(gtx, ipType, colors)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(p.th, sub)
						lbl.Color = colors.DisabledFg
						return lbl.Layout(gtx)
					})
				}),
			)
		})
	})
}

func (p *MainPage) formatConnectionTarget(conn coreapi.Connection) string {
	host, port, isIP := splitHostPort(conn.Destination)

	if conn.Domain != "" {
		if port != "" {
			return conn.Domain + ":" + port
		}
		return conn.Domain
	}

	if host != "" {
		if port != "" {
			if isIP {
				return net.JoinHostPort(host, port)
			}
			return host + ":" + port
		}
		return host
	}

	id := conn.ID
	if len(id) > 8 {
		id = id[:8]
	}
	return id
}

// splitHostPort extracts host, port and whether the host is an IP from an
// address string. It handles both bracketed IPv6 forms and bare forms produced
// by some backends (e.g. "2606:4700::1:443").
func splitHostPort(addr string) (host, port string, isIP bool) {
	if addr == "" {
		return "", "", false
	}
	if h, p, err := net.SplitHostPort(addr); err == nil {
		return h, p, net.ParseIP(h) != nil
	}
	if i := strings.LastIndex(addr, ":"); i > 0 {
		candidate := addr[i+1:]
		if isPortString(candidate) {
			hostPart := addr[:i]
			if ip := net.ParseIP(hostPart); ip != nil {
				return hostPart, candidate, true
			}
			return hostPart, candidate, false
		}
	}
	if ip := net.ParseIP(addr); ip != nil {
		return addr, "", true
	}
	return addr, "", false
}

func isPortString(s string) bool {
	if s == "" || len(s) > 5 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func (p *MainPage) ipVersionLabel(destination string) string {
	host, _, isIP := splitHostPort(destination)
	if !isIP || host == "" {
		return ""
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return ""
	}
	if ip.To4() != nil {
		return "IPv4"
	}
	return "IPv6"
}

func (p *MainPage) connRowBtn(id string) *widget.Clickable {
	p.connMu.RLock()
	btn := p.connRows[id]
	p.connMu.RUnlock()
	if btn != nil {
		return btn
	}
	p.connMu.Lock()
	defer p.connMu.Unlock()
	if p.connRows[id] == nil {
		p.connRows[id] = &widget.Clickable{}
	}
	return p.connRows[id]
}

func (p *MainPage) showConnectionDetails(conn coreapi.Connection) {
	p.dialog.ShowCustom(localengine.T("connection_details", "title"), func(gtx layout.Context) layout.Dimensions {
		return p.layoutConnectionDetails(gtx, conn)
	})
}

func (p *MainPage) layoutConnectionDetails(gtx layout.Context, conn coreapi.Connection) layout.Dimensions {
	inbound := conn.Inbound
	if inbound == "" && conn.InboundType != "" {
		inbound = conn.InboundType
	} else if conn.InboundType != "" {
		inbound = fmt.Sprintf("%s (%s)", conn.Inbound, conn.InboundType)
	}
	outbound := conn.Outbound
	if outbound == "" && conn.OutboundType != "" {
		outbound = conn.OutboundType
	} else if conn.OutboundType != "" {
		outbound = fmt.Sprintf("%s (%s)", conn.Outbound, conn.OutboundType)
	}

	rows := []struct{ k, v string }{
		{localengine.T("connection_details", "id"), conn.ID},
		{localengine.T("connection_details", "inbound"), inbound},
		{localengine.T("connection_details", "network"), conn.Network},
		{localengine.T("connection_details", "source"), conn.Source},
		{localengine.T("connection_details", "destination"), conn.Destination},
		{localengine.T("connection_details", "domain"), conn.Domain},
		{localengine.T("connection_details", "outbound"), outbound},
		{localengine.T("connection_details", "chain"), strings.Join(conn.Chain, " → ")},
		{localengine.T("connection_details", "uplink"), formatBytes(conn.UplinkTotal)},
		{localengine.T("connection_details", "downlink"), formatBytes(conn.DownlinkTotal)},
		{localengine.T("connection_details", "created"), formatTime(conn.CreatedAt)},
	}
	if conn.ProcessInfo.UserName != "" {
		rows = append(rows,
			struct{ k, v string }{localengine.T("connection_details", "user"), conn.ProcessInfo.UserName},
			struct{ k, v string }{localengine.T("connection_details", "process"), conn.ProcessInfo.ProcessPath},
		)
	}

	children := make([]layout.FlexChild, 0, len(rows)+2)
	for _, r := range rows {
		r := r
		children = append(children, p.detailRow(r.k, r.v))
	}

	if p.closeDetailsBtn.Clicked(gtx) {
		go p.closeConnection(conn.ID)
		p.dialog.HideCustom()
	}
	children = append(children,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return widgets.VSpace(gtx, unit.Dp(16))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Button(p.th, &p.closeDetailsBtn, localengine.T("connection_details", "close")).Layout(gtx)
		}),
	)

	return widgets.DialogSpacedList(gtx, children...)
}

func (p *MainPage) detailRow(label, value string) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		if value == "" {
			return layout.Dimensions{}
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start, Spacing: layout.SpaceBetween}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: unit.Dp(12), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
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

// ---------- input handling ----------

func (p *MainPage) handleClicks(gtx layout.Context) {
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
	if p.closeConnsBtn.Clicked(gtx) {
		go p.closeConnections()
	}
	for i := range p.tabBtns {
		if p.tabBtns[i].Clicked(gtx) {
			p.activeTab = mainTab(i)
		}
	}
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
	if !sameStrings(p.lastConfigNames, names) {
		p.configDropdown.SetOptions(names, nil, nil)
		p.lastConfigNames = names
	}
	if p.lastActiveConfigName != activeName {
		p.configDropdown.SetValue(activeName)
		p.lastActiveConfigName = activeName
	}
}

func (p *MainPage) syncUIFromData() {
	p.apiMu.Lock()
	mode := p.apiMode
	p.apiMu.Unlock()

	if p.lastModeValue != mode {
		p.modeDropdown.SetValue(mode)
		p.lastModeValue = mode
	}
}

func (p *MainPage) formatMode(m string) string {
	label := localengine.T("main", "api", "mode_"+m)
	if label == "mode_"+m {
		return m
	}
	return label
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
		p.spinAngle += dt * 6
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
		}
	}()
}

// ---------- API refresh ----------

func (p *MainPage) refreshLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if !p.ctrl.Backend().IsRunning() {
			p.clearAPIState()
			continue
		}
		client := p.ctrl.Backend().APIClient()
		if client == nil {
			p.clearAPIState()
			continue
		}
		p.fetchAPI(client)
	}
}

func (p *MainPage) clearAPIState() {
	p.apiMu.Lock()
	changed := p.apiClient != nil || p.apiStatus.Version != "" || p.apiMode != "" || len(p.apiGroups) > 0 || len(p.apiConns) > 0 || p.apiErr != ""
	p.apiClient = nil
	p.apiStatus = coreapi.Status{}
	p.apiMode = ""
	p.apiGroups = nil
	p.apiConns = nil
	p.apiErr = ""
	p.apiMu.Unlock()
	p.resetTrafficState()
	if changed && p.invalidate != nil {
		p.invalidate()
	}
}

func (p *MainPage) fetchAPI(client coreapi.CoreAPIClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var errMsg string
	status, err := client.Status(ctx)
	if err != nil {
		errMsg = err.Error()
	}

	mode, err := client.Mode(ctx)
	if err != nil && errMsg == "" {
		errMsg = err.Error()
	}

	groups, err := client.Groups(ctx)
	if err != nil && errMsg == "" {
		errMsg = err.Error()
	}

	conns, err := client.Connections(ctx)
	if err != nil && errMsg == "" {
		errMsg = err.Error()
	}

	p.apiMu.Lock()
	p.apiClient = client
	if status != nil {
		p.apiStatus = *status
	}
	p.apiMode = mode
	p.apiGroups = groups
	p.apiConns = conns
	p.apiErr = errMsg
	p.lastRefresh = time.Now()
	p.apiMu.Unlock()

	p.recordConnectionTraffic(conns)
	p.pruneConnRows(conns)

	if p.invalidate != nil {
		p.invalidate()
	}
}

func (p *MainPage) pruneConnRows(conns []coreapi.Connection) {
	active := make(map[string]struct{}, len(conns))
	for _, c := range conns {
		active[c.ID] = struct{}{}
	}
	p.connMu.Lock()
	defer p.connMu.Unlock()
	for key := range p.connRows {
		if strings.ContainsAny(key, ":/") {
			continue
		}
		if _, ok := active[key]; !ok {
			delete(p.connRows, key)
		}
	}
}

func (p *MainPage) setMode(mode string) {
	client := p.ctrl.Backend().APIClient()
	if client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.SetMode(ctx, mode); err != nil {
		p.ctrl.Backend().Terminal().Infof("Failed to set mode: %v", err)
		return
	}
	p.refreshNow()
}

func (p *MainPage) selectNode(group, node string) {
	client := p.ctrl.Backend().APIClient()
	if client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.SelectGroup(ctx, group, node); err != nil {
		p.ctrl.Backend().Terminal().Infof("Failed to select node: %v", err)
		return
	}
	p.refreshNow()
}

func (p *MainPage) testGroup(group string) {
	client := p.ctrl.Backend().APIClient()
	if client == nil {
		return
	}
	p.groupMu.Lock()
	p.groupTesting[group] = true
	p.groupTestMsg[group] = ""
	p.groupMu.Unlock()
	if p.invalidate != nil {
		p.invalidate()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	results, err := client.URLTest(ctx, group, defaultURLTestURL, 5*time.Second)
	if err != nil {
		p.groupMu.Lock()
		p.groupTestMsg[group] = fmt.Sprintf(localengine.T("main", "api", "url_test_error"), err)
		p.groupTesting[group] = false
		p.groupMu.Unlock()
		if p.invalidate != nil {
			p.invalidate()
		}
		return
	}
	p.groupMu.Lock()
	if p.groupDelays[group] == nil {
		p.groupDelays[group] = make(map[string]int)
	}
	for tag, delay := range results {
		p.groupDelays[group][tag] = delay
	}

	var total, count int
	for _, delay := range results {
		total += delay
		count++
	}
	if count > 0 {
		p.groupTestMsg[group] = fmt.Sprintf(localengine.T("main", "api", "url_test_result"), total/count, count)
	} else {
		p.groupTestMsg[group] = localengine.T("main", "api", "url_test_empty")
	}
	p.groupTesting[group] = false
	p.groupMu.Unlock()
	if p.invalidate != nil {
		p.invalidate()
	}
}

func (p *MainPage) closeConnections() {
	client := p.ctrl.Backend().APIClient()
	if client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.CloseConnections(ctx); err != nil {
		p.ctrl.Backend().Terminal().Infof("Failed to close connections: %v", err)
		return
	}
	p.refreshNow()
}

func (p *MainPage) closeConnection(id string) {
	client := p.ctrl.Backend().APIClient()
	if client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.CloseConnection(ctx, id); err != nil {
		p.ctrl.Backend().Terminal().Infof("Failed to close connection: %v", err)
	}
}

func (p *MainPage) refreshNow() {
	client := p.ctrl.Backend().APIClient()
	if client == nil {
		return
	}
	go p.fetchAPI(client)
}

// ---------- traffic graphs ----------

func (p *MainPage) trafficLoop() {
	var lastClient coreapi.CoreAPIClient
	const healthTimeout = 5 * time.Second
	backoff := 2 * time.Second
	for {
		if !p.ctrl.Backend().IsRunning() {
			if lastClient != nil {
				p.resetTrafficState()
				lastClient = nil
			}
			backoff = 2 * time.Second
			time.Sleep(500 * time.Millisecond)
			continue
		}
		client := p.ctrl.Backend().APIClient()
		if client == nil {
			if lastClient != nil {
				p.resetTrafficState()
				lastClient = nil
			}
			backoff = 2 * time.Second
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if client != lastClient {
			p.resetTrafficState()
			lastClient = client
			backoff = 2 * time.Second
		}

		ctx, cancel := context.WithCancel(context.Background())
		ch, stop, err := client.SubscribeStatus(ctx, 1*time.Second)
		if err != nil {
			p.setTrafficErr(err.Error())
			cancel()
			time.Sleep(backoff)
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		p.setTrafficErr("")

		// The watchdog breaks out if the subscription channel is stuck so
		// we never accumulate blocked goroutines/connections on Windows.
		watchdog := time.NewTimer(healthTimeout)
		received := false
		for {
			select {
			case ev, ok := <-ch:
				if !ok {
					// Subscription closed; clean up and resubscribe.
					stop()
					cancel()
					if !watchdog.Stop() {
						<-watchdog.C
					}
					goto nextIter
				}
				received = true
				if !watchdog.Stop() {
					select {
					case <-watchdog.C:
					default:
					}
				}
				watchdog.Reset(healthTimeout)
				if ev.Error != nil {
					if p.ctrl.Backend().IsRunning() {
						p.setTrafficErr(ev.Error.Error())
					}
					stop()
					cancel()
					goto nextIter
				}
				p.setTrafficErr("")
				backoff = 2 * time.Second
				p.recordTraffic(ev.Status)
			case <-watchdog.C:
				if !received {
					// No events for healthTimeout: the stream is silent. Fall back to
					// connection-derived traffic instead of keeping a dead stream open.
					stop()
					cancel()
					goto nextIter
				}
				received = false
				watchdog.Reset(healthTimeout)
			case <-ctx.Done():
				stop()
				cancel()
				if !watchdog.Stop() {
					<-watchdog.C
				}
				goto nextIter
			}
		}
	nextIter:
		p.setTrafficErr("")
		time.Sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (p *MainPage) setTrafficErr(msg string) {
	p.trafficMu.Lock()
	p.trafficErr = msg
	p.trafficMu.Unlock()
}

func (p *MainPage) resetTrafficState() {
	p.trafficMu.Lock()
	defer p.trafficMu.Unlock()
	p.upSpark = widgets.NewSparkline(theme.Current().Colors().Success, maxGraphPoints)
	p.downSpark = widgets.NewSparkline(theme.Current().Colors().Info, maxGraphPoints)
	p.lastUpTotal = 0
	p.lastDownTotal = 0
	p.lastTraffic = time.Time{}
	p.currentUpRate = ""
	p.currentDnRate = ""
	p.trafficErr = ""
	p.lastConnUpTotal = 0
	p.lastConnDownTotal = 0
	p.lastConnTraffic = time.Time{}
}

func (p *MainPage) recordTraffic(s coreapi.Status) {
	p.trafficMu.Lock()
	defer p.trafficMu.Unlock()

	now := time.Now()
	upRate := float64(s.Uplink)
	dnRate := float64(s.Downlink)

	if upRate == 0 && dnRate == 0 && s.TrafficAvailable {
		dt := now.Sub(p.lastTraffic).Seconds()
		if dt > 0 && !p.lastTraffic.IsZero() {
			upDelta := s.UplinkTotal - p.lastUpTotal
			dnDelta := s.DownlinkTotal - p.lastDownTotal
			if upDelta >= 0 {
				upRate = float64(upDelta) / dt
			}
			if dnDelta >= 0 {
				dnRate = float64(dnDelta) / dt
			}
		}
	}

	p.lastUpTotal = s.UplinkTotal
	p.lastDownTotal = s.DownlinkTotal
	p.lastTraffic = now

	p.upSpark.Add(upRate)
	p.downSpark.Add(dnRate)
	p.currentUpRate = formatSpeed(upRate)
	p.currentDnRate = formatSpeed(dnRate)

	p.invalidateMaybe()
}

func (p *MainPage) invalidateMaybe() {
	if p.invalidate == nil {
		return
	}
	now := time.Now()
	if now.Sub(p.lastInvalidate) >= 500*time.Millisecond {
		p.lastInvalidate = now
		p.invalidate()
	}
}

func (p *MainPage) recordConnectionTraffic(conns []coreapi.Connection) {
	var upTotal, downTotal int64
	for _, c := range conns {
		upTotal += c.UplinkTotal
		downTotal += c.DownlinkTotal
	}

	p.trafficMu.Lock()
	defer p.trafficMu.Unlock()

	now := time.Now()
	if !p.lastConnTraffic.IsZero() {
		dt := now.Sub(p.lastConnTraffic).Seconds()
		if dt > 0 {
			upDelta := upTotal - p.lastConnUpTotal
			dnDelta := downTotal - p.lastConnDownTotal
			if upDelta >= 0 && dnDelta >= 0 {
				upRate := float64(upDelta) / dt
				dnRate := float64(dnDelta) / dt
				// Prefer live traffic events when available; fall back to
				// connection totals if no traffic update arrived recently.
				if now.Sub(p.lastTraffic).Seconds() > 2 {
					p.upSpark.Add(upRate)
					p.downSpark.Add(dnRate)
					p.currentUpRate = formatSpeed(upRate)
					p.currentDnRate = formatSpeed(dnRate)
					p.invalidateMaybe()
				}
			}
		}
	}
	p.lastConnUpTotal = upTotal
	p.lastConnDownTotal = downTotal
	p.lastConnTraffic = now
}

// ---------- helpers ----------

func formatBytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	units := []string{"kB", "MB", "GB", "TB"}
	f := float64(b) / 1024
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

func formatSpeed(v float64) string {
	if v < 1024 {
		return fmt.Sprintf("%.0f B/s", v)
	}
	units := []string{"kB/s", "MB/s", "GB/s"}
	f := v / 1024
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

func formatRelative(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	return fmt.Sprintf("%d d ago", int(d.Hours()/24))
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}
