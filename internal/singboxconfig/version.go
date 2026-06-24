package singboxconfig

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a simple major.minor.patch version used for sing-box releases.
type Version struct {
	Major int
	Minor int
	Patch int
}

// ParseVersion parses a version string like "1.14.0" or "1.8.0-beta".
// Pre-release suffixes are ignored for comparison.
func ParseVersion(s string) (Version, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Version{}, fmt.Errorf("empty version")
	}
	// Strip pre-release/build metadata.
	if idx := strings.IndexAny(s, "-+"); idx != -1 {
		s = s[:idx]
	}
	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return Version{}, fmt.Errorf("invalid version %q", s)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, fmt.Errorf("invalid major in %q: %w", s, err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Version{}, fmt.Errorf("invalid minor in %q: %w", s, err)
	}
	patch := 0
	if len(parts) == 3 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return Version{}, fmt.Errorf("invalid patch in %q: %w", s, err)
		}
	}
	return Version{Major: major, Minor: minor, Patch: patch}, nil
}

// Less reports whether v is strictly less than o.
func (v Version) Less(o Version) bool {
	if v.Major != o.Major {
		return v.Major < o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor < o.Minor
	}
	return v.Patch < o.Patch
}

// Equal reports whether v equals o.
func (v Version) Equal(o Version) bool {
	return v.Major == o.Major && v.Minor == o.Minor && v.Patch == o.Patch
}

// LessOrEqual reports whether v is less than or equal to o.
func (v Version) LessOrEqual(o Version) bool {
	return v.Less(o) || v.Equal(o)
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}
