package githuburl

// Project represents a GitHub owner/repo pair and can construct URLs.
type Project struct {
	Owner string
	Repo  string
}

// DefaultProject returns the project's GitHub repository configuration.
func DefaultProject() Project {
	return Project{
		Owner: "he11ah0und",
		Repo:  "sing-box-ez",
	}
}

// Slug returns the "owner/repo" form.
func (p Project) Slug() string {
	return p.Owner + "/" + p.Repo
}

// RepoURL returns the web repository URL.
func (p Project) RepoURL() string {
	return "https://github.com/" + p.Slug()
}

// APIReleasesURL returns the GitHub API endpoint for releases.
func (p Project) APIReleasesURL() string {
	return "https://api.github.com/repos/" + p.Slug() + "/releases"
}

// APILatestReleaseURL returns the GitHub API endpoint for the latest release.
func (p Project) APILatestReleaseURL() string {
	return p.APIReleasesURL() + "/latest"
}

// WebReleaseURL returns the web URL for a specific release tag.
func (p Project) WebReleaseURL(tag string) string {
	return p.RepoURL() + "/releases/tag/" + tag
}

// WebLatestReleaseURL returns the web URL for the latest release page.
func (p Project) WebLatestReleaseURL() string {
	return p.RepoURL() + "/releases/latest"
}
