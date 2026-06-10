package version

import (
	"testing"
	"time"
)

func localDateStr(rfc string) string {
	t, _ := time.Parse(time.RFC3339, rfc)
	return t.Local().Format("2006-01-02 15:04:05")
}

func TestInfo(t *testing.T) {
	tests := []struct {
		name      string
		branch    string
		buildDate string
		commit    string
		want      string
	}{
		{
			name:   "all unknown",
			branch: "dev",
			want:   "dev",
		},
		{
			name:   "branch only",
			branch: "main",
			want:   "main",
		},
		{
			name:   "branch and commit",
			branch: "main",
			commit: "abc1234",
			want:   "main (abc1234)",
		},
		{
			name:      "branch and date",
			branch:    "main",
			buildDate: "2026-06-05T12:00:00Z",
			want:      "main (" + localDateStr("2026-06-05T12:00:00Z") + ")",
		},
		{
			name:      "all fields",
			branch:    "main",
			buildDate: "2026-06-05T12:00:00Z",
			commit:    "abc1234",
			want:      "main (" + localDateStr("2026-06-05T12:00:00Z") + " abc1234)",
		},
		{
			name:      "empty builddate falls back to commit",
			branch:    "main",
			buildDate: "",
			commit:    "abc1234",
			want:      "main (abc1234)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldBranch, oldBuildDate, oldCommit := Branch, BuildDate, Commit
			defer func() {
				Branch, BuildDate, Commit = oldBranch, oldBuildDate, oldCommit
			}()

			Branch, BuildDate, Commit = tt.branch, tt.buildDate, tt.commit
			if got := Info(); got != tt.want {
				t.Errorf("Info() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildFlags(t *testing.T) {
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
			name:      "minimal",
			buildOS:   "linux",
			buildArch: "amd64",
			want:      "amd64-linux",
		},
		{
			name:          "with compiler",
			buildOS:       "linux",
			buildArch:     "amd64",
			buildCompiler: "gcc",
			want:          "amd64-linux-gcc",
		},
		{
			name:      "gui",
			buildOS:   "linux",
			buildArch: "amd64",
			buildGUI:  "1",
			want:      "amd64-linux-gui",
		},
		{
			name:      "cli",
			buildOS:   "linux",
			buildArch: "amd64",
			buildGUI:  "0",
			want:      "amd64-linux-cli",
		},
		{
			name:         "with backend",
			buildOS:      "linux",
			buildArch:    "amd64",
			buildGUI:     "1",
			buildBackend: "wayland",
			want:         "amd64-linux-gui-wayland",
		},
		{
			name:          "full",
			buildOS:       "linux",
			buildArch:     "amd64",
			buildCompiler: "gcc",
			buildGUI:      "1",
			buildBackend:  "wayland",
			want:          "amd64-linux-gcc-gui-wayland",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldOS, oldArch, oldCompiler, oldGUI, oldBackend, oldDev := BuildOS, BuildArch, BuildCompiler, BuildGUI, BuildBackend, BuildDev
			defer func() {
				BuildOS, BuildArch, BuildCompiler, BuildGUI, BuildBackend, BuildDev = oldOS, oldArch, oldCompiler, oldGUI, oldBackend, oldDev
			}()

			BuildOS, BuildArch, BuildCompiler, BuildGUI, BuildBackend = tt.buildOS, tt.buildArch, tt.buildCompiler, tt.buildGUI, tt.buildBackend
			BuildDev = "false"
			if got := BuildFlags(); got != tt.want {
				t.Errorf("BuildFlags() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildDateTime(t *testing.T) {
	tests := []struct {
		name      string
		buildDate string
		wantErr   bool
		want      time.Time
	}{
		{
			name:      "valid",
			buildDate: "2026-06-05T12:00:00Z",
			want:      time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC).Local(),
		},
		{
			name:    "unknown",
			wantErr: true,
		},
		{
			name:      "empty",
			buildDate: "",
			wantErr:   true,
		},
		{
			name:      "invalid format",
			buildDate: "June 5, 2026",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldBuildDate := BuildDate
			defer func() { BuildDate = oldBuildDate }()

			BuildDate = tt.buildDate
			got, err := BuildDateTime()
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildDateTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !got.Equal(tt.want) {
					t.Errorf("BuildDateTime() = %v, want %v", got, tt.want)
				}
				if got.Location() != time.Local {
					t.Errorf("BuildDateTime() location = %v, want Local", got.Location())
				}
			}
		})
	}
}
