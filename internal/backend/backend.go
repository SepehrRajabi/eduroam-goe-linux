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

// Package backend defines the interface implemented by network management
// backends (NetworkManager, wpa_supplicant, iwd, ...) and a registry of the
// available implementations.
package backend

import (
	"context"
	"fmt"
	"io"
	"log"
	"sort"
	"sync"

	"github.com/SepehrRajabi/eduroam-goe-linux/internal/creds"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/profile"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/store"
)

// Info is static metadata about a backend.
type Info struct {
	// ID is the stable identifier used on the command line.
	ID string
	// Name is the human-readable name.
	Name string
	// Description is a one-line explanation shown in the UI.
	Description string
	// Priority orders backends; lower values are preferred when
	// auto-selecting.
	Priority int
	// SupportsWired is true if the backend can also configure 802.1X on
	// wired Ethernet.
	SupportsWired bool
	// PlaintextSecrets is true if the backend writes the user's password
	// in clear text to a file.
	PlaintextSecrets bool
}

// Status is the result of probing a backend on the current system.
type Status struct {
	Available bool
	// Detail is extra information, e.g. the detected daemon version, or why
	// the backend is unavailable.
	Detail string
}

// Request bundles everything a backend needs to install the profile.
type Request struct {
	Profile *profile.Profile
	Creds   creds.Credentials
	Store   *store.Store
	// Wired asks the backend to also configure wired 802.1X.
	Wired bool
	// Log receives debug output. Never nil when passed to Install.
	Log *log.Logger
}

// Result describes the outcome of an installation.
type Result struct {
	// Notes are human-readable lines to show to the user.
	Notes []string
}

// Backend is a network management system able to receive an eduroam
// configuration.
type Backend interface {
	Info() Info
	// Probe checks whether the backend can be used on this system.
	Probe(ctx context.Context) Status
	// Supports reports whether the backend can handle the given profile
	// (e.g. its EAP method). A nil error means supported.
	Supports(p *profile.Profile) error
	// Install configures the network(s) described by the request.
	Install(ctx context.Context, req Request) (*Result, error)
}

var (
	mu       sync.RWMutex
	registry = map[string]Backend{}
)

// Register makes a backend available. It is intended to be called from the
// init function of backend packages.
func Register(b Backend) {
	mu.Lock()
	defer mu.Unlock()
	id := b.Info().ID
	if _, dup := registry[id]; dup {
		panic("backend: duplicate registration of " + id)
	}
	registry[id] = b
}

// All returns all registered backends ordered by priority.
func All() []Backend {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Backend, 0, len(registry))
	for _, b := range registry {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Info(), out[j].Info()
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		return a.ID < b.ID
	})
	return out
}

// Get returns the backend with the given ID.
func Get(id string) (Backend, error) {
	mu.RLock()
	defer mu.RUnlock()
	b, ok := registry[id]
	if !ok {
		return nil, fmt.Errorf("unknown backend %q", id)
	}
	return b, nil
}

// Probed pairs a backend with its probe result and profile compatibility.
type Probed struct {
	Backend Backend
	Status  Status
	// Unsupported is non-nil if the backend cannot handle the profile.
	Unsupported error
}

// Usable reports whether the backend can be selected.
func (p Probed) Usable() bool { return p.Status.Available && p.Unsupported == nil }

// ProbeAll probes every registered backend.
func ProbeAll(ctx context.Context, p *profile.Profile) []Probed {
	all := All()
	out := make([]Probed, len(all))
	var wg sync.WaitGroup
	for i, b := range all {
		wg.Go(func() {
			out[i] = Probed{Backend: b, Status: b.Probe(ctx), Unsupported: b.Supports(p)}
		})
	}
	wg.Wait()
	return out
}

// FirstUsable returns the preferred usable backend, if any.
func FirstUsable(ps []Probed) (Backend, bool) {
	for _, p := range ps {
		if p.Usable() {
			return p.Backend, true
		}
	}
	return nil, false
}

// DiscardLogger is a logger that drops everything.
var DiscardLogger = log.New(io.Discard, "", 0)
