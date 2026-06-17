//go:build !noplugins

package plugins

// initUI is a no-op stub; custom plugin UI is not implemented for the Gio backend.
func (e *Engine) initUI() {}

// uiModuleDef returns an empty API definition for the ui module.
func uiModuleDef() LuaModuleDef {
	return LuaModuleDef{
		Name:  "ui",
		Desc:  "Widget creation is not available in this backend.",
		Funcs: []LuaFuncDef{},
	}
}
