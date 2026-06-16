package updater

import (
	"context"
	"errors"
	"os"

	"sing-box-ez/internal/framework/logger"
	"sing-box-ez/internal/framework/version"
)

// Manager orchestrates update discovery and installation using configurable
// source and apply backends.
type Manager struct {
	Name string
	Source Source
	Apply  Apply
	Log    *logger.LogTerminal
	// AssetCriteria selects assets for this manager. If zero, defaults to the
	// current build's platform tags.
	AssetCriteria AssetCriteria
}

// NewManager creates a new Manager with a scoped logger allocated from the
// given parent terminal.
func NewManager(parent *logger.LogTerminal, name string) *Manager {
	return &Manager{Name: name, Log: parent.Allocate(name)}
}

// Check returns update information for the given channel.
func (m *Manager) Check(ctx context.Context, channel string) (*UpdateInfo, error) {
	if m.Source == nil {
		return nil, m.Log.Errorf("updater manager has no source backend configured")
	}
	m.Log.Infof("checking for updates on channel %q", channel)
	release, err := m.Source.LatestRelease(ctx, channel)
	if err != nil {
		if errors.Is(err, ErrNoRelease) {
			// The source already logs "no releases found for channel ..."; avoid
			// duplicating the message here.
			current := currentVersionLabel(channel)
			return &UpdateInfo{Current: current, Latest: current, ReleaseCount: 0}, nil
		}
		return nil, err
	}

	tags := m.AssetCriteria.Tags
	useFallback := false
	if len(tags) == 0 {
		tags = currentAssetTags(false)
		useFallback = version.BuildBackend != "" && version.BuildOS == "linux"
	}
	info, err := updateInfoFrom(release, channel, tags, useFallback)
	if err != nil {
		return nil, err
	}
	if info.ReleaseCount > 0 {
		m.Log.Infof("update available: %s → %s", info.Current, info.Latest)
	} else {
		m.Log.Debugf("no update available on channel %q", channel)
	}
	return info, nil
}

// Install installs the update described by info.
func (m *Manager) Install(ctx context.Context, info *UpdateInfo, progress func(downloaded, total int64)) error {
	if m.Source == nil {
		return m.Log.Errorf("updater manager has no source backend configured")
	}
	if m.Apply == nil {
		return m.Log.Errorf("updater manager has no apply backend configured")
	}
	if info == nil {
		return m.Log.Errorf("no update info provided")
	}
	m.Log.Infof("installing update %s using %s", info.Latest, m.Apply.Name())
	if err := m.Apply.Apply(ctx, m.Source, *info, progress); err != nil {
		return err
	}
	m.Log.Infof("install completed")
	return nil
}

// Channels returns the list of available update channels.
func (m *Manager) Channels(ctx context.Context) ([]Channel, error) {
	if m.Source == nil {
		return nil, m.Log.Errorf("updater manager has no source backend configured")
	}
	m.Log.Debugf("listing channels")
	channels, err := m.Source.ListChannels(ctx)
	if err != nil {
		return nil, err
	}
	m.Log.Debugf("found %d channels", len(channels))
	return channels, nil
}

// ReleaseNotes fetches release metadata for a specific version.
func (m *Manager) ReleaseNotes(ctx context.Context, version string) (Release, error) {
	if m.Source == nil {
		return Release{}, m.Log.Errorf("updater manager has no source backend configured")
	}
	m.Log.Debugf("fetching release notes for %q", version)
	release, err := m.Source.ReleaseByVersion(ctx, version)
	if err != nil {
		return Release{}, err
	}
	return release, nil
}

// DownloadAsset downloads a release asset to the given path.
func (m *Manager) DownloadAsset(ctx context.Context, asset Asset, dest string, progress func(downloaded, total int64)) error {
	if m.Source == nil {
		return m.Log.Errorf("updater manager has no source backend configured")
	}
	m.Log.Infof("downloading asset %s → %s", asset.Name, dest)
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0750)
	if err != nil {
		return m.Log.Errorf("open dest failed: %v", err)
	}
	defer f.Close()
	if err := m.Source.DownloadAsset(ctx, asset, f, progress); err != nil {
		return err
	}
	m.Log.Infof("download completed")
	return nil
}
