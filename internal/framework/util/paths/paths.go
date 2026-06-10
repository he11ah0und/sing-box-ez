package paths

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// DataDir is the root directory for application data.
// It can be overridden via --data-dir before Init() is called.
var DataDir string

// SetDataDir overrides the default data directory.
func SetDataDir(dir string) {
	DataDir = dir
}

// Init creates the default data directory and subdirectories.
// It should be called after any SetDataDir override.
func Init() {
	if DataDir == "" {
		DataDir = defaultDataDir()
	}
	_ = os.MkdirAll(DataDir, 0750)
	_ = os.MkdirAll(filepath.Join(DataDir, "configs"), 0750)
	_ = os.MkdirAll(filepath.Join(DataDir, "plugins"), 0750)
	_ = os.MkdirAll(filepath.Join(DataDir, "docs"), 0750)
}

func defaultDataDir() string {
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = os.Getenv("LOCALAPPDATA")
		}
		if appData == "" {
			appData = os.TempDir()
		}
		return filepath.Join(appData, "sing-box-ez")
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		return filepath.Join(home, ".sing-box-ez")
	}
}

// Data returns a path inside the data directory.
func Data(name string) string {
	return filepath.Join(DataDir, name)
}

// CoreBinary returns the path to the sing-box core executable.
func CoreBinary() string {
	if runtime.GOOS == "windows" {
		return Data("sing-box.exe")
	}
	return Data("sing-box")
}

// AppConfig returns the path to the application config file.
func AppConfig() string {
	return Data("config.json")
}

// Profiles returns the path to the profiles config file.
func Profiles() string {
	return Data("profiles.json")
}

// CachedConfig returns the path to a cached config by profile name.
func CachedConfig(name string) string {
	return Data(filepath.Join("configs", name+".json"))
}

// ListCachedConfigs returns all profile names that have a cached config.
func ListCachedConfigs() ([]string, error) {
	entries, err := os.ReadDir(Data("configs"))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return names, nil
}

// HasCachedConfig reports whether a cached config exists for the given profile.
func HasCachedConfig(name string) bool {
	_, err := os.Stat(CachedConfig(name))
	return err == nil
}

// PIDFile returns the path to the PID file used by the CLI.
func PIDFile() string {
	return Data(".pid")
}

// PluginDocsDir returns the default directory for generated plugin API docs.
func PluginDocsDir() string {
	return "docs/plugin-api"
}

// PluginDefsDir returns the default directory for EmmyLua definition files.
func PluginDefsDir() string {
	return "docs/plugin-defs"
}

// OpenDataDir opens the data directory in the system's file manager.
func OpenDataDir() error {
	switch runtime.GOOS {
	case "windows":
		// #nosec G204 — explorer is a system binary; DataDir is the app's managed data directory.
		return exec.Command("explorer", DataDir).Start()
	case "darwin":
		// #nosec G204 — open is a system binary; DataDir is the app's managed data directory.
		return exec.Command("open", DataDir).Start()
	default:
		// #nosec G204 — xdg-open is a system binary; DataDir is the app's managed data directory.
		return exec.Command("xdg-open", DataDir).Start()
	}
}
