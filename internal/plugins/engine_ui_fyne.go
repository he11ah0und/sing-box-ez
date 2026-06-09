//go:build fyne && !noplugins

package plugins

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	lua "github.com/yuin/gopher-lua"
)

// initUI registers the ui module into Lua for the fyne backend.
func (e *Engine) initUI() {
	mod := e.L.NewTable()

	// tabRegistry holds the tab content VBox for each tab ID.
	tabRegistry := make(map[int]*fyne.Container)
	tabCounter := 0

	e.L.SetField(mod, "add_tab", e.L.NewFunction(func(L *lua.LState) int {
		title := L.CheckString(1)
		content := container.NewVBox()
		scroll := container.NewScroll(content)
		tab := container.NewTabItem(title, scroll)
		fyne.Do(func() {
			e.builder.tabs.Append(tab)
			e.builder.tabs.Select(tab)
		})
		tabCounter++
		id := tabCounter
		tabRegistry[id] = content
		L.Push(lua.LNumber(id))
		return 1
	}))

	e.L.SetField(mod, "add_label", e.L.NewFunction(func(L *lua.LState) int {
		id := int(L.CheckNumber(1))
		text := L.CheckString(2)
		box, ok := tabRegistry[id]
		if !ok {
			L.Push(lua.LString("invalid tab id"))
			return 1
		}
		fyne.Do(func() {
			box.Add(widget.NewLabel(text))
		})
		return 0
	}))

	e.L.SetField(mod, "add_button", e.L.NewFunction(func(L *lua.LState) int {
		id := int(L.CheckNumber(1))
		text := L.CheckString(2)
		fn := L.CheckFunction(3)
		box, ok := tabRegistry[id]
		if !ok {
			L.Push(lua.LString("invalid tab id"))
			return 1
		}
		fyne.Do(func() {
			btn := widget.NewButton(text, func() {
				L.Push(fn)
				if err := L.PCall(0, 0, nil); err != nil {
					e.sink("[plugin:ui-error] " + err.Error())
				}
			})
			box.Add(btn)
		})
		return 0
	}))

	e.L.SetField(mod, "add_entry", e.L.NewFunction(func(L *lua.LState) int {
		id := int(L.CheckNumber(1))
		placeholder := L.CheckString(2)
		fn := L.CheckFunction(3)
		box, ok := tabRegistry[id]
		if !ok {
			L.Push(lua.LString("invalid tab id"))
			return 1
		}
		fyne.Do(func() {
			entry := widget.NewEntry()
			entry.SetPlaceHolder(placeholder)
			entry.OnSubmitted = func(s string) {
				L.Push(fn)
				L.Push(lua.LString(s))
				if err := L.PCall(1, 0, nil); err != nil {
					e.sink("[plugin:ui-error] " + err.Error())
				}
			}
			box.Add(entry)
		})
		return 0
	}))

	e.L.SetGlobal("ui", mod)
}

// uiModuleDef returns the API definition for the ui module.
func uiModuleDef() LuaModuleDef {
	return LuaModuleDef{
		Name: "ui",
		Desc: "Widget creation for custom tabs.",
		Funcs: []LuaFuncDef{
			{
				Name: "add_tab",
				Sig:  "ui.add_tab(title) -> tab_id",
				Desc: "Creates a new tab with the given title and returns a numeric tab ID.",
				Args: []LuaArgDef{
					{Name: "title", Type: "string", Desc: "The tab title shown in the tab bar."},
				},
				Returns: []LuaArgDef{
					{Name: "tab_id", Type: "number", Desc: "Numeric ID used to reference this tab in other ui functions."},
				},
				Example: `local id = ui.add_tab("My Plugin")`,
			},
			{
				Name: "add_label",
				Sig:  "ui.add_label(tab_id, text)",
				Desc: "Adds a text label to the specified tab.",
				Args: []LuaArgDef{
					{Name: "tab_id", Type: "number", Desc: "The numeric tab ID returned by ui.add_tab()."},
					{Name: "text", Type: "string", Desc: "The label text to display."},
				},
				Example: `ui.add_label(id, "Hello from Lua!")`,
			},
			{
				Name: "add_button",
				Sig:  "ui.add_button(tab_id, text, callback)",
				Desc: "Adds a button to the specified tab.",
				Args: []LuaArgDef{
					{Name: "tab_id", Type: "number", Desc: "The numeric tab ID returned by ui.add_tab()."},
					{Name: "text", Type: "string", Desc: "The button label text."},
					{Name: "callback", Type: "function", Desc: "A Lua function with no arguments. Called when the button is clicked."},
				},
				Example: `ui.add_button(id, "Click me", function() log.info("clicked") end)`,
			},
			{
				Name: "add_entry",
				Sig:  "ui.add_entry(tab_id, placeholder, on_submitted)",
				Desc: "Adds a text entry (input field) to the specified tab.",
				Args: []LuaArgDef{
					{Name: "tab_id", Type: "number", Desc: "The numeric tab ID returned by ui.add_tab()."},
					{Name: "placeholder", Type: "string", Desc: "Placeholder text shown when the field is empty."},
					{Name: "on_submitted", Type: "function", Desc: "A Lua function(text) called when the user presses Enter. text is the entered string."},
				},
				Example: `ui.add_entry(id, "Type here...", function(text) log.info("entered: " .. text) end)`,
			},
		},
	}
}
