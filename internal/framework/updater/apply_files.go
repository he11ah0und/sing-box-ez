package updater

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/logger"
	luavm "sing-box-ez/internal/framework/lua"
	frameworkprogress "sing-box-ez/internal/framework/progress"
)

// FilesUpdateApply downloads one or more release assets and writes each to a
// configured destination path using the framework file system. It is intended
// for updating auxiliary binaries (e.g. sing-box core) or any other files
// without touching the running application executable.
type FilesUpdateApply struct {
	Log           *logger.LogTerminal
	FS            fs.FileSystem
	BaseDir       string
	InstallScript []byte
}

// NewFilesUpdateApply creates a FilesUpdateApply with a logger allocated from parent.
func NewFilesUpdateApply(parent *logger.LogTerminal, fsys fs.FileSystem) *FilesUpdateApply {
	return &FilesUpdateApply{Log: parent.Allocate("apply"), FS: fsys}
}

// Name returns the apply strategy identifier.
func (a *FilesUpdateApply) Name() string { return "files-update" }

// Apply downloads every file listed in info.Files and replaces the destination files.
func (a *FilesUpdateApply) Apply(ctx context.Context, source Source, info UpdateInfo, onProgress func(downloaded, total int64)) error {
	if len(info.Files) == 0 {
		return a.Log.Errorf("no files to update")
	}

	for _, uf := range info.Files {
		if err := a.updateFile(ctx, source, info, uf, onProgress); err != nil {
			return err
		}
	}
	return nil
}

func (a *FilesUpdateApply) updateFile(ctx context.Context, source Source, info UpdateInfo, uf UpdateFile, onProgress func(downloaded, total int64)) error {
	if uf.Asset.URL == "" {
		return a.Log.Errorf("file %q has no download URL", uf.DestPath)
	}
	if uf.DestPath == "" {
		return a.Log.Errorf("destination path not configured")
	}
	if a.FS == nil {
		return a.Log.Errorf("file system not configured")
	}

	tmpName := fmt.Sprintf(".sing-box-ez-%d.tmp", os.Getpid())
	tmpDir := filepath.Dir(uf.DestPath)
	if uf.Asset.Format != FormatRaw {
		tmpDir = uf.DestPath
	}
	if err := a.FS.MkdirAll(tmpDir, 0750); err != nil {
		return a.Log.Errorf("cannot prepare temp dir for %q: %v", uf.DestPath, err)
	}
	tmpPath := filepath.Join(tmpDir, tmpName)
	if a.BaseDir != "" {
		// NewArchiveFS opens the path directly with os.Open, so it must be
		// absolute to be found regardless of the process working directory.
		tmpPath = filepath.Join(a.BaseDir, tmpPath)
	}

	f, err := a.FS.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0750)
	if err != nil {
		return a.Log.Errorf("cannot create temporary file for %q: %v", uf.DestPath, err)
	}
	cleanup := func() { _ = a.FS.Remove(tmpPath) }

	downloadErr := source.DownloadAsset(ctx, uf.Asset, f, onProgress)
	if closeErr := f.Close(); closeErr != nil && downloadErr == nil {
		downloadErr = closeErr
	}
	if downloadErr != nil {
		cleanup()
		return fmt.Errorf("download %s failed: %w", uf.Asset.Name, downloadErr)
	}

	if len(a.InstallScript) > 0 {
		assetFS, err := a.assetFS(uf.Asset.Format, tmpPath)
		if err != nil {
			cleanup()
			return a.Log.Errorf("open asset fs: %v", err)
		}
		defer a.closeAssetFS(assetFS)

		vm := luavm.NewVM(a.Log, a.BaseDir, a.FS, []string{uf.DestPath}, tmpPath)
		vm.Progress = toProgressConfig("copy", onProgress)
		if assetFS != nil {
			vm.SetAssetFS(assetFS)
		}
		defer vm.Close()

		result, err := vm.Run(a.InstallScript, luavm.InstallContext{
			Asset: luavm.AssetInfo{
				Path:   tmpPath,
				Format: uf.Asset.Format,
				Name:   uf.Asset.Name,
				Size:   uf.Asset.Size,
			},
			Release: luavm.ReleaseInfo{
				Version:     info.Latest,
				Channel:     "",
				Body:        info.LatestBody,
				PublishedAt: info.LatestDate,
			},
		})
		if err != nil {
			cleanup()
			return err
		}
		if result.ReplaceBinary != "" {
			_ = result.ReplaceBinary // files-update does not replace running binary
		}
		cleanup()
		return nil
	}

	switch uf.Asset.Format {
	case FormatRaw:
		if err := a.FS.Rename(tmpPath, uf.DestPath); err != nil {
			cleanup()
			return a.Log.Errorf("replace %q failed: %v", uf.DestPath, err)
		}
		return nil
	case FormatZIP, FormatTarGz, FormatTarBz2:
		assetFS, err := fs.NewArchiveFS(tmpPath, uf.Asset.Format)
		if err != nil {
			cleanup()
			return a.Log.Errorf("open archive %q: %v", tmpPath, err)
		}
		if err := fs.Copy(ctx, assetFS, a.FS, ".", uf.DestPath, fs.CopyOptions{
			Recursive:    true,
			PreserveMode: true,
			Progress:     toProgressConfig("copy", onProgress),
		}); err != nil {
			cleanup()
			return a.Log.Errorf("extract archive %s to %q failed: %v", uf.Asset.Name, uf.DestPath, err)
		}
		cleanup()
		return nil
	default:
		cleanup()
		return a.Log.Errorf("unsupported asset format %q for %q", uf.Asset.Format, uf.Asset.Name)
	}
}

func (a *FilesUpdateApply) assetFS(format, tmpPath string) (fs.FileSystem, error) {
	switch format {
	case FormatRaw:
		return fs.NewOSFileSystem(filepath.Dir(tmpPath)), nil
	case FormatZIP, FormatTarGz, FormatTarBz2:
		return fs.NewArchiveFS(tmpPath, format)
	default:
		return nil, fmt.Errorf("unsupported asset format %q", format)
	}
}

func (a *FilesUpdateApply) closeAssetFS(fsys fs.FileSystem) {
	// ArchiveFS may hold resources in the future; for now no explicit close needed.
	_ = fsys
}

func toProgressConfig(op string, fn func(downloaded, total int64)) *frameworkprogress.Config {
	if fn == nil {
		return nil
	}
	return &frameworkprogress.Config{
		Callback: func(s frameworkprogress.State) {
			if s.Op == op {
				fn(s.Current, s.Total)
			}
		},
		Interval: 0,
	}
}
