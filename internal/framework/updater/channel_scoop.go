//go:build scoop

package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
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

// ChannelCurrentVersion reports the running build's own version from its build
// metadata; no Scoop CLI is invoked for that.
func ChannelCurrentVersion() (string, error) {
	return currentVersionLabel(""), nil
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
