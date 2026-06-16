package localengine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sing-box-ez/internal/framework/fs"
)

func resetBundles() {
	bundles = make(map[string]map[string]any)
}

// newTestFS writes the given file contents to a temporary directory and
// returns an OSFileSystem rooted there.
func newTestFS(t *testing.T, files map[string]string) fs.FileSystem {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0644); err != nil {
			t.Fatalf("write test file %s: %v", name, err)
		}
	}
	return fs.NewOSFileSystem(dir)
}

// slashSensitiveFS fails any ReadFile call that uses a backslash separator.
// This reproduces the Windows/embed.FS behaviour on any OS.
type slashSensitiveFS struct {
	fs.FileSystem
	t *testing.T
}

func (s *slashSensitiveFS) ReadFile(name string) ([]byte, error) {
	if strings.Contains(name, "\\") {
		return nil, fmt.Errorf("backslash path %q not allowed", name)
	}
	return s.FileSystem.ReadFile(name)
}

func TestLoadFromFSUsesForwardSlashes(t *testing.T) {
	dir := t.TempDir()
	localesDir := filepath.Join(dir, "locales")
	if err := os.MkdirAll(localesDir, 0750); err != nil {
		t.Fatalf("mkdir locales: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localesDir, "en.yaml"), []byte("key: value\n"), 0644); err != nil {
		t.Fatalf("write en.yaml: %v", err)
	}

	resetBundles()
	wrapped := &slashSensitiveFS{FileSystem: fs.NewOSFileSystem(dir), t: t}
	if err := LoadFromFS(wrapped, "locales"); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	if got := T("key"); got != "value" {
		t.Errorf("expected 'value', got %q", got)
	}
}

func TestSetLanguage(t *testing.T) {
	fsys := newTestFS(t, map[string]string{
		"en.yaml": "tab:\n  settings: Settings\nlocale:\n  name: English\n",
		"ru.yaml": "tab:\n  settings: Настройки\nlocale:\n  name: Русский\n",
		"zh.yaml": "tab:\n  settings: 设置\nlocale:\n  name: 中文\n",
	})

	resetBundles()
	if err := LoadFromFS(fsys, ""); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	t.Logf("Before: %s", T("tab", "settings"))
	SetLanguage("ru")
	t.Logf("After ru: %s", T("tab", "settings"))
	SetLanguage("zh")
	t.Logf("After zh: %s", T("tab", "settings"))

	if got := T("tab", "settings"); got != "设置" {
		t.Errorf("expected Chinese '设置', got %q", got)
	}
}

func TestNestedKeys(t *testing.T) {
	fsys := newTestFS(t, map[string]string{
		"en.yaml": "about:\n  btn:\n    open_repo: Open repo\n    open_data: Open data\n",
	})

	resetBundles()
	if err := LoadFromFS(fsys, ""); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	if got := T("about", "btn", "open_repo"); got != "Open repo" {
		t.Errorf("expected 'Open repo', got %q", got)
	}
	if got := T("about", "btn", "open_data"); got != "Open data" {
		t.Errorf("expected 'Open data', got %q", got)
	}
}

func TestFallbackToEnglish(t *testing.T) {
	fsys := newTestFS(t, map[string]string{
		"en.yaml": "only:\n  english: Hello\n",
		"ru.yaml": "other: Другое\n",
	})

	resetBundles()
	if err := LoadFromFS(fsys, ""); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	SetLanguage("ru")
	if got := T("only", "english"); got != "Hello" {
		t.Errorf("expected English fallback 'Hello', got %q", got)
	}
}

func TestMissingKeyReturnsPath(t *testing.T) {
	resetBundles()
	bundles["en"] = map[string]any{" existing": map[string]any{"key": "value"}}

	if got := T("missing", "key"); got != "missing.key" {
		t.Errorf("expected dotted path fallback, got %q", got)
	}
}

func TestLanguageName(t *testing.T) {
	fsys := newTestFS(t, map[string]string{
		"en.yaml": "locale:\n  name: English\n",
		"ru.yaml": "locale:\n  name: Русский\n",
	})

	resetBundles()
	if err := LoadFromFS(fsys, ""); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	if got := LanguageName("ru"); got != "Русский" {
		t.Errorf("expected 'Русский', got %q", got)
	}
	if got := LanguageName("unknown"); got != "unknown" {
		t.Errorf("expected fallback code, got %q", got)
	}
}

func TestDetectSystemLanguage(t *testing.T) {
	resetBundles()
	bundles["en"] = map[string]any{}
	bundles["ru"] = map[string]any{}
	bundles["zh"] = map[string]any{}

	orig := os.Getenv("LANG")
	defer os.Setenv("LANG", orig)

	os.Setenv("LANG", "ru_RU.UTF-8")
	if got := DetectSystemLanguage(); got != "ru" {
		t.Errorf("expected 'ru', got %q", got)
	}

	os.Setenv("LANG", "unknown_LANG")
	if got := DetectSystemLanguage(); got != "en" {
		t.Errorf("expected fallback 'en', got %q", got)
	}
}

func TestLoadFromDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "en.yaml"), []byte("key: value\n"), 0644); err != nil {
		t.Fatal(err)
	}

	resetBundles()
	if err := LoadFromDir(dir); err != nil {
		t.Fatalf("load from dir: %v", err)
	}

	if got := T("key"); got != "value" {
		t.Errorf("expected 'value', got %q", got)
	}
}
