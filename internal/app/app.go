// Package app wires framework services into the concrete sing-box-ez application.
package app

import (
	"embed"
	"fmt"
	"runtime"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework"
	"sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/logger"
	"sing-box-ez/internal/framework/updater"
)

//go:embed locales/*.yaml
var localesFS embed.FS

//go:embed installers/*.lua
var installersFS embed.FS

// App is the concrete sing-box-ez application. It extends framework.App with
// the loaded configuration, core controller, and updater references.
type App struct {
	*framework.App
	Config      *config.AppConfig
	Controller  *core.Controller
	SelfUpdater *updater.Manager
	CoreUpdater *updater.Manager
}

// NewApp creates a new core App from the loaded configuration.
func NewApp(cfg *config.AppConfig) *App {
	if cfg == nil {
		return nil
	}

	fwApp := framework.NewApp(framework.Config{
		LoggerLimit: cfg.GetLogLimit(),
		BaseDir:     cfg.DataDir,
		LoadLocales: func(load func(fsys fs.FileSystem, dir string) error) error {
			return load(&fs.EmbedFS{FS: localesFS}, "locales")
		},
		BuildUpdaters: func(log *logger.Logger, fsys fs.FileSystem) []*updater.Manager {
			// App self-updater (asset is a raw binary).
			appMgr := updater.NewManager(log.Root, "updater")
			appMgr.Source = updater.NewGitHubBackend(appMgr.Log, "he11ah0und", "sing-box-ez")
			appMgr.Apply = updater.NewSelfUpdateApply(appMgr.Log, fsys)

			// Core updater (downloads sing-box core release archive).
			coreMgr := updater.NewManager(log.Root, "core-updater")
			coreMgr.Source = updater.NewGitHubBackend(coreMgr.Log, "SagerNet", "sing-box")
			coreMgr.AssetCriteria = updater.AssetCriteria{Tags: []string{runtime.GOARCH, runtime.GOOS}}
			coreApply := updater.NewFilesUpdateApply(coreMgr.Log, fsys)
			coreApply.BaseDir = cfg.DataDir
			coreApply.InstallScript = loadInstallScript("core.lua")
			coreMgr.Apply = coreApply

			return []*updater.Manager{appMgr, coreMgr}
		},
	})

	_ = fwApp.FS.MkdirAll("configs", 0750)

	return &App{
		App:         fwApp,
		Config:      cfg,
		Controller:  core.NewController(cfg, fwApp, fwApp.Logger.Root),
		SelfUpdater: findUpdater(fwApp.Updaters, "updater"),
		CoreUpdater: findUpdater(fwApp.Updaters, "core-updater"),
	}
}

func loadInstallScript(name string) []byte {
	data, err := installersFS.ReadFile("installers/" + name)
	if err != nil {
		panic(fmt.Sprintf("failed to load installer script %q: %v", name, err))
	}
	return data
}

func findUpdater(managers []*updater.Manager, name string) *updater.Manager {
	for _, m := range managers {
		if m.Name == name {
			return m
		}
	}
	return nil
}

// Start runs core-specific startup followed by framework startup.
func (a *App) Start() error {
	return a.App.Start()
}

// Stop runs core-specific shutdown followed by framework shutdown.
func (a *App) Stop() error {
	return a.App.Stop()
}
