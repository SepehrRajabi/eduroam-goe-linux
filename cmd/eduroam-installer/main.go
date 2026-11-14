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

// Command eduroam-installer configures eduroam for the University of
// Göttingen on Linux. It is a Go port of the eduroam CAT Python installer
// with pluggable network management backends and a terminal UI.
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SepehrRajabi/eduroam-goe-linux/internal/backend"
	_ "github.com/SepehrRajabi/eduroam-goe-linux/internal/backend/iwd"
	_ "github.com/SepehrRajabi/eduroam-goe-linux/internal/backend/networkmanager"
	_ "github.com/SepehrRajabi/eduroam-goe-linux/internal/backend/wpasupplicant"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/creds"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/profile"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/store"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/sysinfo"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/tui"
)

type options struct {
	debug     bool
	username  string
	password  string
	silent    bool
	pfxFile   string
	backendID string
	wpaConf   bool
	iwdConf   bool
	wired     bool
	list      bool
	about     bool
	version   bool
}

// version is set at release build time via
// -ldflags "-X main.version=<tag>".
var version = "dev"

const aboutText = `eduroam-goe-linux - eduroam installer for the University of Göttingen

A modified Go port of the eduroam CAT Linux installer
(https://github.com/GEANT/CAT), developed on behalf of the GÉANT project.

Original authors:
    Tomasz Wolniewicz
    Michał Gasewicz (Network Manager support)
Original contributors:
    Steffen Klemer, ikreb7, Dimitri Papadopoulos Orfanos, sdasda7777,
    Matt Jolly, Benoit LE TEXIER, jmontes

Go port and modifications: Sepehr Rajabi, 2026.
Not endorsed by GÉANT or the original authors.

Licensed under the GÉANT Standard Open Source Software Outward Licence.
See LICENSE and NOTICE at https://github.com/SepehrRajabi/eduroam-goe-linux
`

func parseFlags() options {
	var o options
	ids := make([]string, 0)
	for _, b := range backend.All() {
		ids = append(ids, b.Info().ID)
	}
	flag.BoolVar(&o.debug, "debug", false, "write debug output (to eduroam-installer-debug.log in TUI mode)")
	flag.BoolVar(&o.debug, "d", false, "shorthand for --debug")
	flag.StringVar(&o.username, "username", "", "set username; use together with --password and --silent")
	flag.StringVar(&o.username, "u", "", "shorthand for --username")
	flag.StringVar(&o.password, "password", "", "set the password; use together with --username and --silent")
	flag.StringVar(&o.password, "p", "", "shorthand for --password")
	flag.BoolVar(&o.silent, "silent", false, "non-interactive installation (no TUI)")
	flag.BoolVar(&o.silent, "s", false, "shorthand for --silent")
	flag.StringVar(&o.pfxFile, "pfxfile", "", "path to the user certificate file (EAP-TLS profiles)")
	flag.StringVar(&o.backendID, "backend", "", "network backend to use: "+strings.Join(ids, ", "))
	flag.BoolVar(&o.wpaConf, "wpa_conf", false, "same as --backend wpa_supplicant")
	flag.BoolVar(&o.iwdConf, "iwd_conf", false, "same as --backend iwd")
	flag.BoolVar(&o.wired, "wired", false, "silent mode: also configure wired 802.1X (if supported)")
	flag.BoolVar(&o.list, "list-backends", false, "probe and list backends, then exit")
	flag.BoolVar(&o.about, "about", false, "show credits and licence information, then exit")
	flag.BoolVar(&o.version, "version", false, "print the version, then exit")
	flag.Parse()
	switch {
	case o.wpaConf:
		o.backendID = "wpa_supplicant"
	case o.iwdConf:
		o.backendID = "iwd"
	}
	return o
}

