//go:build !fyne && !noplugins

package plugins

// initUI is a no-op stub when the fyne backend is not used.
func (e *Engine) initUI() {}

// uiModuleDef returns an empty API definition for the ui module (no-op backend).
func uiModuleDef() LuaModuleDef {
	return LuaModuleDef{
		Name:  "ui",
		Desc:  "Widget creation is not available in this backend.",
		Funcs: []LuaFuncDef{},
	}
}
