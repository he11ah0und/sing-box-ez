package widgets

import (
	"fmt"
	"image/color"
	"strings"

	"gio.tools/icons"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"sing-box-ez/internal/gui/gio/theme"
)

// Dropdown renders a label with a dropdown field underneath.
// Clicking the field opens a modal dialog with selectable options, matching
// the startup mode selector behaviour. When there are many options it adds a
// search box and paginates the list inside the dialog.
type Dropdown struct {
	th          *material.Theme
	dialog      DialogProvider
	label       string
	value       string
	options     []string
	formatValue func(string) string
	onChange    func(string)

	trigger  widget.Clickable
	itemBtns []widget.Clickable

	searchEd          widget.Editor
	searchText        string
	page              int
	pageSize          int
	searchThreshold   int
	searchPlaceholder string
	prevLabel         string
	nextLabel         string

	prevBtn widget.Clickable
	nextBtn widget.Clickable
}

// NewDropdown creates a dropdown widget.
// Clicking the field opens a modal dialog with selectable options, matching
// the startup mode selector behaviour.
// formatValue may be nil; in that case the raw option value is shown.
func NewDropdown(
	th *material.Theme,
	dialog DialogProvider,
	label, value string,
	options []string,
	formatValue func(string) string,
	onChange func(string),
) *Dropdown {
	if formatValue == nil {
		formatValue = func(s string) string { return s }
	}
	d := &Dropdown{
		th:                th,
		dialog:            dialog,
		label:             label,
		value:             value,
		options:           options,
		formatValue:       formatValue,
		onChange:          onChange,
		pageSize:          50,
		searchThreshold:   10,
		searchPlaceholder: "Search...",
		prevLabel:         "Prev",
		nextLabel:         "Next",
	}
	d.searchEd.SingleLine = true
	d.resizeItemBtns()
	return d
}

// SetSearchPlaceholder sets the placeholder text for the search field.
func (d *Dropdown) SetSearchPlaceholder(v string) { d.searchPlaceholder = v }

// SetPageSize sets the number of options shown per page.
func (d *Dropdown) SetPageSize(v int) {
	if v < 1 {
		v = 50
	}
	d.pageSize = v
}

// SetSearchThreshold sets the minimum number of options required to show the
// search field. Use 0 to always show search, or a negative value to never show it.
func (d *Dropdown) SetSearchThreshold(v int) { d.searchThreshold = v }

// SetPaginationLabels overrides the prev/next button labels.
func (d *Dropdown) SetPaginationLabels(prev, next string) {
	d.prevLabel = prev
	d.nextLabel = next
}

// SetValue updates the displayed value.
func (d *Dropdown) SetValue(v string) { d.value = v }

// Show opens the dropdown dialog programmatically.
func (d *Dropdown) Show() {
	d.showDialog()
}

// SetLabel updates the label shown above the dropdown.
func (d *Dropdown) SetLabel(label string) { d.label = label }

// SetOptions replaces the available options and reformats the current value
// if it is no longer present in the new list.
func (d *Dropdown) SetOptions(options []string, formatValue func(string) string, onChange func(string)) {
	d.options = options
	if formatValue != nil {
		d.formatValue = formatValue
	}
	if onChange != nil {
		d.onChange = onChange
	}
	d.resizeItemBtns()
	d.page = 0
	found := false
	for _, o := range options {
		if o == d.value {
			found = true
			break
		}
	}
	if !found && len(options) > 0 {
		d.value = options[0]
	}
}

func (d *Dropdown) resizeItemBtns() {
	if len(d.itemBtns) < len(d.options) {
		d.itemBtns = append(d.itemBtns, make([]widget.Clickable, len(d.options)-len(d.itemBtns))...)
	}
}

// Value returns the current value.
func (d *Dropdown) Value() string { return d.value }

// Layout draws the label + dropdown trigger. Set dirty=true to append a '*' marker.
func (d *Dropdown) Layout(gtx layout.Context, dirty bool) layout.Dimensions {
	colors := theme.Current().Colors()

	if d.trigger.Clicked(gtx) && d.dialog != nil && !d.dialog.Visible() {
		d.showDialog()
	}

	label := d.label
	btnLabel := d.formatValue(d.value)
	if dirty {
		label += " *"
		btnLabel += " *"
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(d.th, label).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return VSpace(gtx, unit.Dp(4))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.layoutTrigger(gtx, btnLabel, colors)
		}),
	)
}

