package fs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"sing-box-ez/internal/framework/progress"
)

// CopyOptions controls the behaviour of Copy.
type CopyOptions struct {
	Recursive    bool
	PreserveMode bool
	Progress     *progress.Config
}

// Copy copies srcPath from srcFS to dstPath on dstFS. If srcPath is a
// directory and Recursive is true, the whole tree is copied. Progress is
// reported with op "copy".
func Copy(ctx context.Context, srcFS, dstFS FileSystem, srcPath, dstPath string, opts CopyOptions) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	reporter := progress.NewReporter(opts.Progress)

	// First pass: walk the source and compute total size.
	entries, total, err := walkCopySource(srcFS, srcPath, opts.Recursive)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		return fmt.Errorf("source %q is a directory; use Recursive=true to copy directories", srcPath)
	}

	if len(entries) == 1 && entries[0].src == srcPath {
		// Single file copy.
		return copyFile(ctx, srcFS, dstFS, entries[0].src, dstPath, entries[0].size, opts, reporter)
	}

	// Directory copy: dstPath is the destination directory.
	for _, e := range entries {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		rel, err := filepath.Rel(srcPath, e.src)
		if err != nil {
			return fmt.Errorf("rel %q: %w", e.src, err)
		}
		dst := filepath.Join(dstPath, rel)
		if err := copyFile(ctx, srcFS, dstFS, e.src, dst, e.size, opts, reporter); err != nil {
			return err
		}
	}
	reporter.Finish("copy", dstPath, total)
	return nil
}

type copyEntry struct {
	src  string
	size int64
}

func walkCopySource(srcFS FileSystem, srcPath string, recursive bool) ([]copyEntry, int64, error) {
	info, err := srcFS.Stat(srcPath)
	if err != nil {
		return nil, 0, err
	}

	if !info.IsDir() {
		return []copyEntry{{src: srcPath, size: info.Size()}}, info.Size(), nil
	}

	if !recursive {
		return nil, 0, fmt.Errorf("source %q is a directory; use Recursive=true", srcPath)
	}

	var entries []copyEntry
	var total int64
	err = walkDir(srcFS, srcPath, func(path string, d os.DirEntry) error {
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		entries = append(entries, copyEntry{src: path, size: info.Size()})
		total += info.Size()
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

func walkDir(fsys FileSystem, root string, fn func(path string, d os.DirEntry) error) error {
	entries, err := fsys.ReadDir(root)
	if err != nil {
		return err
	}
	for _, e := range entries {
		path := filepath.Join(root, e.Name())
		if e.IsDir() {
			if err := walkDir(fsys, path, fn); err != nil {
				return err
			}
			continue
		}
		if err := fn(path, e); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(ctx context.Context, srcFS, dstFS FileSystem, srcPath, dstPath string, size int64, opts CopyOptions, reporter *progress.Reporter) error {
	src, err := srcFS.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source %q: %w", srcPath, err)
	}
	defer src.Close()

	srcInfo, err := srcFS.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("stat source %q: %w", srcPath, err)
	}

	mode := os.FileMode(0640)
	if opts.PreserveMode {
		mode = srcInfo.Mode().Perm()
	}

	if err := dstFS.MkdirAll(filepath.Dir(dstPath), 0750); err != nil {
		return fmt.Errorf("mkdir %q: %w", filepath.Dir(dstPath), err)
	}

	tmp := dstPath + ".tmp"
	dst, err := dstFS.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create dest %q: %w", dstPath, err)
	}

	pw := &progressWriter{
		Writer:   dst,
		Reporter: reporter,
		Op:       "copy",
		Label:    dstPath,
		Total:    size,
	}

	_, copyErr := io.Copy(pw, src)
	closeErr := dst.Close()
	if copyErr != nil {
		_ = dstFS.Remove(tmp)
		return fmt.Errorf("copy %q → %q: %w", srcPath, dstPath, copyErr)
	}
	if closeErr != nil {
		_ = dstFS.Remove(tmp)
		return fmt.Errorf("close dest %q: %w", dstPath, closeErr)
	}

	if err := dstFS.Rename(tmp, dstPath); err != nil {
		_ = dstFS.Remove(tmp)
		return fmt.Errorf("rename %q: %w", dstPath, err)
	}

	return nil
}

// progressWriter wraps an io.Writer and reports progress.
type progressWriter struct {
	Writer   io.Writer
	Reporter *progress.Reporter
	Op       string
	Label    string
	Total    int64
	Current  int64
}

func (w *progressWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	w.Current += int64(n)
	w.Reporter.Report(progress.State{Op: w.Op, Label: w.Label, Current: w.Current, Total: w.Total})
	return n, err
}
