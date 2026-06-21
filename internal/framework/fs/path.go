package fs

import (
	"fmt"
	"path/filepath"
	"strings"
)

// NormalizePath converts OS-style path separators to forward slashes.
func NormalizePath(name string) string {
	return strings.ReplaceAll(name, "\\", "/")
}

// baseName returns the last path element.
func baseName(path string) string {
	path = strings.TrimRight(path, "/")
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}

// parentPath returns the parent directory path or empty string for root.
func parentPath(path string) string {
	path = strings.TrimRight(path, "/")
	if path == "" {
		return ""
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "/" {
		return ""
	}
	return dir
}

// joinChild safely joins a parent path with a child name.
// It rejects names that would escape the FS root.
func joinChild(parentPath, name string) (string, error) {
	name = NormalizePath(name)
	if name == "" || name == "." {
		return parentPath, nil
	}
	if strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("invalid name %q", name)
	}
	name = strings.Trim(name, "/")
	if name == "" {
		return parentPath, nil
	}
	if parentPath == "" {
		return name, nil
	}
	return parentPath + "/" + name, nil
}
