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

// Package creds holds user credentials and the validation logic that the
// CAT installer applies to them.
package creds

import (
	"errors"
	"fmt"
	"strings"

	"github.com/SepehrRajabi/eduroam-goe-linux/internal/profile"
)

// Credentials are what the user supplies to authenticate.
type Credentials struct {
	Username string
	// Password is the EAP password (PEAP/TTLS/PWD) or the PKCS#12 import
	// password (TLS).
	Password string
	// P12 holds the raw PKCS#12 client certificate bundle (TLS only).
	P12 []byte
}

// ValidateUsername mirrors InstallerData.__validate_user_name from the
// Python installer. It returns a user-presentable error on failure.
func ValidateUsername(p *profile.Profile, username string) error {
	wrongFormat := errors.New(profile.MsgWrongUsernameFormat)
	pos := strings.IndexByte(username, '@')

	// Trailing '@' (this also rejects the empty string, as in the original).
	if pos == len(username)-1 {
		return wrongFormat
	}
	// No '@' at all.
	if pos == -1 {
		if p.VerifyUserRealmInput {
			return wrongFormat
		}
		return nil
	}
	// '@' at the beginning.
	if pos == 0 {
		return wrongFormat
	}
	realm := username[pos+1:]
	if p.VerifyUserRealmInput {
		if p.HintUserInput {
			if realm == p.UserRealm {
				return nil
			}
			return fmt.Errorf(profile.MsgWrongRealm, p.UserRealm)
		}
		if strings.HasSuffix(realm, p.UserRealm) {
			return nil
		}
		return fmt.Errorf(profile.MsgWrongRealmSuffix, p.UserRealm)
	}
	if strings.Contains(realm, "@") {
		return wrongFormat
	}
	if strings.HasPrefix(realm, ".") {
		return wrongFormat
	}
	return nil
}

// ValidatePasswords checks that a password was entered and confirmed.
func ValidatePasswords(password, repeat string) error {
	if password == "" || repeat == "" {
		return errors.New(profile.MsgEmptyField)
	}
	if password != repeat {
		return errors.New(profile.MsgPasswordsDiffer)
	}
	return nil
}

// UsernameHint returns the initial value for the username field.
func UsernameHint(p *profile.Profile, given string) string {
	if given != "" {
		return given
	}
	if p.HintUserInput {
		return "@" + p.UserRealm
	}
	return ""
}
