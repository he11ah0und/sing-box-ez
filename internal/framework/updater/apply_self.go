package updater

import (
	"context"
	"os"
	"path/filepath"

	"sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/logger"
)

// selfUpdatePlatform abstracts the platform-specific steps required to replace
// the running executable and restart it. Implementations live in
// selfupdate_unix.go and selfupdate_windows.go.
type selfUpdatePlatform interface {
	// replace makes newExe the current executable. On Unix this overwrites exe
	// directly; on Windows exe is rotated to exe.old first.
	replace(exe, newExe string) error

	// restart starts the updated executable. It does not return on success.
	restart(exe string) error
}

// SelfUpdateApply replaces the running binary and restarts the process.
type SelfUpdateApply struct {
	Log      *logger.LogTerminal
	FS       fs.FileSystem
	Platform selfUpdatePlatform
}

// NewSelfUpdateApply creates a SelfUpdateApply with a logger allocated from parent.
func NewSelfUpdateApply(parent *logger.LogTerminal, fsys fs.FileSystem) *SelfUpdateApply {
	log := parent.Allocate("apply")
	return &SelfUpdateApply{
		Log:      log,
		FS:       fsys,
		Platform: newSelfUpdatePlatform(log),
	}
}

// Name returns the apply strategy identifier.
func (a *SelfUpdateApply) Name() string { return "self-update" }

// Apply downloads the update asset and replaces the running binary.
func (a *SelfUpdateApply) Apply(ctx context.Context, source Source, info UpdateInfo, progress func(downloaded, total int64)) error {
	if info.Asset.URL == "" {
		return a.Log.Errorf("no asset URL provided")
	}
	if a.FS == nil {
		return a.Log.Errorf("file system not configured")
	}

	exe, err := os.Executable()
	if err != nil {
		return a.Log.Errorf("cannot locate executable: %v", err)
	}

	platform := a.Platform
	if platform == nil {
		platform = newSelfUpdatePlatform(a.Log)
	}

	switch info.Asset.Format {
	case FormatRaw:
		return a.applyRaw(ctx, source, info, exe, platform, progress)
	case FormatZIP, FormatTarGz, FormatTarBz2:
		return a.applyArchive(ctx, source, info, exe, platform, progress)
	default:
		return a.Log.Errorf("unsupported asset format %q", info.Asset.Format)
	}
}

func (a *SelfUpdateApply) applyRaw(ctx context.Context, source Source, info UpdateInfo, exe string, platform selfUpdatePlatform, progress func(downloaded, total int64)) error {
	tmp := exe + ".tmp"
	if err := a.downloadAssetToFile(ctx, source, info.Asset, tmp, progress); err != nil {
		return err
	}

	if err := platform.replace(exe, tmp); err != nil {
		_ = a.FS.Remove(tmp)
		return a.Log.Errorf("replace binary failed: %v", err)
	}

	return platform.restart(exe)
}

func (a *SelfUpdateApply) applyArchive(ctx context.Context, source Source, info UpdateInfo, exe string, platform selfUpdatePlatform, progress func(downloaded, total int64)) error {
	tmpDir, err := os.MkdirTemp("", "sing-box-ez-update-*")
	if err != nil {
		return a.Log.Errorf("cannot create extract dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, info.Asset.Name)
	if err := a.downloadAssetToFile(ctx, source, info.Asset, tmpFile, progress); err != nil {
		return err
	}

	if err := extractArchive(a.FS, info.Asset.Format, tmpFile, tmpDir); err != nil {
		return a.Log.Errorf("extract archive failed: %v", err)
	}

	newExe, err := findBinaryInDir(a.FS, tmpDir, filepath.Base(exe))
	if err != nil {
		return a.Log.Errorf("locate updated binary: %v", err)
	}
	if newExe == "" {
		return a.Log.Errorf("binary %q not found in archive", filepath.Base(exe))
	}

	if err := platform.replace(exe, newExe); err != nil {
		return a.Log.Errorf("replace binary failed: %v", err)
	}

	return platform.restart(exe)
}

func (a *SelfUpdateApply) downloadAssetToFile(ctx context.Context, source Source, asset Asset, path string, progress func(downloaded, total int64)) error {
	f, err := a.FS.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0750)
	if err != nil {
		return a.Log.Errorf("cannot create %q: %v", path, err)
	}

	downloadErr := source.DownloadAsset(ctx, asset, f, progress)
	if closeErr := f.Close(); closeErr != nil && downloadErr == nil {
		downloadErr = closeErr
	}
	if downloadErr != nil {
		_ = a.FS.Remove(path)
		return a.Log.Errorf("download failed: %v", downloadErr)
	}
	return nil
}
