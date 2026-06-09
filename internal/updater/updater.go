package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"sing-box-ez/internal/util/githuburl"
	"sing-box-ez/internal/version"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Release represents a GitHub release.
type Release struct {
	TagName         string    `json:"tag_name"`
	TargetCommitish string    `json:"target_commitish"`
	Name            string    `json:"name"`
	Body            string    `json:"body"`
	PublishedAt     time.Time `json:"published_at"`
	HTMLURL         string    `json:"html_url"`
	Prerelease      bool      `json:"prerelease"`
	Assets          []Asset   `json:"assets"`
}

// Branch represents a GitHub repository branch.
type Branch struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

// Asset represents a release binary asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// UpdateInfo holds the result of an update check.
type UpdateInfo struct {
	Current      string
	Latest       string
	ReleaseCount int       // 0 = up to date, 1 = update available
	LatestBody   string    // body of the latest stable release
	LatestDate   time.Time // published_at of the latest release
	AssetURL     string
	AssetName    string
}

// GetLatestRelease returns the most recent release.
func GetLatestRelease() (Release, error) {
	url := githuburl.DefaultProject().APILatestReleaseURL()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := httpClient.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Release{}, fmt.Errorf("github api %s: %s", resp.Status, string(body))
	}

	var r Release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return Release{}, err
	}
	return r, nil
}

// GetReleaseByTag fetches a specific release by its tag name.
// Returns an error if the release does not exist (404) or the API fails.
func GetReleaseByTag(tag string) (Release, error) {
	url := githuburl.DefaultProject().APIReleaseByTagURL(tag)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := httpClient.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return Release{}, fmt.Errorf("release not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Release{}, fmt.Errorf("github api %s: %s", resp.Status, string(body))
	}

	var r Release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return Release{}, err
	}
	return r, nil
}

// CheckUpdate compares the current build against the latest GitHub release by commit hash.
func CheckUpdate(currentBranch string) (*UpdateInfo, error) {
	latest, err := GetLatestRelease()
	if err != nil {
		return nil, err
	}
	return checkUpdateWithLatest(latest, currentBranch)
}

// GetBranches fetches the list of branches from the GitHub API.
func GetBranches() ([]Branch, error) {
	url := githuburl.DefaultProject().APIBranchesURL()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github api %s: %s", resp.Status, string(body))
	}

	var branches []Branch
	if err := json.NewDecoder(resp.Body).Decode(&branches); err != nil {
		return nil, err
	}
	return branches, nil
}

// GetLatestReleaseForBranch returns the most recent non-prerelease release for the given branch.
func GetLatestReleaseForBranch(branch string) (Release, error) {
	url := githuburl.DefaultProject().APIReleasesURL()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := httpClient.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Release{}, fmt.Errorf("github api %s: %s", resp.Status, string(body))
	}

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return Release{}, err
	}

	for _, r := range releases {
		if r.TargetCommitish == branch && !r.Prerelease {
			return r, nil
		}
	}
	return Release{}, fmt.Errorf("no release found for branch %s", branch)
}

// CheckUpdateForBranch checks for updates on the specified branch.
func CheckUpdateForBranch(branch string) (*UpdateInfo, error) {
	latest, err := GetLatestReleaseForBranch(branch)
	if err != nil {
		return nil, err
	}
	return checkUpdateWithLatest(latest, branch)
}

func checkUpdateWithLatest(latest Release, currentBranch string) (*UpdateInfo, error) {
	if latest.Prerelease {
		return &UpdateInfo{Current: currentBranch, Latest: currentBranch, ReleaseCount: 0}, nil
	}

	if commitsMatch(latest.TagName, version.Commit) {
		return &UpdateInfo{Current: currentBranch, Latest: currentBranch, ReleaseCount: 0}, nil
	}

	assetName := guessAssetName()
	info := &UpdateInfo{
		Current:      currentBranch,
		Latest:       latest.TagName,
		ReleaseCount: 1,
		LatestBody:   latest.Body,
		LatestDate:   latest.PublishedAt,
		AssetName:    assetName,
	}

	for _, a := range latest.Assets {
		if a.Name == assetName {
			info.AssetURL = a.BrowserDownloadURL
			break
		}
	}
	return info, nil
}

// commitsMatch checks whether two commit identifiers refer to the same commit.
// One may be a short prefix of the other.
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

// DownloadAsset downloads a release asset to the given path.
func DownloadAsset(url, dest string, progress func(downloaded, total int64)) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}

	// #nosec G302,G304 — dest is a trusted download path passed by the caller; 0750 is required for executable assets.
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0750)
	if err != nil {
		return err
	}
	defer f.Close()

	total := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 64*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
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
