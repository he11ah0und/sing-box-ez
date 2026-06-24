package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework"
	fwcli "sing-box-ez/internal/framework/cli"
	"sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/logger"
	"sing-box-ez/internal/framework/updater"
	"sing-box-ez/internal/framework/version"
	"sing-box-ez/internal/plugins"
)

const (
	defaultGitHubOwner = "he11ah0und"
	defaultGitHubRepo  = "sing-box-ez"
)

// RegisterCommands registers sing-box-ez CLI commands on the framework CLI engine.
func RegisterCommands(cli *fwcli.Engine[*framework.App]) {
	cli.SetBeforeExec(func(app *framework.App) error {
		ensureUpdater()
		return nil
	})

	cli.Register("start", "Start sing-box with auto-update", wrap(cmdStart))
	cli.Register("stop", "Stop running sing-box", wrap(cmdStop))
	cli.Register("update", "Download latest config", wrap(cmdUpdate))
	cli.Register("download", "Download latest sing-box core", wrap(cmdDownload))
	cli.Register("status", "Show running status", wrap(cmdStatus))
	cli.Register("setcap", "Apply CAP_NET_ADMIN capability (Linux CLI, uses sudo)", wrap(cmdSetcap))
	cli.Register("docs", "Generate plugin API docs (mkdocs markdown)", wrap(cmdDocs))
	cli.Register("defs", "Generate VS Code Lua definitions (EmmyLua)", wrap(cmdDefs))
	cli.Register("template", "Generate plugin template", wrap(cmdTemplate),
		fwcli.Arg{Name: "name", Type: fwcli.String, Desc: "Plugin name"},
		fwcli.Arg{Name: "type", Type: fwcli.String, Optional: true, Default: fwcli.StringValue("client"), Desc: "Template type (client, server, both)"},
	)
	cli.Register("install", "Install plugin from URL", wrap(cmdInstall),
		fwcli.Arg{Name: "url", Type: fwcli.String, Desc: "Plugin package URL (zip or tar.gz)"},
	)
	cli.Register("version", "Show version and repository info", wrap(cmdVersion))
	cli.Register("update-check", "Check for app and core updates", wrap(cmdUpdateCheck))
	cli.Register("self-update", "Update this application to latest release", wrap(cmdSelfUpdate))
}

func wrap(fn func(*config.AppConfig, *fwcli.Context) error) fwcli.CommandFunc[*framework.App] {
	return func(app *framework.App, ctx *fwcli.Context) error {
		cfg := app.Config.(*config.AppConfig)

		if remoteAddressFromContext(ctx) != "" {
			return dispatchRemoteCommand(app, ctx)
		}

		return fn(cfg, ctx)
	}
}

func coreBinaryPath(dataDir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(dataDir, "sing-box.exe")
	}
	return filepath.Join(dataDir, "sing-box")
}

func coreExists(dataDir string) bool {
	_, err := os.Stat(coreBinaryPath(dataDir))
	return err == nil
}

func hasCachedConfig(dataDir, name string) bool {
	_, err := os.Stat(filepath.Join(dataDir, "configs", name+".json"))
	return err == nil
}

func configHashMismatch(dataDir string, rec *config.ConfigRecord) bool {
	if rec.IsLocal() || rec.Hash == "" {
		return false
	}
	data, err := os.ReadFile(filepath.Join(dataDir, "configs", rec.Name+".json"))
	if err != nil {
		return false
	}
	return config.HashConfig(data) != rec.Hash
}

func newCoreManager(dataDir string) *core.Manager {
	log := logger.NewLogger(0)
	return core.NewManager(dataDir, fs.NewOS(dataDir), nil, log)
}

// ensureUpdater installs default updater managers when none are configured.
func ensureUpdater() {
	if updater.CurrentManager() != nil {
		return
	}
	gh := updater.NewGitHubBackend(logger.NewLogger(0).Root, defaultGitHubOwner, defaultGitHubRepo)
	updater.SetManager(&updater.Manager{
		Name:   "updater",
		Source: gh,
		Apply:  &updater.SelfUpdateApply{},
	})
}

func latestCoreVersion(dataDir string) (string, error) {
	m := newCoreManager(dataDir)
	if m == nil {
		return "", fmt.Errorf("core updater not configured")
	}
	info, err := m.CheckCoreUpdate(context.Background())
	if err != nil {
		return "", err
	}
	return info.Latest, nil
}

func downloadLatestCore(dataDir string, onProgress func(d, t int64)) (string, error) {
	m := newCoreManager(dataDir)
	if m == nil {
		return "", fmt.Errorf("core updater not configured")
	}
	return m.DownloadCore(onProgress)
}

