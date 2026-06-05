//go:build !noplugins

package plugins

import (
	"time"

	lua "github.com/yuin/gopher-lua"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
)

// configModuleDef returns the API definition for the config module.
func configModuleDef() LuaModuleDef {
	return LuaModuleDef{
		Name: "config",
		Desc: "Config manager (plugin-scoped). Plugins can only see and manage configs they created.",
		Funcs: []LuaFuncDef{
			{
				Name: "list",
				Sig:  "config.list() -> table",
				Desc: "Returns a table of all configs owned by this plugin.",
				Returns: []LuaArgDef{
					{Name: "configs", Type: "table", Desc: "Array of config records. Each record has fields: name (string), url (string), interval (number), last_update (string), next_update (string)."},
				},
				Example: `local configs = config.list()
for i, c in ipairs(configs) do
  log.info(c.name .. " -> " .. c.url)
end`,
			},
			{
				Name: "add",
				Sig:  "config.add(name, url, interval) -> err",
				Desc: "Creates a new config owned by this plugin. Name must be unique across all configs.",
				Args: []LuaArgDef{
					{Name: "name", Type: "string", Desc: "Unique config identifier. Must not collide with existing configs."},
					{Name: "url", Type: "string", Desc: "Subscription URL to download the sing-box config from."},
					{Name: "interval", Type: "number", Desc: "Auto-update interval in hours. Must be > 0. Defaults to 24 if invalid."},
				},
				Returns: []LuaArgDef{
					{Name: "err", Type: "string|nil", Desc: "Error message if the name already exists or save fails, nil on success."},
				},
				Example: `local err = config.add("my-sub", "https://example.com/sub", 12)
if err then
  log.error("add failed: " .. err)
end`,
			},
			{
				Name: "update",
				Sig:  "config.update(name, url, interval) -> err",
				Desc: "Updates an existing config owned by this plugin.",
				Args: []LuaArgDef{
					{Name: "name", Type: "string", Desc: "Name of the config to update. Must be owned by this plugin."},
					{Name: "url", Type: "string", Desc: "New subscription URL."},
					{Name: "interval", Type: "number", Desc: "New auto-update interval in hours."},
				},
				Returns: []LuaArgDef{
					{Name: "err", Type: "string|nil", Desc: "Error message if not found or not owned, nil on success."},
				},
				Example: `local err = config.update("my-sub", "https://example.com/sub", 24)`,
			},
			{
				Name: "remove",
				Sig:  "config.remove(name) -> err",
				Desc: "Removes a config owned by this plugin.",
				Args: []LuaArgDef{
					{Name: "name", Type: "string", Desc: "Name of the config to remove. Must be owned by this plugin."},
				},
				Returns: []LuaArgDef{
					{Name: "err", Type: "string|nil", Desc: "Error message if not found or not owned, nil on success."},
				},
				Example: `local err = config.remove("my-sub")`,
			},
			{
				Name: "set_active",
				Sig:  "config.set_active(name) -> err",
				Desc: "Sets a plugin-owned config as the active config.",
				Args: []LuaArgDef{
					{Name: "name", Type: "string", Desc: "Name of the config to activate. Must be owned by this plugin."},
				},
				Returns: []LuaArgDef{
					{Name: "err", Type: "string|nil", Desc: "Error message if not found or not owned, nil on success."},
				},
				Example: `local err = config.set_active("my-sub")`,
			},
			{
				Name: "get_active",
				Sig:  "config.get_active() -> config, err",
				Desc: "Returns the active config only if it is owned by this plugin.",
				Returns: []LuaArgDef{
					{Name: "config", Type: "table|nil", Desc: "Config record (name, url, interval, last_update, next_update) if owned by this plugin, nil otherwise."},
					{Name: "err", Type: "string|nil", Desc: "Error message if no active config or active config belongs to another plugin, nil on success."},
				},
				Example: `local cfg, err = config.get_active()
if cfg then
  log.info("active: " .. cfg.name)
end`,
			},
			{
				Name: "download",
				Sig:  "config.download(name) -> err",
				Desc: "Downloads and caches the config from its URL.",
				Args: []LuaArgDef{
					{Name: "name", Type: "string", Desc: "Name of the config to download. Must be owned by this plugin and have a URL set."},
				},
				Returns: []LuaArgDef{
					{Name: "err", Type: "string|nil", Desc: "Error message if download fails, nil on success."},
				},
				Example: `local err = config.download("my-sub")
if err then
  log.error("download failed: " .. err)
else
  log.info("config cached")
end`,
			},
		},
	}
}

