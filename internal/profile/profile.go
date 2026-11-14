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

// Package profile holds the institution-specific eduroam configuration
// (the equivalent of the CAT installer's Config class) and the
// user-facing message strings (the Messages class).
package profile

import (
	_ "embed"
	"fmt"
	"strings"
)

// EAP outer methods.
const (
	EAPPEAP = "PEAP"
	EAPTTLS = "TTLS"
	EAPTLS  = "TLS"
	EAPPWD  = "PWD"
)

// InnerSilverBullet is the inner method marker for managed (SilverBullet)
// client certificates shipped inside the installer.
const InnerSilverBullet = "SILVERBULLET"

// Profile describes one eduroam CAT profile.
type Profile struct {
	InstName    string
	ProfileName string
	URL         string
	Email       string
	Title       string

	// Servers are the RADIUS server names in "DNS:host" form.
	Servers []string
	// ServersCN is the ';'-separated list of plain server names used for
	// domain-suffix style matching.
	ServersCN string
	// ServerMatch is the legacy subject-match value.
	ServerMatch string

	SSIDs    []string
	DelSSIDs []string

	EAPOuter          string
	EAPInner          string
	UseOtherTLSID     bool
	AnonymousIdentity string

	// CA is the PEM bundle of trusted CA certificates.
	CA string

	InitInfo         string
	InitConfirmation string
	TermsOfUse       string
	// Wired, when non-empty, enables the "also configure wired" question.
	Wired string

	// SBUserFile is a base64-encoded PKCS#12 bundle for SilverBullet.
	SBUserFile string

	VerifyUserRealmInput bool
	UserRealm            string
	HintUserInput        bool
}

// UsesPassword reports whether the profile authenticates with username and
// password (as opposed to a client certificate).
func (p *Profile) UsesPassword() bool {
	switch p.EAPOuter {
	case EAPPEAP, EAPTTLS, EAPPWD:
		return true
	}
	return false
}

// UsesCertificate reports whether the profile authenticates with a client
// certificate.
func (p *Profile) UsesCertificate() bool { return p.EAPOuter == EAPTLS }

// OuterIdentity returns the anonymous identity if configured, else username.
func (p *Profile) OuterIdentity(username string) string {
	if p.AnonymousIdentity != "" {
		return p.AnonymousIdentity
	}
	return username
}

// InitInfoText renders the introductory text.
func (p *Profile) InitInfoText() string {
	return fmtPositional(p.InitInfo, p.InstName, p.Email, p.URL)
}

// InitConfirmationText renders the membership confirmation text.
func (p *Profile) InitConfirmationText() string {
	return fmtPositional(p.InitConfirmation, p.InstName, p.ProfileName)
}

// CACertificates splits the CA bundle into individual PEM blocks.
func (p *Profile) CACertificates() []string {
	const begin, end = "-----BEGIN CERTIFICATE-----", "-----END CERTIFICATE-----"
	var out []string
	rest := p.CA
	for {
		i := strings.Index(rest, begin)
		if i < 0 {
			return out
		}
		j := strings.Index(rest[i:], end)
		if j < 0 {
			return out
		}
		out = append(out, rest[i:i+j+len(end)])
		rest = rest[i+j+len(end):]
	}
}

// fmtPositional replaces Python-style {0}, {1}, ... placeholders.
func fmtPositional(s string, args ...string) string {
	for i, a := range args {
		s = strings.ReplaceAll(s, fmt.Sprintf("{%d}", i), a)
	}
	return s
}

//go:embed ca.pem
var goettingenCA string

// Goettingen is the profile for University of Göttingen.
var Goettingen = Profile{
	InstName:    "University Goettingen",
	ProfileName: "University of Göttingen",
	URL:         "https://wiki.student.uni-goettingen.de/support/wlan",
	Email:       "campussupport@uni-goettingen.de",
	Title:       "eduroam CAT",
	ServerMatch: "eduroam.gwdg.de",
	EAPOuter:    EAPPEAP,
	EAPInner:    "MSCHAPV2",
	InitInfo: "This installer has been prepared for {0}\n\nMore information and comments:\n\n" +
		"EMAIL: {1}\nWWW: {2}\n\nInstaller created with software from the GEANT project.",
	InitConfirmation:  "This installer will only work properly if you are a member of {0} and the user group: {1}.",
	UserRealm:         "uni-goettingen.de",
	SSIDs:             []string{"eduroam"},
	Servers:           []string{"DNS:eduroam.gwdg.de"},
	ServersCN:         "eduroam.gwdg.de",
	AnonymousIdentity: "wifi@uni-goettingen.de",
	CA:                strings.TrimSpace(goettingenCA),
}

// Default is the profile used by the installer.
var Default = &Goettingen
