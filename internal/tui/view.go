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
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/SepehrRajabi/eduroam-goe-linux/internal/profile"
)

// Layout constants.
const (
	sidebarWidth   = 36 // including padding and border
	minSidebarCols = 84 // below this terminal width the sidebar is hidden
	maxTextWidth   = 72
)

// stage groups steps into the entries shown in the progress sidebar.
type stage struct {
	name  string
	steps []step
}

var stages = []stage{
	{"Welcome", []step{stepIntro}},
	{"Eligibility", []step{stepConfirm, stepTerms, stepDirExists}},
	{"Network backend", []step{stepBackend, stepPlaintext}},
	{"Wired Ethernet", []step{stepWired}},
	{"Credentials", []step{stepCreds, stepP12File, stepP12Pass, stepP12User}},
	{"Install", []step{stepInstalling, stepDone, stepFailed}},
}

// visibleStages returns the stages relevant for this run and the index of
// the current one.
func (m *Model) visibleStages() ([]stage, int) {
	var out []stage
	cur := 0
	for _, st := range stages {
		show := st.name == "Network backend" || st.name == "Install"
		isCur := false
		for _, s := range st.steps {
			if m.applicable(s) {
				show = true
			}
			if s == m.step {
				isCur = true
			}
		}
		if !show && !isCur {
			continue
		}
		if isCur {
			cur = len(out)
		}
		out = append(out, st)
	}
	return out, cur
}

func (m *Model) showSidebar() bool { return m.width >= minSidebarCols }

func (m *Model) panelWidth() int {
	w := m.width
	if m.showSidebar() {
		w -= sidebarWidth
	}
	return max(w, 20)
}

// textWidth is the usable width for wrapped text in the main panel.
func (m *Model) textWidth() int {
	return max(min(m.panelWidth()-panelStyle.GetHorizontalFrameSize(), maxTextWidth), 16)
}

func (m *Model) wrap(s string) string {
	return lipgloss.NewStyle().Width(m.textWidth()).Render(s)
}

// View implements tea.Model.
func (m *Model) View() string {
	header := m.headerView()
	footer := m.footerView()
	bodyH := max(m.height-lipgloss.Height(header)-lipgloss.Height(footer), 1)

	panel := panelStyle.Width(m.panelWidth()).Height(bodyH).MaxHeight(bodyH).Render(m.panelView())
	body := panel
	if m.showSidebar() {
		side := sidebarStyle.
			Width(sidebarWidth - sidebarStyle.GetHorizontalBorderSize()).
			Height(bodyH).MaxHeight(bodyH).
			Render(m.sidebarView(bodyH - sidebarStyle.GetVerticalPadding()))
		body = lipgloss.JoinHorizontal(lipgloss.Top, side, panel)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m *Model) headerView() string {
	left := headerTitleStyle.Render(" ◆ "+m.p.Title+" ") + headerDimStyle.Render(m.p.ProfileName+" ")
	st, cur := m.visibleStages()
	right := headerDimStyle.Render(fmt.Sprintf(" Step %d of %d ", cur+1, len(st)))
	if !m.showSidebar() {
		right = headerDimStyle.Render(fmt.Sprintf(" %s · %d/%d ", st[cur].name, cur+1, len(st)))
	}
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		return headerStyle.Width(m.width).MaxHeight(1).Render(left)
	}
	return left + headerStyle.Render(strings.Repeat(" ", gap)) + right
}

// ---------------------------------------------------------------- sidebar

