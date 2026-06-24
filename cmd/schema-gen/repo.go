package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var versionTagRE = regexp.MustCompile(`^v([0-9]+)\.([0-9]+)\.([0-9]+)$`)

// Repo manages a local clone of the sing-box repository.
type Repo struct {
	Path string
}

// OpenRepo ensures a local clone exists at path and returns a Repo handle.
func OpenRepo(path string, url string) (*Repo, error) {
	isRepo := func(p string) bool {
		if _, err := os.Stat(filepath.Join(p, ".git")); err == nil {
			return true
		}
		// bare repo
		if _, err := os.Stat(filepath.Join(p, "HEAD")); err == nil {
			return true
		}
		return false
	}
	if !isRepo(path) {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return nil, fmt.Errorf("create repo dir: %w", err)
		}
		cmd := exec.Command("git", "clone", "--bare", url, path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("clone repo: %w", err)
		}
	}
	return &Repo{Path: path}, nil
}

// FetchTags updates tags from origin.
func (r *Repo) FetchTags() error {
	cmd := exec.Command("git", "-C", r.Path, "fetch", "--tags", "origin")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("fetch tags: %w", err)
	}
	return nil
}

// VersionTag represents a parsed release tag.
type VersionTag struct {
	Tag   string
	Major int
	Minor int
	Patch int
}

// Less reports whether v is strictly less than o.
func (v VersionTag) Less(o VersionTag) bool {
	if v.Major != o.Major {
		return v.Major < o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor < o.Minor
	}
	return v.Patch < o.Patch
}

// String returns the canonical version string.
func (v VersionTag) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

type versionKey struct {
	key string
	tag VersionTag
}

// StableTags returns sorted stable v1.x.x release tags.
func (r *Repo) StableTags() ([]VersionTag, error) {
	cmd := exec.Command("git", "-C", r.Path, "tag", "-l", "v1.*.*")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	var tags []VersionTag
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := versionTagRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		tag := VersionTag{Tag: line}
		fmt.Sscanf(m[1], "%d", &tag.Major)
		fmt.Sscanf(m[2], "%d", &tag.Minor)
		fmt.Sscanf(m[3], "%d", &tag.Patch)
		tags = append(tags, tag)
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Less(tags[j]) })
	return tags, nil
}

// OnePerMinor filters tags to one representative per minor version (the earliest
// patch). It also appends the latest patch of the highest minor so that recent
// patches are represented.
func OnePerMinor(tags []VersionTag) []VersionTag {
	byMinor := make(map[string][]VersionTag)
	for _, t := range tags {
		key := fmt.Sprintf("%d.%d", t.Major, t.Minor)
		byMinor[key] = append(byMinor[key], t)
	}
	var keys []versionKey
	for k, list := range byMinor {
		keys = append(keys, versionKey{key: k, tag: list[0]})
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].tag.Less(keys[j].tag) })
	var res []VersionTag
	for _, k := range keys {
		list := byMinor[k.key]
		sort.Slice(list, func(i, j int) bool { return list[i].Less(list[j]) })
		res = append(res, list[0])
		// Keep the latest patch of the latest minor.
		if k.key == keys[len(keys)-1].key && len(list) > 1 {
			res = append(res, list[len(list)-1])
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Less(res[j]) })
	return res
}

// ReadFileAt reads a file from a tag via git show.
func (r *Repo) ReadFileAt(tag, path string) ([]byte, error) {
	cmd := exec.Command("git", "-C", r.Path, "show", fmt.Sprintf("%s:%s", tag, path))
	return cmd.Output()
}

// FileExistsAt reports whether a path exists at a tag.
func (r *Repo) FileExistsAt(tag, path string) bool {
	_, err := r.ReadFileAt(tag, path)
	return err == nil
}

// ListFilesAt lists files under prefix at a tag.
func (r *Repo) ListFilesAt(tag, prefix string) ([]string, error) {
	cmd := exec.Command("git", "-C", r.Path, "ls-tree", "-r", "--name-only", tag, prefix)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}
