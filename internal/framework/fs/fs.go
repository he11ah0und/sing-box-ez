// Package fs provides an abstract file-system interface for framework services.
// It wraps OS file operations so that implementations can be swapped (e.g.
// in-memory FS for tests) and adds path-sanitisation for safety.
package fs

import (
	"errors"
	"os"
)

// ErrReadOnly is returned by read-only file-system implementations on any
// write operation.
var ErrReadOnly = errors.New("read-only file system")

// FileSystem abstracts read/write file operations.
type FileSystem interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	Open(name string) (*os.File, error)
	ReadDir(name string) ([]os.DirEntry, error)
	Rename(oldpath, newpath string) error
	Remove(name string) error
	RemoveAll(path string) error
	Stat(name string) (os.FileInfo, error)
	Exists(name string) bool
}
