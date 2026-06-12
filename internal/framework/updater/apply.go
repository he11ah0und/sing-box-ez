package updater

import (
	"context"
	"errors"
	"fmt"
	"os"

	"sing-box-ez/internal/framework/fs"
)

// Apply abstracts how a downloaded update is installed.
type Apply interface {
	// Name returns the apply strategy identifier.
	Name() string

	// Apply installs the update described by info using source for asset download.
	Apply(ctx context.Context, source Source, info UpdateInfo, progress func(downloaded, total int64)) error
}

// SelfUpdateApply replaces the running binary and restarts the process.
type SelfUpdateApply struct {
	// RestartFunc is called after the binary has been replaced. On Unix this
	// should normally re-exec the current process; on Windows it should hand
	// over to a helper and exit. If nil, a platform-specific default is used.
	RestartFunc func(exe string) error
}

// Name returns the apply strategy identifier.
func (a *SelfUpdateApply) Name() string { return "self-update" }

// Apply downloads the update asset and replaces the running binary.
func (a *SelfUpdateApply) Apply(ctx context.Context, source Source, info UpdateInfo, progress func(downloaded, total int64)) error {
	if info.Asset.URL == "" {
		return fmt.Errorf("no asset URL provided")
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot locate executable: %w", err)
	}

	tmp := exe + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0750)
	if err != nil {
		return fmt.Errorf("cannot create temporary binary: %w", err)
	}

	downloadErr := source.DownloadAsset(ctx, info.Asset, f, progress)
	if closeErr := f.Close(); closeErr != nil && downloadErr == nil {
		downloadErr = closeErr
	}
	if downloadErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("download failed: %w", downloadErr)
	}

	// #nosec G302 — tmp is the replacement binary for the current executable; must remain executable.
	if err := os.Chmod(tmp, 0750); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("chmod failed: %w", err)
	}

	if err := os.Rename(tmp, exe); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace failed: %w", err)
	}

	restart := a.RestartFunc
	if restart == nil {
		restart = defaultRestart
	}
	return restart(exe)
}

// CoreUpdateApply downloads a release asset and writes it to a configured
// destination path using the framework file system. It is intended for
// updating the sing-box core (or other auxiliary binaries) without touching
// the running application.
type CoreUpdateApply struct {
	FS       fs.FileSystem
	DestPath string
}

// Name returns the apply strategy identifier.
func (a *CoreUpdateApply) Name() string { return "core-update" }

// Apply downloads the update asset and replaces the destination file.
func (a *CoreUpdateApply) Apply(ctx context.Context, source Source, info UpdateInfo, progress func(downloaded, total int64)) error {
	if info.Asset.URL == "" {
		return fmt.Errorf("no asset URL provided")
	}
	if a.FS == nil {
		return fmt.Errorf("file system not configured")
	}
	if a.DestPath == "" {
		return fmt.Errorf("destination path not configured")
	}

	tmp := a.DestPath + ".tmp"
	f, err := a.FS.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0750)
	if err != nil {
		return fmt.Errorf("cannot create temporary file: %w", err)
	}

	downloadErr := source.DownloadAsset(ctx, info.Asset, f, progress)
	if closeErr := f.Close(); closeErr != nil && downloadErr == nil {
		downloadErr = closeErr
	}
	if downloadErr != nil {
		_ = a.FS.Remove(tmp)
		return fmt.Errorf("download failed: %w", downloadErr)
	}

	if err := a.FS.Rename(tmp, a.DestPath); err != nil {
		_ = a.FS.Remove(tmp)
		return fmt.Errorf("replace failed: %w", err)
	}

	return nil
}

// FileUpdateApply downloads the update asset to a configured destination.
// It is currently a stub; the interface is ready for future use.
type FileUpdateApply struct{}

// Name returns the apply strategy identifier.
func (a *FileUpdateApply) Name() string { return "file-update" }

// Apply is not yet implemented.
func (a *FileUpdateApply) Apply(_ context.Context, _ Source, _ UpdateInfo, _ func(downloaded, total int64)) error {
	return errors.New("file-update apply not implemented")
}
