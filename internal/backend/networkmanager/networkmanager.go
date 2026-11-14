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

// Package networkmanager implements the backend for NetworkManager, talking
// to it over the system D-Bus.
package networkmanager

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/godbus/dbus/v5"

	"github.com/SepehrRajabi/eduroam-goe-linux/internal/backend"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/profile"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/store"
)

const (
	busName       = "org.freedesktop.NetworkManager"
	nmPath        = "/org/freedesktop/NetworkManager"
	settingsPath  = "/org/freedesktop/NetworkManager/Settings"
	settingsIface = "org.freedesktop.NetworkManager.Settings"
	connIface     = "org.freedesktop.NetworkManager.Settings.Connection"

	wiredConnID = "wired-eduroam"
)

// Secret flags (NMSettingSecretFlags).
const (
	SecretFlagNone       uint32 = 0 // stored by NetworkManager (system-wide)
	SecretFlagAgentOwned uint32 = 1 // stored by the user's secret agent
)

// Backend is the NetworkManager backend.
type Backend struct {
	// PasswordFlags controls where NetworkManager keeps the secret.
	// The CAT installer uses SecretFlagAgentOwned.
	PasswordFlags uint32
}

// New returns a backend with the CAT installer's defaults.
func New() *Backend { return &Backend{PasswordFlags: SecretFlagAgentOwned} }

func init() { backend.Register(New()) }

// Info implements backend.Backend.
func (b *Backend) Info() backend.Info {
	return backend.Info{
		ID:            "networkmanager",
		Name:          "NetworkManager",
		Description:   "Add connections to NetworkManager via D-Bus",
		Priority:      10,
		SupportsWired: true,
	}
}

// Supports implements backend.Backend.
func (b *Backend) Supports(p *profile.Profile) error {
	switch p.EAPOuter {
	case profile.EAPPEAP, profile.EAPTTLS, profile.EAPTLS, profile.EAPPWD:
		return nil
	}
	return fmt.Errorf("EAP method %s is not supported", p.EAPOuter)
}

// Probe implements backend.Backend.
func (b *Backend) Probe(ctx context.Context) backend.Status {
	c, err := connect(ctx)
	if err != nil {
		return backend.Status{Detail: err.Error()}
	}
	if !c.version.Supported() {
		return backend.Status{Detail: fmt.Sprintf("%s (%s)", profile.MsgNMNotSupported, c.versionRaw)}
	}
	return backend.Status{Available: true, Detail: "version " + c.versionRaw}
}

// client is a live connection to NetworkManager.
type client struct {
	bus        *dbus.Conn
	version    Version
	versionRaw string
}

func connect(ctx context.Context) (*client, error) {
	bus, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("cannot connect to the system D-Bus: %w", err)
	}
	var v dbus.Variant
	err = bus.Object(busName, nmPath).
		CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, busName, "Version").
		Store(&v)
	if err != nil {
		return nil, fmt.Errorf("NetworkManager is not reachable: %w", err)
	}
	raw, _ := v.Value().(string)
	ver, err := ParseVersion(raw)
	if err != nil {
		return nil, err
	}
	return &client{bus: bus, version: ver, versionRaw: raw}, nil
}

func (c *client) settings() dbus.BusObject { return c.bus.Object(busName, settingsPath) }

type connSettings = map[string]map[string]dbus.Variant

// deleteMatching removes every stored connection for which match returns true.
func (c *client) deleteMatching(ctx context.Context, match func(connSettings) bool, log func(string)) error {
	var paths []dbus.ObjectPath
	if err := c.settings().CallWithContext(ctx, settingsIface+".ListConnections", 0).Store(&paths); err != nil {
		return fmt.Errorf("%s: %w", profile.MsgDBusError, err)
	}
	for _, p := range paths {
		obj := c.bus.Object(busName, p)
		var s connSettings
		if err := obj.CallWithContext(ctx, connIface+".GetSettings", 0).Store(&s); err != nil {
			continue // connection may have vanished or be inaccessible
		}
		if !match(s) {
			continue
		}
		log(fmt.Sprintf("deleting connection %s (%s)", str(s, "connection", "id"), p))
		if err := obj.CallWithContext(ctx, connIface+".Delete", 0).Err; err != nil {
			return fmt.Errorf("delete connection %s: %w", p, err)
		}
	}
	return nil
}

