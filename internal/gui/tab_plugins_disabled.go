//go:build noplugins

package gui

import "fyne.io/fyne/v2/container"

type pluginListItem struct {
	name          string
	version       string
	latestVersion string
	enabled       bool
	sourceType    string
	relations     []string
}

func (g *GUI) buildPluginsTab() *container.TabItem {
	return nil
}

func (g *GUI) initPlugins() {}

func (g *GUI) refreshPluginsList() {}
