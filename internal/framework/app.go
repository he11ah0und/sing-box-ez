// Package framework provides application-level services shared by the core
// business logic and the UI layers.
package framework

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"os"
	"os/exec"
	"runtime"

	"sing-box-ez/internal/framework/cli"
	"sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/rpc"
	"sing-box-ez/internal/framework/updater"

	"github.com/he11ah0und/config"
	"github.com/he11ah0und/localengine"
	"github.com/he11ah0und/logger"
	"github.com/he11ah0und/projectspec"
)

// App is the framework-level application container. It owns cross-cutting
// services such as logging, file-system access, CLI routing and update
// management.
type App struct {
	Logger   *logger.Logger
	FS       fs.FS
	Updaters []*updater.Manager
	BaseDir  string
	Config   config.Config
	CLI      *cli.Engine[*App]
	Backend  rpc.Backend
	// Spec is the parsed declarative project spec (nil when Config.ProjectSpec
	// was not provided). UI layers read config entry metadata (control hints,
	// options, min/max) from it.
	Spec *projectspec.Spec

	// Root is the data directory. ConfigsDir/PluginsDir/DocsDir are
	// pre-created subdirectories with enforced permissions.
	Root       fs.Directory
	ConfigsDir fs.Directory
	PluginsDir fs.Directory
	DocsDir    fs.Directory

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
	// It is used only as a fallback when ProjectSpec declares no config
	// section; spec entries take precedence.
	RegisterConfig func(*config.Sheet)
	// LoadConfig loads persisted settings into the Sheet and returns the
	// application-specific config object. Required.
	LoadConfig func(root fs.Directory, dataDir string, sheet *config.Sheet) (config.Config, error)
	// GetLoggerLimit extracts the log buffer limit from the loaded config.
	// If nil, a default limit is used.
	GetLoggerLimit func(config.Config) int
	// LoadLocales is called during app construction to load localization
	// bundles. The framework passes the localengine.LoadFromDir loader so the
	// implementation can load locale files from any fs.FS source (e.g.
	// embed.FS, os.DirFS, or a subdirectory via fs.Sub). It may call the
	// loader multiple times for different sources.
	// If nil, localengine is only initialised with a logger.
	LoadLocales func(load func(fsys iofs.FS) error) error
	// LoadLogsLocales is called during app construction to load logs-domain
	// locales (key-based log message formats). The framework passes the
	// localengine.LoadLogsFromDir loader. It is optional: when nil or when
	// loading fails, a warning is logged and the app keeps running — keyed
	// log calls then print their key (warn-once per key) instead of a
	// resolved format.
	LoadLogsLocales func(load func(fsys iofs.FS) error) error
	// ProjectSpec is the declarative project.yaml document (usually embedded).
	// When set, the framework loads it with projectspec and builds updater
	// managers from its declarations; a spec error aborts app construction.
	// Takes precedence over BuildUpdaters.
	ProjectSpec []byte
	// LoadInstallScript returns the Lua post-install script by name for
	// updater declarations with a "files" apply backend. Required when the
	// spec uses install_script.
	LoadInstallScript func(name string) []byte
	// BuildUpdaters returns the list of updater managers that should be
	// registered in the app. The framework passes the constructed App so the
	// implementation can access the config, logger and file system. If nil
	// and ProjectSpec is not set, no updaters are registered.
	BuildUpdaters func(*App) []*updater.Manager
	// RegisterCommands registers CLI commands on the engine.
	RegisterCommands func(*cli.Engine[*App])
	// ExtraGlobalFlags are registered before global flags are parsed. Use this
	// for flags that must be available to all commands and be recognized before
	// configuration is loaded.
	ExtraGlobalFlags []cli.Flag
	// RunGUI starts the GUI. It receives the fully constructed App and
	// returns true if the GUI ran successfully. If nil and no CLI command
	// was given, Run exits without error.
	RunGUI func(*App) bool
}

func ensureSubdir(root fs.Directory, name string, perm os.FileMode) (fs.Directory, error) {
	d := root.Subdir(name)
	if err := d.Ensure(perm); err != nil {
		return nil, fmt.Errorf("ensure %q: %w", name, err)
	}
	return d, nil
}

