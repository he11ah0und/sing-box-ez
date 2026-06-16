package updater

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/logger"
	luavm "sing-box-ez/internal/framework/lua"
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
	Log           *logger.LogTerminal
	FS            fs.FileSystem
	BaseDir       string
	InstallScript []byte
	Platform      selfUpdatePlatform
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
func (a *SelfUpdateApply) Apply(ctx context.Context, source Source, info UpdateInfo, onProgress func(downloaded, total int64)) error {
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
		return a.applyRaw(ctx, source, info, exe, platform, onProgress)
	case FormatZIP, FormatTarGz, FormatTarBz2:
		return a.applyArchive(ctx, source, info, exe, platform, onProgress)
	default:
		return a.Log.Errorf("unsupported asset format %q", info.Asset.Format)
	}
}

func (a *SelfUpdateApply) applyRaw(ctx context.Context, source Source, info UpdateInfo, exe string, platform selfUpdatePlatform, onProgress func(downloaded, total int64)) error {
	tmp := exe + ".tmp"
	if err := a.downloadAssetToFile(ctx, source, info.Asset, tmp, onProgress); err != nil {
		return err
	}

	if len(a.InstallScript) > 0 {
		vm := luavm.NewVM(a.Log, a.BaseDir, a.FS, []string{tmp}, tmp)
		vm.Progress = toProgressConfig("copy", onProgress)
		defer vm.Close()

		result, err := vm.Run(a.InstallScript, a.installContext(info, tmp))
		if err != nil {
			_ = a.FS.Remove(tmp)
			return err
		}
		if result.ReplaceBinary != "" {
			tmp = result.ReplaceBinary
		}
	}

	if err := platform.replace(exe, tmp); err != nil {
		_ = a.FS.Remove(tmp)
		return fmt.Errorf("replace binary failed: %w", err)
	}

	return platform.restart(exe)
}

func (a *SelfUpdateApply) applyArchive(ctx context.Context, source Source, info UpdateInfo, exe string, platform selfUpdatePlatform, onProgress func(downloaded, total int64)) error {
	tmpDir, err := os.MkdirTemp("", "sing-box-ez-update-*")
	if err != nil {
		return a.Log.Errorf("cannot create extract dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, info.Asset.Name)
	if err := a.downloadAssetToFile(ctx, source, info.Asset, tmpFile, onProgress); err != nil {
		return err
	}

	var replaceWith string
	if len(a.InstallScript) > 0 {
		assetFS, err := fs.NewArchiveFS(tmpFile, info.Asset.Format)
		if err != nil {
			return a.Log.Errorf("open archive fs: %v", err)
		}

		vm := luavm.NewVM(a.Log, a.BaseDir, a.FS, []string{tmpDir}, tmpFile)
		vm.SetAssetFS(assetFS)
		vm.Progress = toProgressConfig("copy", onProgress)
		defer vm.Close()

		result, err := vm.Run(a.InstallScript, a.installContext(info, tmpFile))
		if err != nil {
			return err
		}
		replaceWith = result.ReplaceBinary
	} else {
		assetFS, err := fs.NewArchiveFS(tmpFile, info.Asset.Format)
		if err != nil {
			return a.Log.Errorf("open archive fs: %v", err)
		}
		newExe, err := findBinaryInDir(assetFS, tmpDir, filepath.Base(exe))
		if err != nil {
			return a.Log.Errorf("locate updated binary: %v", err)
		}
		if newExe == "" {
			return a.Log.Errorf("binary %q not found in archive", filepath.Base(exe))
		}
		replaceWith = newExe
	}

	if replaceWith == "" {
		return a.Log.Errorf("install script did not return replace_binary")
	}

	if err := platform.replace(exe, replaceWith); err != nil {
		return fmt.Errorf("replace binary failed: %w", err)
	}

	return platform.restart(exe)
}

// findBinaryInDir recursively searches fsys under dir for a file named name.
func findBinaryInDir(fsys fs.FileSystem, dir, name string) (string, error) {
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() {
			if found, err := findBinaryInDir(fsys, path, name); err != nil || found != "" {
				return found, err
			}
			continue
		}
		if e.Name() == name {
			return path, nil
		}
	}
	return "", nil
}

func (a *SelfUpdateApply) downloadAssetToFile(ctx context.Context, source Source, asset Asset, path string, onProgress func(downloaded, total int64)) error {
	f, err := a.FS.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0750)
	if err != nil {
		return a.Log.Errorf("cannot create %q: %v", path, err)
	}

	downloadErr := source.DownloadAsset(ctx, asset, f, onProgress)
	if closeErr := f.Close(); closeErr != nil && downloadErr == nil {
		downloadErr = closeErr
	}
	if downloadErr != nil {
		_ = a.FS.Remove(path)
		return fmt.Errorf("download failed: %w", downloadErr)
	}
	return nil
}

func (a *SelfUpdateApply) installContext(info UpdateInfo, assetPath string) luavm.InstallContext {
	return luavm.InstallContext{
		Asset: luavm.AssetInfo{
			Path:   assetPath,
			Format: info.Asset.Format,
			Name:   info.Asset.Name,
			Size:   info.Asset.Size,
		},
		Release: luavm.ReleaseInfo{
			Version:     info.Latest,
			Channel:     "",
			Body:        info.LatestBody,
			PublishedAt: info.LatestDate,
		},
	}
}


