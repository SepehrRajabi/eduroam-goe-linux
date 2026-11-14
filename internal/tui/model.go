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

// Package tui implements the interactive installer frontend with Bubble Tea.
package tui

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/SepehrRajabi/eduroam-goe-linux/internal/backend"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/creds"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/profile"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/store"
	"github.com/SepehrRajabi/eduroam-goe-linux/internal/sysinfo"
)

// Options configure the TUI.
type Options struct {
	Profile *profile.Profile
	Store   *store.Store
	// Username, Password and PFXFile pre-fill the corresponding fields.
	Username string
	Password string
	PFXFile  string
	// Backend forces a backend and skips the selection screen.
	Backend backend.Backend
	Log     *log.Logger
}

// Outcome is the final state of the installer.
type Outcome struct {
	Installed bool
	Aborted   bool
	Err       error
	Result    *backend.Result
	Backend   backend.Backend
}

type step int

// The steps, in the order they are visited. Steps that do not apply are
// skipped by advance().
const (
	stepIntro step = iota
	stepConfirm
	stepTerms
	stepDirExists
	stepBackend
	stepPlaintext
	stepWired
	stepCreds
	stepP12File
	stepP12Pass
	stepP12User
	stepInstalling
	stepDone
	stepFailed
)

// Credential form field indices.
const (
	fieldUser = iota
	fieldPass
	fieldRepeat
)

type (
	probesMsg []backend.Probed
	p12Msg    struct {
		identity string
		err      error
	}
	installMsg struct {
		res *backend.Result
		err error
	}
)

// Model is the Bubble Tea model of the installer.
type Model struct {
	opts Options
	p    *profile.Profile

	step   step
	width  int
	height int

	// yes/no dialogs
	yes bool
	// quit confirmation overlay
	confirmQuit bool
	// quitYes is the highlighted button in the quit dialog (true = Quit)
	quitYes bool

	// backend selection
	probes  []backend.Probed
	cursor  int
	chosen  backend.Backend
	hasWire bool
	wired   bool

	// forms
	inputs []textinput.Model
	focus  int
	errMsg string
	busy   bool
	// reveal shows password fields in clear text (ctrl+r)
	reveal bool

	spinner spinner.Model
	creds   creds.Credentials
	dirWas  bool

	outcome Outcome
}

// New creates the installer model.
func New(opts Options) *Model {
	if opts.Log == nil {
		opts.Log = backend.DiscardLogger
	}
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = selectedStyle
	m := &Model{
		opts:    opts,
		p:       opts.Profile,
		yes:     true,
		spinner: sp,
		chosen:  opts.Backend,
		dirWas:  opts.Store.Exists(),
		hasWire: sysinfo.HasWiredInterface(),
		width:   80,
		height:  24,
	}
	m.creds.Username = opts.Username
	m.creds.Password = opts.Password
	return m
}

// Outcome returns the final result once the program has exited.
func (m *Model) Outcome() Outcome { return m.outcome }

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}
	if m.chosen == nil {
		p := m.p
		cmds = append(cmds, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return probesMsg(backend.ProbeAll(ctx, p))
		})
	}
	return tea.Batch(cmds...)
}

// applicable reports whether a step should be shown.
func (m *Model) applicable(s step) bool {
	p := m.p
	switch s {
	case stepTerms:
		return p.TermsOfUse != ""
	case stepDirExists:
		return m.dirWas
	case stepBackend:
		return m.opts.Backend == nil
	case stepPlaintext:
		return m.chosen != nil && m.chosen.Info().PlaintextSecrets
	case stepWired:
		return m.chosen != nil && m.chosen.Info().SupportsWired && p.Wired != "" && m.hasWire
	case stepCreds:
		return p.UsesPassword()
	case stepP12File:
		return p.UsesCertificate() && p.EAPInner != profile.InnerSilverBullet
	case stepP12Pass:
		return p.UsesCertificate()
	case stepP12User:
		return p.UsesCertificate() && m.creds.Username == ""
	case stepDone, stepFailed:
		return false
	}
	return true
}

func (m *Model) advance() tea.Cmd {
	for s := m.step + 1; s <= stepInstalling; s++ {
		if m.applicable(s) {
			return m.enter(s)
		}
	}
	return m.enter(stepInstalling)
}

func newInput(placeholder, value string, secret bool) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(value)
	ti.CharLimit = 256
	ti.Width = 36
	ti.Prompt = ""
	ti.PlaceholderStyle = mutedStyle
	if secret {
		ti.EchoMode = textinput.EchoPassword
		ti.EchoCharacter = '•'
	}
	return ti
}

