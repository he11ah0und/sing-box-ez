package updater

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/logger"
)

// FilesUpdateApply downloads one or more release assets and writes each to a
// configured destination path using the framework file system. It is intended
// for updating auxiliary binaries (e.g. sing-box core) or any other files
// without touching the running application executable.
type FilesUpdateApply struct {
	Log *logger.LogTerminal
	FS  fs.FileSystem
}

// NewFilesUpdateApply creates a FilesUpdateApply with a logger allocated from parent.
func NewFilesUpdateApply(parent *logger.LogTerminal, fsys fs.FileSystem) *FilesUpdateApply {
	return &FilesUpdateApply{Log: parent.Allocate("apply"), FS: fsys}
}

// Name returns the apply strategy identifier.
func (a *FilesUpdateApply) Name() string { return "files-update" }

// Apply downloads every file listed in info.Files and replaces the destination files.
func (a *FilesUpdateApply) Apply(ctx context.Context, source Source, info UpdateInfo, progress func(downloaded, total int64)) error {
	if len(info.Files) == 0 {
		return a.Log.Errorf("no files to update")
	}

	for _, uf := range info.Files {
		if err := a.updateFile(ctx, source, uf, progress); err != nil {
			return err
		}
	}
	return nil
}

func (a *FilesUpdateApply) updateFile(ctx context.Context, source Source, uf UpdateFile, progress func(downloaded, total int64)) error {
	if uf.Asset.URL == "" {
		return a.Log.Errorf("file %q has no download URL", uf.DestPath)
	}
	if uf.DestPath == "" {
		return a.Log.Errorf("destination path not configured")
	}
	if a.FS == nil {
		return a.Log.Errorf("file system not configured")
	}

	tmpDir := uf.DestPath
	if uf.Asset.Format == FormatRaw {
		tmpDir = filepath.Dir(uf.DestPath)
	}
	if err := a.FS.MkdirAll(tmpDir, 0750); err != nil {
		return a.Log.Errorf("cannot prepare temp dir for %q: %v", uf.DestPath, err)
	}

	tmpName := fmt.Sprintf(".sing-box-ez-%d.tmp", os.Getpid())
	tmpPath := filepath.Join(tmpDir, tmpName)
	f, err := a.FS.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0750)
	if err != nil {
		return a.Log.Errorf("cannot create temporary file for %q: %v", uf.DestPath, err)
	}
	cleanup := func() { _ = a.FS.Remove(tmpPath) }

	downloadErr := source.DownloadAsset(ctx, uf.Asset, f, progress)
	if closeErr := f.Close(); closeErr != nil && downloadErr == nil {
		downloadErr = closeErr
	}
	if downloadErr != nil {
		cleanup()
		return a.Log.Errorf("download %s failed: %v", uf.Asset.Name, downloadErr)
	}

	switch uf.Asset.Format {
	case FormatRaw:
		if err := a.FS.Rename(tmpPath, uf.DestPath); err != nil {
			cleanup()
			return a.Log.Errorf("replace %q failed: %v", uf.DestPath, err)
		}
		return nil
	case FormatZIP, FormatTarGz, FormatTarBz2:
		if err := extractArchive(a.FS, uf.Asset.Format, tmpPath, uf.DestPath); err != nil {
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
