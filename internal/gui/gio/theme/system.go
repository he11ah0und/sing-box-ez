package theme

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// SystemVariant tries to detect the current OS light/dark preference.
func SystemVariant() Variant {
	v := detectSystemVariant()
	if v != "" {
		return v
	}
	return VariantDark
}

func detectSystemVariant() Variant {
	switch runtime.GOOS {
	case "windows":
		return detectWindowsVariant()
	case "darwin":
		return detectMacVariant()
	default:
		return detectUnixVariant()
	}
}

func detectUnixVariant() Variant {
	theme := strings.ToLower(os.Getenv("GTK_THEME"))
	if strings.Contains(theme, "dark") {
		return VariantDark
	}
	if strings.Contains(theme, "light") {
		return VariantLight
	}
	// Try gsettings as a best-effort fallback on GTK desktops.
	if out, err := exec.Command("gsettings", "get", "org.gnome.desktop.interface", "color-scheme").Output(); err == nil {
		s := strings.ToLower(strings.Trim(string(out), "'\"\n "))
		if strings.Contains(s, "dark") {
			return VariantDark
		}
		if strings.Contains(s, "light") {
			return VariantLight
		}
	}
	return ""
}

func detectWindowsVariant() Variant {
	// HKCU\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize
	// AppsUseLightTheme: 0 = dark, 1 = light.
	out, err := exec.Command("reg", "query",
		"HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Themes\\Personalize",
		"/v", "AppsUseLightTheme").Output()
	if err != nil {
		return ""
	}
	if strings.Contains(string(out), "0x0") {
		return VariantDark
	}
	return VariantLight
}

func detectMacVariant() Variant {
	out, err := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").Output()
	if err != nil {
		// defaults returns error when style is not set (light mode).
		return VariantLight
	}
	if strings.TrimSpace(strings.ToLower(string(out))) == "dark" {
		return VariantDark
	}
	return VariantLight
}