func (m *Model) setInputs(in ...textinput.Model) tea.Cmd {
	m.inputs = in
	m.focus = 0
	m.applyReveal()
	return m.focusInput(0)
}

// applyReveal toggles the echo mode of password inputs.
func (m *Model) applyReveal() {
	var secret []int
	switch m.step {
	case stepCreds:
		secret = []int{fieldPass, fieldRepeat}
	case stepP12Pass:
		secret = []int{0}
	}
	for _, i := range secret {
		if i >= len(m.inputs) {
			continue
		}
		if m.reveal {
			m.inputs[i].EchoMode = textinput.EchoNormal
		} else {
			m.inputs[i].EchoMode = textinput.EchoPassword
		}
	}
}

func (m *Model) focusInput(i int) tea.Cmd {
	m.focus = i
	var cmd tea.Cmd
	for j := range m.inputs {
		if j == i {
			cmd = m.inputs[j].Focus()
		} else {
			m.inputs[j].Blur()
		}
	}
	return cmd
}

// enter switches to step s and prepares its state.
func (m *Model) enter(s step) tea.Cmd {
	m.step = s
	m.yes = true
	m.errMsg = ""
	m.busy = false
	m.inputs = nil
	switch s {
	case stepWired:
		m.yes = false
	case stepCreds:
		focus := fieldUser
		user := creds.UsernameHint(m.p, m.creds.Username)
		cmd := m.setInputs(
			newInput("user@"+m.p.UserRealm, user, false),
			newInput("", m.creds.Password, true),
			newInput("", m.creds.Password, true),
		)
		if m.creds.Username != "" {
			focus = fieldPass
		}
		return tea.Batch(cmd, m.focusInput(focus))
	case stepP12File:
		path := m.opts.PFXFile
		if path == "" {
			path = creds.FindP12InDir(".")
		}
		return m.setInputs(newInput("/path/to/certificate.p12", path, false))
	case stepP12Pass:
		if m.p.EAPInner == profile.InnerSilverBullet {
			data, err := base64.StdEncoding.DecodeString(m.p.SBUserFile)
			if err != nil {
				return m.fail(fmt.Errorf("embedded certificate is corrupt: %w", err))
			}
			m.creds.P12 = data
		}
		return m.setInputs(newInput("", m.creds.Password, true))
	case stepP12User:
		return m.setInputs(newInput("user@"+m.p.UserRealm, "", false))
	case stepInstalling:
		return m.install()
	}
	return nil
}

func (m *Model) fail(err error) tea.Cmd {
	m.step = stepFailed
	m.outcome = Outcome{Err: err, Backend: m.chosen}
	return nil
}

func (m *Model) abort() (tea.Model, tea.Cmd) {
	m.outcome = Outcome{Aborted: true}
	return m, tea.Quit
}

func (m *Model) install() tea.Cmd {
	req := backend.Request{
		Profile: m.p,
		Creds:   m.creds,
		Store:   m.opts.Store,
		Wired:   m.wired,
		Log:     m.opts.Log,
	}
	b := m.chosen
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := req.Store.Ensure(); err != nil {
			return installMsg{err: err}
		}
		res, err := b.Install(ctx, req)
		return installMsg{res: res, err: err}
	}
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case probesMsg:
		m.probes = msg
		m.cursor = 0
		for i, p := range m.probes {
			if p.Usable() {
				m.cursor = i
				break
			}
		}
		return m, nil
	case p12Msg:
		m.busy = false
		if msg.err != nil {
			m.opts.Log.Printf("p12: %v", msg.err)
			m.errMsg = profile.MsgIncorrectPassword
			m.inputs[0].SetValue("")
			return m, nil
		}
		if !m.p.UseOtherTLSID && msg.identity != "" && m.creds.Username == "" {
			m.creds.Username = msg.identity
		}
		return m, m.advance()
	case installMsg:
		if msg.err != nil {
			return m, m.fail(msg.err)
		}
		m.step = stepDone
		m.outcome = Outcome{Installed: true, Result: msg.res, Backend: m.chosen}
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, m.updateInputs(msg)
}

func (m *Model) updateInputs(msg tea.Msg) tea.Cmd {
	if len(m.inputs) == 0 {
		return nil
	}
	var cmd tea.Cmd
	m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
	return cmd
}

