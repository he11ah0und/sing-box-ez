//go:build !nogui

package wails

import (
	"context"
	"fmt"
	"strings"
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
		b.toastErr(err)
		return SelfUpdateInfo{}, err
	}
	result := toSelfUpdateInfo(info)
	switch {
	case result.HasUpdate:
		b.toast("success",
			fmt.Sprintf(b.t("about", "update", "available"), result.Latest),
			b.t("about", "update", "current_version")+" "+result.Current+
				" → "+b.t("about", "update", "latest")+" "+result.Latest)
	case result.IsDevBuild:
		b.toast("warning",
			fmt.Sprintf(b.t("about", "update", "dev_build"), result.Current, result.Latest), "")
	default:
		b.toast("info", b.t("about", "update", "up_to_date"), "")
	}
	return result, nil
}

// toSelfUpdateInfo converts an updater result into the UI-facing form,
// applying the same has-update/dev-build logic the legacy Gio UI used.
func toSelfUpdateInfo(info *updater.UpdateInfo) SelfUpdateInfo {
	hasUpdate, isDevBuild := selfUpdateStatus(info)
	return SelfUpdateInfo{
		Current:      info.Current,
		Latest:       info.Latest,
		HasUpdate:    hasUpdate,
		IsDevBuild:   isDevBuild,
		Body:         info.LatestBody,
		LatestDate:   info.LatestDate,
		ReleaseCount: info.ReleaseCount,
	}
}

// selfUpdateStatus reports whether the app is behind the latest release or
// ahead of it (dev build). Ported from the Gio startupUpdateStatus.
func selfUpdateStatus(info *updater.UpdateInfo) (hasUpdate, isDevBuild bool) {
	if info.ReleaseCount == 0 || info.Current == info.Latest {
		return false, false
	}
	currentDate, err := version.CommitDateTime()
	if err != nil {
		return true, false
	}
	switch {
	case currentDate.Before(info.LatestDate):
		return true, false
	case currentDate.After(info.LatestDate):
		return false, true
	}
	return false, false
}

// normalizeCoreVersion ensures a leading "v" so installed and latest core
// versions compare equal. Ported from the Gio GUI.
func normalizeCoreVersion(v string) string {
	if v == "" {
		return v
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

// runUpdateChecks performs the startup/periodic update checks, mirroring the
// legacy Gio flow: first the application self-update, then the core update.
// Each check respects its updates.auto_check_* setting and only reports an
// available update; installation is always initiated by the user.
func (b *Bindings) runUpdateChecks() {
	b.checkSelfUpdateAvailable()
	b.checkCoreUpdateAvailable()
}

func (b *Bindings) checkSelfUpdateAvailable() {
	if b.ic == nil {
		return
	}
	cfg := b.app.Controller.Config()
	if !cfg.MustGet("updates", "auto_check_self").Bool() {
		return
	}
	info, err := b.ic.CheckSelfUpdate()
	if err != nil {
		b.app.Logger.Root.Warnf("background self-update check failed: %v", err)
		return
	}
	hasUpdate, isDevBuild := selfUpdateStatus(info)
	if !hasUpdate && !isDevBuild {
		return
	}
	b.emit("selfupdate:available", toSelfUpdateInfo(info))
}

func (b *Bindings) checkCoreUpdateAvailable() {
	cfg := b.app.Controller.Config()
	if !cfg.MustGet("updates", "auto_check_core").Bool() {
		return
	}
	current, _ := b.app.Controller.GetInstalledCoreVersion()
	latest, err := b.app.Controller.GetLatestCoreVersion()
	if err != nil {
		// The updater/network layers already log the failure with context.
		return
	}
	current = normalizeCoreVersion(current)
	latest = normalizeCoreVersion(latest)
	if current == latest || latest == "" {
		return
	}
	b.emit("core:version", map[string]string{"latest": latest})
	b.emit("core:update_available", map[string]string{
		"current": current,
		"latest":  latest,
	})
}

// InstallSelfUpdate downloads and installs the latest app update for a branch.
func (b *Bindings) InstallSelfUpdate(branch string) error {
	u := b.ic.SelfUpdater()
	if u == nil {
		err := fmt.Errorf("self updater not configured")
		b.toastErr(err)
		return err
	}
	info, err := b.ic.CheckSelfUpdateForBranch(branch)
	if err != nil {
		b.toastErr(err)
		return err
	}
	ctx, cancel := context.WithTimeout(b.ctx, 10*time.Minute)
	defer cancel()
	err = u.Install(ctx, info, func(downloaded, total int64) {
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
	if err != nil {
		b.toastErr(err)
		return err
	}
	b.toastT("success", []string{"about", "update", "installed"})
	return nil
}

// OpenDataDir opens the application data directory in the file manager.
func (b *Bindings) OpenDataDir() error {
	if err := b.app.Controller.OpenDataDir(); err != nil {
		b.toastErr(err)
		return err
	}
	return nil
}

// OpenURL opens a URL in the default browser.
func (b *Bindings) OpenURL(url string) error {
	if err := openurl.OpenURL(url); err != nil {
		b.toastErr(err)
		return err
	}
	return nil
}

// OpenProjectURL opens the application repository page in the default
// browser. The URL comes from the self-updater source, not from a literal.
func (b *Bindings) OpenProjectURL() error {
	l, ok := b.app.SelfUpdater.Source.(updater.Linker)
	if !ok {
		return fmt.Errorf("self-updater source %q does not expose project URLs", b.app.SelfUpdater.Source.Name())
	}
	return b.OpenURL(l.ProjectURL())
}

// OpenReleaseURL opens the release page for the given tag in the default
// browser. The URL comes from the self-updater source, not from a literal.
func (b *Bindings) OpenReleaseURL(tag string) error {
	l, ok := b.app.SelfUpdater.Source.(updater.Linker)
	if !ok {
		return fmt.Errorf("self-updater source %q does not expose project URLs", b.app.SelfUpdater.Source.Name())
	}
	return b.OpenURL(l.ReleaseURL(tag))
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
		b.toastT("error", []string{"about", "release_notes", "error"})
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
