package fs

import (
	"fmt"
	"os"
	"path/filepath"

	"sing-box-ez/internal/framework/logger"
)

// OSFileSystem implements FileSystem using the real OS file system.
// All paths are resolved relative to BaseDir with directory-traversal
// protection (leading ".." components are stripped).
type OSFileSystem struct {
	BaseDir string
	Log     *logger.LogTerminal
}

// NewOSFileSystem creates a new OS-backed file system rooted at baseDir.
func NewOSFileSystem(baseDir string) *OSFileSystem {
	return &OSFileSystem{BaseDir: baseDir}
}

// NewOSFileSystemWithLog creates a new OS-backed file system and attaches a
// scoped logger under the given parent terminal.
func NewOSFileSystemWithLog(parent *logger.LogTerminal, baseDir string) *OSFileSystem {
	return &OSFileSystem{BaseDir: baseDir, Log: parent.Allocate("fs")}
}

func (fs *OSFileSystem) resolve(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	clean := filepath.Clean("/" + name)
	if len(clean) > 0 && clean[0] == filepath.Separator {
		clean = clean[1:]
	}
	return filepath.Join(fs.BaseDir, clean)
}

func (fs *OSFileSystem) errf(format string, v ...interface{}) error {
	if fs.Log != nil {
		return fs.Log.Errorf(format, v...)
	}
	return fmt.Errorf(format, v...)
}

// ReadFile reads the named file.
func (fs *OSFileSystem) ReadFile(name string) ([]byte, error) {
	data, err := os.ReadFile(fs.resolve(name))
	if err != nil {
		return nil, fs.errf("read %q: %w", name, err)
	}
	return data, nil
}

// WriteFile writes data to the named file.
func (fs *OSFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	if err := os.WriteFile(fs.resolve(name), data, perm); err != nil {
		return fs.errf("write %q: %w", name, err)
	}
	return nil
}

// MkdirAll creates a directory and all necessary parents.
func (fs *OSFileSystem) MkdirAll(path string, perm os.FileMode) error {
	if err := os.MkdirAll(fs.resolve(path), perm); err != nil {
		return fs.errf("mkdir %q: %w", path, err)
	}
	return nil
}

// OpenFile opens a file with the given flags and permissions.
func (fs *OSFileSystem) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	f, err := os.OpenFile(fs.resolve(name), flag, perm)
	if err != nil {
		return nil, fs.errf("open %q: %w", name, err)
	}
	return f, nil
}

// Open opens a file for reading.
func (fs *OSFileSystem) Open(name string) (*os.File, error) {
	f, err := os.Open(fs.resolve(name))
	if err != nil {
		return nil, fs.errf("open %q: %w", name, err)
	}
	return f, nil
}

// ReadDir reads the named directory.
func (fs *OSFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(fs.resolve(name))
	if err != nil {
		return nil, fs.errf("read dir %q: %w", name, err)
	}
	return entries, nil
}

// Rename renames (moves) a file.
func (fs *OSFileSystem) Rename(oldpath, newpath string) error {
	if err := os.Rename(fs.resolve(oldpath), fs.resolve(newpath)); err != nil {
		return fs.errf("rename %q → %q: %w", oldpath, newpath, err)
	}
	return nil
}

// Remove removes a file.
func (fs *OSFileSystem) Remove(name string) error {
	if err := os.Remove(fs.resolve(name)); err != nil {
		return fs.errf("remove %q: %w", name, err)
	}
	return nil
}

// RemoveAll removes a path and any children.
func (fs *OSFileSystem) RemoveAll(path string) error {
	if err := os.RemoveAll(fs.resolve(path)); err != nil {
		return fs.errf("remove all %q: %w", path, err)
	}
	return nil
}

// Stat returns file info.
func (fs *OSFileSystem) Stat(name string) (os.FileInfo, error) {
	fi, err := os.Stat(fs.resolve(name))
	if err != nil {
		return nil, fs.errf("stat %q: %w", name, err)
	}
	return fi, nil
}

// Exists reports whether the named file exists.
func (fs *OSFileSystem) Exists(name string) bool {
	_, err := fs.Stat(name)
	return err == nil
}
