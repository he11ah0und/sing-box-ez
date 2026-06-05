//go:build !noplugins

package plugins

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// httpModuleDef returns the API definition for the http module.
func httpModuleDef() LuaModuleDef {
	return LuaModuleDef{
		Name: "http",
		Desc: "HTTP client for GET/POST requests.",
		Funcs: []LuaFuncDef{
			{
				Name: "get",
				Sig:  "http.get(url) -> body, err",
				Desc: "Performs a GET request. Returns body and nil on success, or nil and error string on failure.",
				Args: []LuaArgDef{
					{Name: "url", Type: "string", Desc: "The URL to request. Must include scheme (http:// or https://)."},
				},
				Returns: []LuaArgDef{
					{Name: "body", Type: "string", Desc: "Response body as raw string on success, nil on failure."},
					{Name: "err", Type: "string|nil", Desc: "Error message string on failure, nil on success."},
				},
				Example: `local body, err = http.get("https://api.ip.sb/ip")
if err then
  log.error("request failed: " .. err)
else
  log.info("ip: " .. body)
end`,
			},
			{
				Name: "post",
				Sig:  "http.post(url, body) -> body, err",
				Desc: "Performs a POST request with application/json content type.",
				Args: []LuaArgDef{
					{Name: "url", Type: "string", Desc: "The URL to post to. Must include scheme."},
					{Name: "body", Type: "string", Desc: "Request body as a raw string. Typically a JSON string."},
				},
				Returns: []LuaArgDef{
					{Name: "body", Type: "string", Desc: "Response body as raw string on success, nil on failure."},
					{Name: "err", Type: "string|nil", Desc: "Error message string on failure, nil on success."},
				},
				Example: `local resp, err = http.post("https://api.example.com/data", '{"key":"value"}')
if err then
  log.error("post failed: " .. err)
end`,
			},
		},
	}
}

// logModuleDef returns the API definition for the log module.
func logModuleDef() LuaModuleDef {
	return LuaModuleDef{
		Name: "log",
		Desc: "Logging sink into the application log.",
		Funcs: []LuaFuncDef{
			{
				Name: "info",
				Sig:  "log.info(msg)",
				Desc: "Logs an info-level message prefixed with [plugin].",
				Args: []LuaArgDef{
					{Name: "msg", Type: "string", Desc: "The message to log."},
				},
				Example: `log.info("plugin initialised")`,
			},
			{
				Name: "warn",
				Sig:  "log.warn(msg)",
				Desc: "Logs a warning-level message prefixed with [plugin:warn].",
				Args: []LuaArgDef{
					{Name: "msg", Type: "string", Desc: "The warning message to log."},
				},
				Example: `log.warn("deprecated API used")`,
			},
			{
				Name: "error",
				Sig:  "log.error(msg)",
				Desc: "Logs an error-level message prefixed with [plugin:error].",
				Args: []LuaArgDef{
					{Name: "msg", Type: "string", Desc: "The error message to log."},
				},
				Example: `log.error("something went wrong")`,
			},
		},
	}
}

// registerHTTP registers the http module into the Lua state.
func registerHTTP(L *lua.LState) {
	mod := L.NewTable()
	L.SetField(mod, "get", L.NewFunction(luaHTTPGet))
	L.SetField(mod, "post", L.NewFunction(luaHTTPPost))
	L.SetGlobal("http", mod)
}

func luaHTTPGet(L *lua.LState) int {
	url := L.CheckString(1)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	if resp.StatusCode != http.StatusOK {
		L.Push(lua.LNil)
		L.Push(lua.LString(fmt.Sprintf("HTTP %d", resp.StatusCode)))
		return 2
	}
	L.Push(lua.LString(body))
	L.Push(lua.LNil)
	return 2
}

func luaHTTPPost(L *lua.LState) int {
	url := L.CheckString(1)
	body := L.CheckString(2)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		L.Push(lua.LNil)
		L.Push(lua.LString(fmt.Sprintf("HTTP %d", resp.StatusCode)))
		return 2
	}
	L.Push(lua.LString(data))
	L.Push(lua.LNil)
	return 2
}

// registerLog registers the log module into the Lua state.
func registerLog(L *lua.LState, sink func(string)) {
	mod := L.NewTable()
	L.SetField(mod, "info", L.NewFunction(func(L *lua.LState) int {
		sink("[plugin] " + L.CheckString(1))
		return 0
	}))
	L.SetField(mod, "warn", L.NewFunction(func(L *lua.LState) int {
		sink("[plugin:warn] " + L.CheckString(1))
		return 0
	}))
	L.SetField(mod, "error", L.NewFunction(func(L *lua.LState) int {
		sink("[plugin:error] " + L.CheckString(1))
		return 0
	}))
	L.SetGlobal("log", mod)
}
