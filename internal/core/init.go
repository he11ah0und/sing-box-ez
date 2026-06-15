package core

import (
	"sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/logger"
	"sing-box-ez/internal/framework/net"
	"sing-box-ez/internal/framework/updater"
)

// Global framework-backed services used by core helpers.
// These are set once during application startup by Init.
var (
	FS          fs.FileSystem
	Net         *net.Client
	Log         *logger.LogTerminal
	CoreUpdater *updater.Manager
)

// Init wires core package-level helpers to the framework services.
func Init(baseDir string, fsys fs.FileSystem, netClient *net.Client, logTerminal *logger.LogTerminal) {
	BaseDir = baseDir
	FS = fsys
	Net = netClient
	Log = logTerminal
}