func (m *Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if k.Type == tea.KeyCtrlC {
		if m.step == stepDone {
			return m, tea.Quit
		}
		return m.abort()
	}

	if m.confirmQuit {
		switch strings.ToLower(k.String()) {
		case "left", "right", "h", "l", "tab", "shift+tab":
			m.quitYes = !m.quitYes
		case "enter":
			if m.quitYes {
				return m.abort()
			}
			m.confirmQuit = false
		case "y":
			return m.abort()
		case "n", "esc":
			m.confirmQuit = false
		}
		return m, nil
	}

	switch m.step {
	case stepDone, stepFailed:
		switch k.String() {
		case "enter", "q", "esc":
			return m, tea.Quit
		}
		return m, nil
	case stepInstalling:
		return m, nil
	}

	if k.Type == tea.KeyEsc {
		m.confirmQuit = true
		m.quitYes = false // default to the safe choice
		return m, nil
	}

	if k.Type == tea.KeyCtrlR && (m.step == stepCreds || m.step == stepP12Pass) {
		m.reveal = !m.reveal
		m.applyReveal()
		return m, nil
	}

	switch m.step {
	case stepIntro:
		if k.Type == tea.KeyEnter {
			return m, m.advance()
		}
	case stepConfirm, stepTerms, stepDirExists, stepPlaintext, stepWired:
		return m.handleYesNo(k)
	case stepBackend:
		return m.handleBackendKey(k)
	case stepCreds:
		return m.handleCredsKey(k)
	case stepP12File, stepP12Pass, stepP12User:
		if k.Type == tea.KeyEnter && !m.busy {
			return m.submitSingle()
		}
		if m.busy {
			return m, nil
		}
		return m, m.updateInputs(k)
	}
	return m, nil
}

func (m *Model) handleYesNo(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(k.String()) {
	case "left", "right", "h", "l", "tab", "shift+tab":
		m.yes = !m.yes
		return m, nil
	case "y":
		m.yes = true
	case "n":
		m.yes = false
	case "enter":
	default:
		return m, nil
	}
	if m.step == stepWired {
		m.wired = m.yes
		return m, m.advance()
	}
	if !m.yes {
		return m.abort()
	}
	return m, m.advance()
}

func (m *Model) handleBackendKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.probes == nil {
		return m, nil
	}
	switch k.String() {
	case "up", "k":
		for i := m.cursor - 1; i >= 0; i-- {
			if m.probes[i].Usable() {
				m.cursor = i
				break
			}
		}
	case "down", "j":
		for i := m.cursor + 1; i < len(m.probes); i++ {
			if m.probes[i].Usable() {
				m.cursor = i
				break
			}
		}
	case "q":
		return m.abort()
	case "enter":
		if len(m.probes) == 0 || !m.probes[m.cursor].Usable() {
			return m, nil
		}
		m.chosen = m.probes[m.cursor].Backend
		m.opts.Log.Printf("selected backend %s", m.chosen.Info().ID)
		return m, m.advance()
	}
	return m, nil
}

func (m *Model) handleCredsKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "tab", "down":
		return m, m.focusInput((m.focus + 1) % len(m.inputs))
	case "shift+tab", "up":
		return m, m.focusInput((m.focus + len(m.inputs) - 1) % len(m.inputs))
	case "enter":
		if m.focus < len(m.inputs)-1 {
			return m, m.focusInput(m.focus + 1)
		}
		user := strings.TrimSpace(m.inputs[fieldUser].Value())
		pass := m.inputs[fieldPass].Value()
		rep := m.inputs[fieldRepeat].Value()
		if err := creds.ValidateUsername(m.p, user); err != nil {
			m.errMsg = err.Error()
			return m, m.focusInput(fieldUser)
		}
		if err := creds.ValidatePasswords(pass, rep); err != nil {
			m.errMsg = err.Error()
			m.inputs[fieldPass].SetValue("")
			m.inputs[fieldRepeat].SetValue("")
			return m, m.focusInput(fieldPass)
		}
		m.creds.Username, m.creds.Password = user, pass
		return m, m.advance()
	}
	return m, m.updateInputs(k)
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

func (m *Model) submitSingle() (tea.Model, tea.Cmd) {
	val := strings.TrimSpace(m.inputs[0].Value())
	if val == "" {
		m.errMsg = profile.MsgEmptyField
		return m, nil
	}
	switch m.step {
	case stepP12File:
		path := expandHome(val)
		data, err := os.ReadFile(path)
		if err != nil {
			m.errMsg = fmt.Sprintf("%s: %s", profile.MsgUserCertMissing, path)
			return m, nil
		}
		m.creds.P12 = data
		return m, m.advance()
	case stepP12Pass:
		pass := m.inputs[0].Value()
		m.creds.Password = pass
		m.busy = true
		m.errMsg = ""
		data := m.creds.P12
		return m, func() tea.Msg {
			id, err := creds.P12Identity(data, pass)
			return p12Msg{identity: id, err: err}
		}
	case stepP12User:
		m.creds.Username = val
		return m, m.advance()
	}
	return m, nil
}
