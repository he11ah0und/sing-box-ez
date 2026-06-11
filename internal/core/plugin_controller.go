package core

import (
	"fmt"
	"sing-box-ez/internal/framework/logger"
)

// PluginController manages plugin operations and logging.
type PluginController struct {
	terminal *logger.LogTerminal
}

// Terminal returns the logging terminal used by this controller.
func (c *PluginController) Terminal() *logger.LogTerminal {
	return c.terminal
}

// NewPluginController creates a new plugin controller.
func NewPluginController(terminal *logger.LogTerminal) *PluginController {
	return &PluginController{terminal: terminal}
}

// Discover discovers plugins and logs errors.
func (c *PluginController) Discover(pm interface{ Discover() error }) error {
	err := pm.Discover()
	if err != nil {
		c.terminal.Errorf("discover error: " + err.Error())
		return err
	}
	return nil
}

// Toggle toggles a plugin and logs the result.
func (c *PluginController) Toggle(pm interface{ Toggle(string) error }, name string) error {
	err := pm.Toggle(name)
	if err != nil {
		c.terminal.Errorf("toggle failed: " + err.Error())
		return err
	}
	c.terminal.Infof("toggled: " + name)
	return nil
}

// CheckUpdate checks for plugin updates and logs the result.
func (c *PluginController) CheckUpdate(pm interface {
	CheckUpdate(string) (bool, string, error)
}, name string) (bool, string, error) {
	hasUpdate, latest, err := pm.CheckUpdate(name)
	if err != nil {
		c.terminal.Errorf("update check failed for " + name + ": " + err.Error())
		return false, "", err
	}
	if hasUpdate {
		c.terminal.Infof("update available for " + name + ": v" + latest)
	} else {
		c.terminal.Infof("%s", name+" is up to date")
	}
	return hasUpdate, latest, nil
}

// InstallFromURL installs a plugin from URL and logs the result.
func (c *PluginController) InstallFromURL(pm interface{ InstallFromURL(string) error }, url string) error {
	if url == "" {
		c.terminal.Errorf("install: URL is required")
		return fmt.Errorf("URL is required")
	}
	err := pm.InstallFromURL(url)
	if err != nil {
		c.terminal.Errorf("install failed: " + err.Error())
		return err
	}
	c.terminal.Infof("installed from: " + url)
	return nil
}

// GenerateTemplate generates a plugin template using the provided function and logs the result.
func (c *PluginController) GenerateTemplate(generateFunc func(outDir, name, rel string) error, outDir, name, rel string) error {
	if name == "" {
		c.terminal.Errorf("template: name is required")
		return fmt.Errorf("name is required")
	}
	if rel == "" {
		rel = "client"
	}
	if err := generateFunc(outDir, name, rel); err != nil {
		c.terminal.Errorf("template generation failed: " + err.Error())
		return err
	}
	c.terminal.Infof("template generated: " + outDir)
	return nil
}

// GenerateDocs generates plugin API docs using the provided function and logs the result.
func (c *PluginController) GenerateDocs(generateFunc func(outDir string) error, outDir string) error {
	if err := generateFunc(outDir); err != nil {
		c.terminal.Errorf("docs generation failed: " + err.Error())
		return err
	}
	c.terminal.Infof("API docs generated: " + outDir)
	return nil
}

// GenerateDefs generates VS Code Lua definitions using the provided function and logs the result.
func (c *PluginController) GenerateDefs(generateFunc func(outDir string) error, outDir string) error {
	if err := generateFunc(outDir); err != nil {
		c.terminal.Errorf("defs generation failed: " + err.Error())
		return err
	}
	c.terminal.Infof("VS Code Lua defs generated: " + outDir)
	return nil
}

// ManagerLogCallback returns a logger callback suitable for plugins.Manager.
func (c *PluginController) ManagerLogCallback() func(string) {
	return func(line string) {
		c.terminal.Infof("%s", line)
	}
}