func cmdStart(cfg *config.AppConfig, _ *fwcli.Context) error {
	dataDir := cfg.DataDir
	if !coreExists(dataDir) {
		fmt.Println("Core not found, downloading latest...")
		_, err := downloadLatestCore(dataDir, func(d, t int64) {
			pct := float64(d) / float64(t) * 100
			fmt.Printf("\rDownload: %.1f%% (%d / %d bytes)", pct, d, t)
		})
		if err != nil {
			return fmt.Errorf("download core failed: %w", err)
		}
		fmt.Println()
	}

	ver, _ := core.GetCoreVersion(coreBinaryPath(dataDir))
	if ver != "" {
		fmt.Println("Core version:", ver)
	}

	active := cfg.GetActiveConfig()
	if active == nil {
		return fmt.Errorf("no active config set, use GUI or edit profiles.yaml")
	}

	if active.IsLocal() {
		if !hasCachedConfig(dataDir, active.Name) {
			m := newCoreManager(dataDir)
			if err := m.CreateLocalConfig(active.Name); err != nil {
				return fmt.Errorf("failed to create local config: %w", err)
			}
		}
	} else if active.ShouldUpdate() || !hasCachedConfig(dataDir, active.Name) || (cfg.MustGet("updates", "auto_update_on_hash_mismatch").Bool() && configHashMismatch(dataDir, active)) {
		fmt.Println("Updating config...")
		m := newCoreManager(dataDir)
		m.SetConfigName(active.Name)
		m.SetConfigURL(active.URL)
		data, err := m.UpdateConfig()
		if err != nil {
			if !hasCachedConfig(dataDir, active.Name) {
				return fmt.Errorf("config download failed: %w", err)
			}
			fmt.Println("Using existing local config")
		} else {
			active.Hash = config.HashConfig(data)
			cfg.SetLastUpdateFor(active.Name, time.Now())
			_ = cfg.Save()
			fmt.Println("Config updated")
		}
	}

	m := newCoreManager(dataDir)
	m.SetConfigURL(active.URL)
	m.SetConfigName(active.Name)
	m.SetElevated(cfg.MustGet("privileges", "run_as_admin").Bool())

	if err := m.Start(); err != nil {
		return fmt.Errorf("start failed: %w", err)
	}

	pid := m.GetPID()
	if pid > 0 {
		_ = os.WriteFile(filepath.Join(cfg.DataDir, ".pid"), []byte(strconv.Itoa(pid)), 0600)
	}

	fmt.Printf("sing-box started (PID %d)\n", pid)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Stopping sing-box...")
	if err := m.Stop(); err != nil {
		fmt.Printf("stop warning: %v\n", err)
	}
	_ = os.Remove(filepath.Join(cfg.DataDir, ".pid"))
	return nil
}

func cmdStop(cfg *config.AppConfig, _ *fwcli.Context) error {
	dataDir := cfg.DataDir
	data, err := os.ReadFile(filepath.Join(dataDir, ".pid"))
	if err != nil {
		return fmt.Errorf("pid file not found, is sing-box running?")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf("invalid pid file")
	}

	elevated := cfg.MustGet("privileges", "run_as_admin").Bool()
	if core.HasNetAdminCapability(coreBinaryPath(dataDir)) {
		elevated = false
	}

	fmt.Printf("Stopping process %d...\n", pid)
	if err := core.KillProcess(pid, elevated); err != nil {
		fmt.Printf("kill warning: %v\n", err)
	}
	_ = os.Remove(filepath.Join(dataDir, ".pid"))
	fmt.Println("Stopped")
	return nil
}

func cmdUpdate(cfg *config.AppConfig, _ *fwcli.Context) error {
	dataDir := cfg.DataDir
	active := cfg.GetActiveConfig()
	if active == nil {
		return fmt.Errorf("no active config set")
	}
	if active.IsLocal() {
		return fmt.Errorf("local config cannot be updated from a URL")
	}
	fmt.Println("Downloading config...")
	m := newCoreManager(dataDir)
	m.SetConfigName(active.Name)
	m.SetConfigURL(active.URL)
	data, err := m.UpdateConfig()
	if err != nil {
		return err
	}
	active.Hash = config.HashConfig(data)
	cfg.SetLastUpdateFor(active.Name, time.Now())
	_ = cfg.Save()
	fmt.Println("Config updated")
	return nil
}

func cmdDownload(cfg *config.AppConfig, _ *fwcli.Context) error {
	ver, err := latestCoreVersion(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("failed to check latest version: %w", err)
	}
	fmt.Println("Latest version:", ver)
	fmt.Println("Downloading...")
	_, err = downloadLatestCore(cfg.DataDir, func(d, t int64) {
		pct := float64(d) / float64(t) * 100
		fmt.Printf("\rDownload: %.1f%% (%d / %d bytes)", pct, d, t)
	})
	if err != nil {
		return err
	}
	fmt.Println()
	fmt.Println("Core downloaded")
	return nil
}

