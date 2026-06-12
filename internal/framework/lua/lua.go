// Package lua provides a sandboxed Lua runtime for install scripts.
package lua

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"

	frameworkfs "sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/logger"
	"sing-box-ez/internal/framework/progress"
	"sing-box-ez/internal/framework/version"
)

// VM is a sandboxed Lua runtime for running install scripts.
type VM struct {
	L             *lua.LState
	Log           *logger.LogTerminal
	BaseDir       string
	MainFS        frameworkfs.FileSystem
	AssetFS       frameworkfs.FileSystem
	AllowedWrites []string
	Progress      *progress.Config
}

// AssetInfo describes the downloaded asset passed to the install script.
type AssetInfo struct {
	Path   string
	Format string
	Name   string
	Size   int64
}

// ReleaseInfo describes the release being installed.
type ReleaseInfo struct {
	Version     string
	Channel     string
	Body        string
	PublishedAt time.Time
}

// InstallContext provides runtime information to the install script.
type InstallContext struct {
	Asset   AssetInfo
	Release ReleaseInfo
}

// InstallResult is returned by an install script.
type InstallResult struct {
	ReplaceBinary string
	Restart       bool
}

// NewVM creates a new Lua VM with the given sandbox settings.
func NewVM(parent *logger.LogTerminal, baseDir string, mainFS frameworkfs.FileSystem, allowedWrites []string) *VM {
	if baseDir == "" {
		baseDir = "."
	}
	return &VM{
		L:             lua.NewState(),
		Log:           parent.Allocate("lua"),
		BaseDir:       baseDir,
		MainFS:        mainFS,
		AllowedWrites: allowedWrites,
	}
}

// Close releases the Lua VM.
func (vm *VM) Close() {
	vm.L.Close()
}

// Run executes the install script and returns the parsed result.
func (vm *VM) Run(script []byte, ctx InstallContext) (*InstallResult, error) {
	if err := vm.registerFS(ctx); err != nil {
		return nil, fmt.Errorf("register fs: %w", err)
	}
	vm.registerPlatform()
	vm.registerRelease(ctx.Release)
	vm.registerLog()

	if err := vm.L.DoString(string(script)); err != nil {
		return nil, vm.Log.Errorf("install script failed: %v", err)
	}

	return vm.parseResult()
}

func (vm *VM) registerFS(ctx InstallContext) error {
	L := vm.L

	// Main writable FS.
	fsTbl := L.NewTable()
	L.SetField(fsTbl, "read_file", L.NewFunction(vm.luaReadFile))
	L.SetField(fsTbl, "write_file", L.NewFunction(vm.luaWriteFile))
	L.SetField(fsTbl, "mkdir", L.NewFunction(vm.luaMkdir))
	L.SetField(fsTbl, "rename", L.NewFunction(vm.luaRename))
	L.SetField(fsTbl, "remove", L.NewFunction(vm.luaRemove))
	L.SetField(fsTbl, "exists", L.NewFunction(vm.luaExists))
	L.SetField(fsTbl, "list_dir", L.NewFunction(vm.luaListDir))
	L.SetField(fsTbl, "stat", L.NewFunction(vm.luaStat))
	L.SetField(fsTbl, "copy", L.NewFunction(vm.luaCopy))
	L.SetGlobal("fs", fsTbl)

	// Asset read-only FS.
	assetTbl := L.NewTable()
	L.SetField(assetTbl, "path", lua.LString(ctx.Asset.Path))
	L.SetField(assetTbl, "format", lua.LString(ctx.Asset.Format))
	L.SetField(assetTbl, "name", lua.LString(ctx.Asset.Name))
	L.SetField(assetTbl, "size", lua.LNumber(ctx.Asset.Size))

	if vm.AssetFS != nil {
		fsObj := L.NewUserData()
		fsObj.Value = vm.AssetFS
		assetFSTbl := L.NewTable()
		L.SetField(assetFSTbl, "_fs", fsObj)
		L.SetField(assetTbl, "fs", assetFSTbl)
	}
	L.SetGlobal("asset", assetTbl)

	return nil
}

