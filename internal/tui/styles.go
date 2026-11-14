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

package tui

import "github.com/charmbracelet/lipgloss"

var (
	colAccent   = lipgloss.AdaptiveColor{Light: "#1F5FAD", Dark: "#6CA8F0"}
	colAccentBg = lipgloss.AdaptiveColor{Light: "#1F5FAD", Dark: "#2D5F9E"}
	colText     = lipgloss.AdaptiveColor{Light: "#1A1A1A", Dark: "#E6E6E6"}
	colMuted    = lipgloss.AdaptiveColor{Light: "#6B6B6B", Dark: "#8A8A8A"}
	colFaint    = lipgloss.AdaptiveColor{Light: "#C4C4C4", Dark: "#4A4A4A"}
	colError    = lipgloss.AdaptiveColor{Light: "#B3261E", Dark: "#F2827A"}
	colOK       = lipgloss.AdaptiveColor{Light: "#1E7B34", Dark: "#6FCF87"}
	colWarn     = lipgloss.AdaptiveColor{Light: "#8A5A00", Dark: "#F0C060"}
	colOnAccent = lipgloss.Color("#FFFFFF")

	// Frame
	headerStyle      = lipgloss.NewStyle().Background(colAccentBg).Foreground(colOnAccent)
	headerTitleStyle = headerStyle.Bold(true)
	headerDimStyle   = headerStyle.Foreground(lipgloss.AdaptiveColor{Light: "#D6E4F5", Dark: "#B9CCE6"})
	sidebarStyle     = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, true, false, false).
				BorderForeground(colFaint).
				Padding(1, 2, 0, 2)
	panelStyle  = lipgloss.NewStyle().Padding(1, 3, 0, 3)
	footerStyle = lipgloss.NewStyle().Padding(0, 1)

	// Text
	headingStyle  = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	ruleStyle     = lipgloss.NewStyle().Foreground(colFaint)
	textStyle     = lipgloss.NewStyle().Foreground(colText)
	mutedStyle    = lipgloss.NewStyle().Foreground(colMuted)
	errorStyle    = lipgloss.NewStyle().Foreground(colError)
	okStyle       = lipgloss.NewStyle().Foreground(colOK)
	warnStyle     = lipgloss.NewStyle().Foreground(colWarn)
	selectedStyle = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	boldStyle     = lipgloss.NewStyle().Bold(true)
	sectionStyle  = lipgloss.NewStyle().Foreground(colMuted).Bold(true)

	// Help footer
	helpKeyStyle  = lipgloss.NewStyle().Foreground(colText).Bold(true)
	helpDescStyle = lipgloss.NewStyle().Foreground(colMuted)
	helpSepStyle  = lipgloss.NewStyle().Foreground(colFaint)

	// Buttons
	buttonStyle       = lipgloss.NewStyle().Padding(0, 3).Foreground(colMuted).Background(lipgloss.AdaptiveColor{Light: "#EBEBEB", Dark: "#2E2E2E"})
	activeButtonStyle = lipgloss.NewStyle().Padding(0, 3).Bold(true).Foreground(colOnAccent).Background(colAccentBg)

	// Badges
	badgeOK    = lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(colOnAccent).Background(lipgloss.AdaptiveColor{Light: "#1E7B34", Dark: "#2F7D45"})
	badgeError = lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(colOnAccent).Background(lipgloss.AdaptiveColor{Light: "#B3261E", Dark: "#A33A33"})
	badgeWarn  = lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(lipgloss.Color("#1A1A1A")).Background(lipgloss.AdaptiveColor{Light: "#F0C060", Dark: "#E0B050"})

	// Callout box for warnings / quit confirmation
	calloutStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)
