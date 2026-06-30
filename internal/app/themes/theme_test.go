package themes

import (
	"testing"
)

func TestLoadDefaultTheme(t *testing.T) {
	c, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	theme, ok := c.Get("default")
	if !ok {
		t.Fatal("default theme not found")
	}
	if theme.Name != "default" {
		t.Fatalf("unexpected theme name: %s", theme.Name)
	}
	if len(theme.Dark) == 0 || len(theme.Light) == 0 {
		t.Fatal("default theme missing palettes")
	}
}

func TestResolveSystemUsesDarkOrLight(t *testing.T) {
	c, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	dark := c.Resolve("default", "system", true)
	if dark["bg"] != "#121212" {
		t.Fatalf("unexpected dark bg: %s", dark["bg"])
	}

	light := c.Resolve("default", "system", false)
	if light["bg"] != "#FAFAFA" {
		t.Fatalf("unexpected light bg: %s", light["bg"])
	}
}

func TestKeyAliases(t *testing.T) {
	c, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	colors := c.Resolve("default", "dark", true)
	for _, key := range []string{"text", "text-muted", "surface-variant", "primary-hover", "danger"} {
		if _, ok := colors[key]; !ok {
			t.Fatalf("expected key %q in resolved colors", key)
		}
	}
}

func TestParseHex(t *testing.T) {
	cases := map[string]RGBA{
		"#fff":       {R: 255, G: 255, B: 255, A: 255},
		"#1234":      {R: 17, G: 34, B: 51, A: 68},
		"#AABBCC":    {R: 170, G: 187, B: 204, A: 255},
		"#AABBCCDD":  {R: 170, G: 187, B: 204, A: 221},
	}
	for in, want := range cases {
		got, err := ParseHex(in)
		if err != nil {
			t.Fatalf("ParseHex(%q) error: %v", in, err)
		}
		if got != want {
			t.Fatalf("ParseHex(%q) = %+v, want %+v", in, got, want)
		}
	}
}
