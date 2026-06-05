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
)

const repoAPI = "https://api.github.com/repos/he11ah0und/sing-box-ez/releases"

// Release represents a GitHub release.
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
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
	Current   string
	Latest    string
	Releases  []Release // all releases newer than current
	AssetURL  string
	AssetName string
}

// GetReleases fetches all releases from GitHub.
func GetReleases() ([]Release, error) {
	req, err := http.NewRequest("GET", repoAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
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
	releases, err := GetReleases()
	if err != nil {
		return Release{}, err
	}
	if len(releases) == 0 {
		return Release{}, fmt.Errorf("no releases found")
	}
	return releases[0], nil
}

// CheckUpdate compares the current version against GitHub releases.
func CheckUpdate(current string) (*UpdateInfo, error) {
	releases, err := GetReleases()
	if err != nil {
		return nil, err
	}

	var newer []Release
	for _, r := range releases {
		if versionLess(current, r.TagName) {
			newer = append(newer, r)
		}
	}

	if len(newer) == 0 {
		return &UpdateInfo{Current: current, Latest: current}, nil
	}

	assetName := guessAssetName()
	info := &UpdateInfo{
		Current:   current,
		Latest:    newer[0].TagName,
		Releases:  newer,
		AssetName: assetName,
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
	resp, err := http.Get(url)
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
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}

	// Determine GUI suffix from current binary name if possible
	exe, _ := os.Executable()
	base := ""
	if strings.Contains(exe, "nogui") {
		base = "sing-box-ez-" + goos + "-" + goarch + "-nogui"
	} else if goos == "linux" {
		if strings.Contains(exe, "x11") {
			base = "sing-box-ez-" + goos + "-" + goarch + "-x11"
		} else {
			base = "sing-box-ez-" + goos + "-" + goarch + "-wayland"
		}
	} else {
		base = "sing-box-ez-" + goos + "-" + goarch
	}

	// Check for musl
	if strings.Contains(exe, "musl") {
		base += "-musl"
	}

	return base + ext
}

// versionLess compares two semver-ish strings.
func versionLess(a, b string) bool {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")
	if a == b {
		return false
	}
	if a == "" || a == "dev" {
		return true
	}
	if b == "" || b == "dev" {
		return false
	}
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		var na, nb int
		fmt.Sscanf(pa[i], "%d", &na)
		fmt.Sscanf(pb[i], "%d", &nb)
		if na != nb {
			return na < nb
		}
	}
	return len(pa) < len(pb)
}