func (c *client) addConnection(ctx context.Context, s connSettings) (dbus.ObjectPath, error) {
	var path dbus.ObjectPath
	err := c.settings().CallWithContext(ctx, settingsIface+".AddConnection", 0, s).Store(&path)
	return path, err
}

// Install implements backend.Backend.
func (b *Backend) Install(ctx context.Context, req backend.Request) (*backend.Result, error) {
	logf := func(s string) { req.Log.Print(s) }
	c, err := connect(ctx)
	if err != nil {
		return nil, err
	}
	if !c.version.Supported() {
		return nil, errors.New(profile.MsgNMNotSupported)
	}
	logf("NetworkManager version " + c.versionRaw)

	ca, err := req.Store.SaveCA(req.Profile.CACertificates(), c.version.NeedsCAPath())
	if err != nil {
		return nil, fmt.Errorf("save CA certificate: %w", err)
	}
	if _, err := os.Stat(ca.Path); err != nil {
		return nil, fmt.Errorf("%s: %w", profile.MsgCertError, err)
	}

	var p12Path string
	if req.Profile.UsesCertificate() {
		if p12Path, err = req.Store.SaveUserP12(req.Creds.P12); err != nil {
			return nil, fmt.Errorf("save client certificate: %w", err)
		}
	}

	res := &backend.Result{}
	for _, ssid := range req.Profile.SSIDs {
		if err := c.deleteMatching(ctx, wifiSSIDMatcher(ssid), logf); err != nil {
			return nil, err
		}
		s := b.wifiSettings(req, c.version, ca, p12Path, ssid)
		path, err := c.addConnection(ctx, s)
		if err != nil {
			return nil, fmt.Errorf("add connection %q: %w", ssid, err)
		}
		logf(fmt.Sprintf("added %s as %s", ssid, path))
		res.Notes = append(res.Notes, fmt.Sprintf("Added Wi-Fi connection %q", ssid))
	}
	for _, ssid := range req.Profile.DelSSIDs {
		if err := c.deleteMatching(ctx, wifiSSIDMatcher(ssid), logf); err != nil {
			return nil, err
		}
	}
	if req.Wired {
		isWired := func(s connSettings) bool {
			return str(s, "connection", "type") == "802-3-ethernet" && str(s, "connection", "id") == wiredConnID
		}
		if err := c.deleteMatching(ctx, isWired, logf); err != nil {
			return nil, err
		}
		if _, err := c.addConnection(ctx, b.wiredSettings(req, c.version, ca, p12Path)); err != nil {
			return nil, fmt.Errorf("add wired connection: %w", err)
		}
		res.Notes = append(res.Notes, fmt.Sprintf("Added wired connection %q", wiredConnID))
	}
	return res, nil
}

func wifiSSIDMatcher(ssid string) func(connSettings) bool {
	return func(s connSettings) bool {
		if str(s, "connection", "type") != "802-11-wireless" {
			return false
		}
		w, ok := s["802-11-wireless"]["ssid"]
		if !ok {
			return false
		}
		b, _ := w.Value().([]byte)
		return string(b) == ssid
	}
}

func str(s connSettings, group, key string) string {
	v, ok := s[group][key]
	if !ok {
		return ""
	}
	out, _ := v.Value().(string)
	return out
}

func fileURI(path string) []byte { return []byte("file://" + path + "\x00") }

