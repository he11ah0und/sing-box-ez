package updater

import (
	"context"
	"fmt"
	"os"

	"sing-box-ez/internal/framework/logger"
)

// Manager orchestrates update discovery and installation using configurable
// source and apply backends.
type Manager struct {
	Source Source
	Apply  Apply
	Log    *logger.LogTerminal
}

func (m *Manager) log() *logger.LogTerminal {
	if m.Log != nil {
		return m.Log
	}
	return &logger.LogTerminal{}
}

// Check returns update information for the given channel.
func (m *Manager) Check(ctx context.Context, channel string) (*UpdateInfo, error) {
	if m.Source == nil {
		return nil, fmt.Errorf("updater manager has no source backend configured")
	}
	m.log().Debugf("checking for updates on channel %q", channel)
	release, err := m.Source.LatestRelease(ctx, channel)
	if err != nil {
		m.log().Errorf("update check failed: %v", err)
		return nil, err
	}
	info, err := updateInfoFrom(release, channel)
	if err != nil {
		m.log().Errorf("update info parse failed: %v", err)
		return nil, err
	}
	if info.ReleaseCount > 0 {
		m.log().Infof("update available: %s → %s", info.Current, info.Latest)
	} else {
		m.log().Debugf("no update available on channel %q", channel)
	}
	return info, nil
}

// Install installs the update described by info.
func (m *Manager) Install(ctx context.Context, info *UpdateInfo, progress func(downloaded, total int64)) error {
	if m.Source == nil {
		return fmt.Errorf("updater manager has no source backend configured")
	}
	if m.Apply == nil {
		return fmt.Errorf("updater manager has no apply backend configured")
	}
	if info == nil {
		return fmt.Errorf("no update info provided")
	}
	m.log().Infof("installing update %s using %s", info.Latest, m.Apply.Name())
	if err := m.Apply.Apply(ctx, m.Source, *info, progress); err != nil {
		m.log().Errorf("install failed: %v", err)
		return err
	}
	m.log().Infof("install completed")
	return nil
}

// Channels returns the list of available update channels.
func (m *Manager) Channels(ctx context.Context) ([]Channel, error) {
	if m.Source == nil {
		return nil, fmt.Errorf("updater manager has no source backend configured")
	}
	m.log().Debugf("listing channels")
	channels, err := m.Source.ListChannels(ctx)
	if err != nil {
		m.log().Errorf("list channels failed: %v", err)
		return nil, err
	}
	m.log().Debugf("found %d channels", len(channels))
	return channels, nil
}

// ReleaseNotes fetches release metadata for a specific tag.
func (m *Manager) ReleaseNotes(ctx context.Context, tag string) (Release, error) {
	if m.Source == nil {
		return Release{}, fmt.Errorf("updater manager has no source backend configured")
	}
	m.log().Debugf("fetching release notes for %q", tag)
	release, err := m.Source.ReleaseByTag(ctx, tag)
	if err != nil {
		m.log().Errorf("fetch release notes failed: %v", err)
		return Release{}, err
	}
	return release, nil
}

// DownloadAsset downloads a release asset to the given path.
func (m *Manager) DownloadAsset(ctx context.Context, url, dest string, progress func(downloaded, total int64)) error {
	if m.Source == nil {
		return fmt.Errorf("updater manager has no source backend configured")
	}
	m.log().Infof("downloading asset %s → %s", url, dest)
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0750)
	if err != nil {
		m.log().Errorf("open dest failed: %v", err)
		return err
	}
	defer f.Close()
	if err := m.Source.DownloadAsset(ctx, url, f, progress); err != nil {
		m.log().Errorf("download failed: %v", err)
		return err
	}
	m.log().Infof("download completed")
	return nil
}
