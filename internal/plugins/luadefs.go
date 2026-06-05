//go:build !noplugins

package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateLuaDefs writes EmmyLua/LuaCATS definition files for the current
// plugin API surface.  The generated folder can be added to VS Code's
// "Lua.workspace.library" setting so Sumneko's Lua Language Server gives
// IntelliSense, autocompletion and hover docs while editing plugin scripts.
func GenerateLuaDefs(outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	modules := GetAPIModules()

	// Write one definition file per module.
	for _, mod := range modules {
		def := generateModuleDef(mod)
		if err := os.WriteFile(filepath.Join(outDir, mod.Name+".lua"), []byte(def), 0644); err != nil {
			return err
		}
	}

	// globals.lua declares the global variables so the server knows they exist.
	var gb strings.Builder
	gb.WriteString("-- Auto-generated EmmyLua globals for sing-box-ez plugins.\n")
	gb.WriteString("-- Add this folder to VS Code setting:\n")
	fmt.Fprintf(&gb, "--   \"Lua.workspace.library\": [\"%s\"]\n", filepath.ToSlash(outDir))
	gb.WriteString("\n")
	for _, mod := range modules {
		fmt.Fprintf(&gb, "---@type %s\n%s = {}\n\n", mod.Name, mod.Name)
	}
	if err := os.WriteFile(filepath.Join(outDir, "globals.lua"), []byte(gb.String()), 0644); err != nil {
		return err
	}

	// Write a sample .luarc.json that a plugin author can copy into their
	// own project root so VS Code picks up the definitions automatically.
	luarc := "{\n" +
		"  \"$schema\": \"https://raw.githubusercontent.com/LuaLS/vscode-lua/master/setting/schema.json\",\n" +
		"  \"Lua.diagnostics.globals\": [],\n" +
		"  \"Lua.workspace.library\": [\n" +
		"    \"" + filepath.ToSlash(outDir) + "\"\n" +
		"  ],\n" +
		"  \"Lua.workspace.checkThirdParty\": false\n" +
		"}\n"
	if err := os.WriteFile(filepath.Join(outDir, ".luarc.json"), []byte(luarc), 0644); err != nil {
		return err
	}

	return nil
}

func generateModuleDef(mod LuaModuleDef) string {
	var b strings.Builder
	b.WriteString("---@meta\n")
	b.WriteString("\n")
	fmt.Fprintf(&b, "-- %s\n", mod.Desc)
	fmt.Fprintf(&b, "---@class %s\n", mod.Name)
	fmt.Fprintf(&b, "%s = {}\n\n", mod.Name)

	for _, fn := range mod.Funcs {
		// Description
		fmt.Fprintf(&b, "---%s\n", fn.Desc)
		// Parse signature for @param and @return
		params, returns := parseSignature(fn.Sig)
		for _, p := range params {
			fmt.Fprintf(&b, "---@param %s\n", p)
		}
		for _, r := range returns {
			fmt.Fprintf(&b, "---@return %s\n", r)
		}
		fmt.Fprintf(&b, "function %s(%s) end\n\n", fn.Sig[:strings.Index(fn.Sig, "(")], argNames(params))
	}

	return b.String()
}

// parseSignature extracts params and returns from a signature like:
//
//	http.get(url) -> body, err
//	config.add(name, url, interval) -> err
func parseSignature(sig string) (params []string, returns []string) {
	// Split on "->"
	parts := strings.SplitN(sig, "->", 2)
	left := strings.TrimSpace(parts[0])
	// Extract args from parentheses
	argStart := strings.Index(left, "(")
	argEnd := strings.LastIndex(left, ")")
	if argStart != -1 && argEnd != -1 && argEnd > argStart {
		argsStr := left[argStart+1 : argEnd]
		argsStr = strings.TrimSpace(argsStr)
		if argsStr != "" {
			for _, a := range strings.Split(argsStr, ",") {
				a = strings.TrimSpace(a)
				if a == "" {
					continue
				}
				// infer type from common names
				typ := inferType(a)
				params = append(params, fmt.Sprintf("%s %s", a, typ))
			}
		}
	}

	if len(parts) > 1 {
		rights := strings.TrimSpace(parts[1])
		for _, r := range strings.Split(rights, ",") {
			r = strings.TrimSpace(r)
			if r == "" || r == "nil" {
				continue
			}
			// infer type
			typ := inferType(r)
			returns = append(returns, fmt.Sprintf("%s %s", typ, r))
		}
	}
	return
}

// argNames extracts just the argument names from param specs for the function declaration.
func argNames(params []string) string {
	var names []string
	for _, p := range params {
		parts := strings.Fields(p)
		if len(parts) > 0 {
			names = append(names, parts[0])
		}
	}
	return strings.Join(names, ", ")
}

// inferType guesses an EmmyLua type from a Lua variable name.
func inferType(name string) string {
	switch name {
	case "url", "body", "text", "msg", "name", "placeholder", "s", "on_submitted",
		"callback", "fn", "err", "rec", "resp", "cfg":
		return "string"
	case "tab_id", "id", "interval", "i":
		return "number"
	case "configs", "tbl", "item", "arr":
		return "table"
	case "bool", "ok", "enabled":
		return "boolean"
	default:
		if strings.Contains(name, "table") {
			return "table"
		}
		return "any"
	}
}
