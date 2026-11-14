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

package creds

import (
	"testing"

	"github.com/SepehrRajabi/eduroam-goe-linux/internal/profile"
)

func TestValidateUsername(t *testing.T) {
	plain := &profile.Profile{UserRealm: "uni-goettingen.de"}
	strict := &profile.Profile{UserRealm: "uni-goettingen.de", VerifyUserRealmInput: true}
	hinted := &profile.Profile{UserRealm: "uni-goettingen.de", VerifyUserRealmInput: true, HintUserInput: true}

	cases := []struct {
		p    *profile.Profile
		user string
		ok   bool
	}{
		{plain, "", false},
		{plain, "max", true},
		{plain, "max@", false},
		{plain, "@uni-goettingen.de", false},
		{plain, "max@uni-goettingen.de", true},
		{plain, "max@a@b", false},
		{plain, "max@.de", false},
		{strict, "max", false},
		{strict, "max@stud.uni-goettingen.de", true},
		{strict, "max@example.org", false},
		{hinted, "max@uni-goettingen.de", true},
		{hinted, "max@stud.uni-goettingen.de", false},
	}
	for _, c := range cases {
		err := ValidateUsername(c.p, c.user)
		if (err == nil) != c.ok {
			t.Errorf("ValidateUsername(%+v, %q) = %v, want ok=%v", *c.p, c.user, err, c.ok)
		}
	}
}

func TestValidatePasswords(t *testing.T) {
	if ValidatePasswords("a", "a") != nil {
		t.Error("equal passwords rejected")
	}
	if ValidatePasswords("a", "b") == nil {
		t.Error("different passwords accepted")
	}
	if ValidatePasswords("", "") == nil {
		t.Error("empty passwords accepted")
	}
}
