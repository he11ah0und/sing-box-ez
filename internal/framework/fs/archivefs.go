package fs

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ArchiveFS implements FileSystem over an archive file (zip, tar, tar.gz, tar.bz2).
// It is read-only; write operations return ErrReadOnly.
// For tar-based archives entries are read on demand by re-scanning the stream;
// for zip entries random access is used.
type ArchiveFS struct {
	path    string
	format  string
	files   map[string]*archiveEntry
	dirs    map[string][]os.DirEntry
	modTime time.Time
}

type archiveEntry struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool

	// zip only
	zipFile *zip.File

	// tar only: offset in the decompressed tar stream at the start of the file content
	tarOffset int64
}

// NewArchiveFS opens path as an archive and returns a read-only FileSystem.
// Supported formats: "zip", "tar", "tar.gz", "tar.bz2".
func NewArchiveFS(path, format string) (*ArchiveFS, error) {
	switch format {
	case "zip":
		return newZipFS(path)
	case "tar":
		return newTarFS(path, false)
	case "tar.gz":
		return newTarFS(path, true)
	case "tar.bz2":
		return newTarFS(path, false) // bzip2 handled separately
	default:
		return nil, fmt.Errorf("unsupported archive format %q", format)
	}
}

func newZipFS(path string) (*ArchiveFS, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open zip %q: %w", path, err)
	}
	defer r.Close()

	fs := &ArchiveFS{
		path:    path,
		format:  "zip",
		files:   make(map[string]*archiveEntry),
		dirs:    make(map[string][]os.DirEntry),
		modTime: time.Now(),
	}

	for _, f := range r.File {
		name := filepath.ToSlash(f.Name)
		if name == "" || strings.Contains(name, "..") {
			continue
		}
		entry := &archiveEntry{
			name:    name,
			size:    int64(f.UncompressedSize64),
			mode:    f.Mode(),
			modTime: f.Modified,
			isDir:   f.FileInfo().IsDir(),
			zipFile: f,
		}
		fs.addEntry(name, entry)
	}
	fs.ensureDirs()
	return fs, nil
}

func newTarFS(path string, gz bool) (*ArchiveFS, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open tar %q: %w", path, err)
	}
	defer f.Close()

	var r io.Reader = f
	if gz {
		gr, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("gzip header: %w", err)
		}
		r = gr
	} else {
		// peek magic for bzip2
		buf := make([]byte, 2)
		if _, err := f.Read(buf); err == nil {
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return nil, err
			}
			if buf[0] == 'B' && buf[1] == 'Z' {
				r = bzip2.NewReader(f)
			}
		}
	}

	cr := &countingReader{Reader: r}
	tr := tar.NewReader(cr)

	fs := &ArchiveFS{
		path:    path,
		format:  "tar",
		files:   make(map[string]*archiveEntry),
		dirs:    make(map[string][]os.DirEntry),
		modTime: time.Now(),
	}
	if gz {
		fs.format = "tar.gz"
	}

	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar header: %w", err)
		}

		name := filepath.ToSlash(h.Name)
		if name == "" || strings.Contains(name, "..") {
			continue
		}

		offset := cr.bytes
		entry := &archiveEntry{
			name:      name,
			size:      h.Size,
			mode:      os.FileMode(h.Mode).Perm(),
			modTime:   h.ModTime,
			isDir:     h.Typeflag == tar.TypeDir,
			tarOffset: offset,
		}
		fs.addEntry(name, entry)

		// Skip content to update the counter past this entry.
		if _, err := io.Copy(io.Discard, tr); err != nil {
			return nil, fmt.Errorf("skip tar content: %w", err)
		}
	}

	fs.ensureDirs()
	return fs, nil
}

func (fs *ArchiveFS) addEntry(name string, e *archiveEntry) {
	fs.files[name] = e
	// Register in parent directories.
	dir := filepath.ToSlash(filepath.Dir(name))
	for {
		fs.dirs[dir] = append(fs.dirs[dir], &archiveDirEntry{e: e, parent: dir})
		if dir == "." || dir == "/" {
			break
		}
		dir = filepath.ToSlash(filepath.Dir(dir))
	}
}

func (fs *ArchiveFS) ensureDirs() {
	for dir := range fs.dirs {
		if _, ok := fs.files[dir]; !ok {
			fs.files[dir] = &archiveEntry{
				name:    dir,
				isDir:   true,
				mode:    0750,
				modTime: fs.modTime,
			}
		}
	}
}

// ReadFile reads the named file from the archive.
func (fs *ArchiveFS) ReadFile(name string) ([]byte, error) {
	f, err := fs.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// WriteFile always returns ErrReadOnly.
func (fs *ArchiveFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return ErrReadOnly
}

// MkdirAll always returns ErrReadOnly.
func (fs *ArchiveFS) MkdirAll(path string, perm os.FileMode) error {
	return ErrReadOnly
}

// OpenFile always returns ErrReadOnly for write flags.
func (fs *ArchiveFS) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0 {
		return nil, ErrReadOnly
	}
	return fs.Open(name)
}

