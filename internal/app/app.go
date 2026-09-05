// Package app wires framework services into the concrete sing-box-ez application.
package app

import (
	"context"
	"embed"
	"fmt"
	iofs "io/fs"
	"os"
	"os/signal"
	"runtime/debug"

	fwconfig "github.com/he11ah0und/config"
	"sing-box-ez/internal/cli"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework"
	fwcli "sing-box-ez/internal/framework/cli"
	"sing-box-ez/internal/framework/rpc"
	"sing-box-ez/internal/framework/updater"
)

//go:embed locales/*.yaml
var localesFS embed.FS

//go:embed locales/logs/*.yaml
var logsLocalesFS embed.FS

//go:embed installers/*.lua
var installersFS embed.FS

//go:embed project.yaml
var projectYAML []byte

// App is the concrete sing-box-ez application. It extends framework.App with
// the loaded configuration, core controller, and updater references.
type App struct {
	*framework.App
	Profiles          *config.Profiles
	Controller        *core.Controller
	Backend           rpc.Backend
	SelfUpdater       *updater.Manager
	CoreUpdater       *updater.Manager
	host              string
	StartCoreOnLaunch bool
	runGUI            func(*App) bool
}

// New creates a new sing-box-ez App from command-line arguments.
func New(args []string, runGUI func(*App) bool) (*App, error) {
	fwApp, err := framework.NewApp(framework.Config{
		Args:           args,
		DefaultDataDir: func() string { return framework.DefaultDataDir("sing-box-ez") },
		LoadConfig:     config.Load,
		GetLoggerLimit: func(conf fwconfig.Config) int {
			cfg := conf.(*config.AppConfig)
			return cfg.Int("log", "limit")
		},
		LoadLocales: func(load func(fsys iofs.FS) error) error {
			sub, err := iofs.Sub(localesFS, "locales")
			if err != nil {
				return err
			}
			return load(sub)
		},
		LoadLogsLocales: func(load func(fsys iofs.FS) error) error {
			sub, err := iofs.Sub(logsLocalesFS, "locales/logs")
			if err != nil {
				return err
			}
			return load(sub)
		},
		ProjectSpec:       projectYAML,
		LoadInstallScript: loadInstallScript,
		RegisterCommands:  cli.RegisterCommands,
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
			{
				Name: "start-core",
				Desc: "Start the sing-box core automatically when the program launches",
				Type: fwcli.Bool,
			},
		},
	})
	if err != nil {
		return nil, err
	}

	// Log the commit the binary was built from. The framework has already
	// installed the logs resolver, so this keyed call resolves here.
	fwApp.Logger.Root.TInfof("app.started", buildCommit())

	cfg := fwApp.Config.(*config.AppConfig)

	// Pick the core source mode on a fresh install: a compatible system
	// sing-box wins, otherwise the managed download. Manual "custom" is only
	// ever chosen by the user.
	if cfg.MustGet("core", "source", "mode").String() == "" {
		mode := core.DetectCoreSourceMode(cfg)
		_ = cfg.MustGet("core", "source", "mode").Update(mode)
		_ = cfg.Save()
		fwApp.Logger.Root.TInfof("app.core_source_detected", mode)
	}

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
	if v, ok := fwApp.CLI.GlobalValue("start-core"); ok {
		app.StartCoreOnLaunch = fwcli.AsBool(v)
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
			_ = a.Logger.Root.TErrorf("rpc.invalid_host_address", err)
			os.Exit(1)
		}
		registry := rpc.NewRegistry()
		a.registerRPC(registry)
		server := rpc.NewServer(registry, transport)
		a.Logger.Root.TInfof("rpc.server_listening", transport.Addr())

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		if err := server.Run(ctx); err != nil {
			_ = a.Logger.Root.TErrorf("rpc.server_error", err)
			os.Exit(1)
		}
		return
	}

	a.App.Run()
}

// buildCommit returns the VCS revision the binary was built from, with a
// "-dirty" suffix for a modified working tree, or "unknown" when the build
// info carries no VCS settings (e.g. installed via `go install` from a
// module proxy without VCS stamping).
func buildCommit() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	rev, dirty := "", false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return "unknown"
	}
	if dirty {
		rev += "-dirty"
	}
	return rev
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
