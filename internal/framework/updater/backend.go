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
	TagName         string    `json:"tag_name"`
	TargetCommitish string    `json:"target_commitish"`
	Name            string    `json:"name"`
	Body            string    `json:"body"`
	PublishedAt     time.Time `json:"published_at"`
	Prerelease      bool      `json:"prerelease"`
	Assets          []Asset   `json:"assets"`
}

// Channel represents an update channel/branch offered by a Source.
type Channel struct {
	ID     string `json:"name"`
	Name   string `json:"-"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

// Asset represents a single downloadable binary artifact.
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
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

	// ReleaseByTag fetches release notes/metadata for a specific version identifier.
	ReleaseByTag(ctx context.Context, tag string) (Release, error)

	// DownloadAsset streams the update payload identified by url to w.
	DownloadAsset(ctx context.Context, url string, w io.Writer, progress func(downloaded, total int64)) error
}

// Backend is the former name of Source. Kept as a type alias for backwards
// compatibility; new code should use Source.
type Backend = Source