func (vm *VM) registerPlatform() {
	L := vm.L
	tbl := L.NewTable()
	L.SetField(tbl, "os", lua.LString(version.BuildOS))
	L.SetField(tbl, "arch", lua.LString(version.BuildArch))
	L.SetField(tbl, "compiler", lua.LString(version.BuildCompiler))
	L.SetField(tbl, "gui", lua.LString(version.BuildGUI))
	L.SetField(tbl, "backend", lua.LString(version.BuildBackend))
	L.SetGlobal("platform", tbl)
}

func (vm *VM) registerRelease(r ReleaseInfo) {
	L := vm.L
	tbl := L.NewTable()
	L.SetField(tbl, "version", lua.LString(r.Version))
	L.SetField(tbl, "channel", lua.LString(r.Channel))
	L.SetField(tbl, "body", lua.LString(r.Body))
	L.SetField(tbl, "published_at", lua.LString(r.PublishedAt.Format(time.RFC3339)))
	L.SetGlobal("release", tbl)
}

func (vm *VM) registerLog() {
	L := vm.L
	tbl := L.NewTable()
	L.SetField(tbl, "info", L.NewFunction(vm.luaLogInfo))
	L.SetField(tbl, "warn", L.NewFunction(vm.luaLogWarn))
	L.SetField(tbl, "error", L.NewFunction(vm.luaLogError))
	L.SetGlobal("log", tbl)
}

func (vm *VM) parseResult() (*InstallResult, error) {
	L := vm.L
	result := L.Get(-1)
	if result.Type() != lua.LTTable {
		return &InstallResult{}, nil
	}
	tbl := result.(*lua.LTable)

	res := &InstallResult{}
	if v := L.GetField(tbl, "replace_binary"); v.Type() == lua.LTString {
		res.ReplaceBinary = string(v.(lua.LString))
	}
	if v := L.GetField(tbl, "restart"); v.Type() == lua.LTBool {
		res.Restart = bool(v.(lua.LBool))
	}
	return res, nil
}

// --- path sandbox helpers ---

func (vm *VM) resolvePath(path string) (string, error) {
	path = filepath.FromSlash(path)
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Abs(filepath.Join(vm.BaseDir, path))
}

func (vm *VM) isReadAllowed(abs string) bool {
	abs = filepath.Clean(abs)
	for _, allowed := range vm.AllowedWrites {
		if isUnderOrEqual(abs, allowed) {
			return true
		}
	}
	if isUnderOrEqual(abs, vm.BaseDir) {
		return true
	}
	return false
}

func (vm *VM) isWriteAllowed(abs string) bool {
	abs = filepath.Clean(abs)
	for _, allowed := range vm.AllowedWrites {
		if isUnderOrEqual(abs, allowed) {
			return true
		}
	}
	return false
}

func isUnderOrEqual(child, parent string) bool {
	child = filepath.Clean(child)
	parent = filepath.Clean(parent)
	if child == parent {
		return true
	}
	prefix := parent + string(filepath.Separator)
	return strings.HasPrefix(child, prefix)
}

func (vm *VM) checkRead(path string) (string, error) {
	abs, err := vm.resolvePath(path)
	if err != nil {
		return "", err
	}
	if !vm.isReadAllowed(abs) {
		return "", fmt.Errorf("read access denied: %s", path)
	}
	return abs, nil
}

func (vm *VM) checkWrite(path string) (string, error) {
	abs, err := vm.resolvePath(path)
	if err != nil {
		return "", err
	}
	if !vm.isWriteAllowed(abs) {
		return "", fmt.Errorf("write access denied: %s", path)
	}
	return abs, nil
}

// --- fs functions ---

