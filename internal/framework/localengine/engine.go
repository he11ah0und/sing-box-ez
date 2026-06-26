// Package localengine provides a tiny in-memory localization engine.
// It reads nested YAML locale files and supports lookup by path segments,
// e.g. localengine.T("about", "btn", "open_repo").
package localengine

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"sing-box-ez/internal/framework/base"
	"sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/logger"

	yamlutil "sing-box-ez/internal/framework/util/yaml"
)

var (
	bundles     = make(map[string]map[string]any)
	currentLang = "en"
	baseLogger  base.Base
)

// SetLogger sets the logger used by the localengine package.
// It allocates a "localengine" terminal from the given parent terminal.
// It should be called once during application initialization instead of
// assigning the package-level Log variable directly.
func SetLogger(parent *logger.LogTerminal) {
	baseLogger.Init(parent, "localengine")
}

// LoadFromDir reads all *.yaml files from dir and parses them as locales.
func LoadFromDir(dir fs.Directory) error {
	entries, err := dir.ReadDir()
	if err != nil {
		return baseLogger.Errorf("read locale dir %q: %w", dir.Path(), err)
	}
	loaded := 0
	for _, e := range entries {
		if _, ok := e.(fs.Directory); ok {
			continue
		}
		file, ok := e.(fs.File)
		if !ok {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := file.Read()
		if err != nil {
			return baseLogger.Errorf("read locale %s: %w", e.Name(), err)
		}
		lang := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		baseLogger.Debugf("loading locale %s", lang)
		if err := loadLanguage(lang, data); err != nil {
			return baseLogger.Errorf("load locale %s: %w", e.Name(), err)
		}
		loaded++
	}
	baseLogger.Infof("loaded %d locale(s) from %s", loaded, dir.Path())
	validateLocales()
	return nil
}

// LoadFromOSDir reads all *.yaml files from dir on the local filesystem.
func LoadFromOSDir(dir string) error {
	return LoadFromDir(fs.NewOS(dir).Root())
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
	baseLogger.WarnMissing(path)
	return joinPath(path)
}

// SetLanguage sets the active language. Falls back to English for unknown codes.
func SetLanguage(code string) {
	if _, ok := bundles[code]; ok {
		currentLang = code
		baseLogger.Infof("language set to %s", code)
	} else {
		currentLang = "en"
		baseLogger.Warnf("language %s not available, falling back to en", code)
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
		baseLogger.Debugf("no LANG/LC_ALL set, defaulting to en")
		return "en"
	}
	if idx := strings.Index(lang, "."); idx != -1 {
		lang = lang[:idx]
	}
	base, _, _ := strings.Cut(lang, "_")
	if slices.Contains(AvailableLanguages(), base) {
		baseLogger.Debugf("detected system language: %s", base)
		return base
	}
	baseLogger.Debugf("system language %s not available, defaulting to en", base)
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

// validateLocales compares all loaded locale bundles and warns about missing
// keys. It is called automatically after LoadFromDir finishes.
func validateLocales() {
	if len(bundles) == 0 {
		return
	}

	paths := make(map[string]map[string]struct{})
	for lang, tree := range bundles {
		collectKeys(tree, nil, func(path []string) {
			dotted := joinPath(path)
			if paths[dotted] == nil {
				paths[dotted] = make(map[string]struct{})
			}
			paths[dotted][lang] = struct{}{}
		})
	}

	langs := make([]string, 0, len(bundles))
	for lang := range bundles {
		langs = append(langs, lang)
	}
	slices.Sort(langs)

	dotted := make([]string, 0, len(paths))
	for k := range paths {
		dotted = append(dotted, k)
	}
	slices.Sort(dotted)

	for _, key := range dotted {
		present := paths[key]
		if len(present) == len(bundles) {
			continue
		}
		missing := make([]string, 0, len(bundles))
		for _, lang := range langs {
			if _, ok := present[lang]; !ok {
				missing = append(missing, lang)
			}
		}
		baseLogger.Warnf("locale key %q missing in: %s", key, strings.Join(missing, ", "))
	}
}

// collectKeys walks a locale tree and calls yield for every leaf path.
func collectKeys(tree map[string]any, path []string, yield func([]string)) {
	for k, v := range tree {
		p := keyPath(path, k)
		if sub, ok := v.(map[string]any); ok {
			collectKeys(sub, p, yield)
		} else {
			yield(p)
		}
	}
}

// keyPath returns a new path slice with key appended.
func keyPath(path []string, key string) []string {
	p := make([]string, len(path)+1)
	copy(p, path)
	p[len(path)] = key
	return p
}
