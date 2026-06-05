package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const DataDir = "sing-box-ez-data"

func init() {
	_ = os.MkdirAll(DataDir, 0755)
	_ = os.MkdirAll(filepath.Join(DataDir, "configs"), 0755)
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
