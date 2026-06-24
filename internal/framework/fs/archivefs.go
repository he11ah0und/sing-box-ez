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

// NewArchiveFS opens path as an archive and returns a read-only FS.
// Supported formats: "zip", "tar", "tar.gz", "tar.bz2".
func NewArchiveFS(path, format string) (FS, error) {
	switch format {
	case "zip":
		return newZipFS(path)
	case "tar":
		return newTarFS(path, false)
	case "tar.gz":
		return newTarFS(path, true)
	case "tar.bz2":
		return newTarFS(path, false)
	default:
		return nil, fmt.Errorf("unsupported archive format %q", format)
	}
}

type archiveFS struct {
	path      string
	format    string
	files     map[string]*archiveEntry
	dirs      map[string][]*archiveEntry
	modTime   time.Time
	zipReader *zip.ReadCloser
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

func newZipFS(path string) (FS, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open zip %q: %w", path, err)
	}

	fsys := &archiveFS{
		path:      path,
		format:    "zip",
		files:     make(map[string]*archiveEntry),
		dirs:      make(map[string][]*archiveEntry),
		modTime:   time.Now(),
		zipReader: r,
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
		fsys.addEntry(name, entry)
	}
	fsys.ensureDirs()
	return fsys, nil
}

func newTarFS(path string, gz bool) (FS, error) {
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

	fsys := &archiveFS{
		path:    path,
		format:  "tar",
		files:   make(map[string]*archiveEntry),
		dirs:    make(map[string][]*archiveEntry),
		modTime: time.Now(),
	}
	if gz {
		fsys.format = "tar.gz"
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
		fsys.addEntry(name, entry)

		if _, err := io.Copy(io.Discard, tr); err != nil {
			return nil, fmt.Errorf("skip tar content: %w", err)
		}
	}

	fsys.ensureDirs()
	return fsys, nil
}

func (fsys *archiveFS) Root() Directory {
	return &archiveDirectory{archiveNode{fs: fsys, path: ""}}
}

func (fsys *archiveFS) addEntry(name string, e *archiveEntry) {
	fsys.files[name] = e
	dir := filepath.ToSlash(filepath.Dir(name))
	for {
		fsys.dirs[dir] = append(fsys.dirs[dir], e)
		if dir == "." || dir == "/" {
			break
		}
		dir = filepath.ToSlash(filepath.Dir(dir))
	}
}

func (fsys *archiveFS) ensureDirs() {
	for dir := range fsys.dirs {
		if _, ok := fsys.files[dir]; !ok {
			fsys.files[dir] = &archiveEntry{
				name:    dir,
				isDir:   true,
				mode:    0750,
				modTime: fsys.modTime,
			}
		}
	}
}

func (fsys *archiveFS) readOnly(op string) error {
	return fmt.Errorf("%s: %w", op, ErrReadOnly)
}

type archiveNode struct {
	fs   *archiveFS
	path string
}

func (n *archiveNode) archivePath() string {
	if n.path == "" {
		return "."
	}
	return n.path
}

func (n *archiveNode) raw() (*archiveEntry, error) {
	entry, ok := n.fs.files[n.archivePath()]
	if !ok {
		return nil, fmt.Errorf("entry not found: %s", n.path)
	}
	return entry, nil
}

func (n *archiveNode) Name() string { return baseName(n.path) }

func (n *archiveNode) Path() string { return n.path }

func (n *archiveNode) Exists() bool {
	_, err := n.raw()
	return err == nil
}

func (n *archiveNode) Stat() (os.FileInfo, error) {
	raw, err := n.raw()
	if err != nil {
		return nil, err
	}
	return &archiveFileInfo{e: raw}, nil
}

func (n *archiveNode) Parent() Directory {
	p := parentPath(n.path)
	if p == "" && n.path == "" {
		return nil
	}
	return &archiveDirectory{archiveNode{fs: n.fs, path: p}}
}

func (n *archiveNode) Rename(newName string) error { return n.fs.readOnly("rename") }

func (n *archiveNode) Remove() error                { return n.fs.readOnly("remove") }
func (n *archiveNode) Chmod(perm os.FileMode) error { return n.fs.readOnly("chmod") }

type archiveDirectory struct {
	archiveNode
}

func (d *archiveDirectory) File(name string) File {
	p, _ := joinChild(d.path, name)
	return &archiveFile{archiveNode{fs: d.fs, path: p}}
}

func (d *archiveDirectory) Subdir(name string) Directory {
	p, _ := joinChild(d.path, name)
	return &archiveDirectory{archiveNode{fs: d.fs, path: p}}
}

