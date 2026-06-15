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

// NewUpdaterController creates a new updater controller.
func NewUpdaterController(fwApp *framework.App, parent *logger.LogTerminal) *UpdaterController {
	return &UpdaterController{fwApp: fwApp, terminal: parent.Allocate("updater")}
}

// Terminal returns the controller's logger terminal.
func (c *UpdaterController) Terminal() *logger.LogTerminal {
	return c.terminal
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

// GetBranches fetches available repository branches.
func (c *UpdaterController) GetBranches() ([]updater.Channel, error) {
	m := c.manager()
	if m == nil {
		return nil, fmt.Errorf("updater manager not configured")
	}
	return m.Channels(context.Background())
}

// ApplySelfUpdate performs a self-update and logs the result.
func (c *UpdaterController) ApplySelfUpdate(asset updater.Asset, onProgress func(downloaded, total int64)) error {
	if asset.URL == "" {
		return c.terminal.Errorf("Self-update: no matching asset for this system")
	}
	m := c.manager()
	if m == nil {
		return c.terminal.Errorf("updater manager not configured")
	}
	info := &updater.UpdateInfo{Asset: asset, AssetURL: asset.URL, AssetName: asset.Name}
	if err := m.Install(context.Background(), info, onProgress); err != nil {
		return err
	}
	return nil
}

// CheckSelfUpdate checks for self updates and logs the result.
func (c *UpdaterController) CheckSelfUpdate() (*updater.UpdateInfo, error) {
	m := c.manager()
	if m == nil {
		return nil, c.terminal.Errorf("updater manager not configured")
	}
	info, err := m.Check(context.Background(), version.Branch)
	if err != nil {
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
	info, err := m.Check(context.Background(), branch)
	if err != nil {
		return nil, err
	}
	if info.ReleaseCount == 0 {
		c.terminal.Infof("Branch %s is up to date: %s", branch, info.Current)
	} else {
		c.terminal.Infof("Update available on %s: %s → %s", branch, info.Current, info.Latest)
	}
	return info, nil
}

// FetchReleaseNotes fetches release notes and logs errors.
// Returns the release and an error (err != nil only when Version is non-empty).
func (c *UpdaterController) FetchReleaseNotes(commit string) (updater.Release, error) {
	m := c.manager()
	if m == nil {
		return updater.Release{}, fmt.Errorf("updater manager not configured")
	}
	release, err := m.ReleaseNotes(context.Background(), commit)
	if err != nil {
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
	m := coreUpdater()
	if m == nil {
		return nil, currentVer, "", fmt.Errorf("core updater not configured")
	}
	coreInfo, err := m.Check(context.Background(), "")
	if err != nil {
		return nil, currentVer, "", err
	}
	return nil, currentVer, coreInfo.Latest, nil
}
