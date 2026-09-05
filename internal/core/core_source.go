package core

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/mod/semver"

	"sing-box-ez/internal/config"
)

// MinCoreVersion is the oldest sing-box release the app is compatible with;
// older cores lack config fields the overrides rely on.
const MinCoreVersion = "1.12.0"

// Core source modes for the core.source.mode setting.
const (
	// CoreSourceOfficial downloads and manages sing-box into the data dir
	// (the classic behavior).
	CoreSourceOfficial = "official"
	// CoreSourceSystem uses a sing-box binary installed on the system (found
	// via PATH and the usual install locations).
	CoreSourceSystem = "system"
	// CoreSourceCustom uses a user-picked binary path.
	CoreSourceCustom = "custom"
)

// ErrCoreIncompatible marks a core binary that runs but is older than
// MinCoreVersion (or whose version cannot be parsed).
var ErrCoreIncompatible = errors.New("core version is not supported")

// managedCoreBinary is the path of the app-managed core in the data dir.
func managedCoreBinary(baseDir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(baseDir, "sing-box.exe")
	}
	return filepath.Join(baseDir, "sing-box")
}

// ResolveCoreBinary returns the core binary path for the configured source
// mode, falling back to the managed binary when the selection is unusable
// (unset custom path, system binary vanished).
func ResolveCoreBinary(cfg *config.AppConfig, baseDir string) string {
	switch cfg.MustGet("core", "source", "mode").String() {
	case CoreSourceSystem:
		if p := FindSystemCore(); p != "" {
			return p
		}
	case CoreSourceCustom:
		if p := strings.TrimSpace(cfg.MustGet("core", "source", "custom_path").String()); p != "" {
			return p
		}
	}
	return managedCoreBinary(baseDir)
}

// FindSystemCore locates a sing-box binary installed on the system: PATH
// first, then the usual install locations. Returns "" when none exists.
func FindSystemCore() string {
	if p, err := exec.LookPath("sing-box"); err == nil {
		return p
	}
	candidates := []string{"/usr/bin/sing-box", "/usr/local/bin/sing-box"}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".local", "bin", "sing-box"))
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// DetectCoreSourceMode picks the initial core source mode for a fresh
// install: a compatible system sing-box wins, otherwise the managed download.
func DetectCoreSourceMode(cfg *config.AppConfig) string {
	if p := FindSystemCore(); p != "" {
		if _, err := CheckCoreCompatibility(p); err == nil {
			return CoreSourceSystem
		}
	}
	return CoreSourceOfficial
}

// CheckCoreCompatibility runs the binary's "version" command and verifies the
// reported version against MinCoreVersion. The returned string is the parsed
// version (without a leading "v").
func CheckCoreCompatibility(path string) (string, error) {
	ver, err := GetCoreVersion(path)
	if err != nil {
		return "", err
	}
	v := ver
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if !semver.IsValid(v) {
		return ver, fmt.Errorf("%w: cannot parse version %q (need >= %s)", ErrCoreIncompatible, ver, MinCoreVersion)
	}
	if semver.Compare(v, "v"+MinCoreVersion) < 0 {
		return ver, fmt.Errorf("%w: %s < %s", ErrCoreIncompatible, ver, MinCoreVersion)
	}
	return ver, nil
}
