// Package fs provides an object-oriented file-system abstraction.
//
// A FS represents a root file system. Its Root() returns a Directory, from which
// applications can obtain File and Directory objects, scan entries, and perform
// I/O without manually concatenating paths.
package fs

import (
	"errors"
	"os"
)

// ErrReadOnly is returned by read-only file-system implementations on any
// write operation.
var ErrReadOnly = errors.New("read-only file system")

// FS is the root of a file-system abstraction.
type FS interface {
	Root() Directory
}

// Entry represents a file-system entry (file or directory).
type Entry interface {
	// Name returns the entry base name.
	Name() string
	// Path returns the entry path relative to the FS root.
	Path() string
	// Exists reports whether the entry exists.
	Exists() bool
	// Stat returns file info for the entry.
	Stat() (os.FileInfo, error)
	// Parent returns the parent directory. For the root directory it returns nil.
	Parent() Directory
	// Rename renames the entry within its parent directory.
	Rename(newName string) error
	// Remove removes the entry.
	Remove() error
	// Chmod changes the entry's permissions.
	Chmod(perm os.FileMode) error
}

// Directory represents a directory entry in a FS.
type Directory interface {
	Entry
	// File returns a file object inside this directory.
	File(name string) File
	// Subdir returns a subdirectory object inside this directory.
	Subdir(name string) Directory
	// ReadDir returns the direct children of this directory.
	ReadDir() ([]Entry, error)
	// MkdirAll ensures this directory exists, creating parents if needed.
	MkdirAll(perm os.FileMode) error
	// RemoveAll removes this directory and all of its children.
	RemoveAll() error
	// CopyTo recursively copies this directory into dst.
	CopyTo(dst Directory) error
	// Ensure ensures the directory exists with the requested permissions.
	Ensure(perm os.FileMode) error
}

// File represents a file entry in a FS.
type File interface {
	Entry
	// Read reads the whole file content.
	Read() ([]byte, error)
	// Write writes data to the file, creating/truncating it.
	Write(data []byte, perm os.FileMode) error
	// Open opens the file for reading.
	Open() (*os.File, error)
	// OpenFile opens the file with the given flags and permissions.
	OpenFile(flag int, perm os.FileMode) (*os.File, error)
	// AtomicWrite writes data via a temporary file and renames it into place.
	AtomicWrite(data []byte, perm os.FileMode) error
	// CopyTo copies this file's content into dst.
	CopyTo(dst File) error
}
