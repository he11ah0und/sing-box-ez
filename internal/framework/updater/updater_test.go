package updater

import (
	"testing"
	"time"

	"sing-box-ez/internal/framework/version"
)

func TestCheckUpdateWithLatest(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		latest        Release
		currentBranch string
		commit        string
		wantLatest    string
		wantCount     int
		wantAssetURL  string
		wantErr       bool
	}{
		{
			name: "latest is prerelease",
			latest: Release{
				Version: "eb6db22", Prerelease: true, PublishedAt: now,
			},
			currentBranch: "main",
			commit:        "eb6db22",
			wantLatest:    "main",
			wantCount:     0,
		},
		{
			name: "commit matches latest version exactly",
			latest: Release{
				Version: "eb6db22", PublishedAt: now,
			},
			currentBranch: "main",
			commit:        "eb6db22",
			wantLatest:    "main",
			wantCount:     0,
		},
		{
			name: "full commit matches short version",
			latest: Release{
				Version: "eb6db22", PublishedAt: now,
			},
			currentBranch: "main",
			commit:        "eb6db22e67e2c4ff043e3e66559eba0ab61d1660",
			wantLatest:    "main",
			wantCount:     0,
		},
		{
			name: "short commit matches full version",
			latest: Release{
				Version: "eb6db22e67e2c4ff043e3e66559eba0ab61d1660", PublishedAt: now,
			},
			currentBranch: "main",
			commit:        "eb6db22",
			wantLatest:    "main",
			wantCount:     0,
		},
		{
			name: "different commit — update available",
			latest: Release{
				Version:     "eb6db22",
				PublishedAt: now,
				Assets:      []Asset{{Name: "sing-box-ez-amd64-linux", URL: "http://example.com/v1", Tags: []string{"amd64", "linux"}}},
			},
			currentBranch: "main",
			commit:        "4cce75f",
			wantLatest:    "eb6db22",
			wantCount:     1,
			wantAssetURL:  "http://example.com/v1",
		},
		{
			name: "unknown commit — update available",
			latest: Release{
				Version:     "eb6db22",
				PublishedAt: now,
				Assets:      []Asset{{Name: "sing-box-ez-amd64-linux", URL: "http://example.com/v1", Tags: []string{"amd64", "linux"}}},
			},
			currentBranch: "main",
			commit:        "unknown",
			wantLatest:    "eb6db22",
			wantCount:     1,
			wantAssetURL:  "http://example.com/v1",
		},
		{
			name: "asset not found",
			latest: Release{
				Version:     "eb6db22",
				PublishedAt: now,
				Assets:      []Asset{{Name: "other-asset", URL: "http://example.com/other", Tags: []string{"other"}}},
			},
			currentBranch: "main",
			commit:        "4cce75f",
			wantLatest:    "eb6db22",
			wantCount:     1,
			wantAssetURL:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldBranch, oldCommit := version.Branch, version.Commit
			oldOS, oldArch := version.BuildOS, version.BuildArch
			oldCompiler, oldGUI, oldBackend := version.BuildCompiler, version.BuildGUI, version.BuildBackend
			defer func() {
				version.Branch, version.Commit = oldBranch, oldCommit
				version.BuildOS, version.BuildArch = oldOS, oldArch
				version.BuildCompiler, version.BuildGUI, version.BuildBackend = oldCompiler, oldGUI, oldBackend
			}()

			version.Branch = tt.currentBranch
			version.Commit = tt.commit
			version.BuildOS = "linux"
			version.BuildArch = "amd64"
			version.BuildCompiler = ""
			version.BuildGUI = ""
			version.BuildBackend = ""

			info, err := updateInfoFrom(tt.latest, tt.currentBranch, currentAssetTags(false), false)
			if (err != nil) != tt.wantErr {
				t.Fatalf("updateInfoFrom() error = %v, wantErr %v", err, tt.wantErr)
			}
			if info.Latest != tt.wantLatest {
				t.Errorf("Latest = %q, want %q", info.Latest, tt.wantLatest)
			}
			if info.ReleaseCount != tt.wantCount {
				t.Errorf("ReleaseCount = %d, want %d", info.ReleaseCount, tt.wantCount)
			}
			if info.AssetURL != tt.wantAssetURL {
				t.Errorf("AssetURL = %q, want %q", info.AssetURL, tt.wantAssetURL)
			}
		})
	}
}

func TestCommitsMatch(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{"exact", "eb6db22", "eb6db22", true},
		{"full vs short", "eb6db22e67e2c4ff043e3e66559eba0ab61d1660", "eb6db22", true},
		{"short vs full", "eb6db22", "eb6db22e67e2c4ff043e3e66559eba0ab61d1660", true},
		{"different", "eb6db22", "4cce75f", false},
		{"empty a", "", "eb6db22", false},
		{"empty b", "eb6db22", "", false},
		{"both empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := commitsMatch(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("commitsMatch(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCurrentAssetTags(t *testing.T) {
	tests := []struct {
		name          string
		buildOS       string
		buildArch     string
		buildCompiler string
		buildGUI      string
		buildBackend  string
		fallback      bool
		want          []string
	}{
		{
			name:      "linux gui wayland gcc",
			buildOS:   "linux",
			buildArch: "amd64",
			buildCompiler: "gcc",
			buildGUI:      "1",
			buildBackend:  "wayland",
			want:          []string{"amd64", "linux", "gcc", "gui", "wayland"},
		},
		{
			name:      "windows gui",
			buildOS:   "windows",
			buildArch: "amd64",
			buildGUI:  "1",
			want:      []string{"amd64", "windows", "gui"},
		},
		{
			name:          "linux cli musl",
			buildOS:       "linux",
			buildArch:     "amd64",
			buildCompiler: "musl",
			buildGUI:      "0",
			want:          []string{"amd64", "linux", "musl", "cli"},
		},
		{
			name:      "darwin cli",
			buildOS:   "darwin",
			buildArch: "arm64",
			buildGUI:  "0",
			want:      []string{"arm64", "darwin", "cli"},
		},
		{
			name:      "minimal known vars",
			buildOS:   "linux",
			buildArch: "amd64",
			want:      []string{"amd64", "linux"},
		},
		{
			name:         "linux backend fallback",
			buildOS:      "linux",
			buildArch:    "amd64",
			buildGUI:     "1",
			buildBackend: "wayland",
			fallback:     true,
			want:         []string{"amd64", "linux", "gui"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldOS, oldArch := version.BuildOS, version.BuildArch
			oldCompiler, oldGUI, oldBackend := version.BuildCompiler, version.BuildGUI, version.BuildBackend
			defer func() {
				version.BuildOS, version.BuildArch = oldOS, oldArch
				version.BuildCompiler, version.BuildGUI, version.BuildBackend = oldCompiler, oldGUI, oldBackend
			}()

			version.BuildOS, version.BuildArch, version.BuildCompiler, version.BuildGUI, version.BuildBackend = tt.buildOS, tt.buildArch, tt.buildCompiler, tt.buildGUI, tt.buildBackend
			got := currentAssetTags(tt.fallback)
			if len(got) != len(tt.want) {
				t.Fatalf("currentAssetTags(%v) = %v, want %v", tt.fallback, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("currentAssetTags(%v)[%d] = %q, want %q", tt.fallback, i, got[i], tt.want[i])
				}
			}
		})
	}
}
