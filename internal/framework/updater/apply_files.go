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
	FS            fs.FS
	BaseDir       string
	InstallScript []byte
	// BeforeInstall is called after the asset has been downloaded but before
	// it is applied. It can be used to stop a running process whose binary is
	// about to be replaced.
	BeforeInstall func() error
}

// NewFilesUpdateApply creates a FilesUpdateApply with a logger allocated from parent.
func NewFilesUpdateApply(parent *logger.LogTerminal, fsys fs.FS) *FilesUpdateApply {
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
	if err := a.validateFile(uf); err != nil {
		return err
	}
	tmpFile, tmpPath, cleanup, err := a.downloadTemp(ctx, source, uf, onProgress)
	if err != nil {
		return err
	}
	if a.BeforeInstall != nil {
		if err := a.BeforeInstall(); err != nil {
			cleanup()
			return err
		}
	}
	if len(a.InstallScript) > 0 {
		return a.applyInstallScript(ctx, uf, info, tmpPath, cleanup, onProgress)
	}
	return a.applyDownloaded(uf, tmpFile, tmpPath)
}

func (a *FilesUpdateApply) validateFile(uf UpdateFile) error {
	if uf.Asset.URL == "" {
		return a.Log.Errorf("file %q has no download URL", uf.DestPath)
	}
	if uf.DestPath == "" {
		return a.Log.Errorf("destination path not configured")
	}
	if a.FS == nil {
		return a.Log.Errorf("file system not configured")
	}
	return nil
}

func (a *FilesUpdateApply) downloadTemp(ctx context.Context, source Source, uf UpdateFile, onProgress func(downloaded, total int64)) (fs.File, string, func(), error) {
	tmpName := fmt.Sprintf(".sing-box-ez-%d.tmp", os.Getpid())
	tmpDir := filepath.Dir(uf.DestPath)
	if uf.Asset.Format != FormatRaw {
		tmpDir = uf.DestPath
	}
	tmpDirObj := a.FS.Root().Subdir(tmpDir)
	if err := tmpDirObj.MkdirAll(0750); err != nil {
		return nil, "", nil, a.Log.Errorf("cannot prepare temp dir for %q: %v", uf.DestPath, err)
	}
	tmpPath := filepath.Join(tmpDir, tmpName)
	if a.BaseDir != "" {
		tmpPath = filepath.Join(a.BaseDir, tmpPath)
	}

	tmpFile := a.FS.Root().File(tmpPath)
	f, err := tmpFile.OpenFile(os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0750)
	if err != nil {
		return nil, "", nil, a.Log.Errorf("cannot create temporary file for %q: %v", uf.DestPath, err)
	}
	cleanup := func() { _ = tmpFile.Remove() }

	downloadErr := source.DownloadAsset(ctx, uf.Asset, f, onProgress)
	if closeErr := f.Close(); closeErr != nil && downloadErr == nil {
		downloadErr = closeErr
	}
	if downloadErr != nil {
		cleanup()
		return nil, "", nil, fmt.Errorf("download %s failed: %w", uf.Asset.Name, downloadErr)
	}
	return tmpFile, tmpPath, cleanup, nil
}

func (a *FilesUpdateApply) applyInstallScript(ctx context.Context, uf UpdateFile, info UpdateInfo, tmpPath string, cleanup func(), onProgress func(downloaded, total int64)) error {
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

func (a *FilesUpdateApply) applyDownloaded(uf UpdateFile, tmpFile fs.File, tmpPath string) error {
	switch uf.Asset.Format {
	case FormatRaw:
		if err := tmpFile.Rename(filepath.Base(uf.DestPath)); err != nil {
			_ = tmpFile.Remove()
			return a.Log.Errorf("replace %q failed: %v", uf.DestPath, err)
		}
		return nil
	case FormatZIP, FormatTarGz, FormatTarBz2:
		assetFS, err := fs.NewArchiveFS(tmpPath, uf.Asset.Format)
		if err != nil {
			_ = tmpFile.Remove()
			return a.Log.Errorf("open archive %q: %v", tmpPath, err)
		}
		if err := assetFS.Root().CopyTo(a.FS.Root().Subdir(uf.DestPath)); err != nil {
			_ = tmpFile.Remove()
			return a.Log.Errorf("extract archive %s to %q failed: %v", uf.Asset.Name, uf.DestPath, err)
		}
		_ = tmpFile.Remove()
		return nil
	default:
		_ = tmpFile.Remove()
		return a.Log.Errorf("unsupported asset format %q for %q", uf.Asset.Format, uf.Asset.Name)
	}
}

func (a *FilesUpdateApply) assetFS(format, tmpPath string) (fs.FS, error) {
	switch format {
	case FormatRaw:
		return fs.NewOS(filepath.Dir(tmpPath)), nil
	case FormatZIP, FormatTarGz, FormatTarBz2:
		return fs.NewArchiveFS(tmpPath, format)
	default:
		return nil, fmt.Errorf("unsupported asset format %q", format)
	}
}

func (a *FilesUpdateApply) closeAssetFS(fsys fs.FS) {
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