func (m *Model) sidebarView(height int) string {
	inner := sidebarWidth - sidebarStyle.GetHorizontalFrameSize()
	st, cur := m.visibleStages()

	// Progress
	steps := []string{sectionStyle.Render("PROGRESS")}
	for i, s := range st {
		var icon, name string
		switch {
		case i < cur || m.step == stepDone:
			icon, name = okStyle.Render("✓"), textStyle.Render(s.name)
		case i == cur && m.step == stepFailed:
			icon, name = errorStyle.Render("✗"), errorStyle.Render(s.name)
		case i == cur:
			icon, name = selectedStyle.Render("▸"), selectedStyle.Render(s.name)
		default:
			icon, name = mutedStyle.Render("○"), mutedStyle.Render(s.name)
		}
		steps = append(steps, icon+" "+name)
	}

	// Summary of the choices made so far, one aligned line per entry.
	const keyW = 10
	summary := []string{sectionStyle.Render("SUMMARY")}
	kv := func(k, v string) {
		summary = append(summary, mutedStyle.Width(keyW).Render(k)+textStyle.Render(truncate(v, inner-keyW)))
	}
	kv("Network", strings.Join(m.p.SSIDs, ", "))
	kv("Method", m.p.EAPOuter+methodSuffix(m.p))
	if m.chosen != nil {
		kv("Backend", m.chosen.Info().Name)
	}
	if m.creds.Username != "" && m.step > stepP12User {
		kv("Identity", m.creds.Username)
	}
	if m.step > stepWired && m.wired {
		kv("Wired", "802.1X")
	}

	upper := strings.Join(steps, "\n") + "\n\n" + strings.Join(summary, "\n")

	// Support block, anchored to the bottom.
	support := strings.Join([]string{
		ruleStyle.Render(strings.Repeat("─", inner)),
		sectionStyle.Render("SUPPORT"),
		textStyle.Render(truncate(m.p.Email, inner)),
		lipgloss.NewStyle().Foreground(colFaint).Render(truncate("eduroam CAT · GÉANT project", inner)),
	}, "\n")

	gap := height - lipgloss.Height(upper) - lipgloss.Height(support)
	if gap < 1 { // not enough room: drop the support block
		return upper
	}
	return upper + strings.Repeat("\n", gap) + support
}

