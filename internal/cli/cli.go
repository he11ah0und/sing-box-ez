package cli

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/paths"
	"sing-box-ez/internal/plugins"
)

// cmdDef describes a CLI command.
type cmdDef struct {
	desc string
	fn   func(*config.AppConfig, []string) error
}

var commands = map[string]cmdDef{
	"start":    {"Start sing-box with auto-update", cmdStart},
	"stop":     {"Stop running sing-box", cmdStop},
	"update":   {"Download latest config", cmdUpdate},
	"download": {"Download latest sing-box core", cmdDownload},
	"status":   {"Show running status", cmdStatus},
	"setcap":   {"Apply CAP_NET_ADMIN capability (Linux CLI, uses sudo)", cmdSetcap},
	"docs":     {"Generate plugin API docs (mkdocs markdown)", cmdDocs},
	"defs":     {"Generate VS Code Lua definitions (EmmyLua)", cmdDefs},
	"template": {"Generate plugin template <name> [client|server|both]", cmdTemplate},
	"install":  {"Install plugin from URL <url>", cmdInstall},
}

// PrintHelp writes auto-generated help to w.
func PrintHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  sing-box-ez <command>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commands:")

	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		fmt.Fprintf(w, "  %-9s %s\n", name, commands[name].desc)
	}
}

func Run(args []string) error {
	if len(args) < 1 {
		PrintHelp(os.Stderr)
		return fmt.Errorf("")
	}

	cmd, ok := commands[args[0]]
	if !ok {
		PrintHelp(os.Stderr)
		return fmt.Errorf("unknown command: %s", args[0])
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	return cmd.fn(cfg, args[1:])
}

func cmdStart(cfg *config.AppConfig, _ []string) error {
	if !core.CoreExists() {
		fmt.Println("Core not found, downloading latest...")
		_, err := core.DownloadCore("", func(d, t int64) {
			pct := float64(d) / float64(t) * 100
			fmt.Printf("\rDownload: %.1f%% (%d / %d bytes)", pct, d, t)
		})
		if err != nil {
			return fmt.Errorf("download core failed: %w", err)
		}
		fmt.Println()
	}

	ver, _ := core.GetCoreVersion(core.GetCorePath())
	if ver != "" {
		fmt.Println("Core version:", ver)
	}

	active := cfg.GetActiveConfig()
	if active == nil || active.URL == "" {
		return fmt.Errorf("no active config URL set, use GUI or edit config.json")
	}

	if active.ShouldUpdate() || !core.HasCachedConfig(active.Name) {
		fmt.Println("Updating config...")
		if err := core.DownloadConfigFor(active.Name, active.URL); err != nil {
			if !core.HasCachedConfig(active.Name) {
				return fmt.Errorf("config download failed: %w", err)
			}
			fmt.Println("Using existing local config")
		} else {
			cfg.SetLastUpdateFor(active.Name, time.Now())
			cfg.Save()
			fmt.Println("Config updated")
		}
	}

	manager := core.NewManager(active.URL)
	manager.SetConfigName(active.Name)
	manager.SetElevated(cfg.RunAsAdmin)

	if err := manager.Start(); err != nil {
		return fmt.Errorf("start failed: %w", err)
	}

	pid := manager.GetPID()
	if pid > 0 {
		os.WriteFile(paths.PIDFile(), []byte(strconv.Itoa(pid)), 0644)
	}

	fmt.Printf("sing-box started (PID %d)\n", pid)

	// Block until interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	fmt.Println("Stopping sing-box...")
	manager.Stop()
	os.Remove(paths.PIDFile())
	return nil
}

func cmdStop(cfg *config.AppConfig, _ []string) error {
	data, err := os.ReadFile(paths.PIDFile())
	if err != nil {
		return fmt.Errorf("pid file not found, is sing-box running?")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf("invalid pid file")
	}

	elevated := cfg.RunAsAdmin
	if core.HasNetAdminCapability(core.GetCorePath()) {
		elevated = false
	}

	fmt.Printf("Stopping process %d...\n", pid)
	if err := core.KillProcess(pid, elevated); err != nil {
		fmt.Printf("kill warning: %v\n", err)
	}
	os.Remove(paths.PIDFile())
	fmt.Println("Stopped")
	return nil
}

func cmdUpdate(cfg *config.AppConfig, _ []string) error {
	active := cfg.GetActiveConfig()
	if active == nil || active.URL == "" {
		return fmt.Errorf("no active config URL set")
	}
	fmt.Println("Downloading config...")
	if err := core.DownloadConfigFor(active.Name, active.URL); err != nil {
		return err
	}
	cfg.SetLastUpdateFor(active.Name, time.Now())
	cfg.Save()
	fmt.Println("Config updated")
	return nil
}

func cmdDownload(cfg *config.AppConfig, _ []string) error {
	ver, err := core.GetLatestVersion()
	if err != nil {
		return fmt.Errorf("failed to check latest version: %w", err)
	}
	fmt.Println("Latest version:", ver)
	fmt.Println("Downloading...")
	_, err = core.DownloadCore("", func(d, t int64) {
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

func cmdStatus(cfg *config.AppConfig, _ []string) error {
	data, err := os.ReadFile(paths.PIDFile())
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
	os.Remove(paths.PIDFile())
	return nil
}

func cmdSetcap(cfg *config.AppConfig, _ []string) error {
	corePath := core.GetCorePath()
	if !core.CoreExists() {
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

func cmdDocs(cfg *config.AppConfig, _ []string) error {
	outDir := paths.PluginDocsDir()
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

func cmdDefs(cfg *config.AppConfig, _ []string) error {
	outDir := paths.PluginDefsDir()
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
	fmt.Println("     OR add this to VS Code settings.json:")
	fmt.Printf("       \"Lua.workspace.library\": [\"%s\"]\n", filepath.ToSlash(outDir))
	return nil
}

func cmdTemplate(cfg *config.AppConfig, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: sing-box-ez template <name> [client|server|both]")
	}
	name := args[0]
	rel := "client"
	if len(args) > 1 {
		rel = args[1]
	}
	outDir := paths.Data(filepath.Join("plugins", name))
	if err := plugins.GeneratePluginTemplate(outDir, name, rel); err != nil {
		return err
	}
	fmt.Println("Plugin template generated:", outDir)
	return nil
}

func cmdInstall(cfg *config.AppConfig, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: sing-box-ez install <url>")
	}
	url := args[0]
	fmt.Println("Installing plugin from:", url)
	mf, err := plugins.InstallFromURL(url)
	if err != nil {
		return err
	}
	fmt.Printf("Installed: %s v%s (%s)\n", mf.Name, mf.Version, mf.SourceType)
	return nil
}