func (vm *VM) luaReadFile(L *lua.LState) int {
	path := L.CheckString(1)
	abs, err := vm.checkRead(path)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	data, err := vm.MainFS.ReadFile(abs)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	L.Push(lua.LString(data))
	L.Push(lua.LNil)
	return 2
}

func (vm *VM) luaWriteFile(L *lua.LState) int {
	path := L.CheckString(1)
	data := L.CheckString(2)
	mode := os.FileMode(0640)
	if L.GetTop() >= 3 {
		mode = os.FileMode(L.CheckNumber(3))
	}

	abs, err := vm.checkWrite(path)
	if err != nil {
		L.Push(lua.LString(err.Error()))
		return 1
	}
	if err := vm.MainFS.MkdirAll(filepath.Dir(abs), 0750); err != nil {
		L.Push(lua.LString(err.Error()))
		return 1
	}
	if err := vm.MainFS.WriteFile(abs, []byte(data), mode); err != nil {
		L.Push(lua.LString(err.Error()))
		return 1
	}
	L.Push(lua.LNil)
	return 1
}

func (vm *VM) luaMkdir(L *lua.LState) int {
	path := L.CheckString(1)
	abs, err := vm.checkWrite(path)
	if err != nil {
		L.Push(lua.LString(err.Error()))
		return 1
	}
	if err := vm.MainFS.MkdirAll(abs, 0750); err != nil {
		L.Push(lua.LString(err.Error()))
		return 1
	}
	L.Push(lua.LNil)
	return 1
}

func (vm *VM) luaRename(L *lua.LState) int {
	oldPath := L.CheckString(1)
	newPath := L.CheckString(2)
	oldAbs, err := vm.checkWrite(oldPath)
	if err != nil {
		L.Push(lua.LString(err.Error()))
		return 1
	}
	newAbs, err := vm.checkWrite(newPath)
	if err != nil {
		L.Push(lua.LString(err.Error()))
		return 1
	}
	if err := vm.MainFS.Rename(oldAbs, newAbs); err != nil {
		L.Push(lua.LString(err.Error()))
		return 1
	}
	L.Push(lua.LNil)
	return 1
}

func (vm *VM) luaRemove(L *lua.LState) int {
	path := L.CheckString(1)
	abs, err := vm.checkWrite(path)
	if err != nil {
		L.Push(lua.LString(err.Error()))
		return 1
	}
	if err := vm.MainFS.Remove(abs); err != nil {
		L.Push(lua.LString(err.Error()))
		return 1
	}
	L.Push(lua.LNil)
	return 1
}

func (vm *VM) luaExists(L *lua.LState) int {
	path := L.CheckString(1)
	abs, err := vm.checkRead(path)
	if err != nil {
		L.Push(lua.LBool(false))
		return 1
	}
	L.Push(lua.LBool(vm.MainFS.Exists(abs)))
	return 1
}

func (vm *VM) luaListDir(L *lua.LState) int {
	path := L.CheckString(1)
	abs, err := vm.checkRead(path)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	entries, err := vm.MainFS.ReadDir(abs)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	tbl := L.NewTable()
	for _, e := range entries {
		info, _ := e.Info()
		item := L.NewTable()
		L.SetField(item, "name", lua.LString(e.Name()))
		L.SetField(item, "is_dir", lua.LBool(e.IsDir()))
		if info != nil {
			L.SetField(item, "size", lua.LNumber(info.Size()))
			L.SetField(item, "mode", lua.LNumber(info.Mode().Perm()))
		}
		L.RawSetInt(tbl, tbl.Len()+1, item)
	}
	L.Push(tbl)
	L.Push(lua.LNil)
	return 2
}

func (vm *VM) luaStat(L *lua.LState) int {
	path := L.CheckString(1)
	abs, err := vm.checkRead(path)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	info, err := vm.MainFS.Stat(abs)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}
	tbl := L.NewTable()
	L.SetField(tbl, "name", lua.LString(info.Name()))
	L.SetField(tbl, "size", lua.LNumber(info.Size()))
	L.SetField(tbl, "mode", lua.LNumber(info.Mode().Perm()))
	L.SetField(tbl, "is_dir", lua.LBool(info.IsDir()))
	L.SetField(tbl, "mod_time", lua.LString(info.ModTime().Format(time.RFC3339)))
	L.Push(tbl)
	L.Push(lua.LNil)
	return 2
}

