package fs

import (
	"embed"
	"io"
	"io/fs"
	"os"
)

// EmbedFS implements FileSystem over an embed.FS.
// All write operations return ErrReadOnly.
// Because embed.FS.Open returns fs.File rather than *os.File, Open copies
// the embedded content to a temporary file to satisfy the interface.
type EmbedFS struct {
	FS embed.FS
}

// ReadFile reads the named file from the embedded file system.
func (e *EmbedFS) ReadFile(name string) ([]byte, error) {
	return e.FS.ReadFile(name)
}

// WriteFile always returns ErrReadOnly.
func (e *EmbedFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return ErrReadOnly
}

// MkdirAll always returns ErrReadOnly.
func (e *EmbedFS) MkdirAll(path string, perm os.FileMode) error {
	return ErrReadOnly
}

// OpenFile always returns ErrReadOnly.
func (e *EmbedFS) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return nil, ErrReadOnly
}

// ReadDir reads the named directory inside the embedded file system.
func (e *EmbedFS) ReadDir(name string) ([]os.DirEntry, error) {
	return fs.ReadDir(e.FS, name)
}

// Open opens the named file for reading.
// The content is copied to a temporary file so that the returned value is
// an *os.File as required by the FileSystem interface.
func (e *EmbedFS) Open(name string) (*os.File, error) {
	src, err := e.FS.Open(name)
	if err != nil {
		return nil, err
	}
	defer src.Close()

	tmp, err := os.CreateTemp("", "embed-*")
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(tmp, src); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, err
	}

	return tmp, nil
}

// Rename always returns ErrReadOnly.
func (e *EmbedFS) Rename(oldpath, newpath string) error {
	return ErrReadOnly
}

// Remove always returns ErrReadOnly.
func (e *EmbedFS) Remove(name string) error {
	return ErrReadOnly
}

// RemoveAll always returns ErrReadOnly.
func (e *EmbedFS) RemoveAll(path string) error {
	return ErrReadOnly
}

// Stat returns file info for the named file.
func (e *EmbedFS) Stat(name string) (os.FileInfo, error) {
	return fs.Stat(e.FS, name)
}

// Exists reports whether the named file exists in the embedded file system.
func (e *EmbedFS) Exists(name string) bool {
	_, err := e.Stat(name)
	return err == nil
}
