package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// extractArchive extracts the archive read from r into the directory dest.
// Supported formats: FormatZIP, FormatTarGz, FormatTarBz2.
func extractArchive(format string, r io.Reader, dest string) error {
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return fmt.Errorf("resolve destination: %w", err)
	}
	if err := os.MkdirAll(absDest, 0750); err != nil {
		return fmt.Errorf("create destination: %w", err)
	}

	switch format {
	case FormatZIP:
		return extractZIP(r, absDest)
	case FormatTarGz:
		gr, err := gzip.NewReader(r)
		if err != nil {
			return fmt.Errorf("gzip header: %w", err)
		}
		defer gr.Close()
		return extractTar(gr, absDest)
	case FormatTarBz2:
		return extractTar(bzip2.NewReader(r), absDest)
	default:
		return fmt.Errorf("unsupported archive format %q", format)
	}
}

func extractZIP(r io.Reader, dest string) error {
	var zr *zip.Reader
	if ra, ok := r.(io.ReaderAt); ok {
		size, err := sizeFromReaderAt(ra)
		if err != nil {
			return err
		}
		zr, err = zip.NewReader(ra, size)
		if err != nil {
			return fmt.Errorf("zip header: %w", err)
		}
	} else {
		tmp, err := os.CreateTemp("", "sing-box-ez-zip-*.tmp")
		if err != nil {
			return fmt.Errorf("create temp file: %w", err)
		}
		defer os.Remove(tmp.Name())

		size, err := io.Copy(tmp, r)
		if err != nil {
			_ = tmp.Close()
			return fmt.Errorf("buffer zip: %w", err)
		}
		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("rewind temp file: %w", err)
		}
		zr, err = zip.NewReader(tmp, size)
		_ = tmp.Close()
		if err != nil {
			return fmt.Errorf("zip header: %w", err)
		}
	}

	for _, f := range zr.File {
		if err := extractZIPFile(f, dest); err != nil {
			return err
		}
	}
	return nil
}

func extractZIPFile(f *zip.File, dest string) error {
	name := filepath.FromSlash(f.Name)
	if name == "" || strings.Contains(name, "..") {
		return nil
	}
	path, err := safeJoin(dest, name)
	if err != nil {
		return err
	}

	if f.FileInfo().IsDir() {
		return os.MkdirAll(path, 0750)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("open zip entry %q: %w", f.Name, err)
	}
	defer rc.Close()

	mode := f.Mode()
	if mode == 0 {
		mode = 0750
	}
	return writeExtractedFile(path, rc, mode.Perm(), f.Modified)
}

func extractTar(r io.Reader, dest string) error {
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar header: %w", err)
		}

		name := filepath.FromSlash(h.Name)
		if name == "" || strings.Contains(name, "..") {
			continue
		}
		path, err := safeJoin(dest, name)
		if err != nil {
			return err
		}

		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0750); err != nil {
				return fmt.Errorf("create dir %q: %w", path, err)
			}
		case tar.TypeReg:
			mode := os.FileMode(h.Mode).Perm()
			if mode == 0 {
				mode = 0750
			}
			if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
				return fmt.Errorf("create dir: %w", err)
			}
			if err := writeExtractedFile(path, tr, mode, h.ModTime); err != nil {
				return err
			}
		case tar.TypeSymlink:
			// Skip symlinks during update to avoid path traversal surprises.
			continue
		default:
			// Ignore special files.
			continue
		}
	}
	return nil
}

func writeExtractedFile(path string, r io.Reader, mode os.FileMode, mtime time.Time) error {
	tmp := path + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create %q: %w", tmp, err)
	}

	_, copyErr := io.Copy(out, r)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write %q: %w", path, copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close %q: %w", tmp, closeErr)
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %q: %w", path, err)
	}

	if !mtime.IsZero() {
		_ = os.Chtimes(path, time.Now(), mtime)
	}
	return nil
}

func safeJoin(dest, name string) (string, error) {
	path := filepath.Join(dest, name)
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", path, err)
	}
	prefix := filepath.Clean(dest) + string(filepath.Separator)
	if abs != filepath.Clean(dest) && !strings.HasPrefix(abs, prefix) {
		return "", fmt.Errorf("illegal archive entry %q", name)
	}
	return abs, nil
}

func sizeFromReaderAt(ra io.ReaderAt) (int64, error) {
	if s, ok := ra.(interface{ Size() int64 }); ok {
		return s.Size(), nil
	}
	if s, ok := ra.(interface{ Stat() (os.FileInfo, error) }); ok {
		fi, err := s.Stat()
		if err != nil {
			return 0, fmt.Errorf("stat reader: %w", err)
		}
		return fi.Size(), nil
	}
	return 0, fmt.Errorf("zip extraction requires a seekable reader")
}
