// Package i18n provides internationalization support via go-i18n/v2.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localeFS embed.FS

var bundle *i18n.Bundle
var localizer *i18n.Localizer

func init() {
	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	// Load all locale files.
	files, err := localeFS.ReadDir("locales")
	if err != nil {
		panic(fmt.Sprintf("failed to read locale dir: %v", err))
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		data, err := localeFS.ReadFile("locales/" + f.Name())
		if err != nil {
			panic(fmt.Sprintf("failed to read locale %s: %v", f.Name(), err))
		}
		bundle.MustParseMessageFileBytes(data, f.Name())
	}
}

// AvailableLanguages returns the list of supported language codes.
func AvailableLanguages() []string {
	return []string{"en", "ru", "zh"}
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
	// Strip encoding suffix, e.g. "en_US.UTF-8" -> "en_US"
	if idx := strings.Index(lang, "."); idx != -1 {
		lang = lang[:idx]
	}
	base, _, _ := strings.Cut(lang, "_")
	if slices.Contains(AvailableLanguages(), base) {
		return base
	}
	return "en"
}

// SetLanguage sets the active language for the application.
// Falls back to English for unknown codes.
func SetLanguage(code string) {
	tag := language.MustParse(code)
	localizer = i18n.NewLocalizer(bundle, tag.String())
}

// T returns the localized string for the given message ID.
// Optional template data can be passed for interpolation.
func T(id string, data ...map[string]any) string {
	if localizer == nil {
		SetLanguage("en")
	}
	var tmplData map[string]any
	if len(data) > 0 {
		tmplData = data[0]
	}
	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    id,
		TemplateData: tmplData,
	})
	if err != nil {
		return id // fallback to message ID on error
	}
	return msg
}
