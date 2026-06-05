//go:build !noplugins

package plugins

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// InstallFromURL downloads a plugin package (zip or tar.gz), extracts it,
// validates the manifest, and moves it into the plugins directory.
// Returns the loaded manifest.
func InstallFromURL(url string) (*Manifest, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download HTTP %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "plugin-*.tmp")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("save failed: %w", err)
	}
	tmpFile.Close()

	// Determine format and extract.
	contentType := resp.Header.Get("Content-Type")
	extractDir, err := os.MkdirTemp("", "plugin-extract-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(extractDir)

	switch {
	case strings.HasSuffix(url, ".zip") || contentType == "application/zip":
		if err := extractZip(tmpFile.Name(), extractDir); err != nil {
			return nil, fmt.Errorf("extract zip failed: %w", err)
		}
	case strings.HasSuffix(url, ".tar.gz") || strings.HasSuffix(url, ".tgz") ||
		contentType == "application/gzip" || contentType == "application/x-gzip":
		if err := extractTarGz(tmpFile.Name(), extractDir); err != nil {
			return nil, fmt.Errorf("extract tar.gz failed: %w", err)
		}
	default:
		// Try zip first, then tar.gz.
		if err := extractZip(tmpFile.Name(), extractDir); err == nil {
			break
		}
		os.RemoveAll(extractDir)
		extractDir, _ = os.MkdirTemp("", "plugin-extract-*")
		if err := extractTarGz(tmpFile.Name(), extractDir); err != nil {
			return nil, fmt.Errorf("unknown archive format (tried zip and tar.gz): %w", err)
		}
	}

	// Find manifest.json (may be nested one level deep).
	mf, pdir, err := findManifest(extractDir)
	if err != nil {
		return nil, err
	}

	// Move to final destination.
	destDir := filepath.Join(PluginDir(), mf.Name)
	if _, err := os.Stat(destDir); err == nil {
		return nil, fmt.Errorf("plugin %s already exists", mf.Name)
	}
	if err := os.Rename(pdir, destDir); err != nil {
		return nil, fmt.Errorf("install failed: %w", err)
	}

	mf.SourceType = "package"
	mf.SourceURL = url
	mf.Enabled = true
	return mf, nil
}

// findManifest searches for manifest.json inside extractDir.
// It returns the manifest, the directory containing it, and any error.
func findManifest(root string) (*Manifest, string, error) {
	// Try root first.
	mf, err := LoadManifest(root)
	if err == nil {
		return mf, root, nil
	}

	// Try immediate subdirectories.
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, "", err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(root, e.Name())
		mf, err = LoadManifest(sub)
		if err == nil {
			return mf, sub, nil
		}
	}
	return nil, "", fmt.Errorf("manifest.json not found in archive")
}

func extractZip(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		path := filepath.Join(dst, f.Name)
		if !strings.HasPrefix(path, filepath.Clean(dst)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path in zip: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTarGz(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		path := filepath.Join(dst, hdr.Name)
		if !strings.HasPrefix(path, filepath.Clean(dst)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path in tar: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}
