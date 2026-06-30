//go:build !nogui

package wails

import (
	"context"
	"fmt"
	"time"

	"sing-box-ez/internal/framework/updater"
	"sing-box-ez/internal/framework/util/openurl"
	"sing-box-ez/internal/framework/version"
)

// VersionInfo holds build metadata for the about page.
type VersionInfo struct {
	Branch      string `json:"branch"`
	Commit      string `json:"commit"`
	CommitDate  string `json:"commitDate"`
	BuildDate   string `json:"buildDate"`
	BuildFlags  string `json:"buildFlags"`
	IsDev       bool   `json:"isDev"`
	HumanCommit string `json:"humanCommit"`
	HumanBuild  string `json:"humanBuild"`
}

// UpdateChannel is a UI-facing release channel.
type UpdateChannel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SelfUpdateInfo is a UI-friendly self-update check result.
type SelfUpdateInfo struct {
	Current      string    `json:"current"`
	Latest       string    `json:"latest"`
	HasUpdate    bool      `json:"hasUpdate"`
	IsDevBuild   bool      `json:"isDevBuild"`
	Body         string    `json:"body"`
	LatestDate   time.Time `json:"latestDate"`
	ReleaseCount int       `json:"releaseCount"`
}

// GetVersionInfo returns build and version metadata.
func (b *Bindings) GetVersionInfo() VersionInfo {
	commitDate := ""
	if dt, err := version.CommitDateTime(); err == nil {
		commitDate = dt.Format("2006-01-02 15:04:05")
	}
	buildDate := ""
	if dt, err := version.BuildDateTime(); err == nil {
		buildDate = dt.Format("2006-01-02 15:04:05")
	}
	return VersionInfo{
		Branch:      version.Branch,
		Commit:      version.Commit,
		CommitDate:  commitDate,
		BuildDate:   buildDate,
		BuildFlags:  version.BuildFlags(),
		IsDev:       version.IsDev(),
		HumanCommit: humanDurationOrEmpty(version.CommitDateTime()),
		HumanBuild:  humanDurationOrEmpty(version.BuildDateTime()),
	}
}

// GetBranches returns the available release channels.
func (b *Bindings) GetBranches() ([]UpdateChannel, error) {
	channels, err := b.ic.GetBranches()
	if err != nil {
		return nil, err
	}
	out := make([]UpdateChannel, len(channels))
	for i, c := range channels {
		out[i] = UpdateChannel{ID: c.ID, Name: c.Name}
	}
	return out, nil
}

// CheckSelfUpdate checks for an app update on the given branch.
func (b *Bindings) CheckSelfUpdate(branch string) (SelfUpdateInfo, error) {
	info, err := b.ic.CheckSelfUpdateForBranch(branch)
	if err != nil {
		return SelfUpdateInfo{}, err
	}
	hasUpdate := false
	isDevBuild := false
	if info.ReleaseCount > 0 && info.Current != info.Latest {
		currentDate, dateErr := version.CommitDateTime()
		if dateErr != nil {
			hasUpdate = true
		} else {
			switch {
			case currentDate.Before(info.LatestDate):
				hasUpdate = true
			case currentDate.After(info.LatestDate):
				isDevBuild = true
			}
		}
	}
	return SelfUpdateInfo{
		Current:      info.Current,
		Latest:       info.Latest,
		HasUpdate:    hasUpdate,
		IsDevBuild:   isDevBuild,
		Body:         info.LatestBody,
		LatestDate:   info.LatestDate,
		ReleaseCount: info.ReleaseCount,
	}, nil
}

// InstallSelfUpdate downloads and installs the latest app update for a branch.
func (b *Bindings) InstallSelfUpdate(branch string) error {
	u := b.ic.SelfUpdater()
	if u == nil {
		return fmt.Errorf("self updater not configured")
	}
	info, err := b.ic.CheckSelfUpdateForBranch(branch)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(b.ctx, 10*time.Minute)
	defer cancel()
	return u.Install(ctx, info, func(downloaded, total int64) {
		progress := 0
		if total > 0 {
			progress = int(float64(downloaded) / float64(total) * 100)
		}
		b.emit("selfupdate:progress", map[string]int64{
			"downloaded": downloaded,
			"total":      total,
			"progress":   int64(progress),
		})
	})
}

// OpenDataDir opens the application data directory in the file manager.
func (b *Bindings) OpenDataDir() error {
	return b.app.Controller.OpenDataDir()
}

// OpenURL opens a URL in the default browser.
func (b *Bindings) OpenURL(url string) error {
	return openurl.OpenURL(url)
}

// GetReleaseNotes fetches release notes for a specific version.
func humanDurationOrEmpty(t time.Time, err error) string {
	if err != nil {
		return ""
	}
	return version.HumanDuration(t)
}

func (b *Bindings) GetReleaseNotes(ver string) (string, error) {
	release, err := updater.GetReleaseByVersion(ver)
	if err != nil {
		if release.Version == "" {
			return "", fmt.Errorf("release notes not found")
		}
		return "", err
	}
	dateStr := release.PublishedAt.Local().Format("2006-01-02 15:04:05")
	ago := version.HumanDuration(release.PublishedAt)
	return fmt.Sprintf("# %s: %s\n\nReleased: %s (%s)\n\n%s",
		release.Version, release.Name, dateStr, ago, release.Body), nil
}