// truncate shortens s to at most w cells, adding an ellipsis.
func truncate(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

func methodSuffix(p *profile.Profile) string {
	if p.EAPInner != "" && p.EAPInner != profile.InnerSilverBullet && p.UsesPassword() {
		return "/" + p.EAPInner
	}
	return ""
}

// ---------------------------------------------------------------- footer

type binding struct{ key, desc string }

func (m *Model) footerView() string {
	var parts []string
	for _, b := range m.bindings() {
		parts = append(parts, helpKeyStyle.Render(b.key)+" "+helpDescStyle.Render(b.desc))
	}
	line := strings.Join(parts, helpSepStyle.Render("  ·  "))
	rule := ruleStyle.Render(strings.Repeat("─", max(m.width, 1)))
	return rule + "\n" + footerStyle.Width(m.width).MaxHeight(1).Render(line)
}

func (m *Model) bindings() []binding {
	quit := binding{"esc", "quit"}
	if m.confirmQuit {
		return []binding{{"←/→", "choose"}, {"enter", "confirm"}, {"esc", "stay"}}
	}
	switch m.step {
	case stepIntro:
		return []binding{{"enter", "continue"}, quit}
	case stepConfirm, stepTerms, stepDirExists, stepPlaintext, stepWired:
		return []binding{{"←/→", "choose"}, {"y/n", "answer"}, {"enter", "confirm"}, quit}
	case stepBackend:
		if m.probes == nil {
			return []binding{quit}
		}
		return []binding{{"↑/↓", "select"}, {"enter", "use backend"}, quit}
	case stepCreds:
		return []binding{{"tab/↑/↓", "move"}, {"enter", "next/submit"}, {"ctrl+r", m.revealLabel()}, quit}
	case stepP12Pass:
		return []binding{{"enter", "submit"}, {"ctrl+r", m.revealLabel()}, quit}
	case stepP12File, stepP12User:
		return []binding{{"enter", "submit"}, quit}
	case stepInstalling:
		return []binding{{"", "please wait…"}}
	case stepDone, stepFailed:
		return []binding{{"enter", "exit"}}
	}
	return nil
}

func (m *Model) revealLabel() string {
	if m.reveal {
		return "hide password"
	}
	return "show password"
}

// ---------------------------------------------------------------- panel

func (m *Model) titled(title, body string) string {
	return headingStyle.Render(title) + "\n" +
		ruleStyle.Render(strings.Repeat("─", min(lipgloss.Width(title)+2, m.textWidth()))) + "\n\n" + body
}

func (m *Model) panelView() string {
	if m.confirmQuit {
		box := calloutStyle.BorderForeground(colWarn).Width(min(m.textWidth(), 48)).Render(
			warnStyle.Bold(true).Render(profile.MsgQuit) + "\n\n" +
				mutedStyle.Render("No changes have been made to your network configuration yet.") + "\n\n" +
				m.buttons(m.quitYes, "Quit", "Stay"))
		return m.titled("Quit installer", box)
	}

	switch m.step {
	case stepIntro:
		return m.titled("Welcome", m.wrap(m.p.InitInfoText())+"\n\n"+activeButtonStyle.Render("Get started →"))

	case stepConfirm:
		return m.question("Eligibility", m.p.InitConfirmationText(), profile.MsgContinue)

	case stepTerms:
		return m.question("Terms of use", m.p.TermsOfUse, profile.MsgContinue)

	case stepDirExists:
		return m.question("Existing configuration",
			fmt.Sprintf(profile.MsgCatDirExists, m.opts.Store.Dir), profile.MsgContinue)

	case stepPlaintext:
		warn := calloutStyle.BorderForeground(colWarn).Width(m.textWidth()).
			Render(badgeWarn.Render("WARNING") + "\n\n" + profile.MsgPlaintextWarning)
		return m.titled(m.chosen.Info().Name, warn+"\n\n"+boldStyle.Render("Write the file?")+"\n\n"+m.buttons(m.yes, "Yes", "No"))

	case stepWired:
		return m.question("Wired Ethernet", profile.MsgWired, "")

	case stepBackend:
		return m.backendView()

	case stepCreds:
		return m.credsView()

	case stepP12File:
		return m.titled("Client certificate",
			m.wrap("Select your "+profile.MsgP12Title+".")+"\n\n"+
				m.field("Certificate file", 0, "~ is expanded to your home directory")+m.errLine())

	case stepP12Pass:
		s := m.wrap("Your certificate is protected by a password.") + "\n\n" +
			m.field("Import password", 0, "")
		if m.busy {
			s += "\n" + m.spinner.View() + " checking certificate…"
		}
		return m.titled("Certificate password", s+m.errLine())

	case stepP12User:
		return m.titled("Username",
			m.wrap("Your identity could not be determined from the certificate.")+"\n\n"+
				m.field("Username", 0, "e.g. user@"+m.p.UserRealm)+m.errLine())

	case stepInstalling:
		name := "backend"
		if m.chosen != nil {
			name = m.chosen.Info().Name
		}
		return m.titled("Installing", m.spinner.View()+" Configuring "+boldStyle.Render(name)+"…")

	case stepDone:
		return m.doneView()

	case stepFailed:
		s := badgeError.Render("✗ FAILED") + "\n\n" + errorStyle.Render(m.wrap(fmt.Sprint(m.outcome.Err))) +
			"\n\n" + mutedStyle.Render(m.wrap("Run the installer with --debug for details, or contact "+m.p.Email+"."))
		return m.titled("Installation failed", s)
	}
	return ""
}

func (m *Model) buttons(yes bool, yesLabel, noLabel string) string {
	y, n := buttonStyle, buttonStyle
	if yes {
		y = activeButtonStyle
	} else {
		n = activeButtonStyle
	}
	return y.Render(yesLabel) + "  " + n.Render(noLabel)
}

func (m *Model) question(title, text, prompt string) string {
	s := m.wrap(text) + "\n\n"
	if prompt != "" {
		s += boldStyle.Render(prompt) + "\n\n"
	}
	return m.titled(title, s+m.buttons(m.yes, "Yes", "No"))
}

func (m *Model) errLine() string {
	if m.errMsg == "" {
		return ""
	}
	return "\n" + errorStyle.Render(m.wrap("✗ "+m.errMsg))
}

// field renders a labelled text input with a focus bar.
func (m *Model) field(label string, idx int, hint string) string {
	if idx >= len(m.inputs) {
		return ""
	}
	bar, lab := ruleStyle.Render("│ "), mutedStyle.Render(label)
	if idx == m.focus {
		bar, lab = selectedStyle.Render("┃ "), selectedStyle.Render(label)
	}
	s := lab + "\n" + bar + m.inputs[idx].View() + "\n"
	if hint != "" {
		s += mutedStyle.Render("  "+hint) + "\n"
	}
	return s
}

func (m *Model) credsView() string {
	var s strings.Builder
	s.WriteString(m.wrap(profile.MsgCredentialsPrompt) + "\n\n")
	hint := ""
	if m.p.UserRealm != "" {
		hint = "e.g. user@" + m.p.UserRealm
	}
	s.WriteString(m.field("Username", fieldUser, hint) + "\n")
	s.WriteString(m.field("Password", fieldPass, "") + "\n")
	s.WriteString(m.field("Repeat password", fieldRepeat, ""))
	s.WriteString(m.errLine())
	return m.titled("Credentials", s.String())
}

func (m *Model) backendView() string {
	if m.probes == nil {
		return m.titled("Network backend", m.spinner.View()+" Detecting network management backends…")
	}
	var s strings.Builder
	s.WriteString(m.wrap("Choose how eduroam should be configured on this system:") + "\n\n")
	anyUsable := false
	indent := lipgloss.NewStyle().PaddingLeft(4).Width(m.textWidth())
	for i, p := range m.probes {
		info := p.Backend.Info()
		var mark, name, state, detail string
		switch {
		case p.Unsupported != nil:
			mark, name = mutedStyle.Render("⊘"), mutedStyle.Render(info.Name)
			state, detail = errorStyle.Render("unsupported"), p.Unsupported.Error()
		case !p.Status.Available:
			mark, name = mutedStyle.Render("⊘"), mutedStyle.Render(info.Name)
			state, detail = mutedStyle.Render("unavailable"), p.Status.Detail
		default:
			anyUsable = true
			mark, name = "○", textStyle.Render(info.Name)
			if i == m.cursor {
				mark, name = selectedStyle.Render("◉"), selectedStyle.Render(info.Name)
			}
			state, detail = okStyle.Render("available"), p.Status.Detail
		}
		cursor := "  "
		if i == m.cursor && p.Usable() {
			cursor = selectedStyle.Render("▌ ")
		}
		line := cursor + mark + " " + name
		pad := max(m.textWidth()-lipgloss.Width(line)-lipgloss.Width(state), 1)
		s.WriteString(line + strings.Repeat(" ", pad) + state + "\n")
		s.WriteString(indent.Render(mutedStyle.Render(info.Description)) + "\n")
		if detail != "" {
			s.WriteString(indent.Render(lipgloss.NewStyle().Foreground(colFaint).Render(detail)) + "\n")
		}
		if i < len(m.probes)-1 {
			s.WriteString("\n")
		}
	}
	if !anyUsable {
		s.WriteString("\n" + errorStyle.Render("No usable backend found."))
	}
	return m.titled("Network backend", strings.TrimRight(s.String(), "\n"))
}

func (m *Model) doneView() string {
	var s strings.Builder
	s.WriteString(badgeOK.Render("✓ "+strings.ToUpper(profile.MsgInstallFinished)) + "\n\n")

	var rows [][2]string
	if m.chosen != nil {
		rows = append(rows, [2]string{"Backend", m.chosen.Info().Name})
	}
	rows = append(rows, [2]string{"Networks", strings.Join(m.p.SSIDs, ", ")})
	if m.creds.Username != "" {
		rows = append(rows, [2]string{"Identity", m.creds.Username})
	}
	if m.wired {
		rows = append(rows, [2]string{"Wired", "802.1X configured"})
	}
	for _, r := range rows {
		s.WriteString(mutedStyle.Width(12).Render(r[0]) + textStyle.Render(r[1]) + "\n")
	}
	if m.outcome.Result != nil && len(m.outcome.Result.Notes) > 0 {
		s.WriteString("\n")
		for _, n := range m.outcome.Result.Notes {
			s.WriteString(m.wrap("• "+n) + "\n")
		}
	}
	return m.titled("Done", strings.TrimRight(s.String(), "\n"))
}
