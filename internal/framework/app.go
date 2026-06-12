// Package framework provides application-level services shared by the core
// business logic and the UI layers.
package framework

import (
	"errors"
	"os/exec"
	"runtime"

	"sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/framework/logger"
	"sing-box-ez/internal/framework/updater"
)

// App is the framework-level application container. It owns cross-cutting
// services such as logging, file-system access, and update management.
// Core code can embed *framework.App and extend Start()/Stop() to add
// domain-specific lifecycle.
type App struct {
	Logger   *logger.Logger
	FS       fs.FileSystem
	Updaters []*updater.Manager
	BaseDir  string
}

// Config is used to construct a framework App.
type Config struct {
	LoggerLimit int
	// BaseDir is the root directory for the file system. If empty the current
	// working directory is used.
	BaseDir string
	// LoadLocales is called during app construction to load localization
	// bundles. The framework passes the localengine.LoadFromFS loader so the
	// implementation can load locale files from any backend (e.g. embed.FS,
	// OS directory, or a custom fs.FileSystem). It may call the loader
	// multiple times for different sources.
	// If nil, localengine is only initialised with a logger.
	LoadLocales func(load func(fsys fs.FileSystem, dir string) error) error
	// BuildUpdaters returns the list of updater managers that should be
	// registered in the app. The framework passes the root logger and the
	// file system so each manager can allocate its own scoped terminal and
	// perform file operations safely. If nil, no updaters are registered.
	BuildUpdaters func(log *logger.Logger, fsys fs.FileSystem) []*updater.Manager
}

// NewApp creates a new framework App with the given configuration.
func NewApp(cfg Config) *App {
	if cfg.LoggerLimit <= 0 {
		cfg.LoggerLimit = 1000
	}
	if cfg.BaseDir == "" {
		cfg.BaseDir = "."
	}

	log := logger.NewLogger(cfg.LoggerLimit)

	localengine.SetLogger(log.Root)
	if cfg.LoadLocales != nil {
		_ = cfg.LoadLocales(localengine.LoadFromFS)
	}
	appFS := fs.NewOSFileSystem(cfg.BaseDir)

	var updaters []*updater.Manager
	if cfg.BuildUpdaters != nil {
		updaters = cfg.BuildUpdaters(log, appFS)
	}

	// Register the first updater with an Apply backend as the default manager
	// for package-level helpers.
	for _, mgr := range updaters {
		if mgr.Apply != nil {
			updater.SetManager(mgr)
			break
		}
	}

	return &App{
		Logger:   log,
		FS:       appFS,
		Updaters: updaters,
		BaseDir:  cfg.BaseDir,
	}
}

// Start starts framework-level services. It is safe to call even when core
// code overrides Start() on a wrapper type.
func (a *App) Start() error {
	if a == nil {
		return errors.New("framework.App is nil")
	}
	_ = a.FS.MkdirAll("", 0750)
	a.Logger.Root.Infof("framework started")
	return nil
}

// Stop stops framework-level services.
func (a *App) Stop() error {
	if a == nil {
		return errors.New("framework.App is nil")
	}
	a.Logger.Root.Infof("framework stopped")
	return nil
}

// OpenDataDir opens the app's base directory in the system file manager.
func (a *App) OpenDataDir() error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", a.BaseDir).Start()
	case "darwin":
		return exec.Command("open", a.BaseDir).Start()
	default:
		return exec.Command("xdg-open", a.BaseDir).Start()
	}
}

// Base returns the embedded framework App for wrapper types.
func (a *App) Base() *App { return a }
