package core

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GetCoreVersion runs the core binary and returns its reported version.
func GetCoreVersion(corePath string) (string, error) {
	if _, err := os.Stat(corePath); err != nil {
		return "", err
	}
	cmd := exec.Command(corePath, "version")
	setNoWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	parts := strings.Fields(string(out))
	for i, p := range parts {
		if p == "version" && i+1 < len(parts) {
			return parts[i+1], nil
		}
	}
	return "", fmt.Errorf("unable to parse version from: %s", string(out))
}