// Open opens the named file for reading.
func (fs *ArchiveFS) Open(name string) (*os.File, error) {
	name = filepath.ToSlash(name)
	e, ok := fs.files[name]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", name)
	}
	if e.isDir {
		return nil, fmt.Errorf("is a directory: %s", name)
	}

	var r io.Reader
	switch fs.format {
	case "zip":
		rc, err := e.zipFile.Open()
		if err != nil {
			return nil, fmt.Errorf("open zip entry %q: %w", name, err)
		}
		defer rc.Close()
		r = rc
	case "tar", "tar.gz":
		var err error
		r, err = fs.openTarEntry(e.tarOffset, e.size)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported archive format %q", fs.format)
	}

	tmp, err := os.CreateTemp("", "archivefs-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("read archive entry %q: %w", name, err)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, err
	}
	return tmp, nil
}

func (fs *ArchiveFS) openTarEntry(targetOffset, size int64) (io.Reader, error) {
	f, err := os.Open(fs.path)
	if err != nil {
		return nil, err
	}
	// The returned offsetFileReader owns f and closes it when the entry is
	// fully read or explicitly closed; do not close f here.

	var r io.Reader = f
	if fs.format == "tar.gz" {
		gr, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		r = gr
	}
	// bzip2 does not support seeking; for now treat as sequential re-read.

	cr := &countingReader{Reader: r}
	tr := tar.NewReader(cr)

	for {
		_, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if cr.bytes == targetOffset {
			// Return a reader that closes the underlying file when done.
			return &offsetFileReader{r: io.LimitReader(cr, size), f: f}, nil
		}
		if _, err := io.Copy(io.Discard, tr); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("tar entry not found")
}

// offsetFileReader wraps a reader and closes the underlying file on EOF or Close.
type offsetFileReader struct {
	r    io.Reader
	f    *os.File
	done bool
}

func (r *offsetFileReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if err == io.EOF {
		r.done = true
		_ = r.f.Close()
	}
	return n, err
}

func (r *offsetFileReader) Close() error {
	if !r.done {
		_ = r.f.Close()
	}
	return nil
}

// ReadDir reads the named directory inside the archive.
func (fs *ArchiveFS) ReadDir(name string) ([]os.DirEntry, error) {
	name = filepath.ToSlash(name)
	if name == "" {
		name = "."
	}
	entries, ok := fs.dirs[name]
	if !ok {
		return nil, fmt.Errorf("directory not found: %s", name)
	}
	// Sort for stable ordering.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}

// Rename always returns ErrReadOnly.
func (fs *ArchiveFS) Rename(oldpath, newpath string) error {
	return ErrReadOnly
}

// Remove always returns ErrReadOnly.
func (fs *ArchiveFS) Remove(name string) error {
	return ErrReadOnly
}

// RemoveAll always returns ErrReadOnly.
func (fs *ArchiveFS) RemoveAll(path string) error {
	return ErrReadOnly
}

// Stat returns file info for the named file.
func (fs *ArchiveFS) Stat(name string) (os.FileInfo, error) {
	name = filepath.ToSlash(name)
	e, ok := fs.files[name]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", name)
	}
	return &archiveFileInfo{e: e}, nil
}

// Exists reports whether the named file exists in the archive.
func (fs *ArchiveFS) Exists(name string) bool {
	_, err := fs.Stat(name)
	return err == nil
}

// countingReader counts bytes read through it.
type countingReader struct {
	Reader io.Reader
	bytes  int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.Reader.Read(p)
	c.bytes += int64(n)
	return n, err
}

// archiveDirEntry implements os.DirEntry for archive entries.
type archiveDirEntry struct {
	e      *archiveEntry
	parent string
}

func (d *archiveDirEntry) Name() string {
	name := d.e.name
	if d.parent != "" && d.parent != "." {
		name = strings.TrimPrefix(name, d.parent+"/")
	}
	return filepath.Base(name)
}

func (d *archiveDirEntry) IsDir() bool  { return d.e.isDir }
func (d *archiveDirEntry) Type() os.FileMode {
	if d.e.isDir {
		return os.ModeDir
	}
	return d.e.mode.Type()
}
func (d *archiveDirEntry) Info() (os.FileInfo, error) {
	return &archiveFileInfo{e: d.e}, nil
}

// archiveFileInfo implements os.FileInfo.
type archiveFileInfo struct {
	e *archiveEntry
}

func (fi *archiveFileInfo) Name() string       { return filepath.Base(fi.e.name) }
func (fi *archiveFileInfo) Size() int64        { return fi.e.size }
func (fi *archiveFileInfo) Mode() os.FileMode  { return fi.e.mode }
func (fi *archiveFileInfo) ModTime() time.Time { return fi.e.modTime }
func (fi *archiveFileInfo) IsDir() bool        { return fi.e.isDir }
func (fi *archiveFileInfo) Sys() any           { return nil }
