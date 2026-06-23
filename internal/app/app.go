// Package app wires framework services into the concrete sing-box-ez application.
package app

import (
	"embed"
	"fmt"
	"runtime"

	"sing-box-ez/internal/cli"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework"
	fwconfig "sing-box-ez/internal/framework/config"
	"sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/updater"
)

//go:embed locales/*.yaml
var localesFS embed.FS

//go:embed themes/*.yaml
var ThemesFS embed.FS

//go:embed installers/*.lua
var installersFS embed.FS

// App is the concrete sing-box-ez application. It extends framework.App with
// the loaded configuration, core controller, and updater references.
type App struct {
	*framework.App
	Profiles    *config.Profiles
	Controller  *core.Controller
	SelfUpdater *updater.Manager
	CoreUpdater *updater.Manager
	runGUI      func(*App) bool
}

// New creates a new sing-box-ez App from command-line arguments.
func New(args []string, runGUI func(*App) bool) (*App, error) {
	fwApp, err := framework.NewApp(framework.Config{
		Args:           args,
		DefaultDataDir: func() string { return framework.DefaultDataDir("sing-box-ez") },
		RegisterConfig: registerConfig,
		LoadConfig:     config.Load,
		GetLoggerLimit: func(conf fwconfig.Config) int {
			cfg := conf.(*config.AppConfig)
			return cfg.Int("log", "limit")
		},
		LoadLocales: func(load func(dir fs.Directory) error) error {
			return load(fs.Embed(localesFS).Root().Subdir("locales"))
		},
		BuildUpdaters:    buildUpdaters,
		RegisterCommands: cli.RegisterCommands,
	})
	if err != nil {
		return nil, err
	}

	cfg := fwApp.Config.(*config.AppConfig)

	app := &App{
		App:         fwApp,
		Profiles:    cfg.Profiles,
		Controller:  core.NewController(cfg, fwApp, fwApp.Logger.Root),
		SelfUpdater: findUpdater(fwApp.Updaters, "updater"),
		CoreUpdater: findUpdater(fwApp.Updaters, "core-updater"),
		runGUI:      runGUI,
	}
	app.App.SetRunGUI(func(_ *framework.App) bool {
		return app.runGUI(app)
	})

	return app, nil
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

func buildUpdaters(app *framework.App) []*updater.Manager {
	cfg := app.Config.(*config.AppConfig)
	log := app.Logger
	fsys := app.FS

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
}

// registerConfig defines the sing-box-ez configuration schema.
func registerConfig(sheet *fwconfig.Sheet) {
	sheet.Register([]string{"core", "auto_restart"}, fwconfig.TypeBool, true)
	sheet.Register([]string{"core", "watch_logs"}, fwconfig.TypeBool, true)

	sheet.Register([]string{"log", "level"}, fwconfig.TypeString, "info")
	sheet.Register([]string{"log", "limit"}, fwconfig.TypeInt, 100)

	sheet.Register([]string{"ui", "show_logs"}, fwconfig.TypeBool, false)
	sheet.Register([]string{"ui", "language"}, fwconfig.TypeString, "")
	sheet.Register([]string{"ui", "desktop_notifications"}, fwconfig.TypeBool, true)
	sheet.Register([]string{"ui", "theme"}, fwconfig.TypeString, "default")
	sheet.Register([]string{"ui", "theme_mode"}, fwconfig.TypeString, "system")

	sheet.Register([]string{"privileges", "run_as_admin"}, fwconfig.TypeBool, false)

	sheet.Register([]string{"updates", "auto_check_self"}, fwconfig.TypeBool, true)
	sheet.Register([]string{"updates", "auto_check_core"}, fwconfig.TypeBool, true)
	sheet.Register([]string{"updates", "auto_update_configs"}, fwconfig.TypeBool, true)
	sheet.Register([]string{"updates", "auto_update_configs_interval_hours"}, fwconfig.TypeInt, 1)
	sheet.Register([]string{"updates", "auto_restart_on_config_update"}, fwconfig.TypeBool, true)
	sheet.Register([]string{"updates", "background_update_check_interval_hours"}, fwconfig.TypeInt, 2)
	sheet.Register([]string{"updates", "default_interval_hours"}, fwconfig.TypeInt, 24)

	sheet.Register([]string{"plugins", "enabled"}, fwconfig.TypeBool, false)
	sheet.Register([]string{"plugins", "developer"}, fwconfig.TypeBool, false)
}
