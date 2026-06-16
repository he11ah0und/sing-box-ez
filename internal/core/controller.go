package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework"
	"sing-box-ez/internal/framework/logger"
	"sing-box-ez/internal/framework/updater"
)

// Controller is the core application API used by both CLI and GUI.
// It owns the sing-box process manager, config lifecycle, and privilege helpers.
type Controller struct {
	cfg        *config.AppConfig
	fwApp      *framework.App
	manager    *Manager
	processor  *CoreLogProcessor
	terminal   *logger.LogTerminal
	privileges *PrivilegeController
}

// NewController creates a new controller and wires framework services.
func NewController(cfg *config.AppConfig, fwApp *framework.App, parent *logger.LogTerminal) *Controller {
	active := cfg.GetActiveConfig()

	var coreUpdater *updater.Manager
	if fwApp != nil {
		for _, m := range fwApp.Updaters {
			if m.Name == "core-updater" {
				coreUpdater = m
				break
			}
		}
	}

	manager := NewManager(cfg.DataDir, fwApp.FS, coreUpdater, fwApp.Logger)
	if active != nil {
		manager.SetConfigName(active.Name)
	}
	manager.SetElevated(cfg.RunAsAdmin)

	logWriter := NewCoreLogWriter()
	manager.SetLogOutput(logWriter)

	terminal := parent.Allocate("controller")
	processor := NewCoreLogProcessor(cfg, manager, logWriter, terminal)
	processor.Start()

	privileges := NewPrivilegeController(cfg, manager, terminal)

	return &Controller{
		cfg:        cfg,
		fwApp:      fwApp,
		manager:    manager,
		processor:  processor,
		terminal:   terminal,
		privileges: privileges,
	}
}

// Close shuts down the controller.
func (c *Controller) Close() {
	if c.manager.IsRunning() {
		_ = c.manager.Stop()
	}
	if c.processor != nil {
		c.processor.Stop()
	}
}

// Config returns the application configuration.
func (c *Controller) Config() *config.AppConfig {
	return c.cfg
}

// Framework returns the framework app.
func (c *Controller) Framework() *framework.App {
	return c.fwApp
}

// Manager returns the core process manager.
func (c *Controller) Manager() *Manager {
	return c.manager
}

// LogProcessor returns the core log processor.
func (c *Controller) LogProcessor() *CoreLogProcessor {
	return c.processor
}

// Terminal returns the controller's logger terminal.
func (c *Controller) Terminal() *logger.LogTerminal {
	return c.terminal
}

// ---------- Logging ----------

func (c *Controller) Log(msg string) {
	c.fwApp.Logger.Log(msg)
}

func (c *Controller) GetLogLines() []string {
	return c.fwApp.Logger.GetLines()
}

func (c *Controller) ClearLogs() {
	c.fwApp.Logger.Clear()
}

// ---------- Core lifecycle ----------

func (c *Controller) PrepareConfig() (*config.ConfigRecord, error) {
	if !c.CoreExists() {
		return nil, errors.New("core not found. Please download it first")
	}

	active := c.cfg.GetActiveConfig()
	if active == nil {
		return nil, errors.New("no active config. Please add and activate a config in the Configs tab")
	}
	c.manager.SetConfigURL(active.URL)
	c.manager.SetConfigName(active.Name)

	if active.ShouldUpdate() || !c.HasCachedConfig(active.Name) {
		c.fwApp.Logger.Log("Updating config...")
		if err := c.manager.UpdateConfig(); err != nil {
			c.fwApp.Logger.Log("Config download issue: " + err.Error())
			if !c.HasCachedConfig(active.Name) {
				return nil, errors.New("no config available")
			}
			c.fwApp.Logger.Log("Using existing config")
		} else {
			c.cfg.SetLastUpdateFor(active.Name, time.Now())
			_ = c.cfg.Save()
			c.fwApp.Logger.Log("Config updated")
		}
	}

	return active, nil
}

func (c *Controller) Start() error {
	if err := c.manager.Start(); err != nil {
		return err
	}
	c.fwApp.Logger.Log("Sing-box started")
	return nil
}

func (c *Controller) Stop() error {
	if err := c.manager.Stop(); err != nil {
		return err
	}
	c.fwApp.Logger.Log("Sing-box stopped")
	return nil
}

func (c *Controller) Restart() error {
	c.fwApp.Logger.Log("Restarting...")
	if err := c.manager.Restart(); err != nil {
		return err
	}
	c.fwApp.Logger.Log("Sing-box restarted")
	return nil
}

func (c *Controller) IsRunning() bool {
	return c.manager.IsRunning()
}

func (c *Controller) GetPID() int {
	return c.manager.GetPID()
}

