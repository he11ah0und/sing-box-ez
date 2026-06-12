package fs

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"

	"sing-box-ez/internal/framework/logger"
)

// EmbedFS implements FileSystem over an embed.FS.
// All write operations return ErrReadOnly.
// Because embed.FS.Open returns fs.File rather than *os.File, Open copies
// the embedded content to a temporary file to satisfy the interface.
type EmbedFS struct {
	FS  embed.FS
	Log *logger.LogTerminal
}

func (e *EmbedFS) errf(format string, v ...interface{}) error {
	if e.Log != nil {
		return e.Log.Errorf(format, v...)
	}
	return fmt.Errorf(format, v...)
}

// ReadFile reads the named file from the embedded file system.
func (e *EmbedFS) ReadFile(name string) ([]byte, error) {
	data, err := e.FS.ReadFile(name)
	if err != nil {
		return nil, e.errf("read %q: %w", name, err)
	}
	return data, nil
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
	entries, err := fs.ReadDir(e.FS, name)
	if err != nil {
		return nil, e.errf("read dir %q: %w", name, err)
	}
	return entries, nil
}

// Open opens the named file for reading.
// The content is copied to a temporary file so that the returned value is
// an *os.File as required by the FileSystem interface.
func (e *EmbedFS) Open(name string) (*os.File, error) {
	src, err := e.FS.Open(name)
	if err != nil {
		return nil, e.errf("open %q: %w", name, err)
	}
	defer src.Close()

	tmp, err := os.CreateTemp("", "embed-*")
	if err != nil {
		return nil, e.errf("create temp for %q: %w", name, err)
	}

	if _, err := io.Copy(tmp, src); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, e.errf("copy %q: %w", name, err)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, e.errf("seek temp for %q: %w", name, err)
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
	fi, err := fs.Stat(e.FS, name)
	if err != nil {
		return nil, e.errf("stat %q: %w", name, err)
	}
	return fi, nil
}

// Exists reports whether the named file exists in the embedded file system.
func (e *EmbedFS) Exists(name string) bool {
	_, err := e.Stat(name)
	return err == nil
}
