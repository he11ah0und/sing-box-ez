// Package fs provides an abstract file-system interface for framework services.
// It wraps OS file operations so that implementations can be swapped (e.g.
// in-memory FS for tests) and adds path-sanitisation for safety.
package fs

import (
	"errors"
	"os"
	"strings"
)

// ErrReadOnly is returned by read-only file-system implementations on any
// write operation.
var ErrReadOnly = errors.New("read-only file system")

// NormalizePath converts OS-style path separators to forward slashes.
// This is required for embed.FS and is safe for cross-platform use because
// Go's OS file APIs also accept forward-slash paths on Windows.
func NormalizePath(name string) string {
	return strings.ReplaceAll(name, "\\", "/")
}

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