func (c *Controller) SetConfigURL(url string) {
	c.manager.SetConfigURL(url)
}

func (c *Controller) SetConfigName(name string) {
	c.manager.SetConfigName(name)
}

func (c *Controller) SetElevated(v bool) {
	c.manager.SetElevated(v)
}

func (c *Controller) UpdateConfig() error {
	return c.manager.UpdateConfig()
}

func (c *Controller) UpdateConfigNow(name, url string) error {
	if err := c.DownloadConfigFor(name, url); err != nil {
		return c.terminal.Errorf("Update failed: %v", err)
	}
	c.cfg.SetLastUpdateFor(name, time.Now())
	_ = c.cfg.Save()
	c.terminal.Infof("Config updated: " + name)
	return nil
}

// ---------- Core binary ----------

func (c *Controller) CoreExists() bool {
	_, err := c.fwApp.FS.Stat(c.manager.coreBinary())
	return err == nil
}

func (c *Controller) GetInstalledCoreVersion() (string, error) {
	return GetCoreVersion(c.manager.coreBinary())
}

func (c *Controller) GetLatestCoreVersion() (string, error) {
	info, err := c.manager.CheckCoreUpdate(context.Background())
	if err != nil {
		return "", err
	}
	return info.Latest, nil
}

func (c *Controller) DownloadCoreWithProgress(onProgress func(downloaded, total int64)) (string, error) {
	return c.DownloadCore(onProgress)
}

func (c *Controller) DownloadCore(onProgress ProgressFunc) (string, error) {
	if c.manager.updater == nil {
		return "", c.terminal.Errorf("core updater not configured")
	}
	info, err := c.manager.updater.Check(context.Background(), "")
	if err != nil {
		return "", err
	}
	if info.ReleaseCount == 0 {
		c.terminal.Infof("Core is up to date: %s", info.Current)
		return c.manager.coreBinary(), nil
	}

	c.terminal.Infof("Latest core version: %s", info.Latest)
	info.Files = []updater.UpdateFile{{
		Asset:    info.Asset,
		DestPath: ".",
	}}
	if err := c.manager.updater.Install(context.Background(), info, onProgress); err != nil {
		return "", c.terminal.Errorf("Failed to download core: %v", err)
	}
	c.terminal.Infof("Core downloaded to: %s", c.manager.coreBinary())
	return c.manager.coreBinary(), nil
}

// ---------- Config management ----------

func (c *Controller) HasCachedConfig(name string) bool {
	_, err := c.fwApp.FS.Stat(c.manager.cachedConfig(name))
	return err == nil
}

func (c *Controller) DownloadConfigFor(name, url string) error {
	return c.manager.DownloadConfigFor(name, url)
}

func (c *Controller) AddFirstConfig(name, url string) error {
	if url == "" {
		return c.terminal.Errorf("First run: empty config URL")
	}
	rec := config.ConfigRecord{
		Name:                name,
		URL:                 url,
		UpdateIntervalHours: c.cfg.UpdateIntervalHours,
		Parent:              "user",
	}
	c.cfg.AddConfig(rec)
	c.cfg.SetActiveName(name)
	c.cfg.SetFirstRunDone(true)
	_ = c.cfg.Save()
	c.manager.SetConfigURL(url)
	c.manager.SetConfigName(name)
	c.terminal.Infof("First config added: " + name)
	return nil
}

func (c *Controller) AddConfig(rec config.ConfigRecord) error {
	if rec.Name == "" || rec.URL == "" {
		return c.terminal.Errorf("Name and URL are required")
	}
	if c.cfg.GetConfigByName(rec.Name) != nil {
		return c.terminal.Errorf("Config with this name already exists")
	}
	c.cfg.AddConfig(rec)
	if c.cfg.GetActiveName() == "" {
		c.cfg.SetActiveName(rec.Name)
		c.manager.SetConfigURL(rec.URL)
		c.manager.SetConfigName(rec.Name)
	}
	_ = c.cfg.Save()
	c.terminal.Infof("Config added: " + rec.Name)
	return nil
}

func (c *Controller) EditConfig(oldName string, rec config.ConfigRecord) error {
	if rec.Name == "" || rec.URL == "" {
		return c.terminal.Errorf("Name and URL are required")
	}
	if rec.Name != oldName {
		if c.cfg.GetConfigByName(rec.Name) != nil {
			return c.terminal.Errorf("Config with name %q already exists", rec.Name)
		}
		c.cfg.RenameConfig(oldName, rec.Name)
	}
	c.cfg.UpdateConfig(rec.Name, rec)
	_ = c.cfg.Save()
	if oldName == c.cfg.GetActiveName() || rec.Name == c.cfg.GetActiveName() {
		c.manager.SetConfigURL(rec.URL)
	}
	if rec.Name != oldName {
		c.terminal.Infof("Config renamed: " + oldName + " -> " + rec.Name)
	} else {
		c.terminal.Infof("Config updated: " + rec.Name)
	}
	return nil
}

