package app

import (
	"io/fs"
	"slices"
	"testing"

	"github.com/he11ah0und/localengine"
)

// TestLocalesPlatformParity loads the embedded UI locales for every supported
// platform and requires the merged key sets of all languages to be identical.
// Platform overlay files (<lang>.<goos>.yaml) are merged over the base bundle
// on their platform and skipped elsewhere, so this catches overlays that
// drift between languages on any platform — not just the host's.
func TestLocalesPlatformParity(t *testing.T) {
	sub, err := fs.Sub(localesFS, "locales")
	if err != nil {
		t.Fatalf("sub: %v", err)
	}
	langs := []string{"en", "ru", "zh"}
	for _, goos := range []string{"linux", "windows", "darwin"} {
		t.Run(goos, func(t *testing.T) {
			store := localengine.New()
			if err := store.LoadFromDirPlatform(sub, goos); err != nil {
				t.Fatalf("load: %v", err)
			}
			reference := sorted(store.LeafPaths(langs[0]))
			for _, lang := range langs[1:] {
				got := sorted(store.LeafPaths(lang))
				if !slices.Equal(reference, got) {
					missing, extra := diffKeys(reference, got)
					t.Errorf("%s key set differs from %s: missing %v, extra %v", lang, langs[0], missing, extra)
				}
			}
		})
	}
}

// TestLocalesPlatformOverlays pins which keys live in platform overlays:
// linux gets setcap, windows gets restart-as-admin, darwin gets neither.
func TestLocalesPlatformOverlays(t *testing.T) {
	sub, err := fs.Sub(localesFS, "locales")
	if err != nil {
		t.Fatalf("sub: %v", err)
	}
	setcap := "settings.privileges.setcap"
	restartAdmin := "settings.privileges.restart_admin"
	cases := []struct {
		goos           string
		wantSetcap     bool
		wantRestartAdm bool
	}{
		{"linux", true, false},
		{"windows", false, true},
		{"darwin", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.goos, func(t *testing.T) {
			store := localengine.New()
			if err := store.LoadFromDirPlatform(sub, tc.goos); err != nil {
				t.Fatalf("load: %v", err)
			}
			_, hasSetcap := store.LookupString("en", "settings", "privileges", "setcap")
			if hasSetcap != tc.wantSetcap {
				t.Errorf("key %s present=%v, want %v", setcap, hasSetcap, tc.wantSetcap)
			}
			_, hasRestart := store.LookupString("en", "settings", "privileges", "restart_admin")
			if hasRestart != tc.wantRestartAdm {
				t.Errorf("key %s present=%v, want %v", restartAdmin, hasRestart, tc.wantRestartAdm)
			}
		})
	}
}

func sorted(keys []string) []string {
	out := slices.Clone(keys)
	slices.Sort(out)
	return out
}

// diffKeys returns the keys missing from and extra in got relative to ref;
// both inputs must be sorted.
func diffKeys(ref, got []string) (missing, extra []string) {
	refSet := make(map[string]bool, len(ref))
	for _, k := range ref {
		refSet[k] = true
	}
	gotSet := make(map[string]bool, len(got))
	for _, k := range got {
		gotSet[k] = true
		if !refSet[k] {
			extra = append(extra, k)
		}
	}
	for _, k := range ref {
		if !gotSet[k] {
			missing = append(missing, k)
		}
	}
	return missing, extra
}
