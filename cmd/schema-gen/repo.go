package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var versionTagRE = regexp.MustCompile(`^v([0-9]+)\.([0-9]+)\.([0-9]+)(?:[-.]?(alpha|beta|rc)\.(\d+))?$`)

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

// VersionTag represents a parsed release tag, including optional pre-release.
type VersionTag struct {
	Tag           string
	Major         int
	Minor         int
	Patch         int
	Prerelease    string // "alpha", "beta", "rc", or empty for stable
	PrereleaseNum int
}

// prereleaseKind returns the ordering weight of a pre-release label.
// alpha < beta < rc < stable.
func prereleaseKind(p string) int {
	switch p {
	case "alpha":
		return 0
	case "beta":
		return 1
	case "rc":
		return 2
	default:
		return 3 // stable
	}
}

// IsStable reports whether the tag is a stable release.
func (v VersionTag) IsStable() bool {
	return v.Prerelease == ""
}

// Less reports whether v is strictly less than o.
func (v VersionTag) Less(o VersionTag) bool {
	if v.Major != o.Major {
		return v.Major < o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor < o.Minor
	}
	if v.Patch != o.Patch {
		return v.Patch < o.Patch
	}
	vk, ok := prereleaseKind(v.Prerelease), prereleaseKind(o.Prerelease)
	if vk != ok {
		return vk < ok
	}
	return v.PrereleaseNum < o.PrereleaseNum
}

// String returns the canonical version string.
func (v VersionTag) String() string {
	if v.Prerelease == "" {
		return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	}
	return fmt.Sprintf("%d.%d.%d-%s.%d", v.Major, v.Minor, v.Patch, v.Prerelease, v.PrereleaseNum)
}

type versionKey struct {
	key string
	tag VersionTag
}

// ReleaseTags returns sorted release tags including pre-releases.
func (r *Repo) ReleaseTags() ([]VersionTag, error) {
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
		tag.Major, _ = strconv.Atoi(m[1])
		tag.Minor, _ = strconv.Atoi(m[2])
		tag.Patch, _ = strconv.Atoi(m[3])
		tag.Prerelease = m[4]
		if m[5] != "" {
			tag.PrereleaseNum, _ = strconv.Atoi(m[5])
		}
		tags = append(tags, tag)
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Less(tags[j]) })
	return tags, nil
}

// OnePerMinor filters tags to one representative per minor version (the latest
// release). If a stable release exists for a patch, it is preferred over a
// pre-release of the same patch; otherwise the latest pre-release is used.
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
		// Prefer the latest stable if any; otherwise the latest pre-release.
		chosen := list[len(list)-1]
		for i := len(list) - 1; i >= 0; i-- {
			if list[i].IsStable() {
				chosen = list[i]
				break
			}
		}
		res = append(res, chosen)
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
