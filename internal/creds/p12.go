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
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// ErrIncorrectPassword is returned when a PKCS#12 bundle cannot be
// decrypted with the supplied password.
var ErrIncorrectPassword = errors.New("incorrect password")

var oidEmailAddress = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 1}

// P12Identity decrypts a PKCS#12 bundle and returns the identity found in
// the client certificate (CN, falling back to the e-mail address).
//
// Pure Go decoding is tried first; if the bundle uses an algorithm the Go
// library does not understand, the openssl CLI is used as a fallback
// (including its -legacy mode), like the Python installer does.
func P12Identity(data []byte, password string) (string, error) {
	_, cert, _, err := pkcs12.DecodeChain(data, password)
	if err == nil {
		return identityFromCert(cert), nil
	}
	if errors.Is(err, pkcs12.ErrIncorrectPassword) {
		return "", ErrIncorrectPassword
	}
	cert, oerr := p12CertViaOpenSSL(data, password)
	if oerr != nil {
		return "", fmt.Errorf("%w (go: %v; openssl: %v)", ErrIncorrectPassword, err, oerr)
	}
	return identityFromCert(cert), nil
}

func identityFromCert(cert *x509.Certificate) string {
	if cn := cert.Subject.CommonName; cn != "" {
		return cn
	}
	for _, n := range cert.Subject.Names {
		if n.Type.Equal(oidEmailAddress) {
			if s, ok := n.Value.(string); ok && s != "" {
				return s
			}
		}
	}
	if len(cert.EmailAddresses) > 0 {
		return cert.EmailAddresses[0]
	}
	return ""
}

func p12CertViaOpenSSL(data []byte, password string) (*x509.Certificate, error) {
	if _, err := exec.LookPath("openssl"); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp("", "eduroam-*.p12")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return nil, err
	}
	tmp.Close()

	var lastErr error
	for _, extra := range [][]string{nil, {"-legacy"}} {
		args := append([]string{"pkcs12"}, extra...)
		args = append(args, "-in", tmp.Name(), "-passin", "env:EDUROAM_P12_PASS", "-nokeys", "-clcerts")
		cmd := exec.Command("openssl", args...)
		cmd.Env = append(os.Environ(), "EDUROAM_P12_PASS="+password)
		out, err := cmd.Output()
		if err != nil {
			lastErr = err
			continue
		}
		for rest := out; ; {
			var block *pem.Block
			block, rest = pem.Decode(rest)
			if block == nil {
				break
			}
			if block.Type == "CERTIFICATE" {
				return x509.ParseCertificate(block.Bytes)
			}
		}
		lastErr = errors.New("no certificate in openssl output")
	}
	return nil, lastErr
}

// FindP12InDir returns the only *.p12/*.pfx file in dir, or "" if there is
// not exactly one.
func FindP12InDir(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	found := ""
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".p12" || ext == ".pfx" {
			n++
			found = e.Name()
		}
	}
	if n == 1 {
		return found
	}
	return ""
}
