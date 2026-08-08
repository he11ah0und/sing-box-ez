//go:build !nogui

package wails

import (
	"fmt"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/he11ah0und/localengine"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/singboxconfig"
)

// Toast is a user-facing notification emitted by the backend. The frontend
// only displays it; toast text is always composed and localized in Go.
type Toast struct {
	Level       string `json:"level"` // info | success | warning | error
	Text        string `json:"text"`
	Description string `json:"description,omitempty"`
}

// toast emits a toast event with an already localized message.
func (b *Bindings) toast(level, text, description string) {
	b.emit("toast", Toast{Level: level, Text: text, Description: description})
}

// t resolves a locale key for the current language, falling back to English
// and finally to the key itself (mirroring the frontend missing-key rule).
func (b *Bindings) t(path ...string) string {
	if v, ok := localengine.LookupString(localengine.CurrentLanguage(), path...); ok {
		return v
	}
	if v, ok := localengine.LookupString("en", path...); ok {
		return v
	}
	return strings.Join(path, ".")
}

// toastT emits a localized toast. The locale value may contain fmt verbs.
func (b *Bindings) toastT(level string, path []string, args ...any) {
	text := b.t(path...)
	if len(args) > 0 {
		text = fmt.Sprintf(text, args...)
	}
	b.toast(level, text, "")
}

// toastErr emits an error toast for a failed action. When actionPath is
// given, the localized action label prefixes the raw (English) error text.
func (b *Bindings) toastErr(err error, actionPath ...string) {
	text := err.Error()
	if len(actionPath) > 0 {
		text = b.t(actionPath...) + ": " + text
	}
	b.toast("error", text, "")
}

// CopyLogs copies the app or core log buffer ("app"/"core") to the clipboard
// as plain text and reports it with a toast.
func (b *Bindings) CopyLogs(source string) {
	var lines []string
	if source == "core" {
		lines = b.app.Controller.GetCoreLogCleanLines()
	} else {
		raw := b.app.Controller.GetLogLines()
		lines = make([]string, len(raw))
		for i, line := range raw {
			lines[i] = core.StripANSIEscapes(line)
		}
	}
	if app := application.Get(); app != nil {
		app.Clipboard.SetText(strings.Join(lines, "\n"))
	}
	b.toastT("success", []string{"log", "copied"})
}

// CopyValidationReport formats the validation result of the named profile as
// localized plain text, copies it to the clipboard and reports it with a
// toast.
func (b *Bindings) CopyValidationReport(name string) {
	result, err := b.app.Controller.ValidateConfig(name)
	if err != nil {
		b.toastErr(err)
		return
	}
	if app := application.Get(); app != nil {
		app.Clipboard.SetText(b.validationReportText(result))
	}
	b.toastT("success", []string{"validation", "copied"})
}

// validationReportText renders the validation result as localized plain text
// (the same layout the frontend used for clipboard copy).
func (b *Bindings) validationReportText(r singboxconfig.ValidationResult) string {
	fieldLine := func(f singboxconfig.DeprecatedField) string {
		parts := []string{f.Path}
		if f.Deprecated != "" {
			parts = append(parts, b.t("validation", "field", "deprecated")+" "+f.Deprecated)
		}
		if f.Removed != "" {
			parts = append(parts, b.t("validation", "field", "removed")+" "+f.Removed)
		}
		if f.Replacement != "" {
			parts = append(parts, b.t("validation", "field", "replacement")+": "+f.Replacement)
		}
		return strings.Join(parts, " — ")
	}
	var lines []string
	if len(r.Errors) > 0 {
		lines = append(lines, fmt.Sprintf(b.t("validation", "errors_title"), len(r.Errors)))
		for _, f := range r.Errors {
			lines = append(lines, "- "+fieldLine(f))
		}
	}
	if len(r.Warnings) > 0 {
		lines = append(lines, fmt.Sprintf(b.t("validation", "warnings_title"), len(r.Warnings)))
		for _, f := range r.Warnings {
			lines = append(lines, "- "+fieldLine(f))
		}
	}
	if len(r.Info) > 0 {
		lines = append(lines, b.t("validation", "info_title"))
		for _, i := range r.Info {
			lines = append(lines, "- "+i)
		}
	}
	if len(lines) == 0 {
		lines = append(lines, b.t("validation", "ok"))
	}
	return strings.Join(lines, "\n")
}
