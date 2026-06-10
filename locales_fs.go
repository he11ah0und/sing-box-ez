package main

import "embed"

//go:embed locales/*.yaml
var localesFS embed.FS
