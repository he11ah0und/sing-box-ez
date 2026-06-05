package plugins

// LuaArgDef describes a single function argument.
type LuaArgDef struct {
	Name string // argument name, e.g. "url"
	Type string // Lua type, e.g. "string", "number", "function"
	Desc string // human-readable description of what this argument is for
}

// LuaFuncDef describes a single function in a Lua module.
type LuaFuncDef struct {
	Name    string      // e.g. "get"
	Sig     string      // full signature e.g. "http.get(url) -> body, err"
	Desc    string      // one-line description
	Args    []LuaArgDef // per-argument details
	Returns []LuaArgDef // return value details
	Example string      // Lua example code (may be empty)
}

// LuaModuleDef describes a Lua module exposed to plugins.
type LuaModuleDef struct {
	Name  string
	Desc  string
	Funcs []LuaFuncDef
}

// GetAPIModules returns the complete plugin API surface.
// This is the single source of truth for docs, templates, and validation.
// When you add a new Lua function, add it here and it will appear in
// generated docs and plugin templates automatically.
func GetAPIModules() []LuaModuleDef {
	return []LuaModuleDef{
		httpModuleDef(),
		logModuleDef(),
		uiModuleDef(),
		configModuleDef(),
	}
}
