package core

import (
	"fmt"
	"os"
	"path/filepath"

	"sing-box-ez/internal/singboxconfig"
)

// applyCacheFileOverride injects an experimental.cache_file section when the
// config does not declare one. With a cache file present the core persists the
// selected outbound of every selector group and restores it on the next start,
// so a node the user picked in the GUI survives restarts instead of falling
// back to the first outbound of the group. The cache file is per profile
// (<dataDir>/cache/<name>.db): different profiles keep independent selections.
// A config that brings its own cache_file section is left untouched.
func (c *Controller) applyCacheFileOverride(data []byte, configName string) ([]byte, error) {
	if configName == "" {
		return data, nil
	}
	cachePath := filepath.Join(c.cfg.DataDir, "cache", configName+".db")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0750); err != nil {
		return nil, fmt.Errorf("create core cache dir: %w", err)
	}

	version, _ := c.GetInstalledCoreVersion()
	parser, err := singboxconfig.NewConfigParserForVersion(version)
	if err != nil {
		c.terminal.TWarnf("core.invalid_core_version", version, err)
		parser = singboxconfig.NewConfigParser()
	}

	output, _, err := parser.Override(data, func(tree map[string]any) bool {
		exp, _ := tree["experimental"].(map[string]any)
		if exp == nil {
			exp = make(map[string]any)
		}
		if _, exists := exp["cache_file"]; !exists {
			exp["cache_file"] = map[string]any{
				"enabled": true,
				"path":    cachePath,
			}
			tree["experimental"] = exp
		}
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("apply cache file override: %w", err)
	}
	return output, nil
}
