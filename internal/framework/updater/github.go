package updater

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"sing-box-ez/internal/framework/logger"
	"sing-box-ez/internal/framework/net"
	frameworkprogress "sing-box-ez/internal/framework/progress"
)

const defaultGitHubAPI = "https://api.github.com"

// GitHubBackend fetches releases from the GitHub Releases API.
type GitHubBackend struct {
	BaseURL string
	Owner   string
	Repo    string
	Net     *net.Client
	Log     *logger.LogTerminal
}

// NewGitHubBackend returns a GitHubBackend for the public GitHub API.
// The logger terminal is allocated from parent as "gh-{owner}-{repo}" so
// multiple GitHub backends can be distinguished in the logs.
func NewGitHubBackend(parent *logger.LogTerminal, owner, repo string) *GitHubBackend {
	return &GitHubBackend{
		BaseURL: defaultGitHubAPI,
		Owner:   owner,
		Repo:    repo,
		Net:     net.NewClient(parent),
		Log:     parent.Allocate("gh-" + owner + "-" + repo),
	}
}

// NewGitHubEnterpriseBackend returns a GitHubBackend for a custom GitHub Enterprise instance.
func NewGitHubEnterpriseBackend(parent *logger.LogTerminal, baseURL, owner, repo string) *GitHubBackend {
	return &GitHubBackend{
		BaseURL: baseURL,
		Owner:   owner,
		Repo:    repo,
		Net:     net.NewClient(parent),
		Log:     parent.Allocate("gh-" + owner + "-" + repo),
	}
}

func (b *GitHubBackend) slug() string { return b.Owner + "/" + b.Repo }

func (b *GitHubBackend) apiBase() string {
	if b.BaseURL == "" {
		return defaultGitHubAPI
	}
	return b.BaseURL
}

func (b *GitHubBackend) apiReposURL() string {
	return b.apiBase() + "/repos/" + b.slug()
}

func (b *GitHubBackend) apiReleasesURL() string {
	return b.apiReposURL() + "/releases"
}

func (b *GitHubBackend) apiLatestReleaseURL() string {
	return b.apiReleasesURL() + "/latest"
}

func (b *GitHubBackend) apiReleaseByTagURL(tag string) string {
	return b.apiReleasesURL() + "/tags/" + tag
}

func (b *GitHubBackend) apiBranchesURL() string {
	return b.apiReposURL() + "/branches"
}

// Name returns the backend identifier.
func (b *GitHubBackend) Name() string { return "github" }

func (b *GitHubBackend) netClient() *net.Client {
	if b.Net != nil {
		return b.Net
	}
	return net.NewClient(b.Log)
}

func (b *GitHubBackend) newGitHubRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	return req, nil
}

