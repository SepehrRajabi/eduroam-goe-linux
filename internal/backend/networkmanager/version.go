// eduroam-goe-linux — a modified Go port of the eduroam CAT Linux installer.
//
// Based on the eduroam CAT Linux installer (https://github.com/GEANT/CAT),
// developed on behalf of the GÉANT project; copyright GEANT Association /
// GEANT Limited. Original authors: Tomasz Wolniewicz, Michał Gasewicz and
// contributors (see NOTICE).
//
// Modified by Sepehr Rajabi, 2026: ported to Go, with a Bubble Tea terminal
// UI and pluggable network backends. This is not the Original Work.
//
// Licensed under the GÉANT Standard Open Source Software Outward Licence.
// See the LICENSE and NOTICE files for the full licence text and the retained
// copyright and Attribution Notices.

package networkmanager

import (
	"fmt"
	"regexp"
	"strconv"
)

// Version is a NetworkManager version number.
type Version struct {
	Major, Minor, Micro int
}

var versionRE = regexp.MustCompile(`^(\d+)\.(\d+)(?:\.(\d+))?`)

// ParseVersion parses strings such as "1.46.0" or "1.54.3-1.fc42".
func ParseVersion(s string) (Version, error) {
	m := versionRE.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("unparsable NetworkManager version %q", s)
	}
	var v Version
	v.Major, _ = strconv.Atoi(m[1])
	v.Minor, _ = strconv.Atoi(m[2])
	if m[3] != "" {
		v.Micro, _ = strconv.Atoi(m[3])
	}
	return v, nil
}

// Compare returns -1, 0 or 1.
func (v Version) Compare(o Version) int {
	for _, d := range [][2]int{{v.Major, o.Major}, {v.Minor, o.Minor}, {v.Micro, o.Micro}} {
		if d[0] < d[1] {
			return -1
		}
		if d[0] > d[1] {
			return 1
		}
	}
	return 0
}

// AtLeast reports v >= o.
func (v Version) AtLeast(major, minor, micro int) bool {
	return v.Compare(Version{major, minor, micro}) >= 0
}

func (v Version) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Micro) }

// Supported reports whether this version exposes the
// org.freedesktop.NetworkManager.Settings API (0.9 and later).
func (v Version) Supported() bool { return v.AtLeast(0, 9, 0) }

// NeedsCAPath reports whether the CA must be passed as a hashed directory
// (ca-path) instead of a file. NetworkManager 1.54.3 up to (excluding)
// 1.58.1 has a bug handling multi-certificate ca-cert bundles.
func (v Version) NeedsCAPath() bool {
	return v.AtLeast(1, 54, 3) && !v.AtLeast(1, 58, 1)
}

// MatchKey returns the 802-1x setting used to pin the RADIUS server name.
func (v Version) MatchKey() string {
	switch {
	case v.AtLeast(1, 24, 0):
		return "domain-match"
	case v.AtLeast(0, 9, 0):
		return "altsubject-matches"
	default:
		return "subject-match"
	}
}
