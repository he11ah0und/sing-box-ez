package core

import (
	"context"
	"fmt"
)

// DownloadConfigFor downloads a remote config profile into the cache.
func DownloadConfigFor(name, url string) error {
	if Net == nil || FS == nil {
		return fmt.Errorf("core services not initialized")
	}
	return Net.DownloadToFile(context.Background(), FS, url, CachedConfig(name))
}

// GetConfigPath returns the cached path for the named config profile.
func GetConfigPath(name string) string {
	return CachedConfig(name)
}
