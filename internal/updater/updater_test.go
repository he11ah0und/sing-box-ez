package updater

import (
	"testing"
	"time"

	"sing-box-ez/internal/version"
)

func TestCheckUpdateWithReleases(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	yesterday := now.Add(-24 * time.Hour)
	lastWeek := now.Add(-7 * 24 * time.Hour)

	tests := []struct {
		name          string
		releases      []Release
		currentBranch string
		setupBuild    func()
		wantLatest    string
		wantCount     int
		wantErr       bool
	}{
		{
			name:          "no releases",
			releases:      nil,
			currentBranch: "main",
			wantLatest:    "main",
			wantCount:     0,
		},
		{
			name: "no stable releases",
			releases: []Release{
				{TagName: "v1.0.0-rc1", Prerelease: true, PublishedAt: now},
			},
			currentBranch: "main",
			wantLatest:    "main",
			wantCount:     0,
		},
		{
			name: "current branch found, newer exists",
			releases: []Release{
				{TagName: "v1.0.0", PublishedAt: lastWeek, Assets: []Asset{{Name: "test-asset", BrowserDownloadURL: "http://example.com/v1"}}},
				{TagName: "v1.1.0", PublishedAt: yesterday, Assets: []Asset{{Name: "test-asset", BrowserDownloadURL: "http://example.com/v2"}}},
				{TagName: "v1.2.0", PublishedAt: now, Assets: []Asset{{Name: "test-asset", BrowserDownloadURL: "http://example.com/v3"}}},
			},
			currentBranch: "v1.0.0",
			wantLatest:    "v1.2.0",
			wantCount:     2,
		},
		{
			name: "current branch found, no newer",
			releases: []Release{
				{TagName: "v1.0.0", PublishedAt: lastWeek},
			},
			currentBranch: "v1.0.0",
			wantLatest:    "v1.0.0",
			wantCount:     0,
		},
		{
			name: "current branch not found, build date newer than releases",
			releases: []Release{
				{TagName: "v1.0.0", PublishedAt: lastWeek},
			},
			currentBranch: "main",
			setupBuild: func() {
				version.BuildDate = "2026-06-05 12:00:00"
			},
			wantLatest: "main",
			wantCount:  0,
		},
		{
			name: "current branch not found, release newer than build date",
			releases: []Release{
				{TagName: "v1.0.0", PublishedAt: now, Assets: []Asset{{Name: "test-asset", BrowserDownloadURL: "http://example.com/v1"}}},
			},
			currentBranch: "main",
			setupBuild: func() {
				version.BuildDate = "2026-06-04 12:00:00"
			},
			wantLatest:   "v1.0.0",
			wantCount:    1,
		},
		{
			name: "current branch not found, unknown build date",
			releases: []Release{
				{TagName: "v1.0.0", PublishedAt: now, Assets: []Asset{{Name: "test-asset", BrowserDownloadURL: "http://example.com/v1"}}},
			},
			currentBranch: "main",
			setupBuild: func() {
				version.BuildDate = "unknown"
			},
			wantLatest:   "v1.0.0",
			wantCount:    1,
		},
		{
			name: "prereleases ignored",
			releases: []Release{
				{TagName: "v2.0.0", PublishedAt: now, Prerelease: true},
				{TagName: "v1.0.0", PublishedAt: lastWeek, Assets: []Asset{{Name: "test-asset", BrowserDownloadURL: "http://example.com/v1"}}},
			},
			currentBranch: "v1.0.0",
			wantLatest:    "v1.0.0",
			wantCount:     0,
		},
		{
			name: "asset not found",
			releases: []Release{
				{TagName: "v1.1.0", PublishedAt: now, Assets: []Asset{{Name: "other-asset", BrowserDownloadURL: "http://example.com/other"}}},
			},
			currentBranch: "v1.0.0",
			wantLatest:    "v1.1.0",
			wantCount:     1,
		},
		{
			name: "multiple newer sorted correctly",
			releases: []Release{
				{TagName: "v1.0.0", PublishedAt: lastWeek},
				{TagName: "v1.2.0", PublishedAt: now, Assets: []Asset{{Name: "test-asset", BrowserDownloadURL: "http://example.com/v3"}}},
				{TagName: "v1.1.0", PublishedAt: yesterday, Assets: []Asset{{Name: "test-asset", BrowserDownloadURL: "http://example.com/v2"}}},
			},
			currentBranch: "v1.0.0",
			wantLatest:    "v1.2.0",
			wantCount:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldBranch, oldBuildDate := version.Branch, version.BuildDate
			defer func() {
				version.Branch, version.BuildDate = oldBranch, oldBuildDate
			}()

			if tt.setupBuild != nil {
				tt.setupBuild()
			} else {
				version.BuildDate = "unknown"
			}

			info, err := checkUpdateWithReleases(tt.releases, tt.currentBranch)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkUpdateWithReleases() error = %v, wantErr %v", err, tt.wantErr)
			}
			if info.Latest != tt.wantLatest {
				t.Errorf("Latest = %q, want %q", info.Latest, tt.wantLatest)
			}
			if info.ReleaseCount != tt.wantCount {
				t.Errorf("ReleaseCount = %d, want %d", info.ReleaseCount, tt.wantCount)
			}

		})
	}
}

func TestGuessAssetName(t *testing.T) {
	tests := []struct {
		name          string
		buildOS       string
		buildArch     string
		buildCompiler string
		buildGUI      string
		buildBackend  string
		want          string
	}{
		{
			name:      "linux gui wayland gcc",
			buildOS:   "linux",
			buildArch: "amd64",
			buildCompiler: "gcc",
			buildGUI:  "1",
			buildBackend: "wayland",
			want:      "sing-box-ez-amd64-linux-gcc-gui-wayland",
		},
		{
			name:      "windows gui",
			buildOS:   "windows",
			buildArch: "amd64",
			buildGUI:  "1",
			want:      "sing-box-ez-amd64-windows-gui.exe",
		},
		{
			name:      "linux cli musl",
			buildOS:   "linux",
			buildArch: "amd64",
			buildCompiler: "musl",
			buildGUI:  "0",
			want:      "sing-box-ez-amd64-linux-musl-cli",
		},
		{
			name:      "darwin cli",
			buildOS:   "darwin",
			buildArch: "arm64",
			buildGUI:  "0",
			want:      "sing-box-ez-arm64-darwin-cli",
		},
		{
			name:      "minimal known vars",
			buildOS:   "linux",
			buildArch: "amd64",
			want:      "sing-box-ez-amd64-linux",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldOS, oldArch, oldCompiler, oldGUI, oldBackend := version.BuildOS, version.BuildArch, version.BuildCompiler, version.BuildGUI, version.BuildBackend
			defer func() {
				version.BuildOS, version.BuildArch, version.BuildCompiler, version.BuildGUI, version.BuildBackend = oldOS, oldArch, oldCompiler, oldGUI, oldBackend
			}()

			version.BuildOS, version.BuildArch, version.BuildCompiler, version.BuildGUI, version.BuildBackend = tt.buildOS, tt.buildArch, tt.buildCompiler, tt.buildGUI, tt.buildBackend
			got := guessAssetName()
			if got != tt.want {
				t.Errorf("guessAssetName() = %q, want %q", got, tt.want)
			}
		})
	}
}
