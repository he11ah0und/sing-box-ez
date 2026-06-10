package i18n

import (
	"testing"
)

func TestSetLanguage(t *testing.T) {
	t.Logf("Before: %s", T("tab.settings"))
	SetLanguage("ru")
	t.Logf("After ru: %s", T("tab.settings"))
	SetLanguage("zh")
	t.Logf("After zh: %s", T("tab.settings"))
}