// registerSpecConfig registers the config entries declared in the project
// spec into the Sheet, preserving declaration order. UI hints (control,
// options, min/max) are not registered — they are frontend metadata.
// "action" control entries carry no value and get no Sheet cell; they are
// dispatched via the GUI's RunConfigAction binding instead.
func registerSpecConfig(sheet *config.Sheet, entries []projectspec.ConfigEntry) {
	for _, e := range entries {
		if e.Control == "action" {
			continue
		}
		var opts []config.Option
		if e.Disabled {
			opts = append(opts, config.WithDisabled(true))
		}
		sheet.Register(e.Path, config.Type(e.Type), e.Default, opts...)
	}
}

func parseDataDir(args []string, extraFlags []cli.Flag, defaultFn func() string) (string, []string, *cli.Engine[*App], error) {
	engine := cli.New[*App]()
	engine.AddGlobalFlag(cli.Flag{
		Name: "data-dir",
		Type: cli.Path,
		Desc: "Override default data directory",
	})
	for _, f := range extraFlags {
		engine.AddGlobalFlag(f)
	}

	globals, remaining, err := engine.ParseGlobals(args)
	if err != nil {
		return "", nil, nil, fmt.Errorf("parse global flags: %w", err)
	}

	dataDir := ""
	if v, ok := globals["data-dir"]; ok {
		dataDir = cli.AsString(v)
	}
	if dataDir == "" {
		if defaultFn == nil {
			return "", nil, nil, fmt.Errorf("DefaultDataDir is required")
		}
		dataDir = defaultFn()
	}
	return dataDir, remaining, engine, nil
}

// NewApp creates a new framework App with the given configuration.
func NewApp(cfg Config) (*App, error) {
	dataDir, remaining, cliEngine, err := parseDataDir(cfg.Args, cfg.ExtraGlobalFlags, cfg.DefaultDataDir)
	if err != nil {
		return nil, err
	}

	// Parse the declarative project spec once, early: the config schema is
	// registered from it and updater managers are built from it below. A spec
	// error aborts construction with the full problem list.
	var spec *projectspec.Spec
	if len(cfg.ProjectSpec) > 0 {
		var err error
		spec, err = projectspec.Load(cfg.ProjectSpec, projectspec.Vars{
			"BASE_DIR": dataDir,
			"DATA_DIR": dataDir,
		})
		if err != nil {
			return nil, err
		}
	}

	tmpLog := logger.NewLogger(1000)

	// Load logs-domain locales and install the key resolver on the boot
	// logger BEFORE the initial config load: the config fs logs through keyed
	// TErrorf calls whose %w chain only survives when the key resolves (an
	// unresolved key becomes the format string, breaking errors.Is checks such
	// as os.ErrNotExist for a missing config.yaml). Both steps degrade
	// gracefully: without logs locales (or with a broken one) keyed log calls
	// warn once per key and print the key itself. The main logger below gets
	// the same resolver once it exists.
	localengine.SetLogger(tmpLog.Root.Allocate("localengine"))
	if cfg.LoadLogsLocales != nil {
		if err := cfg.LoadLogsLocales(localengine.LoadLogsFromDir); err != nil {
			tmpLog.Root.Allocate("localengine").TWarnf("localengine.load_logs_locales", err)
		}
	}
	tmpLog.Root.SetResolver(localengine.LogResolver())

	tmpFS := fs.NewOSWithLog(dataDir, tmpLog.Root)
	tmpRoot := tmpFS.Root()
	if err := tmpRoot.Ensure(0750); err != nil {
		return nil, fmt.Errorf("ensure data dir: %w", err)
	}

	sheet := config.NewSheet(config.SheetOptions{})
	sheet.SetLogger(tmpLog.Root.Allocate("config"))
	if spec != nil && len(spec.Config) > 0 {
		registerSpecConfig(sheet, spec.Config)
	} else if cfg.RegisterConfig != nil {
		cfg.RegisterConfig(sheet)
	}

	conf, err := cfg.LoadConfig(tmpRoot, dataDir, sheet)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	limit := 1000
	if cfg.GetLoggerLimit != nil {
		limit = cfg.GetLoggerLimit(conf)
	}
	log := logger.NewLogger(limit)
	// The logs-domain locales and resolver were installed on the boot logger
	// before the initial config load (see above); the main logger gets the
	// same resolver up front so keyed calls (including fs errors from
	// setupDirectories below) behave the same on every terminal.
	log.Root.SetResolver(localengine.LogResolver())
	sheet.SetLogger(log.Root.Allocate("config"))
	sheet.DebugDisabledCount()
	appFS := fs.NewOSWithLog(dataDir, log.Root)
	root := appFS.Root()

	configsDir, pluginsDir, docsDir, err := setupDirectories(root)
	if err != nil {
		return nil, err
	}

	localengine.SetLogger(log.Root.Allocate("localengine"))
	if cfg.LoadLocales != nil {
		_ = cfg.LoadLocales(localengine.LoadFromDir)
	}

	app := &App{
		Logger:        log,
		FS:            appFS,
		BaseDir:       dataDir,
		Root:          root,
		ConfigsDir:    configsDir,
		PluginsDir:    pluginsDir,
		DocsDir:       docsDir,
		Config:        conf,
		CLI:           cliEngine,
		RemainingArgs: remaining,
		runGUI:        cfg.RunGUI,
		Spec:          spec,
	}

	if spec != nil {
		updaters, err := app.buildUpdatersFromSpec(spec, cfg.LoadInstallScript)
		if err != nil {
			return nil, err
		}
		app.Updaters = updaters
	} else if cfg.BuildUpdaters != nil {
		app.Updaters = cfg.BuildUpdaters(app)
	}
	for _, mgr := range app.Updaters {
		if mgr.Apply != nil {
			updater.SetManager(mgr)
			break
		}
	}

	if cfg.RegisterCommands != nil {
		cfg.RegisterCommands(app.CLI)
	}

	return app, nil
}

