package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sort"
	"time"

	"sing-box-ez/internal/githuburl"
	"sing-box-ez/internal/version"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Release represents a GitHub release.
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Prerelease  bool      `json:"prerelease"`
	Assets      []Asset   `json:"assets"`
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
	ReleaseCount int    // how many stable releases behind
	LatestBody   string // body of the latest stable release
	AssetURL     string
	AssetName    string
}

// GetReleases fetches all releases from GitHub.
func GetReleases() ([]Release, error) {
	req, err := http.NewRequest("GET", githuburl.DefaultProject().APIReleasesURL(), nil)
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

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}
	return releases, nil
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

// CheckUpdate compares the current branch against GitHub releases.
func CheckUpdate(currentBranch string) (*UpdateInfo, error) {
	releases, err := GetReleases()
	if err != nil {
		return nil, err
	}
	return checkUpdateWithReleases(releases, currentBranch)
}

func checkUpdateWithReleases(releases []Release, currentBranch string) (*UpdateInfo, error) {
	var stable []Release
	for _, r := range releases {
		if !r.Prerelease {
			stable = append(stable, r)
		}
	}

	if len(stable) == 0 {
		return &UpdateInfo{Current: currentBranch, Latest: currentBranch, ReleaseCount: 0}, nil
	}

	// Sort by PublishedAt descending (newest first)
	sort.Slice(stable, func(i, j int) bool {
		return stable[i].PublishedAt.After(stable[j].PublishedAt)
	})

	var newer []Release
	currentIdx := -1
	for i, r := range stable {
		if r.TagName == currentBranch {
			currentIdx = i
			break
		}
	}

	if currentIdx >= 0 {
		currentRelease := stable[currentIdx]
		for _, r := range stable {
			if r.PublishedAt.After(currentRelease.PublishedAt) {
				newer = append(newer, r)
			}
		}
	} else {
		// Current branch not found in releases — dev/branch build.
		// Compare by build date if available.
		buildDate, err := version.BuildDateTime()
		if err == nil {
			for _, r := range stable {
				if r.PublishedAt.After(buildDate) {
					newer = append(newer, r)
				}
			}
		} else {
			// Unknown build date — assume all stable releases are newer
			newer = append([]Release{}, stable...)
		}
	}

	if len(newer) == 0 {
		return &UpdateInfo{Current: currentBranch, Latest: currentBranch, ReleaseCount: 0}, nil
	}

	assetName := guessAssetName()
	info := &UpdateInfo{
		Current:      currentBranch,
		Latest:       newer[0].TagName,
		ReleaseCount: len(newer),
		LatestBody:   newer[0].Body,
		AssetName:    assetName,
	}

	for _, a := range newer[0].Assets {
		if a.Name == assetName {
			info.AssetURL = a.BrowserDownloadURL
			break
		}
	}
	return info, nil
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

	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
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

	if version.BuildBackend != "" {
		base += "-" + version.BuildBackend
	}

	return base + ext
}
