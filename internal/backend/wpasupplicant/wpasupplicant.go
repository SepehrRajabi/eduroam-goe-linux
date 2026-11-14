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

// Package wpasupplicant implements a backend that generates a
// wpa_supplicant configuration file instead of configuring the system.
package wpasupplicant

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/SepehrRajabi/eduroam-goe-linux/internal/backend"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/profile"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/store"
)

// FileName is the name of the generated configuration file.
const FileName = "cat_installer.conf"

// Backend is the wpa_supplicant config-file backend.
type Backend struct{}

func init() { backend.Register(Backend{}) }

// Info implements backend.Backend.
func (Backend) Info() backend.Info {
	return backend.Info{
		ID:               "wpa_supplicant",
		Name:             "wpa_supplicant",
		Description:      "Generate a wpa_supplicant configuration file",
		Priority:         30,
		PlaintextSecrets: true,
	}
}

// Probe implements backend.Backend. Generating a file always works; the
// detail notes whether wpa_supplicant is installed.
func (Backend) Probe(context.Context) backend.Status {
	if _, err := exec.LookPath("wpa_supplicant"); err != nil {
		return backend.Status{Available: true, Detail: "file only (wpa_supplicant not found in PATH)"}
	}
	return backend.Status{Available: true, Detail: "file only"}
}

// Supports implements backend.Backend.
func (Backend) Supports(p *profile.Profile) error {
	switch p.EAPOuter {
	case profile.EAPPEAP, profile.EAPTTLS, profile.EAPTLS, profile.EAPPWD:
		return nil
	}
	return fmt.Errorf("EAP method %s is not supported", p.EAPOuter)
}

// Install implements backend.Backend.
func (b Backend) Install(_ context.Context, req backend.Request) (*backend.Result, error) {
	ca, err := req.Store.SaveCA(req.Profile.CACertificates(), false)
	if err != nil {
		return nil, fmt.Errorf("save CA certificate: %w", err)
	}
	var p12 string
	if req.Profile.UsesCertificate() {
		if p12, err = req.Store.SaveUserP12(req.Creds.P12); err != nil {
			return nil, fmt.Errorf("save client certificate: %w", err)
		}
	}
	var sb strings.Builder
	for _, ssid := range req.Profile.SSIDs {
		sb.WriteString(NetworkBlock(req, ca, p12, ssid))
		sb.WriteString("\n")
	}
	path, err := req.Store.WriteFile(FileName, []byte(sb.String()), 0o600)
	if err != nil {
		return nil, err
	}
	return &backend.Result{Notes: []string{
		"Configuration written to " + path,
		"Include it in your wpa_supplicant setup, e.g. wpa_supplicant -i <iface> -c " + path,
	}}, nil
}

// quote produces a wpa_supplicant quoted string.
func quote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// NetworkBlock renders one network={...} block.
func NetworkBlock(req backend.Request, ca store.CALocation, p12Path, ssid string) string {
	p := req.Profile
	lines := []string{
		"ssid=" + quote(ssid),
		"key_mgmt=WPA-EAP",
		"pairwise=CCMP",
		"group=CCMP",
		"eap=" + p.EAPOuter,
		"ca_cert=" + quote(ca.Path),
		"identity=" + quote(req.Creds.Username),
		"altsubject_match=" + quote(strings.Join(p.Servers, ";")),
	}
	switch p.EAPOuter {
	case profile.EAPPEAP, profile.EAPTTLS:
		lines = append(lines,
			"phase2="+quote("auth="+p.EAPInner),
			"password="+quote(req.Creds.Password),
			"anonymous_identity="+quote(p.OuterIdentity(req.Creds.Username)),
		)
	case profile.EAPPWD:
		lines = append(lines, "password="+quote(req.Creds.Password))
	case profile.EAPTLS:
		lines = append(lines,
			"private_key="+quote(p12Path),
			"private_key_passwd="+quote(req.Creds.Password),
		)
	}
	return "network={\n\t" + strings.Join(lines, "\n\t") + "\n}\n"
}
