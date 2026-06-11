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

// GetReleaseByTag fetches release notes for a specific version tag.
func GetReleaseByTag(tag string) (Release, error) {
	if defaultManager == nil {
		return Release{}, managerErr()
	}
	return defaultManager.ReleaseNotes(context.Background(), tag)
}

// DownloadAsset downloads a release asset to the given path.
func DownloadAsset(url, dest string, progress func(downloaded, total int64)) error {
	if defaultManager == nil {
		return managerErr()
	}
	return defaultManager.DownloadAsset(context.Background(), url, dest, progress)
}

// ApplyUpdate applies a self update from the given asset URL.
// Kept for backwards compatibility; prefer using Manager.Install directly.
func ApplyUpdate(assetURL string, progress func(downloaded, total int64)) error {
	if defaultManager == nil {
		return managerErr()
	}
	if defaultManager.Apply == nil {
		return errors.New("updater manager has no apply backend configured")
	}
	info := &UpdateInfo{AssetURL: assetURL}
	return defaultManager.Install(context.Background(), info, progress)
}

func updateInfoFrom(release Release, currentBranch string) (*UpdateInfo, error) {
	if release.Prerelease {
		return &UpdateInfo{Current: currentBranch, Latest: currentBranch, ReleaseCount: 0}, nil
	}

	if commitsMatch(release.TagName, version.Commit) {
		return &UpdateInfo{Current: currentBranch, Latest: currentBranch, ReleaseCount: 0}, nil
	}

	assetName := guessAssetName()
	fallbackName := assetName
	if version.BuildBackend != "" && version.BuildOS == "linux" {
		fallbackName = strings.TrimSuffix(assetName, "-"+version.BuildBackend)
	}

	info := &UpdateInfo{
		Current:      currentBranch,
		Latest:       release.TagName,
		ReleaseCount: 1,
		LatestBody:   release.Body,
		LatestDate:   release.PublishedAt,
		AssetName:    assetName,
	}

	for _, a := range release.Assets {
		if a.Name == assetName {
			info.AssetURL = a.DownloadURL
			break
		}
	}

	if info.AssetURL == "" && fallbackName != assetName {
		for _, a := range release.Assets {
			if a.Name == fallbackName {
				info.AssetURL = a.DownloadURL
				info.AssetName = fallbackName
				break
			}
		}
	}
	return info, nil
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

// guessAssetName tries to determine the correct asset name for this system.
func guessAssetName() string {
	goos := version.BuildOS
	if goos == "unknown" || goos == "" {
		goos = runtime.GOOS
	}
	goarch := version.BuildArch
	if goarch == "unknown" || goarch == "" {
		goarch = runtime.GOARCH
	}

	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}

	base := "sing-box-ez-" + goarch + "-" + goos

	if version.BuildCompiler != "" && version.BuildCompiler != "unknown" {
		base += "-" + version.BuildCompiler
	}

	if version.BuildGUI == "1" {
		base += "-gui"
	} else if version.BuildGUI == "0" {
		base += "-cli"
	}

	if version.BuildBackend != "" && goos == "linux" {
		base += "-" + version.BuildBackend
	}

	return base + ext
}