func (b *GitHubBackend) doJSON(req *http.Request, out any) error {
	resp, err := b.netClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

// LatestRelease implements Source.
func (b *GitHubBackend) LatestRelease(ctx context.Context, channel string) (Release, error) {
	var r Release
	var err error
	if channel != "" {
		r, err = b.latestForChannel(ctx, channel)
	} else {
		r, err = b.latestStable(ctx)
	}
	return r, err
}

func (b *GitHubBackend) latestStable(ctx context.Context) (Release, error) {
	req, err := b.newGitHubRequest(ctx, http.MethodGet, b.apiLatestReleaseURL(), nil)
	if err != nil {
		return Release{}, err
	}
	var raw ghRelease
	if err := b.doJSON(req, &raw); err != nil {
		return Release{}, err
	}
	return b.toRelease(raw), nil
}

func (b *GitHubBackend) latestForChannel(ctx context.Context, channel string) (Release, error) {
	req, err := b.newGitHubRequest(ctx, http.MethodGet, b.apiReleasesURL(), nil)
	if err != nil {
		return Release{}, err
	}
	var raw []ghRelease
	if err := b.doJSON(req, &raw); err != nil {
		return Release{}, err
	}
	for _, r := range raw {
		if r.TargetCommitish == channel && !r.Prerelease {
			return b.toRelease(r), nil
		}
	}
	// No release tied to this channel; fall back to the repository's latest
	// stable release so feature branches still receive updates.
	b.Log.Infof("no release found for channel %s, falling back to latest stable", channel)
	return b.latestStable(ctx)
}

// ListChannels implements Source.
func (b *GitHubBackend) ListChannels(ctx context.Context) ([]Channel, error) {
	req, err := b.newGitHubRequest(ctx, http.MethodGet, b.apiBranchesURL(), nil)
	if err != nil {
		return nil, err
	}
	var raw []ghBranch
	if err := b.doJSON(req, &raw); err != nil {
		return nil, err
	}
	channels := make([]Channel, len(raw))
	for i, br := range raw {
		name := br.Name
		if name == "" {
			name = br.Name
		}
		channels[i] = Channel{ID: br.Name, Name: name}
	}
	return channels, nil
}

// ReleaseByVersion implements Source.
func (b *GitHubBackend) ReleaseByVersion(ctx context.Context, version string) (Release, error) {
	// Try the version as-is first, then with/without a leading 'v'.
	// This accommodates both semver tags ("v1.9.0") and commit-hash tags
	// used by sing-box-ez ("eb6db22").
	candidates := []string{version}
	if strings.HasPrefix(version, "v") {
		candidates = append(candidates, strings.TrimPrefix(version, "v"))
	} else {
		candidates = append(candidates, "v"+version)
	}

	var lastErr error
	for _, tag := range candidates {
		req, err := b.newGitHubRequest(ctx, http.MethodGet, b.apiReleaseByTagURL(tag), nil)
		if err != nil {
			return Release{}, err
		}
		var raw ghRelease
		if err := b.doJSON(req, &raw); err != nil {
			lastErr = err
			continue
		}
		return b.toRelease(raw), nil
	}
	return Release{}, lastErr
}

// DownloadAsset implements Source.
func (b *GitHubBackend) DownloadAsset(ctx context.Context, asset Asset, w io.Writer, progress func(downloaded, total int64)) error {
	if asset.URL == "" {
		return b.Log.Errorf("asset has no download URL")
	}
	c := b.netClient()
	if progress != nil {
		c.Progress = &frameworkprogress.Config{
			Callback: func(s frameworkprogress.State) {
				if s.Op == "download" {
					progress(s.Current, s.Total)
				}
			},
			Interval: 0,
		}
	}
	return c.Download(ctx, asset.URL, w)
}

// --- GitHub API JSON structs ---

type ghRelease struct {
	TagName         string    `json:"tag_name"`
	TargetCommitish string    `json:"target_commitish"`
	Name            string    `json:"name"`
	Body            string    `json:"body"`
	PublishedAt     time.Time `json:"published_at"`
	Prerelease      bool      `json:"prerelease"`
	Assets          []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
}

type ghBranch struct {
	Name string `json:"name"`
}

func (b *GitHubBackend) toRelease(raw ghRelease) Release {
	release := Release{
		Version:     raw.TagName,
		Channel:     raw.TargetCommitish,
		Name:        raw.Name,
		Body:        raw.Body,
		PublishedAt: raw.PublishedAt,
		Prerelease:  raw.Prerelease,
		Assets:      make([]Asset, len(raw.Assets)),
	}
	for i, a := range raw.Assets {
		release.Assets[i] = Asset{
			Name:   a.Name,
			URL:    a.DownloadURL,
			Size:   a.Size,
			Tags:   b.assetTags(release, a.Name),
			Format: formatForName(a.Name),
			Hashes: make(map[string]string),
		}
	}
	return release
}

// assetTags extracts platform tags from an asset filename.
// It strips archive extensions, the project name and version, then splits the
// remainder by '-'.
func (b *GitHubBackend) assetTags(release Release, name string) []string {
	base := stripArchiveExt(name)

	// Strip project name prefix (e.g. "sing-box-", "sing-box-ez-").
	if strings.HasPrefix(base, b.Repo+"-") {
		base = base[len(b.Repo)+1:]
	}

	// Strip version (with and without leading 'v').
	ver := strings.TrimPrefix(release.Version, "v")
	if ver != "" {
		base = strings.ReplaceAll(base, "v"+ver, "")
		base = strings.ReplaceAll(base, ver, "")
	}

	base = strings.ReplaceAll(base, "--", "-")
	base = strings.Trim(base, "-")
	if base == "" {
		return nil
	}
	return strings.Split(base, "-")
}

// formatForName returns the archive format constant for the given asset name.
func formatForName(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		return FormatTarGz
	case strings.HasSuffix(lower, ".tar.bz2"):
		return FormatTarBz2
	case strings.HasSuffix(lower, ".zip"):
		return FormatZIP
	default:
		return FormatRaw
	}
}

// stripArchiveExt removes known archive extensions from a filename so that
// platform tags can be parsed from the remainder.
func stripArchiveExt(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		return name[:len(name)-7]
	case strings.HasSuffix(lower, ".tar.bz2"):
		return name[:len(name)-8]
	case strings.HasSuffix(lower, ".zip"):
		return name[:len(name)-4]
	default:
		if ext := filepath.Ext(name); ext != "" {
			return name[:len(name)-len(ext)]
		}
	}
	return name
}
