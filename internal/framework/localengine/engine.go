// Package localengine provides a tiny in-memory localization engine.
// It reads nested YAML locale files and supports lookup by path segments,
// e.g. localengine.T("about", "btn", "open_repo").
package localengine

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/logger"

	yamlutil "sing-box-ez/internal/framework/util/yaml"
)

var (
	bundles     = make(map[string]map[string]any)
	currentLang = "en"
	logTerminal *logger.LogTerminal
)

// SetLogger sets the logger used by the localengine package.
// It allocates a "localengine" terminal from the given parent terminal.
// It should be called once during application initialization instead of
// assigning the package-level Log variable directly.
func SetLogger(parent *logger.LogTerminal) {
	if parent != nil {
		logTerminal = parent.Allocate("localengine")
	}
}

// LoadFromFS reads all *.yaml files from dir inside fsys and parses them as locales.
func LoadFromFS(fsys fs.FileSystem, dir string) error {
	files, err := fsys.ReadDir(dir)
	if err != nil {
		return logTerminal.Errorf("read locale dir %q: %w", dir, err)
	}
	loaded := 0
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if !strings.HasSuffix(f.Name(), ".yaml") {
			continue
		}
		data, err := fsys.ReadFile(path.Join(dir, f.Name()))
		if err != nil {
			return logTerminal.Errorf("read locale %s: %w", f.Name(), err)
		}
		lang := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
		logTerminal.Debugf("loading locale %s", lang)
		if err := loadLanguage(lang, data); err != nil {
			return logTerminal.Errorf("load locale %s: %w", f.Name(), err)
		}
		loaded++
	}
	logTerminal.Infof("loaded %d locale(s) from %s", loaded, dir)
	return nil
}

// LoadFromDir reads all *.yaml files from dir on the local filesystem.
func LoadFromDir(dir string) error {
	return LoadFromFS(fs.NewOSFileSystem(dir), "")
}

func loadLanguage(lang string, data []byte) error {
	raw, err := yamlutil.LoadTree(data)
	if err != nil {
		return err
	}
	bundles[lang] = raw
	return nil
}

// lookup walks the nested map for the requested path.
func lookup(tree map[string]any, path []string) (string, bool) {
	if len(path) == 0 {
		return "", false
	}
	current, ok := tree[path[0]]
	if !ok {
		return "", false
	}
	for _, p := range path[1:] {
		sub, ok := current.(map[string]any)
		if !ok {
			return "", false
		}
		current, ok = sub[p]
		if !ok {
			return "", false
		}
	}
	s, ok := current.(string)
	return s, ok
}

// joinPath reconstructs a dotted path for error fallback messages.
func joinPath(path []string) string {
	return strings.Join(path, ".")
}

// T returns the localized string for the given path segments.
// Falls back to English and finally to the dotted path itself.
func T(path ...string) string {
	if msg, ok := lookup(bundles[currentLang], path); ok && msg != "" {
		return msg
	}
	if msg, ok := lookup(bundles["en"], path); ok && msg != "" {
		return msg
	}
	return joinPath(path)
}

// SetLanguage sets the active language. Falls back to English for unknown codes.
func SetLanguage(code string) {
	if _, ok := bundles[code]; ok {
		currentLang = code
		logTerminal.Infof("language set to %s", code)
	} else {
		currentLang = "en"
		logTerminal.Warnf("language %s not available, falling back to en", code)
	}
}

// AvailableLanguages returns the list of loaded language codes.
func AvailableLanguages() []string {
	keys := make([]string, 0, len(bundles))
	for k := range bundles {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// DetectSystemLanguage tries to detect the system language from environment
// variables. Falls back to "en" if detection fails or language is unsupported.
func DetectSystemLanguage() string {
	lang := os.Getenv("LANG")
	if lang == "" {
		lang = os.Getenv("LC_ALL")
	}
	if lang == "" {
		logTerminal.Debugf("no LANG/LC_ALL set, defaulting to en")
		return "en"
	}
	if idx := strings.Index(lang, "."); idx != -1 {
		lang = lang[:idx]
	}
	base, _, _ := strings.Cut(lang, "_")
	if slices.Contains(AvailableLanguages(), base) {
		logTerminal.Debugf("detected system language: %s", base)
		return base
	}
	logTerminal.Debugf("system language %s not available, defaulting to en", base)
	return "en"
}

// LanguageName returns the native name of the language for the given code
// (reads locale.name from that language's messages).
func LanguageName(code string) string {
	if b, ok := bundles[code]; ok {
		if name, ok := lookup(b, []string{"locale", "name"}); ok && name != "" {
			return name
		}
	}
	return code
}
