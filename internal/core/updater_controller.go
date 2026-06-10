package core

import (
	"fmt"
	"sing-box-ez/internal/framework/logger"

	"sing-box-ez/internal/framework/updater"
	"sing-box-ez/internal/framework/version"
)

// UpdaterController manages application self-update and release checks.
type UpdaterController struct {
	terminal *logger.LogTerminal
}

// Terminal returns the logging terminal used by this controller.
func (c *UpdaterController) Terminal() *logger.LogTerminal {
	return c.terminal
}

// NewUpdaterController creates a new updater controller.
func NewUpdaterController(terminal *logger.LogTerminal) *UpdaterController {
	return &UpdaterController{terminal: terminal}
}

// CheckSelfUpdate checks for self updates.
func (c *UpdaterController) CheckSelfUpdate() (*updater.UpdateInfo, error) {
	return updater.CheckUpdate(version.Branch)
}

// ApplySelfUpdate applies a self update from the given asset URL.
func (c *UpdaterController) ApplySelfUpdate(assetURL string, onProgress func(int64, int64)) error {
	return updater.ApplyUpdate(assetURL, onProgress)
}

// GetBranches fetches available repository branches.
func (c *UpdaterController) GetBranches() ([]updater.Channel, error) {
	return updater.GetChannels()
}

// ApplySelfUpdateWithLog performs a self-update and logs the result.
func (c *UpdaterController) ApplySelfUpdateWithLog(assetURL string, onProgress func(int64, int64)) error {
	if assetURL == "" {
		c.terminal.Error("Self-update: no matching asset for this system")
		return fmt.Errorf("no matching asset")
	}
	err := c.ApplySelfUpdate(assetURL, onProgress)
	if err != nil {
		c.terminal.Error("Self-update failed: " + err.Error())
		return err
	}
	return nil
}

// CheckSelfUpdateWithLog checks for self updates and logs the result.
func (c *UpdaterController) CheckSelfUpdateWithLog() (*updater.UpdateInfo, error) {
	info, err := c.CheckSelfUpdate()
	if err != nil {
		c.terminal.Error("Update check failed: " + err.Error())
		return nil, err
	}
	if info.ReleaseCount == 0 {
		c.terminal.Info("Already on latest version: " + info.Current)
		return nil, nil
	}
	c.terminal.Infof("Update available: %s → %s (%d releases behind)", info.Current, info.Latest, info.ReleaseCount)
	return info, nil
}

// CheckSelfUpdateForBranch checks for updates on the specified branch and logs the result.
func (c *UpdaterController) CheckSelfUpdateForBranch(branch string) (*updater.UpdateInfo, error) {
	info, err := updater.CheckUpdateForBranch(branch)
	if err != nil {
		c.terminal.Error("Update check failed for branch " + branch + ": " + err.Error())
		return nil, err
	}
	if info.ReleaseCount == 0 {
		c.terminal.Info("Branch " + branch + " is up to date: " + info.Current)
	} else {
		c.terminal.Infof("Update available on %s: %s → %s", branch, info.Current, info.Latest)
	}
	return info, nil
}

// FetchReleaseNotesWithLog fetches release notes and logs errors.
// Returns the release and an error (err != nil only when TagName is non-empty).
func (c *UpdaterController) FetchReleaseNotesWithLog(commit string) (updater.Release, error) {
	release, err := updater.GetReleaseByTag(commit)
	if err != nil {
		if release.TagName == "" {
			return release, nil
		}
		c.terminal.Error("Failed to fetch release notes: " + err.Error())
		return release, err
	}
	return release, nil
}

// CheckUpdates checks for self updates and returns detailed information.
func (c *UpdaterController) CheckUpdates() (*updater.UpdateInfo, string, string, error) {
	info, err := updater.CheckUpdate(version.Branch)
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
