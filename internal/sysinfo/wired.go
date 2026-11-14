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

// Package sysinfo inspects the local system.
package sysinfo

import (
	"net"
	"regexp"
)

var (
	ignoredIface = regexp.MustCompile(`^(lo|wl\w+|docker\d+|virbr\d+|tun\d+|br-\w+|veth\w+)$`)
	wiredIface   = regexp.MustCompile(`(?i)^(eth|en[opsx]|eno|enp|ens)\d`)
)

// IsWiredName reports whether an interface name looks like a physical
// Ethernet device.
func IsWiredName(name string) bool {
	return !ignoredIface.MatchString(name) && wiredIface.MatchString(name)
}

// HasWiredInterface reports whether the system has an Ethernet interface,
// using the same name heuristics as the Python installer.
func HasWiredInterface() bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, i := range ifaces {
		if IsWiredName(i.Name) {
			return true
		}
	}
	return false
}
