package fs

import (
	"embed"
	"fmt"
	"io"
	"os"
)

// Embed creates a read-only FS backed by an embedded file system.
func Embed(fsys embed.FS) FS {
	return &embedFS{fsys: fsys}
}

type embedFS struct {
	fsys embed.FS
}

func (fs *embedFS) Root() Directory {
	return &embedDirectory{embedEntry: embedEntry{fs: fs, path: ""}}
}

func (fs *embedFS) openAsOSFile(name string) (*os.File, error) {
	data, err := fs.fsys.ReadFile(name)
	if err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp("", "embed-*")
	if err != nil {
		return nil, err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, err
	}
	return tmp, nil
}

func (fs *embedFS) readOnly(name string) error {
	return fmt.Errorf("%s: %w", name, ErrReadOnly)
}

type embedEntry struct {
	fs   *embedFS
	path string
}

func (e *embedEntry) Name() string { return baseName(e.path) }

func (e *embedEntry) Path() string { return e.path }

func (e *embedEntry) Exists() bool {
	_, err := e.fs.fsys.Open(e.embedPath())
	return err == nil
}

func (e *embedEntry) Stat() (os.FileInfo, error) {
	f, err := e.fs.fsys.Open(e.embedPath())
	if err != nil {
		return nil, fmt.Errorf("stat %q: %w", e.path, err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat %q: %w", e.path, err)
	}
	return fi, nil
}

func (e *embedEntry) Parent() Directory {
	p := parentPath(e.path)
	if p == "" && e.path == "" {
		return nil
	}
	return &embedDirectory{embedEntry: embedEntry{fs: e.fs, path: p}}
}

func (e *embedEntry) Rename(newName string) error {
	return e.fs.readOnly("rename")
}

func (e *embedEntry) Remove() error {
	return e.fs.readOnly("remove")
}

func (e *embedEntry) Chmod(perm os.FileMode) error {
	return e.fs.readOnly("chmod")
}

func (e *embedEntry) embedPath() string {
	if e.path == "" {
		return "."
	}
	return e.path
}

type embedDirectory struct {
	embedEntry
}

func (d *embedDirectory) File(name string) File {
	p, _ := joinChild(d.path, name)
	return &embedFile{embedEntry: embedEntry{fs: d.fs, path: p}}
}

func (d *embedDirectory) Subdir(name string) Directory {
	p, _ := joinChild(d.path, name)
	return &embedDirectory{embedEntry: embedEntry{fs: d.fs, path: p}}
}

func (d *embedDirectory) ReadDir() ([]Entry, error) {
	entries, err := d.fs.fsys.ReadDir(d.embedPath())
	if err != nil {
		return nil, fmt.Errorf("read dir %q: %w", d.path, err)
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		childPath, err := joinChild(d.path, e.Name())
		if err != nil {
			continue
		}
		ent := embedEntry{fs: d.fs, path: childPath}
		if e.IsDir() {
			out = append(out, &embedDirectory{embedEntry: ent})
		} else {
			out = append(out, &embedFile{embedEntry: ent})
		}
	}
	return out, nil
}

func (d *embedDirectory) MkdirAll(perm os.FileMode) error {
	return d.fs.readOnly("mkdir")
}

func (d *embedDirectory) RemoveAll() error {
	return d.fs.readOnly("remove all")
}

func (d *embedDirectory) CopyTo(dst Directory) error {
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

func (d *embedDirectory) Ensure(perm os.FileMode) error {
	if d.Exists() {
		return nil
	}
	return d.fs.readOnly("ensure dir")
}

type embedFile struct {
	embedEntry
}

func (f *embedFile) Read() ([]byte, error) {
	data, err := f.fs.fsys.ReadFile(f.embedPath())
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", f.path, err)
	}
	return data, nil
}

func (f *embedFile) Write(data []byte, perm os.FileMode) error {
	return f.fs.readOnly("write")
}

func (f *embedFile) Open() (*os.File, error) {
	file, err := f.fs.openAsOSFile(f.embedPath())
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", f.path, err)
	}
	return file, nil
}

func (f *embedFile) OpenFile(flag int, perm os.FileMode) (*os.File, error) {
	return nil, f.fs.readOnly("open file")
}

func (f *embedFile) AtomicWrite(data []byte, perm os.FileMode) error {
	return f.fs.readOnly("atomic write")
}

func (f *embedFile) CopyTo(dst File) error {
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
