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

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SepehrRajabi/eduroam-goe-linux/internal/backend/wpasupplicant"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/profile"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/store"
)

func TestQuitDialogKeys(t *testing.T) {
	m := New(Options{Profile: profile.Default, Store: &store.Store{Dir: t.TempDir() + "/x"},
		Backend: wpasupplicant.Backend{}})
	esc := tea.KeyMsg{Type: tea.KeyEsc}
	right := tea.KeyMsg{Type: tea.KeyRight}
	left := tea.KeyMsg{Type: tea.KeyLeft}

	// Default selection is "Stay": enter closes the dialog.
	drive(m, esc)
	if !m.confirmQuit || m.quitYes {
		t.Fatal("dialog should open with Stay selected")
	}
	drive(m, key("enter"))
	if m.confirmQuit || m.outcome.Aborted {
		t.Fatal("enter on Stay should close the dialog")
	}

	// Arrow keys move the selection; enter on Quit aborts.
	drive(m, esc)
	drive(m, left)
	if !m.quitYes {
		t.Fatal("left should select Quit")
	}
	drive(m, right)
	if m.quitYes {
		t.Fatal("right should select Stay")
	}
	drive(m, left)
	_, cmd := m.Update(key("enter"))
	if !m.outcome.Aborted || cmd == nil {
		t.Fatal("enter on Quit should abort")
	}
}
