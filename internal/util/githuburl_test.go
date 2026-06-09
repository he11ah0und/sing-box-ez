package util

import "testing"

func TestDefaultProject(t *testing.T) {
	p := DefaultProject()
	if p.Owner != "he11ah0und" {
		t.Errorf("Owner = %q, want he11ah0und", p.Owner)
	}
	if p.Repo != "sing-box-ez" {
		t.Errorf("Repo = %q, want sing-box-ez", p.Repo)
	}
}

func TestProjectURLs(t *testing.T) {
	p := Project{Owner: "he11ah0und", Repo: "sing-box-ez"}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"Slug", p.Slug(), "he11ah0und/sing-box-ez"},
		{"RepoURL", p.RepoURL(), "https://github.com/he11ah0und/sing-box-ez"},
		{"APIReleasesURL", p.APIReleasesURL(), "https://api.github.com/repos/he11ah0und/sing-box-ez/releases"},
		{"APILatestReleaseURL", p.APILatestReleaseURL(), "https://api.github.com/repos/he11ah0und/sing-box-ez/releases/latest"},
		{"APIReleaseByTagURL", p.APIReleaseByTagURL("v1.0.0"), "https://api.github.com/repos/he11ah0und/sing-box-ez/releases/tags/v1.0.0"},
		{"WebReleaseURL", p.WebReleaseURL("v1.0.0"), "https://github.com/he11ah0und/sing-box-ez/releases/tag/v1.0.0"},
		{"WebCommitURL", p.WebCommitURL("abc1234"), "https://github.com/he11ah0und/sing-box-ez/commit/abc1234"},
		{"WebLatestReleaseURL", p.WebLatestReleaseURL(), "https://github.com/he11ah0und/sing-box-ez/releases/latest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}
