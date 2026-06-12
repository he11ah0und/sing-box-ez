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

	frameworkfs "sing-box-ez/internal/framework/fs"
)

// extractArchive extracts the archive at archivePath from fsys into the
// directory dest. Supported formats: FormatZIP, FormatTarGz, FormatTarBz2.
func extractArchive(fsys frameworkfs.FileSystem, format, archivePath, dest string) error {
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return fmt.Errorf("resolve destination: %w", err)
	}
	if err := fsys.MkdirAll(absDest, 0750); err != nil {
		return fmt.Errorf("create destination: %w", err)
	}

	f, err := fsys.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	switch format {
	case FormatZIP:
		return extractZIP(fsys, f, absDest)
	case FormatTarGz:
		gr, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("gzip header: %w", err)
		}
		defer gr.Close()
		return extractTar(fsys, gr, absDest)
	case FormatTarBz2:
		return extractTar(fsys, bzip2.NewReader(f), absDest)
	default:
		return fmt.Errorf("unsupported archive format %q", format)
	}
}

func extractZIP(fsys frameworkfs.FileSystem, f *os.File, dest string) error {
	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat archive: %w", err)
	}

	zr, err := zip.NewReader(f, fi.Size())
	if err != nil {
		return fmt.Errorf("zip header: %w", err)
	}

	for _, entry := range zr.File {
		if err := extractZIPFile(fsys, entry, dest); err != nil {
			return err
		}
	}
	return nil
}

func extractZIPFile(fsys frameworkfs.FileSystem, f *zip.File, dest string) error {
	name := filepath.FromSlash(f.Name)
	if name == "" || strings.Contains(name, "..") {
		return nil
	}
	path, err := safeJoin(dest, name)
	if err != nil {
		return err
	}

	if f.FileInfo().IsDir() {
		return fsys.MkdirAll(path, 0750)
	}

	if err := fsys.MkdirAll(filepath.Dir(path), 0750); err != nil {
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
	return writeExtractedFile(fsys, path, rc, mode.Perm())
}

func extractTar(fsys frameworkfs.FileSystem, r io.Reader, dest string) error {
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
			if err := fsys.MkdirAll(path, 0750); err != nil {
				return fmt.Errorf("create dir %q: %w", path, err)
			}
		case tar.TypeReg:
			mode := os.FileMode(h.Mode).Perm()
			if mode == 0 {
				mode = 0750
			}
			if err := fsys.MkdirAll(filepath.Dir(path), 0750); err != nil {
				return fmt.Errorf("create dir: %w", err)
			}
			if err := writeExtractedFile(fsys, path, tr, mode); err != nil {
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

func writeExtractedFile(fsys frameworkfs.FileSystem, path string, r io.Reader, mode os.FileMode) error {
	tmp := path + ".tmp"
	out, err := fsys.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create %q: %w", tmp, err)
	}

	_, copyErr := io.Copy(out, r)
	closeErr := out.Close()
	if copyErr != nil {
		_ = fsys.Remove(tmp)
		return fmt.Errorf("write %q: %w", path, copyErr)
	}
	if closeErr != nil {
		_ = fsys.Remove(tmp)
		return fmt.Errorf("close %q: %w", tmp, closeErr)
	}

	if err := fsys.Rename(tmp, path); err != nil {
		_ = fsys.Remove(tmp)
		return fmt.Errorf("rename %q: %w", path, err)
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

func findBinaryInDir(fsys frameworkfs.FileSystem, dir, base string) (string, error) {
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read dir %q: %w", dir, err)
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() {
			found, err := findBinaryInDir(fsys, path, base)
			if err != nil {
				return "", err
			}
			if found != "" {
				return found, nil
			}
			continue
		}
		if e.Name() == base {
			return path, nil
		}
	}
	return "", nil
}
