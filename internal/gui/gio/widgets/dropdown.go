package widgets

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// Dropdown renders a label with a button underneath that opens a picker dialog.
type Dropdown struct {
	th          *material.Theme
	dialog      DialogProvider
	label       string
	value       string
	options     []string
	formatValue func(string) string
	onChange    func(string)

	btn widget.Clickable
}

// NewDropdown creates a dropdown widget.
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
	return &Dropdown{
		th:          th,
		dialog:      dialog,
		label:       label,
		value:       value,
		options:     options,
		formatValue: formatValue,
		onChange:    onChange,
	}
}

// SetValue updates the displayed value.
func (d *Dropdown) SetValue(v string) { d.value = v }

// SetLabel updates the label shown above the dropdown.
func (d *Dropdown) SetLabel(label string) { d.label = label }

// Value returns the current value.
func (d *Dropdown) Value() string { return d.value }

// Layout draws the label + button. Set dirty=true to append a '*' marker.
func (d *Dropdown) Layout(gtx layout.Context, dirty bool) layout.Dimensions {
	if d.btn.Clicked(gtx) {
		d.openPicker()
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
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return material.Button(d.th, &d.btn, btnLabel).Layout(gtx)
		}),
	)
}

func (d *Dropdown) openPicker() {
	options := d.options
	btns := make([]widget.Clickable, len(options))

	d.dialog.ShowCustom(d.label, func(gtx layout.Context) layout.Dimensions {
		for i := range options {
			if btns[i].Clicked(gtx) {
				d.dialog.HideCustom()
				d.value = options[i]
				if d.onChange != nil {
					d.onChange(options[i])
				}
			}
		}

		children := make([]layout.FlexChild, len(options))
		for i, opt := range options {
			idx := i
			label := d.formatValue(opt)
			if opt == d.value {
				label = "> " + label
			}
			children[i] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Button(d.th, &btns[idx], label).Layout(gtx)
			})
		}
		return DialogSpacedList(gtx, children...)
	})
}
