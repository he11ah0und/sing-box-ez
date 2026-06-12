package app

import (
	"embed"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework"
	"sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/logger"
	"sing-box-ez/internal/framework/updater"
)

//go:embed locales/*.yaml
var localesFS embed.FS

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
				// App self-updater
				appMgr := updater.NewManager(log.Root, "updater")
				appGh := updater.NewGitHubBackend("he11ah0und", "sing-box-ez")
				appGh.Log = appMgr.Log.Allocate("github")
				appMgr.Source = appGh
				appMgr.Apply = &updater.SelfUpdateApply{}

				// Core updater (downloads sing-box core binary via FS)
				coreMgr := updater.NewManager(log.Root, "core-updater")
				coreGh := updater.NewGitHubBackend("SagerNet", "sing-box")
				coreGh.Log = coreMgr.Log.Allocate("github-core")
				coreMgr.Source = coreGh
				coreMgr.Apply = &updater.CoreUpdateApply{
					FS:       fsys,
					DestPath: core.CoreBinary(),
				}

				return []*updater.Manager{appMgr, coreMgr}
			},
		}),
		Config: cfg,
	}
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
