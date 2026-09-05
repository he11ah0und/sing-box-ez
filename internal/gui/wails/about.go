//go:build !nogui

package wails

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/he11ah0und/localengine"
	"github.com/he11ah0und/logger"
	"golang.org/x/mod/semver"
	"sing-box-ez/internal/core"
	fwnet "sing-box-ez/internal/framework/net"
	"sing-box-ez/internal/framework/updater"
	"sing-box-ez/internal/framework/util/openurl"
	"sing-box-ez/internal/framework/version"
)

// VersionInfo holds build metadata for the about page.
type VersionInfo struct {
	Version     string `json:"version"`
	VersionTag  string `json:"versionTag"`
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

// UpdaterEntry describes one update manager declared in the project spec
// (internal/app/project.yaml), exposed to the updates tab.
type UpdaterEntry struct {
	Name          string   `json:"name"`
	SourceBackend string   `json:"sourceBackend"`
	Owner         string   `json:"owner"`
	Repo          string   `json:"repo"`
	BaseURL       string   `json:"baseURL"`
	AssetTags     []string `json:"assetTags"`
	ApplyBackend  string   `json:"applyBackend"`
	BaseDir       string   `json:"baseDir"`
	InstallScript string   `json:"installScript"`
}

// GetUpdateChannel returns the update channel this build belongs to
// (self-update or an external package manager, selected at compile time).
func (b *Bindings) GetUpdateChannel() updater.ChannelInfo {
	return updater.ActiveChannel()
}

// errExternalChannel is returned by self-update bindings on builds managed
// by an external package manager.
var errExternalChannel = errors.New("self-update is unavailable: this build is managed by an external package manager")

// guardExternalChannel reports whether the self-update bindings must refuse
// to run because the build is externally managed.
func (b *Bindings) guardExternalChannel() error {
	if !updater.ActiveChannel().External {
		return nil
	}
	b.toastT("error", []string{"about", "update", "external_managed"})
	return errExternalChannel
}

// GetUpdaters returns the updater definitions from the project spec so the
// updates tab lists every update manager the app was built with.
func (b *Bindings) GetUpdaters() []UpdaterEntry {
	spec := b.app.Spec
	if spec == nil {
		return nil
	}
	out := make([]UpdaterEntry, len(spec.Updaters))
	for i, u := range spec.Updaters {
		out[i] = UpdaterEntry{
			Name:          u.Name,
			SourceBackend: u.Source.Backend,
			Owner:         u.Source.Owner,
			Repo:          u.Source.Repo,
			BaseURL:       u.Source.BaseURL,
			AssetTags:     u.Source.AssetTags,
			ApplyBackend:  u.Apply.Backend,
			BaseDir:       u.Apply.BaseDir,
			InstallScript: u.Apply.InstallScript,
		}
	}
	return out
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
		Version:     version.Version,
		VersionTag:  version.Tag(),
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
	if err := b.guardExternalChannel(); err != nil {
		return nil, err
	}
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
	if err := b.guardExternalChannel(); err != nil {
		return SelfUpdateInfo{}, err
	}
	info, err := b.ic.CheckSelfUpdateForBranch(branch)
	if err != nil {
		b.toastErr(err)
		return SelfUpdateInfo{}, err
	}
	result := toSelfUpdateInfo(info)
	switch {
	case result.HasUpdate:
		b.toast("success",
			b.tf(localengine.Vars{"version": result.Latest}, "about", "update", "available"),
			b.t("about", "update", "current_version")+" "+result.Current+
				" → "+b.t("about", "update", "latest")+" "+result.Latest)
	case result.IsDevBuild:
		b.toast("warning",
			b.tf(localengine.Vars{"current": result.Current, "remote": result.Latest}, "about", "update", "dev_build"), "")
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
// ahead of it (dev build). Semver builds compare by version; legacy hash
// builds fall back to commit-date comparison.
func selfUpdateStatus(info *updater.UpdateInfo) (hasUpdate, isDevBuild bool) {
	if info.ReleaseCount == 0 || info.Current == info.Latest {
		return false, false
	}
	if semver.IsValid(info.Current) && semver.IsValid(info.Latest) {
		switch semver.Compare(info.Latest, info.Current) {
		case 1:
			return true, false
		case -1:
			return false, true
		}
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
	b.verifyUpdateChannel()
	b.checkSelfUpdateAvailable()
	b.checkCoreUpdateAvailable()
}

// verifyUpdateChannel asks the active update channel to confirm the version
// of the running binary. On externally managed builds (AUR, ...) a failure
// means the packaged binary is not tracked by the package manager — the app
// cannot update itself and its provenance is unverified — so the GUI warns
// the user with a blocking danger dialog.
func (b *Bindings) verifyUpdateChannel() {
	channel := updater.ActiveChannel()
	if !channel.External {
		return
	}
	if _, err := updater.ChannelCurrentVersion(); err != nil {
		b.app.Logger.Root.TErrorf("gui.channel_version_check_failed", err)
		b.emit("update:channel_error", map[string]any{
			"channel": channel,
			"error":   err.Error(),
		})
	}
}

func (b *Bindings) checkSelfUpdateAvailable() {
	if b.ic == nil || updater.ActiveChannel().External {
		return
	}
	cfg := b.app.Controller.Config()
	if !cfg.MustGet("updates", "auto_check_self").Bool() {
		return
	}
	info, err := b.ic.CheckSelfUpdate()
	if err != nil {
		b.app.Logger.Root.TWarnf("gui.self_update_check_failed", err)
		b.emitUpdateCheckFailed("app", err)
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
		b.emitUpdateCheckFailed("core", err)
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

// emitUpdateCheckFailed reports a failed update check to the frontend, which
// offers a retry routed through the running core's proxy. Only connectivity
// failures qualify — an HTTP error (rate limit, 404) would not be fixed by a
// proxy retry.
func (b *Bindings) emitUpdateCheckFailed(target string, err error) {
	switch core.DownloadErrorKind(err) {
	case core.FailureRefused, core.FailureTimeout, core.FailureDNS:
	default:
		return
	}
	b.emit("updatecheck:failed", map[string]string{
		"target": target,
		"kind":   core.DownloadErrorKind(err),
	})
}

// proxiedGitHubSource returns a copy of src whose HTTP client routes through
// the local proxy at proxyAddr. Non-GitHub sources cannot be proxied.
func proxiedGitHubSource(src updater.Source, proxyAddr string, parent *logger.LogTerminal) (updater.Source, error) {
	gh, ok := src.(*updater.GitHubBackend)
	if !ok {
		return nil, fmt.Errorf("update source does not support proxying")
	}
	return &updater.GitHubBackend{
		BaseURL: gh.BaseURL,
		Owner:   gh.Owner,
		Repo:    gh.Repo,
		Net:     fwnet.NewClientViaProxy(parent, proxyAddr),
		Log:     gh.Log,
	}, nil
}

// RetryUpdateCheckViaCore repeats a failed update check ("app" or "core")
// through the running core's local proxy, starting the core first (with the
// pre-start config refresh skipped once) when it is down. Starting the core
// just for the check requires a cached config to run from.
func (b *Bindings) RetryUpdateCheckViaCore(target string) error {
	c := b.app.Controller
	if c == nil {
		err := fmt.Errorf("local controller unavailable")
		b.toastErr(err)
		return err
	}
	if !c.IsRunning() {
		active := c.Config().GetActiveConfig()
		if active == nil || !c.HasCachedConfig(active.Name) {
			err := fmt.Errorf("no cached config to start the core with")
			b.toastErr(err)
			return err
		}
		c.SkipNextConfigUpdate()
		var err error
		if b.ic != nil {
			err = b.ic.StartService()
		} else {
			err = c.Start()
		}
		if err != nil {
			b.toastErr(err)
			return err
		}
	}
	addr := c.CoreProxyAddr()
	if addr == "" {
		err := fmt.Errorf("core has no local proxy inbound")
		b.toastErr(err)
		return err
	}

	switch target {
	case "app":
		m := updater.CurrentManager()
		if m == nil {
			err := fmt.Errorf("self updater not configured")
			b.toastErr(err)
			return err
		}
		src, err := proxiedGitHubSource(m.Source, addr, b.app.Logger.Root)
		if err != nil {
			b.toastErr(err)
			return err
		}
		tmp := *m
		tmp.Source = src
		info, err := tmp.Check(context.Background(), version.Branch)
		if err != nil {
			b.toastErr(err)
			return err
		}
		hasUpdate, isDevBuild := selfUpdateStatus(info)
		if hasUpdate || isDevBuild {
			b.emit("selfupdate:available", toSelfUpdateInfo(info))
		} else {
			b.toastT("info", []string{"about", "update", "up_to_date"})
		}
	case "core":
		latest, err := c.CheckCoreUpdateViaProxy(addr)
		if err != nil {
			b.toastErr(err)
			return err
		}
		current, _ := c.GetInstalledCoreVersion()
		current = normalizeCoreVersion(current)
		latest = normalizeCoreVersion(latest)
		b.emit("core:version", map[string]string{"latest": latest})
		if current != latest && latest != "" {
			b.emit("core:update_available", map[string]string{
				"current": current,
				"latest":  latest,
			})
		} else {
			b.toastT("info", []string{"about", "update", "up_to_date"})
		}
	default:
		err := fmt.Errorf("unknown update check target %q", target)
		b.toastErr(err)
		return err
	}
	return nil
}

// InstallSelfUpdate downloads and installs the latest app update for a branch.
func (b *Bindings) InstallSelfUpdate(branch string) error {
	if err := b.guardExternalChannel(); err != nil {
		return err
	}
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
	b.registerUpdateCancel("app", cancel)
	defer b.unregisterUpdateCancel("app")
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
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			// User-requested cancel: the modal is already closing, no toast.
			return nil
		}
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

// selfUpdateLinker resolves the project/release URL provider: the
// self-updater source on regular builds, or the spec-declared "updater"
// source on externally managed builds that ship no self-update manager.
func (b *Bindings) selfUpdateLinker() (updater.Linker, error) {
	if b.app.SelfUpdater != nil {
		if l, ok := b.app.SelfUpdater.Source.(updater.Linker); ok {
			return l, nil
		}
		return nil, fmt.Errorf("self-updater source %q does not expose project URLs", b.app.SelfUpdater.Source.Name())
	}
	if spec := b.app.Spec; spec != nil {
		for _, u := range spec.Updaters {
			if u.Name != "updater" || u.Source.Backend != "github" {
				continue
			}
			var src updater.Source
			if u.Source.BaseURL != "" {
				src = updater.NewGitHubEnterpriseBackend(b.app.Logger.Root, u.Source.BaseURL, u.Source.Owner, u.Source.Repo)
			} else {
				src = updater.NewGitHubBackend(b.app.Logger.Root, u.Source.Owner, u.Source.Repo)
			}
			if l, ok := src.(updater.Linker); ok {
				return l, nil
			}
		}
	}
	return nil, fmt.Errorf("no updater source exposes project URLs")
}

// OpenProjectURL opens the application repository page in the default
// browser. The URL comes from the self-updater source, not from a literal.
func (b *Bindings) OpenProjectURL() error {
	l, err := b.selfUpdateLinker()
	if err != nil {
		return err
	}
	return b.OpenURL(l.ProjectURL())
}

// OpenReleaseURL opens the release page for the given tag in the default
// browser. The URL comes from the self-updater source, not from a literal.
func (b *Bindings) OpenReleaseURL(tag string) error {
	l, err := b.selfUpdateLinker()
	if err != nil {
		return err
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
	if err := b.guardExternalChannel(); err != nil {
		return "", err
	}
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