func cmdStatus(cfg *config.AppConfig, _ *fwcli.Context) error {
	dataDir := cfg.DataDir
	data, err := os.ReadFile(filepath.Join(dataDir, ".pid"))
	if err != nil {
		fmt.Println("Status: not running (no pid file)")
		return nil
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	if pid > 0 && core.ProcessExists(pid) {
		fmt.Printf("Status: running (PID %d)\n", pid)
		return nil
	}
	fmt.Println("Status: not running (stale pid file)")
	_ = os.Remove(filepath.Join(dataDir, ".pid"))
	return nil
}

func cmdSetcap(cfg *config.AppConfig, _ *fwcli.Context) error {
	corePath := coreBinaryPath(cfg.DataDir)
	if _, err := os.Stat(corePath); err != nil {
		return fmt.Errorf("core not found at %s", corePath)
	}
	fmt.Printf("Applying CAP_NET_ADMIN to %s (CLI mode uses sudo)...\n", corePath)
	if err := core.SetNetAdminCapabilityCLI(corePath); err != nil {
		return fmt.Errorf("setcap failed: %w", err)
	}
	fmt.Println("setcap applied successfully.")
	fmt.Println("You can now start sing-box without root (TUN will work).")
	return nil
}

func cmdDocs(cfg *config.AppConfig, _ *fwcli.Context) error {
	outDir := filepath.Join(cfg.DataDir, plugins.DocsDir())
	fmt.Println("Generating plugin API docs to:", outDir)
	if err := plugins.GenerateDocs(outDir); err != nil {
		return fmt.Errorf("docs generation failed: %w", err)
	}
	fmt.Println("Done. Files:")
	fmt.Println("  - index.md")
	fmt.Println("  - dev-guide.md")
	fmt.Println("  - http.md")
	fmt.Println("  - log.md")
	fmt.Println("  - ui.md")
	fmt.Println("  - config.md")
	return nil
}

func cmdDefs(cfg *config.AppConfig, _ *fwcli.Context) error {
	outDir := filepath.Join(cfg.DataDir, plugins.DefsDir())
	fmt.Println("Generating VS Code Lua definitions to:", outDir)
	if err := plugins.GenerateLuaDefs(outDir); err != nil {
		return fmt.Errorf("defs generation failed: %w", err)
	}
	fmt.Println("Done. Files:")
	fmt.Println("  - http.lua")
	fmt.Println("  - log.lua")
	fmt.Println("  - ui.lua")
	fmt.Println("  - config.lua")
	fmt.Println("  - globals.lua")
	fmt.Println("  - .luarc.json")
	fmt.Println("")
	fmt.Println("To use in VS Code:")
	fmt.Println("  1. Install the 'Lua' extension by Sumneko")
	fmt.Println("  2. In your plugin project, copy .luarc.json to the project root")
	fmt.Printf("     OR add this to VS Code settings.json:\n")
	fmt.Printf("       \"Lua.workspace.library\": [\"%s\"]\n", filepath.ToSlash(outDir))
	return nil
}

func cmdTemplate(cfg *config.AppConfig, ctx *fwcli.Context) error {
	name := fwcli.AsString(ctx.Arg("name"))
	rel := fwcli.AsString(ctx.Arg("type"))
	outDir := filepath.Join(cfg.DataDir, "plugins", name)
	if err := plugins.GeneratePluginTemplate(outDir, name, rel); err != nil {
		return err
	}
	fmt.Println("Plugin template generated:", outDir)
	return nil
}

func cmdInstall(cfg *config.AppConfig, ctx *fwcli.Context) error {
	url := fwcli.AsString(ctx.Arg("url"))
	fmt.Println("Installing plugin from:", url)
	mf, err := plugins.InstallFromURL(url)
	if err != nil {
		return err
	}
	fmt.Printf("Installed: %s v%s (%s)\n", mf.Name, mf.Version, mf.SourceType)
	return nil
}

func cmdVersion(_ *config.AppConfig, _ *fwcli.Context) error {
	fmt.Println("sing-box-ez", version.Info())
	fmt.Println("Repository:", "https://github.com/he11ah0und/sing-box-ez")
	return nil
}

func cmdUpdateCheck(_ *config.AppConfig, _ *fwcli.Context) error {
	fmt.Println("Checking for updates...")
	fmt.Println()

	info, err := updater.CheckUpdate(version.Branch)
	if err != nil {
		fmt.Println("App update check failed:", err)
	} else if info.ReleaseCount > 0 {
		fmt.Printf("App: %s → %s (%d release(s) behind)\n", info.Current, info.Latest, info.ReleaseCount)
		fmt.Println()
		fmt.Println("Latest release notes:")
		fmt.Println(info.LatestBody)
	} else {
		fmt.Println("App: up to date (" + info.Current + ")")
	}
	fmt.Println()
	fmt.Println("Core: use GUI to check core updates")
	return nil
}

func cmdSelfUpdate(_ *config.AppConfig, _ *fwcli.Context) error {
	fmt.Println("Checking for application update...")
	info, err := updater.CheckUpdate(version.Branch)
	if err != nil {
		return fmt.Errorf("check failed: %w", err)
	}
	if info.ReleaseCount == 0 {
		fmt.Println("Already up to date.")
		return nil
	}
	if info.Asset.URL == "" {
		return fmt.Errorf("no matching asset found for this system (%s)", info.AssetName)
	}

	fmt.Printf("Updating %s → %s\n", info.Current, info.Latest)
	fmt.Printf("Downloading %s...\n", info.AssetName)

	if err := updater.ApplyUpdate(info.Asset, nil); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	return nil
}