// registerConfig registers the config module into the Lua state.
func registerConfig(L *lua.LState, cfg *config.AppConfig, pluginName string) {
	parent := "pl-" + pluginName
	mod := L.NewTable()

	// config.list() -> table
	L.SetField(mod, "list", L.NewFunction(func(L *lua.LState) int {
		recs := cfg.GetConfigsByParent(parent)
		tbl := L.NewTable()
		for i, rec := range recs {
			item := L.NewTable()
			L.SetField(item, "name", lua.LString(rec.Name))
			L.SetField(item, "url", lua.LString(rec.URL))
			L.SetField(item, "interval", lua.LNumber(rec.UpdateIntervalHours))
			L.SetField(item, "last_update", lua.LString(rec.LastUpdate.Format(time.RFC3339)))
			L.SetField(item, "next_update", lua.LString(rec.NextUpdate().Format(time.RFC3339)))
			tbl.RawSetInt(i+1, item)
		}
		L.Push(tbl)
		return 1
	}))

	// config.add(name, url, interval) -> err or nil
	L.SetField(mod, "add", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		url := L.CheckString(2)
		interval := L.CheckInt(3)
		if interval <= 0 {
			interval = 24
		}

		if cfg.GetConfigByName(name) != nil {
			L.Push(lua.LString("config name already exists"))
			return 1
		}

		rec := config.ConfigRecord{
			Name:                name,
			URL:                 url,
			UpdateIntervalHours: interval,
			Parent:              parent,
		}
		cfg.AddConfig(rec)
		if err := cfg.Save(); err != nil {
			L.Push(lua.LString(err.Error()))
			return 1
		}
		L.Push(lua.LNil)
		return 1
	}))

	// config.update(name, url, interval) -> err or nil
	L.SetField(mod, "update", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		url := L.CheckString(2)
		interval := L.CheckInt(3)
		if interval <= 0 {
			interval = 24
		}

		rec := cfg.GetConfigByNameAndParent(name, parent)
		if rec == nil {
			L.Push(lua.LString("config not found or not owned by this plugin"))
			return 1
		}

		updated := config.ConfigRecord{
			Name:                name,
			URL:                 url,
			UpdateIntervalHours: interval,
			LastUpdate:          rec.LastUpdate,
			Parent:              parent,
		}
		cfg.UpdateConfig(name, updated)
		if err := cfg.Save(); err != nil {
			L.Push(lua.LString(err.Error()))
			return 1
		}
		L.Push(lua.LNil)
		return 1
	}))

	// config.remove(name) -> err or nil
	L.SetField(mod, "remove", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		if cfg.GetConfigByNameAndParent(name, parent) == nil {
			L.Push(lua.LString("config not found or not owned by this plugin"))
			return 1
		}
		cfg.RemoveConfig(name)
		if err := cfg.Save(); err != nil {
			L.Push(lua.LString(err.Error()))
			return 1
		}
		L.Push(lua.LNil)
		return 1
	}))

	// config.set_active(name) -> err or nil
	L.SetField(mod, "set_active", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		if cfg.GetConfigByNameAndParent(name, parent) == nil {
			L.Push(lua.LString("config not found or not owned by this plugin"))
			return 1
		}
		cfg.SetActiveName(name)
		if err := cfg.Save(); err != nil {
			L.Push(lua.LString(err.Error()))
			return 1
		}
		L.Push(lua.LNil)
		return 1
	}))

	// config.get_active() -> config record or nil, err
	L.SetField(mod, "get_active", L.NewFunction(func(L *lua.LState) int {
		rec := cfg.GetActiveConfig()
		if rec == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("no active config"))
			return 2
		}
		if rec.Parent != parent {
			L.Push(lua.LNil)
			L.Push(lua.LString("active config is not owned by this plugin"))
			return 2
		}
		item := L.NewTable()
		L.SetField(item, "name", lua.LString(rec.Name))
		L.SetField(item, "url", lua.LString(rec.URL))
		L.SetField(item, "interval", lua.LNumber(rec.UpdateIntervalHours))
		L.SetField(item, "last_update", lua.LString(rec.LastUpdate.Format(time.RFC3339)))
		L.SetField(item, "next_update", lua.LString(rec.NextUpdate().Format(time.RFC3339)))
		L.Push(item)
		L.Push(lua.LNil)
		return 2
	}))

	// config.download(name) -> err or nil
	// Downloads the config from its URL and caches it.
	L.SetField(mod, "download", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		rec := cfg.GetConfigByNameAndParent(name, parent)
		if rec == nil {
			L.Push(lua.LString("config not found or not owned by this plugin"))
			return 1
		}
		if rec.URL == "" {
			L.Push(lua.LString("config has no URL"))
			return 1
		}
		if err := core.DownloadConfigFor(name, rec.URL); err != nil {
			L.Push(lua.LString(err.Error()))
			return 1
		}
		cfg.SetLastUpdateFor(name, time.Now())
		_ = cfg.Save()
		L.Push(lua.LNil)
		return 1
	}))

	L.SetGlobal("config", mod)
}