func (d *archiveDirectory) ReadDir() ([]Entry, error) {
	name := d.archivePath()
	entries, ok := d.fs.dirs[name]
	if !ok {
		return nil, fmt.Errorf("directory not found: %s", d.path)
	}
	sort.Slice(entries, func(i, j int) bool {
		return baseName(entries[i].name) < baseName(entries[j].name)
	})
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		childPath, err := joinChild(d.path, baseName(e.name))
		if err != nil {
			continue
		}
		node := archiveNode{fs: d.fs, path: childPath}
		if e.isDir {
			out = append(out, &archiveDirectory{node})
		} else {
			out = append(out, &archiveFile{node})
		}
	}
	return out, nil
}

func (d *archiveDirectory) MkdirAll(perm os.FileMode) error { return d.fs.readOnly("mkdir") }

func (d *archiveDirectory) RemoveAll() error { return d.fs.readOnly("remove all") }

func (d *archiveDirectory) CopyTo(dst Directory) error {
	if err := dst.MkdirAll(0750); err != nil {
		return err
	}
	entries, err := d.ReadDir()
	if err != nil {
		return err
	}
	for _, e := range entries {
		switch src := e.(type) {
		case Directory:
			if err := src.CopyTo(dst.Subdir(src.Name())); err != nil {
				return err
			}
		case File:
			if err := src.CopyTo(dst.File(src.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *archiveDirectory) Ensure(perm os.FileMode) error {
	if d.Exists() {
		return nil
	}
	return d.fs.readOnly("ensure dir")
}

type archiveFile struct {
	archiveNode
}

func (f *archiveFile) Read() ([]byte, error) {
	file, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func (f *archiveFile) Write(data []byte, perm os.FileMode) error { return f.fs.readOnly("write") }

func (f *archiveFile) Open() (*os.File, error) {
	raw, err := f.raw()
	if err != nil {
		return nil, err
	}
	if raw.isDir {
		return nil, fmt.Errorf("is a directory: %s", f.path)
	}

	var r io.Reader
	switch f.fs.format {
	case "zip":
		rc, err := raw.zipFile.Open()
		if err != nil {
			return nil, fmt.Errorf("open zip entry %q: %w", f.path, err)
		}
		defer rc.Close()
		r = rc
	case "tar", "tar.gz":
		r, err = f.fs.openTarEntry(raw.tarOffset, raw.size)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported archive format %q", f.fs.format)
	}

	tmp, err := os.CreateTemp("", "archivefs-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("read archive entry %q: %w", f.path, err)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, err
	}
	return tmp, nil
}

func (f *archiveFile) OpenFile(flag int, perm os.FileMode) (*os.File, error) {
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0 {
		return nil, ErrReadOnly
	}
	return f.Open()
}

func (f *archiveFile) AtomicWrite(data []byte, perm os.FileMode) error {
	return f.fs.readOnly("atomic write")
}

func (f *archiveFile) CopyTo(dst File) error {
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}
	mode := info.Mode().Perm()
	if mode == 0 {
		mode = 0640
	}

	if err := dst.Parent().MkdirAll(0750); err != nil {
		return err
	}

	dstFile, err := dst.OpenFile(os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, src); err != nil {
		return fmt.Errorf("copy %q → %q: %w", f.path, dst.Path(), err)
	}
	return nil
}

func (fsys *archiveFS) openTarEntry(targetOffset, size int64) (io.Reader, error) {
	f, err := os.Open(fsys.path)
	if err != nil {
		return nil, err
	}

	var r io.Reader = f
	if fsys.format == "tar.gz" {
		gr, err := gzip.NewReader(f)
		if err != nil {
			_ = f.Close()
			return nil, err
		}
		r = gr
	}

	cr := &countingReader{Reader: r}
	tr := tar.NewReader(cr)

	for {
		_, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = f.Close()
			return nil, err
		}
		if cr.bytes == targetOffset {
			return &offsetFileReader{r: io.LimitReader(cr, size), f: f}, nil
		}
		if _, err := io.Copy(io.Discard, tr); err != nil {
			_ = f.Close()
			return nil, err
		}
	}
	_ = f.Close()
	return nil, fmt.Errorf("tar entry not found")
}

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

type countingReader struct {
	Reader io.Reader
	bytes  int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.Reader.Read(p)
	c.bytes += int64(n)
	return n, err
}

type archiveFileInfo struct {
	e *archiveEntry
}

func (fi *archiveFileInfo) Name() string       { return baseName(fi.e.name) }
func (fi *archiveFileInfo) Size() int64        { return fi.e.size }
func (fi *archiveFileInfo) Mode() os.FileMode  { return fi.e.mode }
func (fi *archiveFileInfo) ModTime() time.Time { return fi.e.modTime }
func (fi *archiveFileInfo) IsDir() bool        { return fi.e.isDir }
func (fi *archiveFileInfo) Sys() any           { return nil }
