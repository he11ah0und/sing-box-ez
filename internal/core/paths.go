package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// BaseDir is the root data directory used by core path helpers.
// It must be set by the application before calling any path functions.
var BaseDir string

// CoreBinary returns the path to the sing-box core executable.
func CoreBinary() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(BaseDir, "sing-box.exe")
	}
	return filepath.Join(BaseDir, "sing-box")
}

// CachedConfig returns the path to a cached config by profile name.
func CachedConfig(name string) string {
	return filepath.Join(BaseDir, "configs", name+".json")
}

// HasCachedConfig reports whether a cached config exists for the given profile.
func HasCachedConfig(name string) bool {
	_, err := os.Stat(CachedConfig(name))
	return err == nil
}

// GetCorePath returns the path to the sing-box core executable.
func GetCorePath() string {
	return CoreBinary()
}

// CoreExists reports whether the core binary is present.
func CoreExists() bool {
	_, err := os.Stat(GetCorePath())
	return err == nil
}

// ListCachedConfigs returns all profile names that have a cached config.
func ListCachedConfigs() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(BaseDir, "configs"))
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
