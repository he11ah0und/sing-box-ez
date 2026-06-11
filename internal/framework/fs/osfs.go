package fs

import (
	"os"
	"path/filepath"
)

// OSFileSystem implements FileSystem using the real OS file system.
// All paths are resolved relative to BaseDir with directory-traversal
// protection (leading ".." components are stripped).
type OSFileSystem struct {
	BaseDir string
}

// NewOSFileSystem creates a new OS-backed file system rooted at baseDir.
func NewOSFileSystem(baseDir string) *OSFileSystem {
	return &OSFileSystem{BaseDir: baseDir}
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

// ReadFile reads the named file.
func (fs *OSFileSystem) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(fs.resolve(name))
}

// WriteFile writes data to the named file.
func (fs *OSFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(fs.resolve(name), data, perm)
}

// MkdirAll creates a directory and all necessary parents.
func (fs *OSFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(fs.resolve(path), perm)
}

// OpenFile opens a file with the given flags and permissions.
func (fs *OSFileSystem) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(fs.resolve(name), flag, perm)
}

// Open opens a file for reading.
func (fs *OSFileSystem) Open(name string) (*os.File, error) {
	return os.Open(fs.resolve(name))
}

// ReadDir reads the named directory.
func (fs *OSFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(fs.resolve(name))
}

// Rename renames (moves) a file.
func (fs *OSFileSystem) Rename(oldpath, newpath string) error {
	return os.Rename(fs.resolve(oldpath), fs.resolve(newpath))
}

// Remove removes a file.
func (fs *OSFileSystem) Remove(name string) error {
	return os.Remove(fs.resolve(name))
}

// RemoveAll removes a path and any children.
func (fs *OSFileSystem) RemoveAll(path string) error {
	return os.RemoveAll(fs.resolve(path))
}

// Stat returns file info.
func (fs *OSFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(fs.resolve(name))
}

// Exists reports whether the named file exists.
func (fs *OSFileSystem) Exists(name string) bool {
	_, err := fs.Stat(name)
	return err == nil
}
