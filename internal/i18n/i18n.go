// Package i18n provides internationalization support via go-i18n/v2.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"

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

// SetLanguage sets the active language for the application.
// Supported: "en", "ru". Falls back to English for unknown codes.
func SetLanguage(code string) {
	tag := language.MustParse(code)
	localizer = i18n.NewLocalizer(bundle, tag.String())
}

// T returns the localized string for the given message ID.
// Optional template data can be passed for interpolation.
func T(id string, data ...map[string]interface{}) string {
	if localizer == nil {
		SetLanguage("en")
	}
	var tmplData map[string]interface{}
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
