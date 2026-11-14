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

// Package iwd implements a backend that writes network provisioning files
// for the iNet wireless daemon (iwd).
package iwd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/SepehrRajabi/eduroam-goe-linux/internal/backend"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/profile"
)

// StateDir is where iwd reads network provisioning files from.
const StateDir = "/var/lib/iwd"

// Backend is the iwd backend.
type Backend struct{}

func init() { backend.Register(Backend{}) }

// Info implements backend.Backend.
func (Backend) Info() backend.Info {
	return backend.Info{
		ID:               "iwd",
		Name:             "iwd",
		Description:      "Write provisioning files to " + StateDir,
		Priority:         20,
		PlaintextSecrets: true,
	}
}

// Probe implements backend.Backend.
func (Backend) Probe(context.Context) backend.Status {
	if fi, err := os.Stat(StateDir); err == nil && fi.IsDir() {
		return backend.Status{Available: true, Detail: StateDir + " found"}
	}
	if _, err := exec.LookPath("iwctl"); err == nil {
		return backend.Status{Available: true, Detail: "iwctl found"}
	}
	return backend.Status{Detail: "iwd does not seem to be installed"}
}

// Supports implements backend.Backend.
func (Backend) Supports(p *profile.Profile) error {
	switch p.EAPOuter {
	case profile.EAPPWD, profile.EAPPEAP, profile.EAPTTLS:
		return nil
	}
	return fmt.Errorf("EAP method %s is not supported by this backend", p.EAPOuter)
}

// Install implements backend.Backend.
func (b Backend) Install(ctx context.Context, req backend.Request) (*backend.Result, error) {
	res := &backend.Result{}
	for _, ssid := range req.Profile.SSIDs {
		cfg, err := Config(req)
		if err != nil {
			return nil, err
		}
		name := ssid + ".8021x"
		target := filepath.Join(StateDir, name)
		if err := writePrivileged(ctx, target, cfg); err != nil {
			req.Log.Printf("writing %s failed: %v", target, err)
			// Leave a copy in the installer directory for manual install.
			local, lerr := req.Store.WriteFile(name, []byte(cfg), 0o600)
			if lerr != nil {
				return nil, errors.Join(err, lerr)
			}
			res.Notes = append(res.Notes,
				fmt.Sprintf("Could not write %s (%v).", target, err),
				fmt.Sprintf("The configuration was saved to %s - copy it to %s as root.", local, StateDir))
			continue
		}
		res.Notes = append(res.Notes, "Wrote "+target)
	}
	return res, nil
}

// writePrivileged writes the file directly, falling back to pkexec when
// permissions are insufficient (as the Python installer does).
func writePrivileged(ctx context.Context, path, content string) error {
	err := os.WriteFile(path, []byte(content), 0o600)
	if err == nil || !errors.Is(err, os.ErrPermission) {
		return err
	}
	if _, lerr := exec.LookPath("pkexec"); lerr != nil {
		return err
	}
	// install(1) creates the file with the right mode; content via stdin.
	cmd := exec.CommandContext(ctx, "pkexec", "install", "-m", "600", "/dev/stdin", path)
	cmd.Stdin = bytes.NewBufferString(content)
	if out, perr := cmd.CombinedOutput(); perr != nil {
		return fmt.Errorf("pkexec: %v: %s", perr, strings.TrimSpace(string(out)))
	}
	return nil
}

// phase2Method maps CAT inner methods to iwd names.
func phase2Method(outer, inner string) string {
	switch strings.ToUpper(inner) {
	case "PAP":
		return "Tunneled-PAP"
	case "CHAP":
		return "Tunneled-CHAP"
	case "MSCHAP":
		return "Tunneled-MSCHAP"
	case "MSCHAPV2":
		if outer == profile.EAPTTLS {
			return "Tunneled-MSCHAPv2"
		}
		return "MSCHAPV2"
	case "GTC":
		return "GTC"
	}
	return inner
}

// Config renders the iwd provisioning file for the profile.
func Config(req backend.Request) (string, error) {
	p := req.Profile
	u, pw := req.Creds.Username, req.Creds.Password
	var sb strings.Builder
	sb.WriteString("[Security]\n")
	switch p.EAPOuter {
	case profile.EAPPWD:
		fmt.Fprintf(&sb, "EAP-Method=PWD\nEAP-Identity=%s\nEAP-Password=%s\n", u, pw)
	case profile.EAPPEAP, profile.EAPTTLS:
		m := p.EAPOuter
		fmt.Fprintf(&sb, "EAP-Method=%s\n", m)
		fmt.Fprintf(&sb, "EAP-Identity=%s\n", p.OuterIdentity(u))
		fmt.Fprintf(&sb, "EAP-%s-CACert=embed:eduroam_ca_cert\n", m)
		fmt.Fprintf(&sb, "EAP-%s-ServerDomainMask=%s\n", m, p.ServersCN)
		fmt.Fprintf(&sb, "EAP-%s-Phase2-Method=%s\n", m, phase2Method(m, p.EAPInner))
		fmt.Fprintf(&sb, "EAP-%s-Phase2-Identity=%s\n", m, u)
		fmt.Fprintf(&sb, "EAP-%s-Phase2-Password=%s\n", m, pw)
	default:
		return "", fmt.Errorf("invalid connection type %s", p.EAPOuter)
	}
	sb.WriteString("\n[Settings]\nAutoConnect=true\n")
	if p.EAPOuter != profile.EAPPWD {
		sb.WriteString("\n[@pem@eduroam_ca_cert]\n")
		sb.WriteString(strings.Join(p.CACertificates(), "\n"))
		sb.WriteString("\n")
	}
	return sb.String(), nil
}
