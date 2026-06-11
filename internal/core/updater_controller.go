package core

import (
	"context"
	"fmt"

	"sing-box-ez/internal/framework"
	"sing-box-ez/internal/framework/logger"

	"sing-box-ez/internal/framework/updater"
	"sing-box-ez/internal/framework/version"
)

// UpdaterController manages application self-update and release checks.
type UpdaterController struct {
	fwApp    *framework.App
	terminal *logger.LogTerminal
}

// Terminal returns the logging terminal used by this controller.
func (c *UpdaterController) Terminal() *logger.LogTerminal {
	return c.terminal
}

// NewUpdaterController creates a new updater controller.
func NewUpdaterController(fwApp *framework.App, terminal *logger.LogTerminal) *UpdaterController {
	return &UpdaterController{fwApp: fwApp, terminal: terminal}
}

func (c *UpdaterController) manager() *updater.Manager {
	if c.fwApp != nil {
		for _, m := range c.fwApp.Updaters {
			if m.Apply != nil {
				return m
			}
		}
		if len(c.fwApp.Updaters) > 0 {
			return c.fwApp.Updaters[0]
		}
	}
	return updater.CurrentManager()
}

// CheckSelfUpdate checks for self updates.
func (c *UpdaterController) CheckSelfUpdate() (*updater.UpdateInfo, error) {
	m := c.manager()
	if m == nil {
		return nil, fmt.Errorf("updater manager not configured")
	}
	return m.Check(context.Background(), version.Branch)
}

// ApplySelfUpdate applies a self update from the given asset URL.
func (c *UpdaterController) ApplySelfUpdate(assetURL string, onProgress func(int64, int64)) error {
	m := c.manager()
	if m == nil {
		return fmt.Errorf("updater manager not configured")
	}
	return m.Install(context.Background(), &updater.UpdateInfo{AssetURL: assetURL}, onProgress)
}

// GetBranches fetches available repository branches.
func (c *UpdaterController) GetBranches() ([]updater.Channel, error) {
	m := c.manager()
	if m == nil {
		return nil, fmt.Errorf("updater manager not configured")
	}
	return m.Channels(context.Background())
}

// ApplySelfUpdateWithLog performs a self-update and logs the result.
func (c *UpdaterController) ApplySelfUpdateWithLog(assetURL string, onProgress func(int64, int64)) error {
	if assetURL == "" {
		c.terminal.Errorf("Self-update: no matching asset for this system")
		return fmt.Errorf("no matching asset")
	}
	err := c.ApplySelfUpdate(assetURL, onProgress)
	if err != nil {
		c.terminal.Errorf("Self-update failed: %s", err.Error())
		return err
	}
	return nil
}

// CheckSelfUpdateWithLog checks for self updates and logs the result.
func (c *UpdaterController) CheckSelfUpdateWithLog() (*updater.UpdateInfo, error) {
	info, err := c.CheckSelfUpdate()
	if err != nil {
		c.terminal.Errorf("Update check failed: %s", err.Error())
		return nil, err
	}
	if info.ReleaseCount == 0 {
		c.terminal.Infof("Already on latest version: %s", info.Current)
		return nil, nil
	}
	c.terminal.Infof("Update available: %s → %s (%d releases behind)", info.Current, info.Latest, info.ReleaseCount)
	return info, nil
}

// CheckSelfUpdateForBranch checks for updates on the specified branch and logs the result.
func (c *UpdaterController) CheckSelfUpdateForBranch(branch string) (*updater.UpdateInfo, error) {
	m := c.manager()
	if m == nil {
		return nil, fmt.Errorf("updater manager not configured")
	}
	info, err := m.Check(context.Background(), branch)
	if err != nil {
		c.terminal.Errorf("Update check failed for branch %s: %s", branch, err.Error())
		return nil, err
	}
	if info.ReleaseCount == 0 {
		c.terminal.Infof("Branch %s is up to date: %s", branch, info.Current)
	} else {
		c.terminal.Infof("Update available on %s: %s → %s", branch, info.Current, info.Latest)
	}
	return info, nil
}

// FetchReleaseNotesWithLog fetches release notes and logs errors.
// Returns the release and an error (err != nil only when TagName is non-empty).
func (c *UpdaterController) FetchReleaseNotesWithLog(commit string) (updater.Release, error) {
	m := c.manager()
	if m == nil {
		return updater.Release{}, fmt.Errorf("updater manager not configured")
	}
	release, err := m.ReleaseNotes(context.Background(), commit)
	if err != nil {
		if release.TagName == "" {
			return release, nil
		}
		c.terminal.Errorf("Failed to fetch release notes: %s", err.Error())
		return release, err
	}
	return release, nil
}

// CheckUpdates checks for self updates and returns detailed information.
func (c *UpdaterController) CheckUpdates() (*updater.UpdateInfo, string, string, error) {
	info, err := c.CheckSelfUpdate()
	if err == nil && info.ReleaseCount > 0 {
		return info, "", "", nil
	}
	currentVer, err := GetCoreVersion(GetCorePath())
	if err != nil || currentVer == "" {
		return nil, "", "", err
	}
	latestVer, err := GetLatestVersion()
	if err != nil {
		return nil, currentVer, "", err
	}
	return nil, currentVer, latestVer, nil
}
