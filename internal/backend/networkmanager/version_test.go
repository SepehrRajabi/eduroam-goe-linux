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

import "testing"

func TestVersion(t *testing.T) {
	cases := []struct {
		in        string
		capath    bool
		match     string
		supported bool
	}{
		{"0.8.4", false, "subject-match", false},
		{"0.9.10", false, "altsubject-matches", true},
		{"1.22.0", false, "altsubject-matches", true},
		{"1.46.0", false, "domain-match", true},
		{"1.54.2", false, "domain-match", true},
		{"1.54.3", true, "domain-match", true},
		{"1.56.0-1.fc43", true, "domain-match", true},
		{"1.58.0", true, "domain-match", true},
		{"1.58.1", false, "domain-match", true},
		{"1.60.0", false, "domain-match", true},
	}
	for _, c := range cases {
		v, err := ParseVersion(c.in)
		if err != nil {
			t.Fatal(err)
		}
		if v.NeedsCAPath() != c.capath || v.MatchKey() != c.match || v.Supported() != c.supported {
			t.Errorf("%s: capath=%v match=%s supported=%v", c.in, v.NeedsCAPath(), v.MatchKey(), v.Supported())
		}
	}
}
