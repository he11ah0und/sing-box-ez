//go:build aur

package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SelfUpdateEnabled reports whether the self-update apply backend is part of
// this build. AUR builds are updated by pacman, so the self-update code is
// compiled out.
const SelfUpdateEnabled = false

// ActiveChannel returns the update channel selected at compile time.
func ActiveChannel() ChannelInfo {
	return ChannelInfo{ID: "aur", Name: "AUR", External: true}
}

// ChannelCurrentVersion reports the version of the package that owns the
// running binary, according to pacman. A failure means this AUR build is
// running outside of pacman management (untracked binary, corrupted package
// database), which the caller must treat as dangerous: the app cannot
// self-update and its provenance is unverified.
func ChannelCurrentVersion() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolve executable symlinks: %w", err)
	}
	out, err := exec.Command("pacman", "-Qo", exe).Output()
	if err != nil {
		return "", fmt.Errorf("pacman -Qo %s: %w", exe, err)
	}
	// Output: "<path> is owned by <pkg> <version>".
	const marker = " is owned by "
	line, _, _ := strings.Cut(string(out), "\n")
	_, tail, ok := strings.Cut(line, marker)
	if !ok {
		return "", fmt.Errorf("unexpected pacman output: %q", strings.TrimSpace(line))
	}
	fields := strings.Fields(tail)
	if len(fields) != 2 {
		return "", fmt.Errorf("unexpected pacman output: %q", strings.TrimSpace(line))
	}
	return fields[1], nil
}

// aurPackage is the AUR package this build belongs to.
const aurPackage = "sing-box-ez-gui-bin"

// ChannelLatestVersion queries the AUR RPC interface for the newest published
// package version instead of relying on an AUR helper being installed.
func ChannelLatestVersion() (string, error) {
	url := "https://aur.archlinux.org/rpc/?v=5&type=info&arg[]=" + aurPackage
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("query AUR for %s: %w", aurPackage, err)
	}
	defer resp.Body.Close()
	var payload struct {
		Results []struct {
			Version string `json:"Version"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode AUR response for %s: %w", aurPackage, err)
	}
	if len(payload.Results) == 0 || payload.Results[0].Version == "" {
		return "", fmt.Errorf("package %s not found on AUR", aurPackage)
	}
	return payload.Results[0].Version, nil
}

// ChannelUpdateHint returns the pacman/AUR-helper commands that update this
// package, so the GUI can suggest them instead of self-updating.
func ChannelUpdateHint() string {
	return "yay -S " + aurPackage + "  # or: paru -S " + aurPackage + "  # or: makepkg -si"
}
