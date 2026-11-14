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

// Package store manages the installer's working directory
// ($XDG_CONFIG_HOME/cat_installer), where CA certificates, the user's
// PKCS#12 bundle and generated configuration files are kept.
package store

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Store is the cat_installer directory.
type Store struct {
	Dir string
}

// ConfigHome returns $XDG_CONFIG_HOME, or $HOME/.config.
func ConfigHome() (string, error) {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}

// Default returns the store located in the user's config directory.
func Default() (*Store, error) {
	cfg, err := ConfigHome()
	if err != nil {
		return nil, err
	}
	return &Store{Dir: filepath.Join(cfg, "cat_installer")}, nil
}

// Exists reports whether the directory already exists.
func (s *Store) Exists() bool {
	fi, err := os.Stat(s.Dir)
	return err == nil && fi.IsDir()
}

// Ensure creates the directory structure with restrictive permissions.
func (s *Store) Ensure() error {
	if err := os.MkdirAll(s.CADir(), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", s.Dir, err)
	}
	return nil
}

// Path joins name onto the store directory.
func (s *Store) Path(name string) string { return filepath.Join(s.Dir, name) }

// CAFile is the path of the single-file CA bundle.
func (s *Store) CAFile() string { return s.Path("ca.pem") }

// CADir is the path of the hashed per-certificate CA directory.
func (s *Store) CADir() string { return s.Path("ca") }

// UserP12 is the path where the user's client certificate is stored.
func (s *Store) UserP12() string { return s.Path("user.p12") }

// WriteFile writes a file into the store with the given permissions.
func (s *Store) WriteFile(name string, data []byte, perm os.FileMode) (string, error) {
	if err := s.Ensure(); err != nil {
		return "", err
	}
	p := s.Path(name)
	if err := os.WriteFile(p, data, perm); err != nil {
		return "", err
	}
	// WriteFile does not change permissions of an existing file.
	return p, os.Chmod(p, perm)
}

// SaveUserP12 stores the client certificate bundle.
func (s *Store) SaveUserP12(data []byte) (string, error) {
	return s.WriteFile("user.p12", data, 0o600)
}

// CALocation describes where the CA certificates were saved.
type CALocation struct {
	Path  string
	IsDir bool // true when Path is an OpenSSL hashed directory (ca-path)
}

// SaveCA writes the CA certificates. When asDir is true the certificates are
// written one per file into CADir and hashed with `openssl rehash`; if that
// does not produce a usable hashed directory it falls back to the single
// CAFile (the same behaviour as the Python installer's save_ca).
func (s *Store) SaveCA(certs []string, asDir bool) (CALocation, error) {
	if err := s.Ensure(); err != nil {
		return CALocation{}, err
	}
	if asDir && len(certs) > 1 {
		if ok, err := s.saveHashedCADir(certs); err != nil {
			return CALocation{}, err
		} else if ok {
			return CALocation{Path: s.CADir(), IsDir: true}, nil
		}
	}
	bundle := strings.Join(certs, "\n") + "\n"
	if _, err := s.WriteFile("ca.pem", []byte(bundle), 0o644); err != nil {
		return CALocation{}, err
	}
	return CALocation{Path: s.CAFile()}, nil
}

func (s *Store) saveHashedCADir(certs []string) (bool, error) {
	dir := s.CADir()
	old, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, e := range old {
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			return false, err
		}
	}
	for i, c := range certs {
		p := filepath.Join(dir, fmt.Sprintf("ca-%d.pem", i))
		if err := os.WriteFile(p, []byte(c+"\n"), 0o644); err != nil {
			return false, err
		}
	}
	if err := exec.Command("openssl", "rehash", dir).Run(); err != nil {
		return false, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 {
			return true, nil
		}
	}
	return false, nil
}
