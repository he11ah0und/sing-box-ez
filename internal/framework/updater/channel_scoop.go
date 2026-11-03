//go:build scoop

package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// SelfUpdateEnabled reports whether the self-update apply backend is part of
// this build. Scoop builds are updated by `scoop update`, so the self-update
// code is compiled out.
const SelfUpdateEnabled = false

// ActiveChannel returns the update channel selected at compile time.
func ActiveChannel() ChannelInfo {
	return ChannelInfo{ID: "scoop", Name: "Scoop", External: true}
}

const scoopPackage = "sing-box-ez-gui"

// ChannelCurrentVersion reports the installed version according to Scoop
// itself (`scoop list`). A failure means the app is not tracked by Scoop, so
// its provenance is unverified, mirroring the AUR pacman check.
func ChannelCurrentVersion() (string, error) {
	out, err := exec.Command("scoop", "list", scoopPackage).Output()
	if err != nil {
		return "", fmt.Errorf("scoop list %s: %w", scoopPackage, err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == scoopPackage {
			return fields[1], nil
		}
	}
	return "", fmt.Errorf("package %s not found in scoop list output", scoopPackage)
}

// ChannelLatestVersion reads the version from the Scoop manifest kept in this
// repository's bucket/ directory on the main branch.
func ChannelLatestVersion() (string, error) {
	const manifestURL = "https://raw.githubusercontent.com/he11ah0und/sing-box-ez/main/bucket/sing-box-ez-gui.json"
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(manifestURL)
	if err != nil {
		return "", fmt.Errorf("fetch Scoop manifest: %w", err)
	}
	defer resp.Body.Close()
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return "", fmt.Errorf("decode Scoop manifest: %w", err)
	}
	if manifest.Version == "" {
		return "", fmt.Errorf("Scoop manifest for %s has no version", scoopPackage)
	}
	return manifest.Version, nil
}

// ChannelUpdateHint returns the Scoop command that updates this package.
func ChannelUpdateHint() string {
	return "scoop update " + scoopPackage
}
