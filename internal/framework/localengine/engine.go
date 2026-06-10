// Package localengine provides a tiny in-memory localization engine.
// It reads nested YAML locale files, flattens keys with dots, and supports
// text/template interpolation (e.g. {{.Name}}).
package localengine

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

var (
	bundles     = make(map[string]map[string]string)
	currentLang = "en"
)

// LoadFromFS reads all *.yaml files from dir inside fs and parses them as locales.
func LoadFromFS(fsys fs.FS, dir string) error {
	files, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return fmt.Errorf("read locale dir %q: %w", dir, err)
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if !strings.HasSuffix(f.Name(), ".yaml") {
			continue
		}
		data, err := fs.ReadFile(fsys, filepath.Join(dir, f.Name()))
		if err != nil {
			return fmt.Errorf("read locale %s: %w", f.Name(), err)
		}
		lang := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
		if err := loadLanguage(lang, data); err != nil {
			return fmt.Errorf("load locale %s: %w", f.Name(), err)
		}
	}
	return nil
}

// LoadFromDir reads all *.yaml files from dir on the local filesystem.
func LoadFromDir(dir string) error {
	return LoadFromFS(os.DirFS(dir), ".")
}

func loadLanguage(lang string, data []byte) error {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	bundles[lang] = flatten(raw, "")
	return nil
}

func flatten(src map[string]any, prefix string) map[string]string {
	dst := make(map[string]string)
	for k, v := range src {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case string:
			dst[key] = val
		case map[string]any:
			for nk, nv := range flatten(val, key) {
				dst[nk] = nv
			}
		default:
			// Non-string leaf values are stringified; this should not happen
			// for normal translation files but keeps the engine forgiving.
			dst[key] = fmt.Sprint(val)
		}
	}
	return dst
}

// T returns the localized string for the given message id.
// Optional template data can be passed for interpolation.
func T(id string, data ...map[string]any) string {
	msg, ok := bundles[currentLang][id]
	if !ok || msg == "" {
		msg = bundles["en"][id]
	}
	if msg == "" {
		return id
	}
	var tmplData map[string]any
	if len(data) > 0 {
		tmplData = data[0]
	}
	if tmplData == nil {
		return msg
	}
	tmpl, err := template.New("t").Parse(msg)
	if err != nil {
		return msg
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, tmplData); err != nil {
		return msg
	}
	return b.String()
}

// SetLanguage sets the active language. Falls back to English for unknown codes.
func SetLanguage(code string) {
	if _, ok := bundles[code]; ok {
		currentLang = code
	} else {
		currentLang = "en"
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
		return "en"
	}
	if idx := strings.Index(lang, "."); idx != -1 {
		lang = lang[:idx]
	}
	base, _, _ := strings.Cut(lang, "_")
	if slices.Contains(AvailableLanguages(), base) {
		return base
	}
	return "en"
}

// LanguageName returns the native name of the language for the given code
// (reads the "locale.name" key from that language's messages).
func LanguageName(code string) string {
	if b, ok := bundles[code]; ok {
		if name, ok := b["locale.name"]; ok && name != "" {
			return name
		}
	}
	return code
}