func (vm *VM) luaCopy(L *lua.LState) int {
	var srcFS frameworkfs.FileSystem
	var srcPath string

	first := L.Get(1)
	switch first.Type() {
	case lua.LTString:
		srcFS = vm.MainFS
		srcPath = string(first.(lua.LString))
	case lua.LTTable:
		fsObj := L.GetField(first.(*lua.LTable), "_fs")
		if fsObj.Type() != lua.LTUserData {
			L.Push(lua.LString("copy source must be a path string or an fs object"))
			return 1
		}
		ud := fsObj.(*lua.LUserData)
		var ok bool
		srcFS, ok = ud.Value.(frameworkfs.FileSystem)
		if !ok {
			L.Push(lua.LString("invalid fs object"))
			return 1
		}
		srcPath = L.CheckString(2)
	default:
		L.Push(lua.LString("copy source must be a path string or an fs object"))
		return 1
	}

	dstPath := L.CheckString(L.GetTop()) // last positional argument
	// Options table is the argument before dst if there are 3 args.
	var opts frameworkfs.CopyOptions
	opts.Recursive = true
	opts.PreserveMode = true
	opts.Progress = vm.Progress

	if L.GetTop() >= 3 && first.Type() == lua.LTString {
		if tbl := L.Get(2); tbl.Type() == lua.LTTable {
			vm.parseCopyOpts(tbl.(*lua.LTable), &opts)
		}
	} else if L.GetTop() >= 4 && first.Type() == lua.LTTable {
		if tbl := L.Get(3); tbl.Type() == lua.LTTable {
			vm.parseCopyOpts(tbl.(*lua.LTable), &opts)
		}
	}

	dstAbs, err := vm.checkWrite(dstPath)
	if err != nil {
		L.Push(lua.LString(err.Error()))
		return 1
	}

	// Source path resolution: if srcFS is main FS, sandbox reads; asset FS is already sandboxed.
	if srcFS == vm.MainFS {
		if _, err := vm.checkRead(srcPath); err != nil {
			L.Push(lua.LString(err.Error()))
			return 1
		}
	}

	if err := frameworkfs.Copy(nil, srcFS, vm.MainFS, srcPath, dstAbs, opts); err != nil {
		L.Push(lua.LString(err.Error()))
		return 1
	}
	L.Push(lua.LNil)
	return 1
}

func (vm *VM) parseCopyOpts(tbl *lua.LTable, opts *frameworkfs.CopyOptions) {
	if v := tbl.RawGetString("recursive"); v.Type() == lua.LTBool {
		opts.Recursive = bool(v.(lua.LBool))
	}
	if v := tbl.RawGetString("preserve_mode"); v.Type() == lua.LTBool {
		opts.PreserveMode = bool(v.(lua.LBool))
	}
}

// --- log functions ---

func (vm *VM) luaLogInfo(L *lua.LState) int {
	vm.Log.Infof("%s", L.CheckString(1))
	return 0
}

func (vm *VM) luaLogWarn(L *lua.LState) int {
	vm.Log.Warnf("%s", L.CheckString(1))
	return 0
}

func (vm *VM) luaLogError(L *lua.LState) int {
	vm.Log.Errorf("%s", L.CheckString(1))
	return 0
}

// ParseBool interprets a Lua value as bool.
func ParseBool(v lua.LValue) bool {
	if v.Type() == lua.LTBool {
		return bool(v.(lua.LBool))
	}
	if v.Type() == lua.LTString {
		s := string(v.(lua.LString))
		b, _ := strconv.ParseBool(s)
		return b
	}
	return false
}
