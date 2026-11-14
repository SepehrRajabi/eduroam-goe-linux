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

package iwd

import (
	"strings"
	"testing"

	"github.com/SepehrRajabi/eduroam-goe-linux/internal/backend"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/creds"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/profile"
)

func TestConfigPEAP(t *testing.T) {
	cfg, err := Config(backend.Request{
		Profile: profile.Default,
		Creds:   creds.Credentials{Username: "max@uni-goettingen.de", Password: "secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"EAP-Method=PEAP",
		"EAP-Identity=wifi@uni-goettingen.de",
		"EAP-PEAP-ServerDomainMask=eduroam.gwdg.de",
		"EAP-PEAP-Phase2-Method=MSCHAPV2",
		"EAP-PEAP-Phase2-Identity=max@uni-goettingen.de",
		"EAP-PEAP-Phase2-Password=secret",
		"[@pem@eduroam_ca_cert]\n-----BEGIN CERTIFICATE-----",
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config missing %q:\n%s", want, cfg)
		}
	}
}
