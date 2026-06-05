package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GeneratePluginTemplate creates a starter plugin directory with manifest.json
// and main.lua generated from the current API surface.
//
//	outDir   — target directory (e.g. "sing-box-ez-data/plugins/my-plugin")
//	name     — plugin name (also used as directory name)
//	relation — "client", "server", or "both" (stored as JSON string or array)
func GeneratePluginTemplate(outDir, name, relation string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	// Build manifest.
	mf := Manifest{
		Name:        name,
		Version:     "1.0.0",
		Author:      "",
		Description: "Plugin generated from API template",
		UpdateURL:   "",
		Entry:       "main.lua",
	}
	// Parse relation into Relations slice.
	switch relation {
	case "client", "server":
		mf.Relations = Relations{relation}
	case "both":
		mf.Relations = Relations{"client", "server"}
	default:
		mf.Relations = Relations{"client"}
	}

	mfData, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "manifest.json"), mfData, 0644); err != nil {
		return err
	}

	// Build main.lua from the live API registry.
	lua := generateLuaTemplate(name)
	if err := os.WriteFile(filepath.Join(outDir, "main.lua"), []byte(lua), 0644); err != nil {
		return err
	}

	// Register state so it appears as a folder plugin on next Discover.
	state, _ := LoadState()
	state.Set(name, PluginState{
		Enabled:       false,
		SourceType:    "folder",
		SourceURL:     "",
		LatestVersion: "",
	})
	_ = state.Save()

	return nil
}

func generateLuaTemplate(pluginName string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "-- %s\n", strings.Repeat("-", 60))
	fmt.Fprintf(&b, "-- Plugin: %s\n", pluginName)
	b.WriteString("-- Auto-generated from the current plugin API surface.\n")
	b.WriteString("-- Remove what you don't need and customise the rest.\n")
	fmt.Fprintf(&b, "-- %s\n\n", strings.Repeat("-", 60))

	for _, mod := range GetAPIModules() {
		fmt.Fprintf(&b, "-- %s module: %s\n", mod.Name, mod.Desc)
		b.WriteString("--\n")
		for _, fn := range mod.Funcs {
			fmt.Fprintf(&b, "-- %s\n", fn.Sig)
			fmt.Fprintf(&b, "--   %s\n", fn.Desc)
			if fn.Example != "" {
				b.WriteString("-- Example:\n")
				lines := strings.Split(fn.Example, "\n")
				for _, line := range lines {
					fmt.Fprintf(&b, "--   %s\n", line)
				}
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("-- Minimal working skeleton\n")
	fmt.Fprintf(&b, "log.info(\"plugin \" .. %q .. \" loaded\")\n", pluginName)

	return b.String()
}
