package fs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"sing-box-ez/internal/framework/logger"
)

// OSFS is an FS implementation backed by the real OS file system.
type OSFS struct {
	BaseDir string
	Log     *logger.LogTerminal
}

// NewOS creates a new OS-backed FS rooted at baseDir.
func NewOS(baseDir string) FS {
	return &OSFS{BaseDir: baseDir}
}

// NewOSWithLog creates a new OS-backed FS with a scoped logger.
func NewOSWithLog(baseDir string, log *logger.LogTerminal) FS {
	return &OSFS{BaseDir: baseDir, Log: log}
}

func (fs *OSFS) Root() Directory {
	return &osDirectory{osEntry: osEntry{fs: fs, path: ""}}
}

func (fs *OSFS) resolve(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	clean := filepath.Clean("/" + name)
	if len(clean) > 0 && clean[0] == filepath.Separator {
		clean = clean[1:]
	}
	return filepath.Join(fs.BaseDir, clean)
}

// relPath converts an absolute path under BaseDir into a relative path
// suitable for the FS root. Relative paths are returned unchanged.
// If the path is absolute and outside BaseDir, ok is false.
func (fs *OSFS) relPath(name string) (string, bool) {
	if !filepath.IsAbs(name) {
		return name, true
	}
	base := filepath.Clean(fs.BaseDir)
	target := filepath.Clean(name)
	rel, err := filepath.Rel(base, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}
	if rel == "." {
		return "", false
	}
	return NormalizePath(rel), true
}

func (fs *OSFS) errf(format string, v ...interface{}) error {
	if fs.Log != nil {
		return fs.Log.Errorf(format, v...)
	}
	return fmt.Errorf(format, v...)
}

type osEntry struct {
	fs   *OSFS
	path string
}

func (e *osEntry) Name() string { return baseName(e.path) }

func (e *osEntry) Path() string { return e.path }

func (e *osEntry) Exists() bool {
	_, err := os.Stat(e.fs.resolve(e.path))
	return err == nil
}

func (e *osEntry) Stat() (os.FileInfo, error) {
	fi, err := os.Stat(e.fs.resolve(e.path))
	if err != nil {
		return nil, e.fs.errf("stat %q: %w", e.path, err)
	}
	return fi, nil
}

func (e *osEntry) Parent() Directory {
	p := parentPath(e.path)
	if p == "" && e.path == "" {
		return nil
	}
	return &osDirectory{osEntry: osEntry{fs: e.fs, path: p}}
}

func (e *osEntry) Rename(newName string) error {
	if err := validateName(newName); err != nil {
		return err
	}
	parent := parentPath(e.path)
	newPath, err := joinChild(parent, newName)
	if err != nil {
		return err
	}
	oldResolved := e.fs.resolve(e.path)
	newResolved := e.fs.resolve(newPath)
	if err := os.Rename(oldResolved, newResolved); err != nil {
		return e.fs.errf("rename %q → %q: %w", e.path, newPath, err)
	}
	return nil
}

func (e *osEntry) Remove() error {
	if err := os.Remove(e.fs.resolve(e.path)); err != nil {
		return e.fs.errf("remove %q: %w", e.path, err)
	}
	return nil
}

func (e *osEntry) Chmod(perm os.FileMode) error {
	if err := os.Chmod(e.fs.resolve(e.path), perm); err != nil {
		return e.fs.errf("chmod %q: %w", e.path, err)
	}
	return nil
}

type osDirectory struct {
	osEntry
}

func (d *osDirectory) File(name string) File {
	local, ok := d.fs.relPath(name)
	if !ok {
		return &osFile{osEntry: osEntry{fs: d.fs, path: ""}}
	}
	p, _ := joinChild(d.path, local)
	return &osFile{osEntry: osEntry{fs: d.fs, path: p}}
}

func (d *osDirectory) Subdir(name string) Directory {
	local, ok := d.fs.relPath(name)
	if !ok {
		return &osDirectory{osEntry: osEntry{fs: d.fs, path: ""}}
	}
	p, _ := joinChild(d.path, local)
	return &osDirectory{osEntry: osEntry{fs: d.fs, path: p}}
}

func (d *osDirectory) ReadDir() ([]Entry, error) {
	entries, err := os.ReadDir(d.fs.resolve(d.path))
	if err != nil {
		return nil, d.fs.errf("read dir %q: %w", d.path, err)
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		childPath, err := joinChild(d.path, e.Name())
		if err != nil {
			continue
		}
		ent := osEntry{fs: d.fs, path: childPath}
		if e.IsDir() {
			out = append(out, &osDirectory{osEntry: ent})
		} else {
			out = append(out, &osFile{osEntry: ent})
		}
	}
	return out, nil
}

func (d *osDirectory) MkdirAll(perm os.FileMode) error {
	if err := os.MkdirAll(d.fs.resolve(d.path), perm); err != nil {
		return d.fs.errf("mkdir %q: %w", d.path, err)
	}
	return nil
}

func (d *osDirectory) RemoveAll() error {
	if err := os.RemoveAll(d.fs.resolve(d.path)); err != nil {
		return d.fs.errf("remove all %q: %w", d.path, err)
	}
	return nil
}

func (d *osDirectory) CopyTo(dst Directory) error {
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

func (d *osDirectory) Ensure(perm os.FileMode) error {
	info, err := d.Stat()
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := d.MkdirAll(perm); err != nil {
			return err
		}
		return d.Chmod(perm)
	}
	if !info.IsDir() {
		return d.fs.errf("ensure %q: not a directory", d.path)
	}
	if info.Mode().Perm() != perm {
		return d.Chmod(perm)
	}
	return nil
}

type osFile struct {
	osEntry
}

func (f *osFile) Read() ([]byte, error) {
	data, err := os.ReadFile(f.fs.resolve(f.path))
	if err != nil {
		return nil, f.fs.errf("read %q: %w", f.path, err)
	}
	return data, nil
}

func (f *osFile) Write(data []byte, perm os.FileMode) error {
	if err := os.WriteFile(f.fs.resolve(f.path), data, perm); err != nil {
		return f.fs.errf("write %q: %w", f.path, err)
	}
	return nil
}

func (f *osFile) Open() (*os.File, error) {
	file, err := os.Open(f.fs.resolve(f.path))
	if err != nil {
		return nil, f.fs.errf("open %q: %w", f.path, err)
	}
	return file, nil
}

func (f *osFile) OpenFile(flag int, perm os.FileMode) (*os.File, error) {
	file, err := os.OpenFile(f.fs.resolve(f.path), flag, perm)
	if err != nil {
		return nil, f.fs.errf("open file %q: %w", f.path, err)
	}
	return file, nil
}

func (f *osFile) AtomicWrite(data []byte, perm os.FileMode) error {
	tmpPath, err := joinChild(parentPath(f.path), f.Name()+".tmp")
	if err != nil {
		return err
	}
	tmp := &osFile{osEntry: osEntry{fs: f.fs, path: tmpPath}}
	if err := tmp.Write(data, perm); err != nil {
		_ = tmp.Remove()
		return err
	}
	oldResolved := f.fs.resolve(tmpPath)
	newResolved := f.fs.resolve(f.path)
	if err := os.Rename(oldResolved, newResolved); err != nil {
		_ = tmp.Remove()
		return f.fs.errf("rename %q → %q: %w", tmpPath, f.path, err)
	}
	return nil
}

func (f *osFile) CopyTo(dst File) error {
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

func validateName(name string) error {
	name = NormalizePath(name)
	if name == "" || name == "." || strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
		return fmt.Errorf("invalid name %q", name)
	}
	return nil
}
