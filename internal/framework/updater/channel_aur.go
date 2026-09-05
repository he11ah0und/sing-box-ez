//go:build aur

package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
