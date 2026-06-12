package updater

import (
	"context"
	"io"
	"net/http"
	"time"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Release represents a single version/release returned by a Source.
type Release struct {
	Version     string
	Channel     string
	Name        string
	Body        string
	PublishedAt time.Time
	Prerelease  bool
	Assets      []Asset
}

// Supported archive formats for Asset.Format. Empty string means a raw file.
const (
	FormatRaw    = ""
	FormatZIP    = "zip"
	FormatTarGz  = "tar.gz"
	FormatTarBz2 = "tar.bz2"
	Format7z     = "7z"
)

// Asset represents a single downloadable binary artifact.
type Asset struct {
	Name   string
	URL    string
	Size   int64
	Hashes map[string]string // e.g. sha256, md5, sha512
	Tags   []string          // platform tags: os, arch, gui, compiler, backend, ...
	Format string            // FormatRaw, FormatZIP, FormatTarGz, FormatTarBz2, Format7z
}

// Channel represents an update channel/branch offered by a Source.
type Channel struct {
	ID   string
	Name string
}

// UpdateInfo holds the result of an update check in a UI-friendly form.
type UpdateInfo struct {
	Current      string
	Latest       string
	ReleaseCount int       // 0 = up to date, 1+ = update available
	LatestBody   string    // body of the latest release
	LatestDate   time.Time // published date of the latest release
	AssetURL     string
	AssetName    string
	Asset        Asset       // full selected asset metadata (used by self-update)
	Files        []UpdateFile // auxiliary files to update (used by files-update)
}

// Source abstracts the source of application updates.
// Implementations may target GitHub releases, a custom update server, etc.
type Source interface {
	// Name returns the source identifier.
	Name() string

	// LatestRelease returns the newest release for the given channel.
	// An empty channel means the source's default channel.
	LatestRelease(ctx context.Context, channel string) (Release, error)

	// ListChannels returns available update channels.
	ListChannels(ctx context.Context) ([]Channel, error)

	// ReleaseByVersion fetches release notes/metadata for a specific version identifier.
	ReleaseByVersion(ctx context.Context, version string) (Release, error)

	// DownloadAsset streams the update payload identified by asset to w.
	DownloadAsset(ctx context.Context, asset Asset, w io.Writer, progress func(downloaded, total int64)) error
}

// Backend is the former name of Source. Kept as a type alias for backwards
// compatibility; new code should use Source.
type Backend = Source

// UpdateFile describes a single non-self file to update.
type UpdateFile struct {
	Asset    Asset
	DestPath string
}

// AssetCriteria describes a desired platform/build.
type AssetCriteria struct {
	Branch  string
	Version string
	Tags    []string // all of these tags must be present in the asset
}

// FindAsset returns the first asset in the release whose tags contain all
// criteria.Tags.
func FindAsset(release Release, criteria AssetCriteria) (Asset, bool) {
	for _, a := range release.Assets {
		if containsAll(a.Tags, criteria.Tags) {
			return a, true
		}
	}
	return Asset{}, false
}

func containsAll(haystack, needles []string) bool {
	if len(needles) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(haystack))
	for _, v := range haystack {
		set[v] = struct{}{}
	}
	for _, v := range needles {
		if _, ok := set[v]; !ok {
			return false
		}
	}
	return true
}
