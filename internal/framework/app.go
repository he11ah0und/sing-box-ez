// Package framework provides application-level services shared by the core
// business logic and the UI layers.
package framework

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"sing-box-ez/internal/framework/cli"
	"sing-box-ez/internal/framework/config"
	"sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/localengine"
	"sing-box-ez/internal/framework/logger"
	"sing-box-ez/internal/framework/updater"
)

// App is the framework-level application container. It owns cross-cutting
// services such as logging, file-system access, CLI routing and update
// management.
type App struct {
	Logger   *logger.Logger
	FS       fs.FileSystem
	Updaters []*updater.Manager
	BaseDir  string
	Config   config.Config
	CLI      *cli.Engine[*App]

	RemainingArgs []string
	runGUI        func(*App) bool
}

// Config is used to construct a framework App.
type Config struct {
	// Args are the command-line arguments (without the program name).
	Args []string
	// DefaultDataDir returns the default data directory when --data-dir is not
	// provided. Required.
	DefaultDataDir func() string
	// RegisterConfig registers the configuration schema on a fresh Sheet.
	RegisterConfig func(*config.Sheet)
	// LoadConfig loads persisted settings into the Sheet and returns the
	// application-specific config object. Required.
	LoadConfig func(dataDir string, sheet *config.Sheet) (config.Config, error)
	// GetLoggerLimit extracts the log buffer limit from the loaded config.
	// If nil, a default limit is used.
	GetLoggerLimit func(config.Config) int
	// LoadLocales is called during app construction to load localization
	// bundles. The framework passes the localengine.LoadFromFS loader so the
	// implementation can load locale files from any backend (e.g. embed.FS,
	// OS directory, or a custom fs.FileSystem). It may call the loader
	// multiple times for different sources.
	// If nil, localengine is only initialised with a logger.
	LoadLocales func(load func(fsys fs.FileSystem, dir string) error) error
	// BuildUpdaters returns the list of updater managers that should be
	// registered in the app. The framework passes the constructed App so the
	// implementation can access the config, logger and file system. If nil,
	// no updaters are registered.
	BuildUpdaters func(*App) []*updater.Manager
	// RegisterCommands registers CLI commands on the engine.
	RegisterCommands func(*cli.Engine[*App])
	// RunGUI starts the GUI. It receives the fully constructed App and
	// returns true if the GUI ran successfully. If nil and no CLI command
	// was given, Run exits without error.
	RunGUI func(*App) bool
}

// parseDataDir extracts --data-dir from args and returns the directory plus
// remaining arguments.
func parseDataDir(args []string) (string, []string) {
	for i := range args {
		if args[i] == "--data-dir" && i+1 < len(args) {
			dir := args[i+1]
			remaining := append(args[:i], args[i+2:]...)
			return dir, remaining
		}
	}
	return "", args
}

// NewApp creates a new framework App with the given configuration.
func NewApp(cfg Config) (*App, error) {
	dataDir, remaining := parseDataDir(cfg.Args)
	if dataDir == "" {
		if cfg.DefaultDataDir == nil {
			return nil, fmt.Errorf("DefaultDataDir is required")
		}
		dataDir = cfg.DefaultDataDir()
	}
	_ = os.MkdirAll(dataDir, 0750)

	tmpLog := logger.NewLogger(1000)
	tmpFS := fs.NewOSFileSystemWithLog(tmpLog.Root, dataDir)
	_ = tmpFS.MkdirAll("configs", 0750)
	_ = tmpFS.MkdirAll("plugins", 0750)
	_ = tmpFS.MkdirAll("docs", 0750)

	sheet := config.NewSheet(config.SheetOptions{Logger: tmpLog.Root})
	if cfg.RegisterConfig != nil {
		cfg.RegisterConfig(sheet)
	}

	conf, err := cfg.LoadConfig(dataDir, sheet)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	limit := 1000
	if cfg.GetLoggerLimit != nil {
		limit = cfg.GetLoggerLimit(conf)
	}
	log := logger.NewLogger(limit)
	appFS := fs.NewOSFileSystemWithLog(log.Root, dataDir)

	localengine.SetLogger(log.Root)
	if cfg.LoadLocales != nil {
		_ = cfg.LoadLocales(localengine.LoadFromFS)
	}

	app := &App{
		Logger:        log,
		FS:            appFS,
		BaseDir:       dataDir,
		Config:        conf,
		CLI:           cli.New[*App](),
		RemainingArgs: remaining,
		runGUI:        cfg.RunGUI,
	}

	if cfg.BuildUpdaters != nil {
		app.Updaters = cfg.BuildUpdaters(app)
		for _, mgr := range app.Updaters {
			if mgr.Apply != nil {
				updater.SetManager(mgr)
				break
			}
		}
	}

	if cfg.RegisterCommands != nil {
		cfg.RegisterCommands(app.CLI)
	}

	return app, nil
}

// Run executes the CLI command if arguments remain, otherwise starts the GUI.
func (a *App) Run() {
	if len(a.RemainingArgs) > 0 {
		if err := a.CLI.Run(a.RemainingArgs, a); err != nil {
			// Errors are already logged by commands; exit with non-zero status.
			os.Exit(1)
		}
		return
	}

	if a.runGUI != nil && !a.runGUI(a) {
		// GUI could not start (no display, no wayland, etc).
		// Exit gracefully so make/run does not show a scary error.
		os.Exit(0)
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