// security8021x builds the "802-1x" setting group.
func (b *Backend) security8021x(req backend.Request, v Version, ca store.CALocation, p12Path string) map[string]dbus.Variant {
	p := req.Profile
	s := map[string]dbus.Variant{
		"eap":      dbus.MakeVariant([]string{strings.ToLower(p.EAPOuter)}),
		"identity": dbus.MakeVariant(req.Creds.Username),
	}
	if ca.IsDir {
		s["ca-path"] = dbus.MakeVariant(ca.Path)
	} else {
		s["ca-cert"] = dbus.MakeVariant(fileURI(ca.Path))
	}
	switch v.MatchKey() {
	case "domain-match":
		s["domain-match"] = dbus.MakeVariant(p.ServersCN)
	case "altsubject-matches":
		s["altsubject-matches"] = dbus.MakeVariant(p.Servers)
	default:
		s["subject-match"] = dbus.MakeVariant(p.ServerMatch)
	}

	switch p.EAPOuter {
	case profile.EAPPEAP, profile.EAPTTLS:
		s["password"] = dbus.MakeVariant(req.Creds.Password)
		s["password-flags"] = dbus.MakeVariant(b.PasswordFlags)
		s["phase2-auth"] = dbus.MakeVariant(strings.ToLower(p.EAPInner))
		if p.AnonymousIdentity != "" {
			s["anonymous-identity"] = dbus.MakeVariant(p.AnonymousIdentity)
		}
	case profile.EAPPWD:
		s["password"] = dbus.MakeVariant(req.Creds.Password)
		s["password-flags"] = dbus.MakeVariant(b.PasswordFlags)
	case profile.EAPTLS:
		s["client-cert"] = dbus.MakeVariant(fileURI(p12Path))
		s["private-key"] = dbus.MakeVariant(fileURI(p12Path))
		s["private-key-password"] = dbus.MakeVariant(req.Creds.Password)
		s["private-key-password-flags"] = dbus.MakeVariant(b.PasswordFlags)
	}
	return s
}

func ipAuto() map[string]dbus.Variant {
	return map[string]dbus.Variant{"method": dbus.MakeVariant("auto")}
}

func permissions() []string {
	name := os.Getenv("USER")
	if name == "" {
		if u, err := user.Current(); err == nil {
			name = u.Username
		}
	}
	if name == "" {
		return []string{}
	}
	return []string{"user:" + name}
}

func (b *Backend) wifiSettings(req backend.Request, v Version, ca store.CALocation, p12 string, ssid string) connSettings {
	return connSettings{
		"connection": {
			"type":        dbus.MakeVariant("802-11-wireless"),
			"uuid":        dbus.MakeVariant(newUUID()),
			"permissions": dbus.MakeVariant(permissions()),
			"id":          dbus.MakeVariant(ssid),
		},
		"802-11-wireless": {
			"ssid":     dbus.MakeVariant([]byte(ssid)),
			"security": dbus.MakeVariant("802-11-wireless-security"),
		},
		"802-11-wireless-security": {
			"key-mgmt": dbus.MakeVariant("wpa-eap"),
			"proto":    dbus.MakeVariant([]string{"rsn"}),
			"pairwise": dbus.MakeVariant([]string{"ccmp"}),
			"group":    dbus.MakeVariant([]string{"ccmp"}),
		},
		"802-1x": b.security8021x(req, v, ca, p12),
		"ipv4":   ipAuto(),
		"ipv6":   ipAuto(),
	}
}

func (b *Backend) wiredSettings(req backend.Request, v Version, ca store.CALocation, p12 string) connSettings {
	sec := b.security8021x(req, v, ca, p12)
	sec["optional"] = dbus.MakeVariant(true)
	sec["auth-timeout"] = dbus.MakeVariant(int32(10))
	return connSettings{
		"connection": {
			"type":                 dbus.MakeVariant("802-3-ethernet"),
			"autoconnect-priority": dbus.MakeVariant(int32(100)),
			"autoconnect-retries":  dbus.MakeVariant(int32(3)),
			"uuid":                 dbus.MakeVariant(newUUID()),
			"permissions":          dbus.MakeVariant(permissions()),
			"id":                   dbus.MakeVariant(wiredConnID),
		},
		"802-1x": sec,
		"ipv4":   ipAuto(),
		"ipv6":   ipAuto(),
	}
}

// newUUID returns a random (version 4) UUID.
func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
