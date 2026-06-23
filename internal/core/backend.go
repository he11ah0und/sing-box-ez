package core

import (
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework/logger"
)

// Backend is the unified abstraction used by GUI pages and InteractiveController.
// It is implemented by the local *Controller and by RemoteController.
type Backend interface {
	IsRunning() bool
	GetPID() int
	Start() error
	Stop() error
	Restart() error
	PrepareConfig() (*config.ConfigRecord, error)

	GetConfigs() []config.ConfigRecord
	GetActiveConfig() *config.ConfigRecord
	GetActiveName() string
	ActivateConfig(name string) error
	AddConfig(rec config.ConfigRecord) error
	EditConfig(oldName string, rec config.ConfigRecord) error
	DeleteConfig(name string) error
	UpdateConfigNow(name, url string) error
	UpdateAllConfigs(progress func(done, total int)) (int, int, error)
	HasCachedConfig(name string) bool

	GetInstalledCoreVersion() (string, error)
	GetLatestCoreVersion() (string, error)
	DownloadCoreWithProgress(progress func(downloaded, total int64)) (string, error)
	DownloadCore(progress ProgressFunc) (string, error)

	GetPrivilegeTabState() PrivilegeTabState
	RestartAsAdmin() error
	SetRunAsAdmin(checked bool) error
	ApplySetcap() error

	OpenDataDir() error
	SetLogLimit(v int)
	SetDefaultInterval(h int)
	SetAutoRestart(checked bool) error

	GetCoreLogLines() []string
	GetCoreLogCleanLines() []string
	GetLogLines() []string
	GetLogLinesAtLeast(minLevel logger.LogLevel) []string
	ClearCoreLogs()
	ClearLogs()

	Config() *config.AppConfig
	Terminal() *logger.LogTerminal
}

// Compile-time check that *Controller implements Backend.
var _ Backend = (*Controller)(nil)
