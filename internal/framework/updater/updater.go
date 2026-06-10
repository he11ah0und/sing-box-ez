package updater

import (
	"context"
	"os"
	"runtime"
	"strings"

	"sing-box-ez/internal/framework/version"
)

var defaultBackend Backend = DefaultGitHubBackend()

// SetBackend configures the update source used by the package-level helpers.
func SetBackend(b Backend) {
	defaultBackend = b
}

// CurrentBackend returns the active update backend.
func CurrentBackend() Backend {
	return defaultBackend
}

// CheckUpdate compares the current build against the latest release.
func CheckUpdate(currentBranch string) (*UpdateInfo, error) {
	release, err := defaultBackend.LatestRelease(context.Background(), currentBranch)
	if err != nil {
		return nil, err
	}
	return updateInfoFrom(release, currentBranch)
}

// CheckUpdateForBranch checks for updates on the specified branch.
func CheckUpdateForBranch(branch string) (*UpdateInfo, error) {
	release, err := defaultBackend.LatestRelease(context.Background(), branch)
	if err != nil {
		return nil, err
	}
	return updateInfoFrom(release, branch)
}

// GetChannels returns the list of available update channels.
func GetChannels() ([]Channel, error) {
	return defaultBackend.ListChannels(context.Background())
}

// GetReleaseByTag fetches release notes for a specific version tag.
func GetReleaseByTag(tag string) (Release, error) {
	return defaultBackend.ReleaseByTag(context.Background(), tag)
}

// DownloadAsset downloads a release asset to the given path.
func DownloadAsset(url, dest string, progress func(downloaded, total int64)) error {
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0750)
	if err != nil {
		return err
	}
	defer f.Close()
	return defaultBackend.DownloadAsset(context.Background(), url, f, progress)
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
