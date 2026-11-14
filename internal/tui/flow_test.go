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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SepehrRajabi/eduroam-goe-linux/internal/backend"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/backend/wpasupplicant"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/profile"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/store"
)

// drive feeds a message and synchronously executes resulting commands
// (except spinner ticks / batches containing blinks) until quiescent.
func drive(m *Model, msg tea.Msg) {
	queue := []tea.Msg{msg}
	for len(queue) > 0 {
		msg, queue = queue[0], queue[1:]
		_, cmd := m.Update(msg)
		if cmd == nil {
			continue
		}
		ch := make(chan tea.Msg, 1)
		go func() { ch <- cmd() }()
		var out tea.Msg
		select {
		case out = <-ch:
		case <-time.After(100 * time.Millisecond): // timers (blink, spinner)
		}
		if out != nil {
			switch out := out.(type) {
			case installMsg, p12Msg, probesMsg:
				queue = append(queue, out)
			case tea.BatchMsg:
				_ = out // blinks etc.
			}
		}
	}
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func typeText(m *Model, s string) {
	for _, r := range s {
		drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func TestFlowWPA(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cat_installer")
	m := New(Options{
		Profile: profile.Default,
		Store:   &store.Store{Dir: dir},
		Backend: wpasupplicant.Backend{},
		Log:     backend.DiscardLogger,
	})
	drive(m, tea.WindowSizeMsg{Width: 90, Height: 40})

	if m.step != stepIntro {
		t.Fatalf("step %d", m.step)
	}
	t.Log("\n" + m.View())
	drive(m, key("enter")) // intro
	if m.step != stepConfirm {
		t.Fatalf("expected confirm, got %d", m.step)
	}
	drive(m, key("y")) // membership
	if m.step != stepPlaintext {
		t.Fatalf("expected plaintext warning, got %d", m.step)
	}
	drive(m, key("y"))
	if m.step != stepCreds {
		t.Fatalf("expected creds, got %d", m.step)
	}

	// invalid username first
	typeText(m, "max@")
	drive(m, key("tab"))
	typeText(m, "pw")
	drive(m, key("tab"))
	typeText(m, "pw")
	drive(m, key("enter"))
	if m.errMsg == "" || m.step != stepCreds {
		t.Fatalf("expected username error")
	}
	t.Log("\n" + m.View())
	// focus returned to the username; passwords are kept
	typeText(m, "uni-goettingen.de")
	drive(m, key("enter"))
	drive(m, key("enter"))
	drive(m, key("enter"))
	if m.step != stepDone {
		t.Fatalf("expected done, got %d (%v)", m.step, m.outcome.Err)
	}
	t.Log("\n" + m.View())

	conf, err := os.ReadFile(filepath.Join(dir, wpasupplicant.FileName))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`identity="max@uni-goettingen.de"`, `password="pw"`,
		`anonymous_identity="wifi@uni-goettingen.de"`, `phase2="auth=MSCHAPV2"`} {
		if !strings.Contains(string(conf), want) {
			t.Errorf("missing %s in\n%s", want, conf)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "ca.pem")); err != nil {
		t.Error(err)
	}
}
