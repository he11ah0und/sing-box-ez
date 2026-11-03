package updater

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

// The releases.atom feed and the releases/expanded_assets/<tag> HTML fragment
// are served from github.com, not api.github.com, so they are not subject to
// the REST API rate limit. They are used for the hot paths (update checks);
// the REST API stays as a fallback for tags the short feed does not carry.
//
// What the Atom feed provides: tag (via the entry link), title, publication
// time and the release notes HTML. What it does NOT provide: the prerelease
// flag and the target branch — both are inferred from the tag name, and the
// asset list is fetched separately from expanded_assets.

type atomEntry struct {
	ID      string `xml:"id"`
	Updated string `xml:"updated"`
	Title   string `xml:"title"`
	Content string `xml:"content"`
	Links   []struct {
		Href string `xml:"href,attr"`
	} `xml:"link"`
}

type atomFeed struct {
	Entries []atomEntry `xml:"entry"`
}

// tag extracts the release tag from the entry's alternate link.
func (e atomEntry) tag() string {
	const marker = "/releases/tag/"
	for _, l := range e.Links {
		if i := strings.Index(l.Href, marker); i >= 0 {
			return l.Href[i+len(marker):]
		}
	}
	return ""
}

func (e atomEntry) publishedAt() time.Time {
	t, _ := time.Parse(time.RFC3339, e.Updated)
	return t
}

// prereleaseTag reports whether the tag looks like a prerelease using the
// backend's configured markers (the Atom feed itself has no such flag).
func (b *GitHubBackend) prereleaseTag(tag string) bool {
	markers := b.PrereleaseMarkers
	if len(markers) == 0 {
		markers = defaultPrereleaseMarkers()
	}
	lower := strings.ToLower(tag)
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// channelForTag maps a tag to a channel using the backend's ChannelTags
// configuration. Empty means no channel was inferred from the tag.
func (b *GitHubBackend) channelForTag(tag string) string {
	for marker, channel := range b.ChannelTags {
		if strings.Contains(tag, marker) {
			return channel
		}
	}
	return ""
}

// isStableChannel reports whether the requested channel is the stable one.
func (b *GitHubBackend) isStableChannel(channel string) bool {
	return channel == "" || channel == b.StableChannel
}

func (b *GitHubBackend) doXML(req *http.Request, out any) error {
	resp, err := b.netClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return xml.NewDecoder(resp.Body).Decode(out)
}

// fetchAtom returns the feed entries, newest first.
func (b *GitHubBackend) fetchAtom(ctx context.Context) ([]atomEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.ProjectURL()+"/releases.atom", nil)
	if err != nil {
		return nil, err
	}
	var feed atomFeed
	if err := b.doXML(req, &feed); err != nil {
		return nil, err
	}
	sort.Slice(feed.Entries, func(i, j int) bool {
		return feed.Entries[i].publishedAt().After(feed.Entries[j].publishedAt())
	})
	return feed.Entries, nil
}

var assetLinkRE = regexp.MustCompile(`href="([^"]*/releases/download/[^"]+)"`)

// assetsFromExpandedAssets reads the per-tag asset HTML fragment and returns
// the assets with absolute download URLs on github.com.
func (b *GitHubBackend) assetsFromExpandedAssets(ctx context.Context, tag string) ([]Asset, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.ProjectURL()+"/releases/expanded_assets/"+tag, nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.netClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var out []Asset
	for _, m := range assetLinkRE.FindAllStringSubmatch(string(body), -1) {
		href := m[1]
		name := href[strings.LastIndex(href, "/")+1:]
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if strings.HasPrefix(href, "/") {
			href = b.webBase() + href
		}
		out = append(out, Asset{Name: name, URL: href, Format: formatForName(name), Hashes: make(map[string]string)})
	}
	return out, nil
}

// releaseFromAtom builds a Release from one feed entry plus its asset list.
func (b *GitHubBackend) releaseFromAtom(ctx context.Context, e atomEntry) (Release, error) {
	tag := e.tag()
	if tag == "" {
		return Release{}, fmt.Errorf("atom entry %q has no release tag", e.ID)
	}
	assets, err := b.assetsFromExpandedAssets(ctx, tag)
	if err != nil {
		return Release{}, err
	}
	channel := b.channelForTag(tag)
	if channel == "" {
		channel = b.StableChannel
	}
	r := Release{
		Version:     tag,
		Channel:     channel,
		Name:        e.Title,
		Body:        e.Content,
		PublishedAt: e.publishedAt(),
		Prerelease:  b.prereleaseTag(tag),
	}
	for i := range assets {
		assets[i].Tags = b.assetTags(r, assets[i].Name)
	}
	r.Assets = assets
	return r, nil
}

// latestFromAtom returns the newest matching release for the channel.
// With an empty channel it picks the newest non-prerelease entry; otherwise it
// prefers a stable release on that branch and falls back to its prereleases.
func (b *GitHubBackend) latestFromAtom(ctx context.Context, channel string) (Release, error) {
	entries, err := b.fetchAtom(ctx)
	if err != nil {
		return Release{}, err
	}
	if b.isStableChannel(channel) {
		for _, e := range entries {
			if !b.prereleaseTag(e.tag()) {
				return b.releaseFromAtom(ctx, e)
			}
		}
		b.Log.TInfof("updater.no_releases_found", channel)
		return Release{}, fmt.Errorf("%w %s", ErrNoRelease, channel)
	}
	for _, e := range entries {
		if b.channelForTag(e.tag()) == channel && !b.prereleaseTag(e.tag()) {
			return b.releaseFromAtom(ctx, e)
		}
	}
	for _, e := range entries {
		if b.channelForTag(e.tag()) == channel && b.prereleaseTag(e.tag()) {
			return b.releaseFromAtom(ctx, e)
		}
	}
	b.Log.TInfof("updater.no_releases_found", channel)
	return Release{}, fmt.Errorf("%w %s", ErrNoRelease, channel)
}

// listFromAtom returns every release present in the feed, newest first.
func (b *GitHubBackend) listFromAtom(ctx context.Context) ([]Release, error) {
	entries, err := b.fetchAtom(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Release, 0, len(entries))
	for _, e := range entries {
		r, err := b.releaseFromAtom(ctx, e)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// channelsFromAtom derives the available channels from the tags in the feed,
// instead of calling the branches API. When no tag-to-channel mapping is
// configured it returns nil so the branches API fallback is used instead of
// guessing a convention.
func (b *GitHubBackend) channelsFromAtom(ctx context.Context) ([]Channel, error) {
	if len(b.ChannelTags) == 0 {
		return nil, nil
	}
	entries, err := b.fetchAtom(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var out []Channel
	if b.StableChannel != "" {
		seen[b.StableChannel] = true
		out = append(out, Channel{ID: b.StableChannel, Name: b.StableChannel})
	}
	for _, e := range entries {
		ch := b.channelForTag(e.tag())
		if ch == "" || seen[ch] {
			continue
		}
		seen[ch] = true
		out = append(out, Channel{ID: ch, Name: ch})
	}
	return out, nil
}

// releaseByVersionAtom finds an exact tag in the short feed. Older tags that
// fell out of the feed return ErrNoRelease so the caller can use the REST API.
func (b *GitHubBackend) releaseByVersionAtom(ctx context.Context, version string) (Release, error) {
	entries, err := b.fetchAtom(ctx)
	if err != nil {
		return Release{}, err
	}
	for _, e := range entries {
		if e.tag() == version {
			return b.releaseFromAtom(ctx, e)
		}
	}
	return Release{}, fmt.Errorf("%w %s", ErrNoRelease, version)
}
