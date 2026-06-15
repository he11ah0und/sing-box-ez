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
	"sing-box-ez/internal/framework/net"
	"sing-box-ez/internal/framework/updater"
)

//go:embed locales/*.yaml
var localesFS embed.FS

//go:embed installers/*.lua
var installersFS embed.FS

// App is the core application container. It extends framework.App with the
// loaded application configuration and acts as the boundary between framework
// services and domain-specific lifecycle.
type App struct {
	*framework.App
	Config *config.AppConfig
}

// NewApp creates a new core App from the loaded configuration.
// localesFS is the embedded file system containing locale YAML files.
func NewApp(cfg *config.AppConfig) *App {
	if cfg == nil {
		return nil
	}
	return &App{
		App: framework.NewApp(framework.Config{
			LoggerLimit: cfg.GetLogLimit(),
			BaseDir:     cfg.DataDir,
			LoadLocales: func(load func(fsys fs.FileSystem, dir string) error) error {
				return load(&fs.EmbedFS{FS: localesFS}, "locales")
			},
			BuildUpdaters: func(log *logger.Logger, fsys fs.FileSystem) []*updater.Manager {
				// Wire core helpers to framework services.
				core.Init(cfg.DataDir, fsys, net.NewClient(log.Root), log.Root)

				// App self-updater (asset is a raw binary; Lua is not required).
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

				// Expose the core updater manager to core controllers.
				core.CoreUpdater = coreMgr

				return []*updater.Manager{appMgr, coreMgr}
			},
		}),
		Config: cfg,
	}
}

func loadInstallScript(name string) []byte {
	data, err := installersFS.ReadFile("installers/" + name)
	if err != nil {
		// Installer scripts are embedded at build time; failure here is a build bug.
		panic(fmt.Sprintf("failed to load installer script %q: %v", name, err))
	}
	return data
}

// Start runs core-specific startup followed by framework startup.
func (a *App) Start() error {
	// Core-specific startup hooks can be added here.
	return a.App.Start()
}

// Stop runs core-specific shutdown followed by framework shutdown.
func (a *App) Stop() error {
	// Core-specific shutdown hooks can be added here.
	return a.App.Stop()
}
