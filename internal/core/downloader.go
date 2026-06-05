package core

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"sing-box-ez/internal/githuburl"
	"sing-box-ez/internal/paths"
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
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return os.WriteFile(paths.CachedConfig(name), data, 0600)
}

func GetConfigPath(name string) string {
	return paths.CachedConfig(name)
}

func HasCachedConfig(name string) bool {
	return paths.HasCachedConfig(name)
}

func ListCachedConfigs() ([]string, error) {
	return paths.ListCachedConfigs()
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func GetLatestVersion() (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(githuburl.DefaultCoreProject().APILatestReleaseURL())
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
	url := githuburl.DefaultCoreProject().DownloadReleaseURL(version, archiveName)

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
	tmpFile.Close()
	if err != nil {
		return "", err
	}

	targetPath := paths.CoreBinary()
	if isZip {
		err = extractZip(tmpFile.Name(), targetPath, binaryName)
	} else {
		err = extractTarGz(tmpFile.Name(), targetPath, binaryName)
	}
	if err != nil {
		return "", err
	}

	if runtime.GOOS != "windows" {
		os.Chmod(targetPath, 0755)
	}

	return targetPath, nil
}

func extractTarGz(archivePath, targetPath, binaryName string) error {
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
			out, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			out.Close()
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
			out, err := os.Create(targetPath)
			if err != nil {
				rc.Close()
				return err
			}
			_, err = io.Copy(out, rc)
			out.Close()
			rc.Close()
			return err
		}
	}
	return fmt.Errorf("binary not found in archive")
}

func GetCorePath() string {
	return paths.CoreBinary()
}

func CoreExists() bool {
	_, err := os.Stat(GetCorePath())
	return err == nil
}
