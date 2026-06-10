package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"sing-box-ez/internal/framework/util/githuburl"
)

// GitHubBackend fetches releases from the GitHub Releases API.
type GitHubBackend struct {
	Project githuburl.Project
	Client  *http.Client
}

// DefaultGitHubBackend returns a GitHubBackend pointing at the project's repo.
func DefaultGitHubBackend() *GitHubBackend {
	return &GitHubBackend{
		Project: githuburl.DefaultProject(),
		Client:  httpClient,
	}
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

// LatestRelease implements Backend.
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
	req, err := b.newGitHubRequest(ctx, http.MethodGet, b.Project.APILatestReleaseURL(), nil)
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
	req, err := b.newGitHubRequest(ctx, http.MethodGet, b.Project.APIReleasesURL(), nil)
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

// ListChannels implements Backend.
func (b *GitHubBackend) ListChannels(ctx context.Context) ([]Channel, error) {
	req, err := b.newGitHubRequest(ctx, http.MethodGet, b.Project.APIBranchesURL(), nil)
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

// ReleaseByTag implements Backend.
func (b *GitHubBackend) ReleaseByTag(ctx context.Context, tag string) (Release, error) {
	req, err := b.newGitHubRequest(ctx, http.MethodGet, b.Project.APIReleaseByTagURL(tag), nil)
	if err != nil {
		return Release{}, err
	}
	var r Release
	if err := b.doJSON(req, &r); err != nil {
		return Release{}, err
	}
	return r, nil
}

// DownloadAsset implements Backend.
func (b *GitHubBackend) DownloadAsset(ctx context.Context, url string, w io.Writer, progress func(downloaded, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
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