func main() {
	o := parseFlags()
	if o.version {
		fmt.Println("eduroam-installer", version)
		return
	}
	if o.about {
		fmt.Printf("Version: %s\n\n", version)
		fmt.Print(aboutText)
		return
	}
	if err := run(o); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(o options) error {
	p := profile.Default
	st, err := store.Default()
	if err != nil {
		return err
	}

	if o.list {
		return listBackends(p)
	}

	var forced backend.Backend
	if o.backendID != "" {
		if forced, err = backend.Get(o.backendID); err != nil {
			return err
		}
		if err := forced.Supports(p); err != nil {
			return fmt.Errorf("%s: %w", o.backendID, err)
		}
	}

	if o.silent {
		logger := backend.DiscardLogger
		if o.debug {
			logger = log.New(os.Stderr, "DEBUG: ", 0)
		}
		return runSilent(o, p, st, forced, logger)
	}

	logger := backend.DiscardLogger
	if o.debug {
		f, err := tea.LogToFile("eduroam-installer-debug.log", "DEBUG: ")
		if err != nil {
			return err
		}
		defer f.Close()
		logger = log.Default()
	}
	m := tui.New(tui.Options{
		Profile:  p,
		Store:    st,
		Username: o.username,
		Password: o.password,
		PFXFile:  o.pfxFile,
		Backend:  forced,
		Log:      logger,
	})
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		return err
	}
	out := m.Outcome()
	switch {
	case out.Aborted:
		os.Exit(1)
	case out.Err != nil:
		return out.Err
	case out.Installed:
		fmt.Println(profile.MsgInstallFinished)
		if out.Result != nil {
			for _, n := range out.Result.Notes {
				fmt.Println("  " + n)
			}
		}
	}
	return nil
}

func listBackends(p *profile.Profile) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, pr := range backend.ProbeAll(ctx, p) {
		state := "usable"
		switch {
		case pr.Unsupported != nil:
			state = "unsupported: " + pr.Unsupported.Error()
		case !pr.Status.Available:
			state = "unavailable"
		}
		info := pr.Backend.Info()
		fmt.Printf("%-16s %-12s %s\n", info.ID, state, pr.Status.Detail)
	}
	return nil
}

// runSilent performs a non-interactive installation, like the Python
// installer's --silent mode.
func runSilent(o options, p *profile.Profile, st *store.Store, b backend.Backend, logger *log.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if b == nil {
		probes := backend.ProbeAll(ctx, p)
		for _, pr := range probes {
			logger.Printf("backend %s: available=%v %s", pr.Backend.Info().ID, pr.Status.Available, pr.Status.Detail)
		}
		var ok bool
		if b, ok = backend.FirstUsable(probes); !ok {
			return errors.New("no usable network backend found")
		}
	}
	logger.Printf("using backend %s", b.Info().ID)

	c := creds.Credentials{Username: o.username, Password: o.password}
	switch {
	case p.UsesPassword():
		if c.Username == "" || c.Password == "" {
			return errors.New("--silent requires --username and --password")
		}
		if err := creds.ValidateUsername(p, c.Username); err != nil {
			return err
		}
	case p.UsesCertificate():
		var err error
		if p.EAPInner == profile.InnerSilverBullet {
			c.P12, err = base64.StdEncoding.DecodeString(p.SBUserFile)
		} else {
			if o.pfxFile == "" {
				return errors.New("--silent requires --pfxfile for this profile")
			}
			c.P12, err = os.ReadFile(o.pfxFile)
			if err != nil {
				err = fmt.Errorf("%s: %w", profile.MsgUserCertMissing, err)
			}
		}
		if err != nil {
			return err
		}
		id, err := creds.P12Identity(c.P12, c.Password)
		if err != nil {
			return err
		}
		if c.Username == "" && !p.UseOtherTLSID {
			c.Username = id
		}
	}

	wired := o.wired && b.Info().SupportsWired && sysinfo.HasWiredInterface()
	if err := st.Ensure(); err != nil {
		return err
	}
	res, err := b.Install(ctx, backend.Request{
		Profile: p, Creds: c, Store: st, Wired: wired, Log: logger,
	})
	if err != nil {
		return err
	}
	fmt.Println(profile.MsgInstallFinished)
	for _, n := range res.Notes {
		fmt.Println("  " + n)
	}
	return nil
}
