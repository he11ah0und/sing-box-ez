package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"sing-box-ez/internal/framework/logger"
)

const defaultGitHubAPI = "https://api.github.com"

// GitHubBackend fetches releases from the GitHub Releases API.
type GitHubBackend struct {
	BaseURL string
	Owner   string
	Repo    string
	Client  *http.Client
	Log     *logger.LogTerminal
}

// NewGitHubBackend returns a GitHubBackend for the public GitHub API.
func NewGitHubBackend(owner, repo string) *GitHubBackend {
	return &GitHubBackend{
		BaseURL: defaultGitHubAPI,
		Owner:   owner,
		Repo:    repo,
		Client:  httpClient,
	}
}

// NewGitHubEnterpriseBackend returns a GitHubBackend for a custom GitHub Enterprise instance.
func NewGitHubEnterpriseBackend(baseURL, owner, repo string) *GitHubBackend {
	return &GitHubBackend{
		BaseURL: baseURL,
		Owner:   owner,
		Repo:    repo,
		Client:  httpClient,
	}
}

func (b *GitHubBackend) log() *logger.LogTerminal {
	if b.Log != nil {
		return b.Log
	}
	return &logger.LogTerminal{}
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

func (b *GitHubBackend) client() *http.Client {
	if b.Client != nil {
		return b.Client
	}
	return httpClient
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
	b.log().Debugf("%s %s", req.Method, req.URL.String())
	resp, err := b.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github api %s: %s", resp.Status, string(body))
	}
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
	var r Release
	if err := b.doJSON(req, &r); err != nil {
		return Release{}, err
	}
	return r, nil
}

func (b *GitHubBackend) latestForChannel(ctx context.Context, channel string) (Release, error) {
	req, err := b.newGitHubRequest(ctx, http.MethodGet, b.apiReleasesURL(), nil)
	if err != nil {
		return Release{}, err
	}
	var releases []Release
	if err := b.doJSON(req, &releases); err != nil {
		return Release{}, err
	}
	for _, r := range releases {
		if r.TargetCommitish == channel && !r.Prerelease {
			return r, nil
		}
	}
	return Release{}, fmt.Errorf("no release found for channel %s", channel)
}

// ListChannels implements Source.
func (b *GitHubBackend) ListChannels(ctx context.Context) ([]Channel, error) {
	req, err := b.newGitHubRequest(ctx, http.MethodGet, b.apiBranchesURL(), nil)
	if err != nil {
		return nil, err
	}
	var channels []Channel
	if err := b.doJSON(req, &channels); err != nil {
		return nil, err
	}
	for i := range channels {
		if channels[i].Name == "" {
			channels[i].Name = channels[i].ID
		}
	}
	return channels, nil
}

// ReleaseByTag implements Source.
func (b *GitHubBackend) ReleaseByTag(ctx context.Context, tag string) (Release, error) {
	req, err := b.newGitHubRequest(ctx, http.MethodGet, b.apiReleaseByTagURL(tag), nil)
	if err != nil {
		return Release{}, err
	}
	var r Release
	if err := b.doJSON(req, &r); err != nil {
		return Release{}, err
	}
	return r, nil
}

// DownloadAsset implements Source.
func (b *GitHubBackend) DownloadAsset(ctx context.Context, url string, w io.Writer, progress func(downloaded, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	b.log().Debugf("GET %s", url)
	resp, err := b.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}

	total := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 64*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
			downloaded += int64(n)
			if progress != nil {
				progress(downloaded, total)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}