func setupDirectories(root fs.Directory) (fs.Directory, fs.Directory, fs.Directory, error) {
	configsDir, err := ensureSubdir(root, "configs", 0750)
	if err != nil {
		return nil, nil, nil, err
	}
	pluginsDir, err := ensureSubdir(root, "plugins", 0750)
	if err != nil {
		return nil, nil, nil, err
	}
	docsDir, err := ensureSubdir(root, "docs", 0750)
	if err != nil {
		return nil, nil, nil, err
	}
	return configsDir, pluginsDir, docsDir, nil
}

// Run executes the CLI command if arguments remain, otherwise starts the GUI.
func (a *App) Run() {
	if a.CLI.HelpRequested() {
		a.CLI.PrintHelp(os.Stdout)
		return
	}

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

// SetRunGUI replaces the GUI runner. Used by wrappers that need a different
// receiver type (e.g. app.App).
func (a *App) SetRunGUI(fn func(*App) bool) {
	a.runGUI = fn
}

// Start starts framework-level services. It is safe to call even when core
// code overrides Start() on a wrapper type.
func (a *App) Start() error {
	if a == nil {
		return errors.New("framework.App is nil")
	}
	if err := a.Root.Ensure(0750); err != nil {
		return err
	}
	a.Logger.Root.TInfof("framework.started")
	return nil
}

// Stop stops framework-level services.
func (a *App) Stop() error {
	if a == nil {
		return errors.New("framework.App is nil")
	}
	a.Logger.Root.TInfof("framework.stopped")
	return nil
}

// OpenDataDir opens the app's base directory in the system file manager.
func (a *App) OpenDataDir() error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", a.BaseDir).Start() // #nosec G204 -- opening app data dir
	case "darwin":
		return exec.Command("open", a.BaseDir).Start() // #nosec G204 -- opening app data dir
	default:
		return exec.Command("xdg-open", a.BaseDir).Start() // #nosec G204 -- opening app data dir
	}
}

// Base returns the embedded framework App for wrapper types.
func (a *App) Base() *App { return a }
