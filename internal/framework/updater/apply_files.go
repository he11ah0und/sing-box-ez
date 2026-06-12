package updater

import (
	"context"
	"os"
	"path/filepath"

	"sing-box-ez/internal/framework/logger"
)

// FilesUpdateApply downloads one or more release assets and writes each to a
// configured destination path. It is intended for updating auxiliary binaries
// (e.g. sing-box core) or any other files without touching the running
// application executable.
type FilesUpdateApply struct {
	Log *logger.LogTerminal
}

// NewFilesUpdateApply creates a FilesUpdateApply with a logger allocated from parent.
func NewFilesUpdateApply(parent *logger.LogTerminal) *FilesUpdateApply {
	return &FilesUpdateApply{Log: parent.Allocate("apply")}
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

	tmpDir := uf.DestPath
	if uf.Asset.Format == FormatRaw {
		tmpDir = filepath.Dir(uf.DestPath)
	}
	if err := os.MkdirAll(tmpDir, 0750); err != nil {
		return a.Log.Errorf("cannot prepare temp dir for %q: %v", uf.DestPath, err)
	}

	tmpFile, err := os.CreateTemp(tmpDir, ".sing-box-ez-*")
	if err != nil {
		return a.Log.Errorf("cannot create temporary file for %q: %v", uf.DestPath, err)
	}
	tmpPath := tmpFile.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	downloadErr := source.DownloadAsset(ctx, uf.Asset, tmpFile, progress)
	if closeErr := tmpFile.Close(); closeErr != nil && downloadErr == nil {
		downloadErr = closeErr
	}
	if downloadErr != nil {
		cleanup()
		return a.Log.Errorf("download %s failed: %v", uf.Asset.Name, downloadErr)
	}

	switch uf.Asset.Format {
	case FormatRaw:
		if err := os.Chmod(tmpPath, 0750); err != nil {
			cleanup()
			return a.Log.Errorf("chmod %q failed: %v", tmpPath, err)
		}
		if err := os.Rename(tmpPath, uf.DestPath); err != nil {
			cleanup()
			return a.Log.Errorf("replace %q failed: %v", uf.DestPath, err)
		}
		return nil
	case FormatZIP, FormatTarGz, FormatTarBz2:
		f, err := os.Open(tmpPath)
		if err != nil {
			cleanup()
			return a.Log.Errorf("open downloaded archive %q: %v", tmpPath, err)
		}
		defer f.Close()

		if err := extractArchive(uf.Asset.Format, f, uf.DestPath); err != nil {
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
