// Package app wires framework services into the concrete sing-box-ez application.
package app

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/signal"
	"runtime"

	"sing-box-ez/internal/cli"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework"
	fwcli "sing-box-ez/internal/framework/cli"
	fwconfig "sing-box-ez/internal/framework/config"
	"sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/rpc"
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
	Backend     rpc.Backend
	SelfUpdater *updater.Manager
	CoreUpdater *updater.Manager
	host        string
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
		ExtraGlobalFlags: []fwcli.Flag{
			{
				Name: "remote",
				Desc: "Execute commands on a remote daemon (tcp://host:port, unix:///path, npipe://name, or auto)",
				Type: fwcli.String,
			},
			{
				Name: "host",
				Desc: "Run as an RPC daemon on the given IPC address (tcp://host:port, unix:///path, npipe://name, or auto)",
				Type: fwcli.String,
			},
		},
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

	// Read global flags that influence operating mode.
	if v, ok := fwApp.CLI.GlobalValue("host"); ok {
		app.host = fwcli.AsString(v)
	}

	registry := rpc.NewRegistry()
	app.registerRPC(registry)

	if v, ok := fwApp.CLI.GlobalValue("remote"); ok {
		addr := fwcli.AsString(v)
		if addr != "" {
			transport, err := rpc.ParseAddress(addr)
			if err != nil {
				return nil, err
			}
			app.Backend = rpc.NewRemoteBackend(transport)
		}
	}
	if app.Backend == nil {
		app.Backend = rpc.NewLocalBackend(registry)
	}
	app.App.Backend = app.Backend

	return app, nil
}

// Run executes the CLI command, runs the RPC daemon with --host, or starts the GUI.
func (a *App) Run() {
	if a.host != "" {
		transport, err := rpc.ParseAddress(a.host)
		if err != nil {
			a.Logger.Root.Errorf("invalid --host address: %v", err)
			os.Exit(1)
		}
		registry := rpc.NewRegistry()
		a.registerRPC(registry)
		server := rpc.NewServer(registry, transport)
		a.Logger.Root.Infof("RPC server listening on %s", transport.Addr())

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		if err := server.Run(ctx); err != nil {
			a.Logger.Root.Errorf("RPC server error: %v", err)
			os.Exit(1)
		}
		return
	}

	a.App.Run()
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

	sheet.Register([]string{"plugins", "enabled"}, fwconfig.TypeBool, false, fwconfig.WithDisabled(true))
	sheet.Register([]string{"plugins", "developer"}, fwconfig.TypeBool, false, fwconfig.WithDisabled(true))

	sheet.Register([]string{"service", "backend"}, fwconfig.TypeString, "embedded")
	sheet.Register([]string{"service", "start_on_app_launch"}, fwconfig.TypeBool, false)
	sheet.Register([]string{"service", "stop_on_app_exit"}, fwconfig.TypeBool, true, fwconfig.WithDisabled(true))

	sheet.Register([]string{"remote", "default_transport"}, fwconfig.TypeString, "auto", fwconfig.WithDisabled(true))
	sheet.Register([]string{"remote", "last_tcp_address"}, fwconfig.TypeString, "", fwconfig.WithDisabled(true))
	sheet.Register([]string{"remote", "last_connection_mode"}, fwconfig.TypeString, "embedded", fwconfig.WithDisabled(true))
	sheet.Register([]string{"remote", "remember_connection_mode"}, fwconfig.TypeBool, true, fwconfig.WithDisabled(true))
	sheet.Register([]string{"remote", "last_passphrase"}, fwconfig.TypeString, "", fwconfig.WithDisabled(true))
}