func (c *Controller) DeleteConfig(name string) error {
	c.cfg.RemoveConfig(name)
	_ = c.cfg.Save()
	c.terminal.Infof("Config deleted: " + name)
	return nil
}

func (c *Controller) ActivateConfig(name string) error {
	rec := c.cfg.GetConfigByName(name)
	if rec == nil {
		return fmt.Errorf("config not found")
	}
	if !c.HasCachedConfig(name) {
		return c.terminal.Errorf("No cached config for: %s", name)
	}
	c.cfg.SetActiveName(name)
	_ = c.cfg.Save()
	c.manager.SetConfigURL(rec.URL)
	c.manager.SetConfigName(name)
	c.terminal.Infof("Activated config: " + name)
	return nil
}

func (c *Controller) UpdateAllConfigs(progress func(done, total int)) (int, int, error) {
	configs := c.cfg.GetConfigs()
	total := 0
	for _, rec := range configs {
		if rec.URL != "" {
			total++
		}
	}
	if total == 0 {
		c.terminal.Infof("No configs to update")
		return 0, 0, nil
	}
	updated := 0
	for _, rec := range configs {
		if rec.URL == "" {
			continue
		}
		c.terminal.Infof("Updating config: " + rec.Name + "...")
		if err := c.DownloadConfigFor(rec.Name, rec.URL); err != nil {
			c.terminal.Errorf("Failed to update %s: %v", rec.Name, err)
		} else {
			c.cfg.SetLastUpdateFor(rec.Name, time.Now())
			updated++
			c.terminal.Infof("Config updated: " + rec.Name)
		}
		if progress != nil {
			progress(updated, total)
		}
	}
	_ = c.cfg.Save()
	c.terminal.Infof("Update all finished (%d/%d)", updated, total)
	return updated, total, nil
}

// ---------- Privileges ----------

func (c *Controller) HasRequiredPrivileges() bool {
	return c.privileges.HasRequiredPrivileges()
}

func (c *Controller) RefreshPrivilegeStatus() string {
	return c.privileges.RefreshPrivilegeStatus()
}

func (c *Controller) GetPrivilegeDialog() *PrivilegeDialog {
	return c.privileges.GetPrivilegeDialog(c.RestartAsAdmin)
}

func (c *Controller) GetPrivilegeTabState() PrivilegeTabState {
	return c.privileges.GetPrivilegeTabState()
}

func (c *Controller) RestartAsAdmin() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	// #nosec G204 — powershell is a system binary; exe comes from os.Executable() and cwd from os.Getwd().
	cmd := exec.Command("powershell", "-WindowStyle", "hidden", "-Command",
		"Start-Process", "-FilePath", exe, "-Verb", "runAs", "-WorkingDirectory", cwd)
	return cmd.Start()
}

func (c *Controller) SetRunAsAdmin(checked bool) error {
	c.cfg.SetRunAsAdmin(checked)
	c.manager.SetElevated(checked)
	if err := c.cfg.Save(); err != nil {
		return c.terminal.Errorf("Failed to save admin setting: %v", err)
	}
	c.terminal.Infof("Admin mode: %v", checked)
	return nil
}

func (c *Controller) ApplySetcap() error {
	if err := SetNetAdminCapabilityGUI(c.manager.coreBinary()); err != nil {
		c.terminal.Errorf("setcap failed: %v", err)
		return c.terminal.Errorf("Tip: run manually: sudo setcap cap_net_admin=+ep ./sing-box")
	}
	c.terminal.Infof("setcap applied successfully.")
	return nil
}

func (c *Controller) ApplyPrivilegeAction(action *PrivilegeAction) (success, needRefresh, needClose bool) {
	return c.privileges.ApplyPrivilegeAction(action)
}

// ---------- App helpers ----------

func (c *Controller) OpenDataDir() error {
	if c.fwApp == nil {
		return nil
	}
	return c.fwApp.OpenDataDir()
}

func (c *Controller) SetLogLimit(v int) {
	c.cfg.SetLogLimit(v)
	_ = c.cfg.Save()
	c.fwApp.Logger.SetLimit(v)
	c.terminal.Infof("Log limit set to %d", v)
}

func (c *Controller) SetDefaultInterval(h int) {
	c.cfg.SetDefaultUpdateInterval(h)
	_ = c.cfg.Save()
	c.terminal.Infof("Default interval set to %dh", h)
}
