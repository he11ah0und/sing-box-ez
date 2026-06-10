package localengine

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestSetLanguage(t *testing.T) {
	// Use a small in-memory FS so tests do not depend on the project root layout.
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("tab:\n  settings: Settings\nlocale:\n  name: English\n")},
		"ru.yaml": &fstest.MapFile{Data: []byte("tab:\n  settings: Настройки\nlocale:\n  name: Русский\n")},
		"zh.yaml": &fstest.MapFile{Data: []byte("tab:\n  settings: 设置\nlocale:\n  name: 中文\n")},
	}

	bundles = make(map[string]map[string]string)
	if err := LoadFromFS(fsys, "."); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	t.Logf("Before: %s", T("tab.settings"))
	SetLanguage("ru")
	t.Logf("After ru: %s", T("tab.settings"))
	SetLanguage("zh")
	t.Logf("After zh: %s", T("tab.settings"))

	if got := T("tab.settings"); got != "设置" {
		t.Errorf("expected Chinese '设置', got %q", got)
	}
}

func TestFlattenNestedKeys(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("about:\n  btn:\n    open_repo: Open repo\n    open_data: Open data\n")},
	}

	bundles = make(map[string]map[string]string)
	if err := LoadFromFS(fsys, "."); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	if got := T("about.btn.open_repo"); got != "Open repo" {
		t.Errorf("expected 'Open repo', got %q", got)
	}
	if got := T("about.btn.open_data"); got != "Open data" {
		t.Errorf("expected 'Open data', got %q", got)
	}
}

func TestTemplateInterpolation(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("greeting: \"Hello {{.Name}}\"\n")},
	}

	bundles = make(map[string]map[string]string)
	if err := LoadFromFS(fsys, "."); err != nil {
		t.Fatalf("load locales: %v", err)
	}

	got := T("greeting", map[string]any{"Name": "World"})
	if got != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", got)
	}
}

func TestLanguageName(t *testing.T) {
	fsys := fstest.MapFS{
		"en.yaml": &fstest.MapFile{Data: []byte("locale:\n  name: English\n")},
		"ru.yaml": &fstest.MapFile{Data: []byte("locale:\n  name: Русский\n")},
	}

	bundles = make(map[string]map[string]string)
	if err := LoadFromFS(fsys, "."); err != nil {
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
	bundles = map[string]map[string]string{"en": {}, "ru": {}, "zh": {}}

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

	bundles = make(map[string]map[string]string)
	if err := LoadFromDir(dir); err != nil {
		t.Fatalf("load from dir: %v", err)
	}

	if got := T("key"); got != "value" {
		t.Errorf("expected 'value', got %q", got)
	}
}
