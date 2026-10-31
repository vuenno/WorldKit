// Package update finds newer WorldKit releases on GitHub, downloads their installer and
// starts it, so WorldKit can update itself.
package update

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrNotVersion means a text isn't a semantic version such as 1.2.3 or v1.2.3-beta.1.
var ErrNotVersion = errors.New("not a version")

// Version is a semantic version (semver.org): major.minor.patch with an optional
// pre-release such as "beta.1". Build metadata ("+abc") is ignored, as semver says.
type Version struct {
	Major, Minor, Patch int
	Prerelease          string
}

// ParseVersion reads "1.2.3", "v1.2.3" or "1.2.3-rc.1".
func ParseVersion(text string) (Version, error) {
	core := strings.TrimPrefix(strings.TrimSpace(text), "v")
	core, _, _ = strings.Cut(core, "+")
	core, prerelease, hasPrerelease := strings.Cut(core, "-")
	if hasPrerelease && !isValidPrerelease(prerelease) {
		return Version{}, fmt.Errorf("%w: %q", ErrNotVersion, text)
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("%w: %q", ErrNotVersion, text)
	}
	var numbers [3]int
	for i, part := range parts {
		number, ok := parseNumber(part)
		if !ok {
			return Version{}, fmt.Errorf("%w: %q", ErrNotVersion, text)
		}
		numbers[i] = number
	}
	return Version{Major: numbers[0], Minor: numbers[1], Patch: numbers[2], Prerelease: prerelease}, nil
}

// parseNumber reads a version number: digits only, no leading zero (semver).
func parseNumber(text string) (int, bool) {
	if text == "" || (len(text) > 1 && text[0] == '0') || strings.TrimLeft(text, "0123456789") != "" {
		return 0, false
	}
	number, err := strconv.Atoi(text)
	return number, err == nil
}

func isValidPrerelease(text string) bool {
	for _, identifier := range strings.Split(text, ".") {
		if identifier == "" || strings.Trim(identifier, "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-") != "" {
			return false
		}
	}
	return true
}

// String writes the version without a leading v: "1.2.3" or "1.2.3-rc.1".
func (v Version) String() string {
	text := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		text += "-" + v.Prerelease
	}
	return text
}

// Compare returns -1 if v is older than other, 1 if newer, 0 if they are the same release.
func (v Version) Compare(other Version) int {
	for _, pair := range [][2]int{{v.Major, other.Major}, {v.Minor, other.Minor}, {v.Patch, other.Patch}} {
		if pair[0] != pair[1] {
			return compareInts(pair[0], pair[1])
		}
	}
	return comparePrereleases(v.Prerelease, other.Prerelease)
}

// IsNewerThan reports whether v is a later release than other.
func (v Version) IsNewerThan(other Version) bool {
	return v.Compare(other) > 0
}

// comparePrereleases orders pre-releases as semver does: none ranks above any, then
// identifier by identifier, numbers below words, and a longer list above its prefix.
func comparePrereleases(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return 1
	}
	if b == "" {
		return -1
	}
	aParts, bParts := strings.Split(a, "."), strings.Split(b, ".")
	for i := range min(len(aParts), len(bParts)) {
		if order := compareIdentifiers(aParts[i], bParts[i]); order != 0 {
			return order
		}
	}
	return compareInts(len(aParts), len(bParts))
}

func compareIdentifiers(a, b string) int {
	aNumber, isANumber := parseNumber(a)
	bNumber, isBNumber := parseNumber(b)
	switch {
	case isANumber && isBNumber:
		return compareInts(aNumber, bNumber)
	case isANumber:
		return -1
	case isBNumber:
		return 1
	}
	return strings.Compare(a, b)
}

func compareInts(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
