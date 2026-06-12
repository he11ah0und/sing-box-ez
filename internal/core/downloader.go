package core

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// ProgressFunc вызывается во время скачивания: скачано, всего.
type ProgressFunc func(downloaded, total int64)

type progressReader struct {
	reader   io.Reader
	total    int64
	current  int64
	callback ProgressFunc
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.reader.Read(buf)
	p.current += int64(n)
	if p.callback != nil {
		p.callback(p.current, p.total)
	}
	return n, err
}

func DownloadConfigFor(name, url string) error {
	if Net == nil || FS == nil {
		return fmt.Errorf("core services not initialized")
	}
	return Net.DownloadToFile(context.Background(), FS, url, CachedConfig(name))
}

func GetConfigPath(name string) string {
	return CachedConfig(name)
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func GetLatestVersion() (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/SagerNet/sing-box/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github api status: %s", resp.Status)
	}
	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	version := strings.TrimPrefix(release.TagName, "v")
	return version, nil
}

func GetCoreVersion(corePath string) (string, error) {
	if _, err := os.Stat(corePath); err != nil {
		return "", err
	}
	cmd := exec.Command(corePath, "version")
	setNoWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	parts := strings.Fields(string(out))
	for i, p := range parts {
		if p == "version" && i+1 < len(parts) {
			return parts[i+1], nil
		}
	}
	return "", fmt.Errorf("unable to parse version from: %s", string(out))
}

func platformSuffix(version string) (archiveName, binaryName string, isZip bool) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	suffix := fmt.Sprintf("%s-%s", goos, goarch)
	if goos == "windows" {
		return fmt.Sprintf("sing-box-%s-%s.zip", version, suffix), "sing-box.exe", true
	}
	return fmt.Sprintf("sing-box-%s-%s.tar.gz", version, suffix), "sing-box", false
}

func DownloadCore(version string, onProgress ProgressFunc) (string, error) {
	if version == "" {
		var err error
		version, err = GetLatestVersion()
		if err != nil {
			return "", fmt.Errorf("failed to get latest version: %w", err)
		}
	}

	archiveName, binaryName, isZip := platformSuffix(version)
	url := "https://github.com/SagerNet/sing-box/releases/download/v" + version + "/" + archiveName

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download core: %s", resp.Status)
	}

	tmpFile, err := os.CreateTemp("", "sing-box-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())

	var src io.Reader = resp.Body
	if onProgress != nil && resp.ContentLength > 0 {
		src = &progressReader{reader: resp.Body, total: resp.ContentLength, callback: onProgress}
	}

	_, err = io.Copy(tmpFile, src)
	_ = tmpFile.Close()
	if err != nil {
		return "", err
	}

	targetPath := CoreBinary()
	if isZip {
		err = extractZip(tmpFile.Name(), targetPath, binaryName)
	} else {
		err = extractTarGz(tmpFile.Name(), targetPath, binaryName)
	}
	if err != nil {
		return "", err
	}

	if runtime.GOOS != "windows" {
		// #nosec G302 — core binary must be executable; path is controlled by the app (CoreBinary).
		_ = os.Chmod(targetPath, 0750)
	}

	return targetPath, nil
}

func extractTarGz(archivePath, targetPath, binaryName string) error {
	// #nosec G304 — archivePath is a temp file created by os.CreateTemp in the same function.
	f, err := os.Open(archivePath)
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
		if strings.HasSuffix(hdr.Name, binaryName) && hdr.FileInfo().Mode().IsRegular() {
			// #nosec G304 — targetPath is the managed core binary path (CoreBinary).
			// #nosec G304 — targetPath is the managed core binary path (CoreBinary).
			// #nosec G304 — targetPath is the managed core binary path (CoreBinary).
			out, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, io.LimitReader(tr, 500*1024*1024))
			_ = out.Close()
			return err
		}
	}
	return fmt.Errorf("binary not found in archive")
}

func extractZip(archivePath, targetPath, binaryName string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, binaryName) && !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			// #nosec G304 — targetPath is the managed core binary path (CoreBinary).
			out, err := os.Create(targetPath)
			if err != nil {
				_ = rc.Close()
				return err
			}
			_, err = io.Copy(out, io.LimitReader(rc, 500*1024*1024))
			_ = out.Close()
			_ = rc.Close()
			return err
		}
	}
	return fmt.Errorf("binary not found in archive")
}

func GetCorePath() string {
	return CoreBinary()
}

func CoreExists() bool {
	_, err := os.Stat(GetCorePath())
	return err == nil
}
