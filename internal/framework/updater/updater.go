package updater

import (
	"context"
	"errors"
	"runtime"
	"strings"

	"sing-box-ez/internal/framework/version"
)

var defaultManager *Manager

// SetManager configures the update manager used by the package-level helpers.
func SetManager(m *Manager) {
	defaultManager = m
}

// CurrentManager returns the active update manager.
func CurrentManager() *Manager {
	return defaultManager
}

// SetBackend configures the source backend on the default manager. Kept for
// backwards compatibility; prefer constructing a Manager and calling SetManager.
func SetBackend(b Source) {
	if defaultManager == nil {
		defaultManager = &Manager{}
	}
	defaultManager.Source = b
}

// CurrentBackend returns the source backend of the default manager. Kept for
// backwards compatibility; prefer Manager.Source.
func CurrentBackend() Source {
	if defaultManager == nil {
		return nil
	}
	return defaultManager.Source
}

func managerErr() error {
	return errors.New("updater manager not configured")
}

// CheckUpdate compares the current build against the latest release.
func CheckUpdate(currentBranch string) (*UpdateInfo, error) {
	if defaultManager == nil {
		return nil, managerErr()
	}
	return defaultManager.Check(context.Background(), currentBranch)
}

// CheckUpdateForBranch checks for updates on the specified branch.
func CheckUpdateForBranch(branch string) (*UpdateInfo, error) {
	if defaultManager == nil {
		return nil, managerErr()
	}
	return defaultManager.Check(context.Background(), branch)
}

// GetChannels returns the list of available update channels.
func GetChannels() ([]Channel, error) {
	if defaultManager == nil {
		return nil, managerErr()
	}
	return defaultManager.Channels(context.Background())
}

// GetReleaseByVersion fetches release notes for a specific version identifier.
func GetReleaseByVersion(version string) (Release, error) {
	if defaultManager == nil {
		return Release{}, managerErr()
	}
	return defaultManager.ReleaseNotes(context.Background(), version)
}

// DownloadAsset downloads a release asset to the given path.
func DownloadAsset(asset Asset, dest string, progress func(downloaded, total int64)) error {
	if defaultManager == nil {
		return managerErr()
	}
	return defaultManager.DownloadAsset(context.Background(), asset, dest, progress)
}

// ApplyUpdate applies a self update from the given asset.
// Kept for backwards compatibility; prefer using Manager.Install directly.
func ApplyUpdate(asset Asset, progress func(downloaded, total int64)) error {
	if defaultManager == nil {
		return managerErr()
	}
	if defaultManager.Apply == nil {
		return errors.New("updater manager has no apply backend configured")
	}
	info := &UpdateInfo{Asset: asset, AssetURL: asset.URL, AssetName: asset.Name}
	return defaultManager.Install(context.Background(), info, progress)
}

func updateInfoFrom(release Release, currentBranch string, tags []string, useFallback bool) (*UpdateInfo, error) {
	current := currentVersionLabel(currentBranch)
	if commitsMatch(release.Version, version.Commit) {
		return &UpdateInfo{Current: current, Latest: current, ReleaseCount: 0}, nil
	}

	asset, ok := FindAsset(release, AssetCriteria{Tags: tags})
	if !ok && useFallback {
		asset, _ = FindAsset(release, AssetCriteria{Tags: currentAssetTags(true)})
	}

	info := &UpdateInfo{
		Current:      current,
		Latest:       release.Version,
		ReleaseCount: 1,
		LatestBody:   release.Body,
		LatestDate:   release.PublishedAt,
		AssetName:    asset.Name,
		AssetURL:     asset.URL,
		Asset:        asset,
	}
	return info, nil
}

// currentVersionLabel returns the current build commit if known, otherwise the
// branch name. This makes update messages show "commit → commit" instead of
// "branch → commit".
func currentVersionLabel(branch string) string {
	if version.Commit != "" && version.Commit != "unknown" {
		return version.Commit
	}
	return branch
}

// currentAssetTags returns the platform tags for the current build.
// If fallback is true, the linux backend tag is omitted.
func currentAssetTags(fallback bool) []string {
	goos := version.BuildOS
	if goos == "unknown" || goos == "" {
		goos = runtime.GOOS
	}
	goarch := version.BuildArch
	if goarch == "unknown" || goarch == "" {
		goarch = runtime.GOARCH
	}

	tags := []string{goarch, goos}
	if version.BuildCompiler != "" && version.BuildCompiler != "unknown" {
		tags = append(tags, version.BuildCompiler)
	}
	if version.BuildGUI == "1" {
		tags = append(tags, "gui")
	} else if version.BuildGUI == "0" {
		tags = append(tags, "cli")
	}
	if version.BuildBackend != "" && goos == "linux" && !fallback {
		tags = append(tags, version.BuildBackend)
	}
	return tags
}

func commitsMatch(a, b string) bool {
	if a == "" || b == "" {
		return a == b
	}
	if a == b {
		return true
	}
	if len(a) > len(b) {
		return strings.HasPrefix(a, b)
	}
	return strings.HasPrefix(b, a)
}


