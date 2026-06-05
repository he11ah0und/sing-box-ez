//go:build !noplugins

package plugins

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// GenerateDocs reflects over the plugin API surface and writes mkdocs markdown files.
// It writes files into outDir (e.g. "docs/plugin-api").
func GenerateDocs(outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	modules := GetAPIModules()

	// Write index.md with overview.
	var idx strings.Builder
	idx.WriteString("---\ntitle: Plugin API Reference\n---\n\n# Plugin API Reference\n\n")
	idx.WriteString("This reference is auto-generated from the real Go plugin API code.\n")
	idx.WriteString("Plugins are written in Lua and have access to the following modules:\n\n")
	for _, mod := range modules {
		fmt.Fprintf(&idx, "- **%s** — %s\n", mod.Name, mod.Desc)
	}
	idx.WriteString("\n## Manifest Format\n\n")
	idx.WriteString("Each plugin must provide a `manifest.json` in its directory:\n\n")
	idx.WriteString("```json\n")
	idx.WriteString("{\n")
	idx.WriteString("  \"name\": \"my-plugin\",\n")
	idx.WriteString("  \"author\": \"Alice\",\n")
	idx.WriteString("  \"version\": \"1.0.0\",\n")
	idx.WriteString("  \"description\": \"Does cool things\",\n")
	idx.WriteString("  \"update_url\": \"https://example.com/my-plugin.zip\",\n")
	idx.WriteString("  \"entrypoint\": \"main.lua\",\n")
	idx.WriteString("  \"relation\": \"client\"\n")
	idx.WriteString("}\n")
	idx.WriteString("```\n\n")
	idx.WriteString("The `relation` field may be `\"client\"`, `\"server\"`, or `[\"client\",\"server\"]`.\n\n")
	idx.WriteString("## Getting Started\n\n")
	idx.WriteString("- [Development Guide](dev-guide.md)\n\n")
	idx.WriteString("## Plugin API Modules\n\n")
	idx.WriteString("See the [Plugin API Overview](api-overview.md) for a summary of all modules,\n")
	idx.WriteString("or jump directly to a module below:\n\n")
	for _, mod := range modules {
		fmt.Fprintf(&idx, "- [%s](api/%s.md)\n", mod.Name, mod.Name)
	}

	if err := os.WriteFile(filepath.Join(outDir, "index.md"), []byte(idx.String()), 0644); err != nil {
		return err
	}

	// Write dev-guide.md
	guide := generateDevGuide(modules)
	if err := os.WriteFile(filepath.Join(outDir, "dev-guide.md"), []byte(guide), 0644); err != nil {
		return err
	}

	// Write api-overview.md
	overview := generateAPIOverview(modules)
	if err := os.WriteFile(filepath.Join(outDir, "api-overview.md"), []byte(overview), 0644); err != nil {
		return err
	}

	// Ensure api/ subdirectory exists.
	apiDir := filepath.Join(outDir, "api")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		return err
	}

	// Write one markdown file per module into api/.
	for _, mod := range modules {
		doc := generateModuleDoc(mod)
		if err := os.WriteFile(filepath.Join(apiDir, mod.Name+".md"), []byte(doc), 0644); err != nil {
			return err
		}
	}

	return nil
}