func (d *Dropdown) filteredIndices() []int {
	query := strings.ToLower(strings.TrimSpace(d.searchText))
	if query == "" {
		indices := make([]int, len(d.options))
		for i := range d.options {
			indices[i] = i
		}
		return indices
	}
	indices := make([]int, 0, len(d.options))
	for i, opt := range d.options {
		if strings.Contains(strings.ToLower(d.formatValue(opt)), query) {
			indices = append(indices, i)
		}
	}
	return indices
}

func (d *Dropdown) totalPages() int {
	n := len(d.filteredIndices())
	if n == 0 {
		return 1
	}
	return (n-1)/d.pageSize + 1
}

func (d *Dropdown) paginatedIndices() []int {
	all := d.filteredIndices()
	if d.pageSize <= 0 || len(all) <= d.pageSize {
		return all
	}
	start := d.page * d.pageSize
	if start > len(all) {
		start = 0
		if d.page > 0 {
			d.page = 0
		}
	}
	end := start + d.pageSize
	if end > len(all) {
		end = len(all)
	}
	return all[start:end]
}

func (d *Dropdown) layoutTrigger(gtx layout.Context, label string, colors theme.Palette) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return material.Clickable(gtx, &d.trigger, func(gtx layout.Context) layout.Dimensions {
		return BorderedCard(gtx, colors.InputBorder, colors.InputBg, unit.Dp(1), unit.Dp(4), unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return material.Body1(d.th, label).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return icons.NavigationArrowDropDown.Layout(gtx, colors.Fg)
				}),
			)
		})
	})
}

func (d *Dropdown) showDialog() {
	d.searchText = ""
	d.searchEd.SetText("")
	d.page = 0
	d.resizeItemBtns()

	title := d.label
	if title == "" {
		title = "Select"
	}
	d.dialog.Show(CustomNoButtons(title, func(gtx layout.Context) layout.Dimensions {
		return d.layoutDialog(gtx)
	}))
}

func (d *Dropdown) layoutDialog(gtx layout.Context) layout.Dimensions {
	colors := theme.Current().Colors()

	if text := d.searchEd.Text(); text != d.searchText {
		d.searchText = text
		d.page = 0
	}
	indices := d.paginatedIndices()
	for _, idx := range indices {
		if idx < len(d.itemBtns) && d.itemBtns[idx].Clicked(gtx) {
			d.value = d.options[idx]
			d.searchText = ""
			d.searchEd.SetText("")
			d.page = 0
			d.dialog.Hide()
			if d.onChange != nil {
				d.onChange(d.options[idx])
			}
		}
	}
	if d.prevBtn.Clicked(gtx) && d.page > 0 {
		d.page--
	}
	if d.nextBtn.Clicked(gtx) && d.page < d.totalPages()-1 {
		d.page++
	}

	showSearch := d.searchThreshold >= 0 && len(d.options) > d.searchThreshold
	showPagination := len(d.filteredIndices()) > d.pageSize

	children := make([]layout.FlexChild, 0, len(indices)+2)
	if showSearch {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Editor(d.th, &d.searchEd, d.searchPlaceholder).Layout(gtx)
			})
		}))
	}
	if len(indices) == 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Body2(d.th, "No results").Layout(gtx)
			})
		}))
	} else {
		for _, idx := range indices {
			idx := idx
			opt := d.options[idx]
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Clickable(gtx, &d.itemBtns[idx], func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					selected := opt == d.value
					hovered := d.itemBtns[idx].Hovered() || d.itemBtns[idx].Pressed()
					bg := colors.Surface
					if selected {
						bg = colors.Primary
					}
					if hovered {
						bg = colors.Hover
					}
					return BorderedCard(gtx, color.NRGBA{}, bg, unit.Dp(0), unit.Dp(0), unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(d.th, d.formatValue(opt))
						if selected && !hovered {
							lbl.Color = colors.OnPrimary
						}
						return lbl.Layout(gtx)
					})
				})
			}))
		}
	}
	if showPagination {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.layoutPagination(gtx, colors)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (d *Dropdown) layoutPagination(gtx layout.Context, colors theme.Palette) layout.Dimensions {
	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Button(d.th, &d.prevBtn, d.prevLabel).Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				text := fmt.Sprintf("%d / %d", d.page+1, d.totalPages())
				return material.Body2(d.th, text).Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Button(d.th, &d.nextBtn, d.nextLabel).Layout(gtx)
			}),
		)
	})
}
