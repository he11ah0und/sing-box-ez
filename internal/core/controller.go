package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework"
	"sing-box-ez/internal/framework/logger"
	"sing-box-ez/internal/framework/updater"
	"sing-box-ez/internal/framework/util/openfile"
	"sing-box-ez/internal/singboxconfig"
)

// Sentinel errors returned by PrepareConfig so callers can react specifically.
var (
	ErrCoreMissing    = errors.New("core not found. Please download it first")
	ErrNoActiveConfig = errors.New("no active config. Please add and activate a config in the Configs tab")
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
	manager.SetElevated(cfg.MustGet("privileges", "run_as_admin").Bool())

	logWriter := NewCoreLogWriter()
	manager.SetLogOutput(logWriter)

	terminal := parent.Allocate("controller")
	processor := NewCoreLogProcessor(cfg, manager, logWriter, terminal)
	processor.Start()

	privileges := NewPrivilegeController(cfg, manager, terminal)

	c := &Controller{
		cfg:        cfg,
		fwApp:      fwApp,
		manager:    manager,
		processor:  processor,
		terminal:   terminal,
		privileges: privileges,
	}
	c.manager.SetConfigTransform(func(data []byte) ([]byte, error) {
		return c.applyLogOverride(data)
	})
	return c
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

// GetConfigs returns all configured profiles.
func (c *Controller) GetConfigs() []config.ConfigRecord {
	return c.cfg.GetConfigs()
}

// GetActiveConfig returns the currently active profile.
func (c *Controller) GetActiveConfig() *config.ConfigRecord {
	return c.cfg.GetActiveConfig()
}

// GetActiveName returns the name of the active profile.
func (c *Controller) GetActiveName() string {
	return c.cfg.GetActiveName()
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

func (c *Controller) GetLogLinesAtLeast(minLevel logger.LogLevel) []string {
	return c.fwApp.Logger.GetLinesAtLeast(minLevel)
}

func (c *Controller) ClearLogs() {
	c.fwApp.Logger.Clear()
}

func (c *Controller) GetCoreLogLines() []string {
	return c.processor.LogBuffer().GetLines()
}

func (c *Controller) GetCoreLogCleanLines() []string {
	return c.processor.LogBuffer().GetCleanLines()
}

func (c *Controller) ClearCoreLogs() {
	c.processor.LogBuffer().Clear()
}

// ---------- Core lifecycle ----------

func (c *Controller) PrepareConfig() (*config.ConfigRecord, error) {
	active := c.cfg.GetActiveConfig()
	if active == nil {
		return nil, ErrNoActiveConfig
	}

	if !c.CoreExists() {
		return nil, ErrCoreMissing
	}
	c.manager.SetConfigURL(active.URL)
	c.manager.SetConfigName(active.Name)

	if active.IsLocal() {
		if !c.HasCachedConfig(active.Name) {
			if err := c.manager.CreateLocalConfig(active.Name); err != nil {
				return nil, c.terminal.Errorf("failed to create local config: %v", err)
			}
		}
		return active, nil
	}

	if active.ShouldUpdate() || !c.HasCachedConfig(active.Name) || (c.cfg.MustGet("updates", "auto_update_on_hash_mismatch").Bool() && c.IsConfigHashMismatch(active.Name)) {
		c.fwApp.Logger.Log("Updating config...")
		data, err := c.manager.UpdateConfig()
		if err != nil {
			c.fwApp.Logger.Log("Config download issue: " + err.Error())
			if !c.HasCachedConfig(active.Name) {
				return nil, errors.New("no config available")
			}
			c.fwApp.Logger.Log("Using existing config")
		} else {
			c.saveConfigHash(active.Name, data)
			c.cfg.SetLastUpdateFor(active.Name, time.Now())
			_ = c.cfg.Save()
			c.fwApp.Logger.Log("Config updated")
		}
	}

	if active.Hash == "" && c.HasCachedConfig(active.Name) {
		if data, err := c.manager.ReadConfigByName(active.Name); err == nil {
			c.saveConfigHash(active.Name, data)
		}
	}

	return active, nil
}

func (c *Controller) Start() error {
	data, err := c.manager.ReadConfig()
	if err != nil {
		return err
	}
	data, err = c.applyLogOverride(data)
	if err != nil {
		return err
	}
	if err := c.manager.StartWithConfig(data); err != nil {
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
	data, err := c.manager.ReadConfig()
	if err != nil {
		return err
	}
	data, err = c.applyLogOverride(data)
	if err != nil {
		return err
	}
	if err := c.manager.Stop(); err != nil {
		return err
	}
	for range 100 {
		if !c.manager.IsRunning() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := c.manager.StartWithConfig(data); err != nil {
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
	_, err := c.manager.UpdateConfig()
	return err
}

func (c *Controller) UpdateConfigNow(name, url string) error {
	rec := c.cfg.GetConfigByName(name)
	if rec != nil && rec.IsLocal() {
		return c.terminal.Errorf("Local config %q cannot be updated from a URL", name)
	}
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
	return c.fwApp.FS.Root().File(c.manager.coreBinary()).Exists()
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

func (c *Controller) HasCachedConfig(name string) bool {
	return c.fwApp.FS.Root().File(c.manager.cachedConfig(name)).Exists()
}

func (c *Controller) DownloadConfigFor(name, url string) error {
	data, err := c.manager.DownloadConfigFor(name, url)
	if err != nil {
		return err
	}
	c.saveConfigHash(name, data)
	return nil
}

func (c *Controller) saveConfigHash(name string, data []byte) {
	rec := c.cfg.GetConfigByName(name)
	if rec == nil {
		return
	}
	hash := config.HashConfig(data)
	if rec.Hash == hash {
		return
	}
	rec.Hash = hash
	c.cfg.UpdateConfig(name, *rec)
	_ = c.cfg.Save()
}

// IsConfigHashMismatch reports whether the cached config content differs from
// the hash stored in the profile. If no hash is stored, it returns false.
func (c *Controller) IsConfigHashMismatch(name string) bool {
	rec := c.cfg.GetConfigByName(name)
	if rec == nil {
		return false
	}
	if rec.Hash == "" || !c.HasCachedConfig(name) {
		return false
	}
	data, err := c.manager.ReadConfigByName(name)
	if err != nil {
		return false
	}
	return rec.Hash != config.HashConfig(data)
}

func (c *Controller) AddConfig(rec config.ConfigRecord) error {
	if rec.Name == "" {
		return c.terminal.Errorf("Name is required")
	}
	if !rec.IsLocal() && rec.URL == "" {
		return c.terminal.Errorf("URL is required for remote configs")
	}
	if c.cfg.GetConfigByName(rec.Name) != nil {
		return c.terminal.Errorf("Config with this name already exists")
	}
	if rec.IsLocal() {
		if rec.Type == "" {
			rec.Type = config.ConfigTypeLocal
		}
		if err := c.manager.CreateLocalConfig(rec.Name); err != nil {
			return c.terminal.Errorf("failed to create local config: %v", err)
		}
		rec.Hash = config.HashConfig([]byte("{}"))
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
	if rec.Name == "" {
		return c.terminal.Errorf("Name is required")
	}
	if !rec.IsLocal() && rec.URL == "" {
		return c.terminal.Errorf("URL is required for remote configs")
	}
	oldRec := c.cfg.GetConfigByName(oldName)
	if oldRec != nil {
		rec.Hash = oldRec.Hash
	}
	if rec.Name != oldName {
		if c.cfg.GetConfigByName(rec.Name) != nil {
			return c.terminal.Errorf("Config with name %q already exists", rec.Name)
		}
		if rec.IsLocal() {
			_ = c.manager.RenameConfigFile(oldName, rec.Name)
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
		if rec.IsLocal() {
			if err := c.manager.CreateLocalConfig(name); err != nil {
				return c.terminal.Errorf("failed to create local config: %v", err)
			}
		} else {
			return c.terminal.Errorf("No cached config for: %s", name)
		}
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
		if !rec.IsLocal() {
			total++
		}
	}
	if total == 0 {
		c.terminal.Infof("No configs to update")
		return 0, 0, nil
	}
	updated := 0
	for _, rec := range configs {
		if rec.IsLocal() {
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

// OpenConfigFile opens the cached config file in the platform default editor.
func (c *Controller) OpenConfigFile(name string) error {
	if !c.HasCachedConfig(name) {
		return c.terminal.Errorf("Config file not found: %s", name)
	}
	path := c.manager.cachedConfig(name)
	c.terminal.Infof("Opening config file: %s", path)
	return openfile.OpenPath(path)
}

// ValidateConfig reads the cached config and runs a deprecation/validation check
// against the installed sing-box core version.
func (c *Controller) ValidateConfig(name string) (singboxconfig.ValidationResult, error) {
	if !c.HasCachedConfig(name) {
		return singboxconfig.ValidationResult{}, c.terminal.Errorf("Config file not found: %s", name)
	}
	path := c.manager.cachedConfig(name)
	data, err := c.fwApp.FS.Root().File(path).Read()
	if err != nil {
		return singboxconfig.ValidationResult{}, c.terminal.Errorf("failed to read config file: %v", err)
	}
	data, err = c.applyLogOverride(data)
	if err != nil {
		return singboxconfig.ValidationResult{}, err
	}

	var parser *singboxconfig.ConfigParser
	if version, err := c.GetInstalledCoreVersion(); err == nil && version != "" {
		parser, err = singboxconfig.NewConfigParserForVersion(version)
		if err != nil {
			c.terminal.Warnf("Invalid core version %q, using latest schema: %v", version, err)
			parser = singboxconfig.NewConfigParser()
		}
	} else {
		parser = singboxconfig.NewConfigParser()
	}

	if _, err := parser.Parse(data); err != nil {
		result := parser.Result()
		return result, err
	}
	result := parser.Result()
	return result, nil
}

// OpenConfigDir opens the directory containing the cached config file.
func (c *Controller) OpenConfigDir(name string) error {
	if !c.HasCachedConfig(name) {
		return c.terminal.Errorf("Config file not found: %s", name)
	}
	path := c.manager.cachedConfig(name)
	dir := filepath.Dir(path)
	c.terminal.Infof("Opening config directory: %s", dir)
	return openfile.OpenPath(dir)
}

// RecreateLocalConfig recreates the local config file if it is missing.
func (c *Controller) RecreateLocalConfig(name string) error {
	rec := c.cfg.GetConfigByName(name)
	if rec == nil {
		return fmt.Errorf("config not found")
	}
	if !rec.IsLocal() {
		return c.terminal.Errorf("Only local configs can be recreated")
	}
	if err := c.manager.CreateLocalConfig(name); err != nil {
		return c.terminal.Errorf("failed to recreate local config: %v", err)
	}
	c.terminal.Infof("Recreated local config: %s", name)
	return nil
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

	// Preserve the current command-line arguments in the elevated restart.
	var argList string
	if len(os.Args) > 1 {
		quoted := make([]string, len(os.Args)-1)
		for i, a := range os.Args[1:] {
			quoted[i] = strconv.Quote(a)
		}
		argList = "@(" + strings.Join(quoted, ",") + ")"
	}

	script := fmt.Sprintf("Start-Process -FilePath %q -Verb runAs -WorkingDirectory %q", exe, cwd)
	if argList != "" {
		script += fmt.Sprintf(" -ArgumentList %s", argList)
	}

	// #nosec G204 — powershell is a system binary; exe and cwd come from the
	// running process, and arguments are preserved from os.Args.
	cmd := exec.Command("powershell", "-WindowStyle", "hidden", "-Command", script)
	if err := cmd.Start(); err != nil {
		return err
	}

	// The elevated copy is being started; terminate the current unprivileged
	// instance so only one copy of the application remains running.
	os.Exit(0)
	return nil
}

func (c *Controller) SetRunAsAdmin(checked bool) error {
	c.cfg.MustGet("privileges", "run_as_admin").Update(checked)
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
	c.cfg.MustGet("log", "limit").Update(v)
	_ = c.cfg.Save()
	c.fwApp.Logger.SetLimit(v)
	c.processor.LogBuffer().SetLimit(v)
	c.terminal.Infof("Log limit set to %d", v)
}

func (c *Controller) SetDefaultInterval(h int) {
	c.cfg.MustGet("updates", "default_interval_hours").Update(h)
	_ = c.cfg.Save()
	c.terminal.Infof("Default interval set to %dh", h)
}

func (c *Controller) SetAutoRestart(checked bool) error {
	c.cfg.MustGet("core", "auto_restart").Update(checked)
	if err := c.cfg.Save(); err != nil {
		return c.terminal.Errorf("Failed to save auto-restart setting: %v", err)
	}
	c.terminal.Infof("Auto-restart: %v", checked)
	return nil
}