func generateAPIOverview(modules []LuaModuleDef) string {
	var b strings.Builder
	b.WriteString("---\ntitle: Plugin API Overview\n---\n\n")
	b.WriteString("# Plugin API Overview\n\n")
	b.WriteString("This page summarises every module and function available to plugins.\n")
	b.WriteString("For detailed argument descriptions, click through to the individual module pages.\n\n")

	for _, mod := range modules {
		fmt.Fprintf(&b, "## `%s` module\n\n", mod.Name)
		fmt.Fprintf(&b, "%s\n\n", mod.Desc)
		for _, fn := range mod.Funcs {
			fmt.Fprintf(&b, "- `%s` — %s\n", fn.Sig, fn.Desc)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func generateModuleDoc(mod LuaModuleDef) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\ntitle: %s module\ndescription: %s\n---\n\n", mod.Name, mod.Desc)
	fmt.Fprintf(&b, "# %s module\n\n", mod.Name)
	fmt.Fprintf(&b, "%s\n\n", mod.Desc)

	for _, fn := range mod.Funcs {
		fmt.Fprintf(&b, "## `%s`\n\n", fn.Sig)
		fmt.Fprintf(&b, "%s\n\n", fn.Desc)

		if len(fn.Args) > 0 {
			b.WriteString("### Arguments\n\n")
			b.WriteString("| Name | Type | Description |\n")
			b.WriteString("|------|------|-------------|\n")
			for _, a := range fn.Args {
				fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", a.Name, a.Type, a.Desc)
			}
			b.WriteString("\n")
		}

		if len(fn.Returns) > 0 {
			b.WriteString("### Returns\n\n")
			b.WriteString("| Name | Type | Description |\n")
			b.WriteString("|------|------|-------------|\n")
			for _, r := range fn.Returns {
				fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", r.Name, r.Type, r.Desc)
			}
			b.WriteString("\n")
		}

		if fn.Example != "" {
			b.WriteString("### Example\n\n")
			b.WriteString("```lua\n")
			b.WriteString(fn.Example)
			b.WriteString("\n```\n\n")
		}
	}

	return b.String()
}

func generateDevGuide(modules []LuaModuleDef) string {
	var b strings.Builder
	b.WriteString("---\ntitle: Plugin Development Guide\n---\n\n")
	b.WriteString("# Plugin Development Guide\n\n")
	b.WriteString("This guide walks you through setting up your development environment,\n")
	b.WriteString("creating a plugin, testing it, and publishing it.\n\n")

	// 1. Prerequisites
	b.WriteString("## 1. Prerequisites\n\n")
	b.WriteString("- **VS Code** — any recent version.\n")
	b.WriteString("- **Lua extension by Sumneko** — install from the VS Code marketplace.\n")
	b.WriteString("- **sing-box-ez binary** — you need the app binary to generate definitions\n")
	b.WriteString("  and test plugins.\n\n")

	// 2. Set up VS Code IntelliSense
	b.WriteString("## 2. Set up VS Code IntelliSense\n\n")
	b.WriteString("The app can generate **EmmyLua / LuaCATS definition files** directly from\n")
	b.WriteString("the real Go API code so you get autocompletion, hover docs and diagnostics.\n\n")
	b.WriteString("```bash\n# Generate definitions into docs/plugin-defs/\nsing-box-ez defs\n\n# Or via make\nmake defs\n```\n\n")
	b.WriteString("This produces:\n")
	b.WriteString("```\nplugin-defs/\n├── globals.lua      -- declares http, log, ui, config globals\n├── http.lua\n├── log.lua\n├── ui.lua\n├── config.lua\n└── .luarc.json      -- VS Code workspace config\n```\n\n")
	b.WriteString("**Option A** — copy `.luarc.json` into the root of your plugin project.\n\n")
	b.WriteString("**Option B** — add the defs folder to VS Code `settings.json`:\n")
	b.WriteString("```json\n\"Lua.workspace.library\": [\"docs/plugin-defs\"]\n```\n\n")
	b.WriteString("Now when you edit `main.lua` you will see:\n")
	b.WriteString("- Autocomplete for all API functions\n")
	b.WriteString("- Hover tooltips with signatures and descriptions\n")
	b.WriteString("- Parameter hints\n\n")

	// 3. Create a plugin template
	b.WriteString("## 3. Create a plugin template\n\n")
	b.WriteString("The app can generate a starter plugin with the correct structure\n")
	b.WriteString("and up-to-date API examples.\n\n")
	b.WriteString("### CLI\n")
	b.WriteString("```bash\nsing-box-ez template my-plugin client\n```\n")
	b.WriteString("Valid relations: `client`, `server`, `both`.\n\n")
	b.WriteString("### GUI\n")
	b.WriteString("Open the **Plugins** tab → click **Generate Template** → fill name and relation.\n\n")
	b.WriteString("This creates a new folder under `sing-box-ez-data/plugins/my-plugin/`:\n")
	b.WriteString("```\nmy-plugin/\n├── manifest.json\n└── main.lua\n```\n\n")
	b.WriteString("### `manifest.json`\n")
	b.WriteString("```json\n")
	b.WriteString("{\n")
	b.WriteString("  \"name\": \"my-plugin\",\n")
	b.WriteString("  \"author\": \"Your Name\",\n")
	b.WriteString("  \"version\": \"1.0.0\",\n")
	b.WriteString("  \"description\": \"Short description\",\n")
	b.WriteString("  \"entrypoint\": \"main.lua\",\n")
	b.WriteString("  \"update_url\": \"https://example.com/my-plugin.zip\",\n")
	b.WriteString("  \"relation\": \"client\"\n")
	b.WriteString("}\n")
	b.WriteString("```\n\n")
	b.WriteString("- `update_url` is **optional** — it points to a zip/tar.gz package containing\n")
	b.WriteString("  the latest version of your plugin. Required for auto-update.\n")
	b.WriteString("- `relation` can be `\"client\"`, `\"server\"`, or `[\"client\",\"server\"]`.\n\n")

	// 4. Write plugin code
	b.WriteString("## 4. Write plugin code\n\n")
	b.WriteString("Edit `main.lua`. All examples below are taken from the current API:\n\n")
	b.WriteString("### Hello World\n")
	b.WriteString("```lua\n")
	b.WriteString("log.info(\"my-plugin loaded\")\n")
	b.WriteString("\n")
	b.WriteString("local id = ui.add_tab(\"My Plugin\")\n")
	b.WriteString("ui.add_label(id, \"Hello from Lua!\")\n")
	b.WriteString("ui.add_button(id, \"Click me\", function()\n")
	b.WriteString("  log.info(\"button clicked\")\n")
	b.WriteString("end)\n")
	b.WriteString("```\n\n")
	b.WriteString("### Config management\n")
	b.WriteString("```lua\n")
	b.WriteString("-- Add a subscription config owned by this plugin\n")
	b.WriteString("local err = config.add(\"my-sub\", \"https://example.com/sub\", 12)\n")
	b.WriteString("if err then\n")
	b.WriteString("  log.error(\"add failed: \" .. err)\n")
	b.WriteString("end\n")
	b.WriteString("\n")
	b.WriteString("-- Download and cache it\n")
	b.WriteString("err = config.download(\"my-sub\")\n")
	b.WriteString("if not err then\n")
	b.WriteString("  config.set_active(\"my-sub\")\n")
	b.WriteString("end\n")
	b.WriteString("```\n\n")
	b.WriteString("### HTTP requests\n")
	b.WriteString("```lua\n")
	b.WriteString("local body, err = http.get(\"https://api.ip.sb/ip\")\n")
	b.WriteString("if err then\n")
	b.WriteString("  log.error(\"request failed: \" .. err)\n")
	b.WriteString("else\n")
	b.WriteString("  log.info(\"public IP: \" .. body)\n")
	b.WriteString("end\n")
	b.WriteString("```\n\n")

	// 5. Test the plugin
	b.WriteString("## 5. Test the plugin\n\n")
	b.WriteString("Place your plugin folder in the app's plugin directory and restart the app:\n")
	b.WriteString("```bash\n")
	b.WriteString("cp -r my-plugin sing-box-ez-data/plugins/\n")
	b.WriteString("# Then restart sing-box-ez GUI\n")
	b.WriteString("```\n\n")
	b.WriteString("In the **Plugins** tab you will see your plugin listed.\n")
	b.WriteString("- **Double-click** a plugin row to toggle it on/off.\n")
	b.WriteString("- **Right-click** to open the info panel with full manifest details.\n\n")

	// 6. Generate API docs
	b.WriteString("## 6. Generate API documentation\n\n")
	b.WriteString("The docs are auto-generated from the real Go source code, so they are always\n")
	b.WriteString("in sync with the binary you are running.\n\n")
	b.WriteString("```bash\n")
	b.WriteString("# Generate markdown docs\nsing-box-ez docs\n\n")
	b.WriteString("# Serve with mkdocs (requires mkdocs installed)\nmake docs\n```\n\n")
	b.WriteString("If you don't have `mkdocs`, install it first:\n")
	b.WriteString("```bash\npip install mkdocs\n```\n\n")

	// 7. Publishing
	b.WriteString("## 7. Publishing a plugin\n\n")
	b.WriteString("Plugins can be distributed as **zip** or **tar.gz** packages.\n\n")
	b.WriteString("### Package format\n")
	b.WriteString("```\nmy-plugin.zip\n└── my-plugin/\n    ├── manifest.json\n    └── main.lua\n```\n\n")
	b.WriteString("The archive may contain a single top-level folder (the plugin name)\n")
	b.WriteString("or the files directly at the root.\n\n")
	b.WriteString("### Install from URL\n")
	b.WriteString("Users can install your plugin via URL:\n\n")
	b.WriteString("```bash\n")
	b.WriteString("sing-box-ez install https://example.com/my-plugin.zip\n")
	b.WriteString("```\n\n")
	b.WriteString("Or via GUI: **Plugins** tab → **Install from URL**.\n\n")
	b.WriteString("### Auto-update\n")
	b.WriteString("Set `update_url` in `manifest.json` to a URL that returns the latest\n")
	b.WriteString("`manifest.json`. The app can then check for updates via\n")
	b.WriteString("**Check Update** or **Check All Updates**.\n\n")
	b.WriteString("> **Note:** Folder plugins (created manually on disk) cannot be auto-updated.\n")
	b.WriteString("> Only package-installed plugins support update checking.\n\n")

	// 8. Reference
	b.WriteString("## 8. API Reference\n\n")
	b.WriteString("See the [Plugin API Overview](api-overview.md) for a summary of all modules,\n")
	b.WriteString("or browse the detailed module pages:\n\n")
	for _, mod := range modules {
		fmt.Fprintf(&b, "- [%s](api/%s.md)\n", mod.Name, mod.Name)
	}

	return b.String()
}

// writeFile utility.
func writeFile(path string, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// CopyFile copies src to dst.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
