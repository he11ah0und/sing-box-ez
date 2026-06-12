package updater

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

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
	Platform selfUpdatePlatform
}

// NewSelfUpdateApply creates a SelfUpdateApply with a logger allocated from parent.
func NewSelfUpdateApply(parent *logger.LogTerminal) *SelfUpdateApply {
	return &SelfUpdateApply{
		Log:      parent.Allocate("apply"),
		Platform: newSelfUpdatePlatform(),
	}
}

// Name returns the apply strategy identifier.
func (a *SelfUpdateApply) Name() string { return "self-update" }

// Apply downloads the update asset and replaces the running binary.
func (a *SelfUpdateApply) Apply(ctx context.Context, source Source, info UpdateInfo, progress func(downloaded, total int64)) error {
	if info.Asset.URL == "" {
		return a.Log.Errorf("no asset URL provided")
	}

	exe, err := os.Executable()
	if err != nil {
		return a.Log.Errorf("cannot locate executable: %v", err)
	}

	platform := a.Platform
	if platform == nil {
		platform = newSelfUpdatePlatform()
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
		_ = os.Remove(tmp)
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

	f, err := os.Open(tmpFile)
	if err != nil {
		return a.Log.Errorf("open downloaded archive: %v", err)
	}
	extractErr := extractArchive(info.Asset.Format, f, tmpDir)
	_ = f.Close()
	if extractErr != nil {
		return a.Log.Errorf("extract archive failed: %v", extractErr)
	}

	newExe, err := findBinaryInDir(tmpDir, filepath.Base(exe))
	if err != nil {
		return a.Log.Errorf("locate updated binary: %v", err)
	}

	if err := platform.replace(exe, newExe); err != nil {
		return a.Log.Errorf("replace binary failed: %v", err)
	}

	return platform.restart(exe)
}

func (a *SelfUpdateApply) downloadAssetToFile(ctx context.Context, source Source, asset Asset, path string, progress func(downloaded, total int64)) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0750)
	if err != nil {
		return a.Log.Errorf("cannot create %q: %v", path, err)
	}

	downloadErr := source.DownloadAsset(ctx, asset, f, progress)
	if closeErr := f.Close(); closeErr != nil && downloadErr == nil {
		downloadErr = closeErr
	}
	if downloadErr != nil {
		_ = os.Remove(path)
		return a.Log.Errorf("download failed: %v", downloadErr)
	}
	return nil
}

func findBinaryInDir(dir, base string) (string, error) {
	var found string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == base {
			found = path
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("binary %q not found in archive", base)
	}
	return found, nil
}
