package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"lazydev/internal/config"
	"lazydev/internal/devserver"
	"lazydev/internal/prdtemplate"
	"lazydev/internal/project"
	"lazydev/internal/state"
)

const (
	colPhase  = 14
	colPort   = 5
	colCost   = 7
	colBranch = 22
)

type layout struct {
	width, leftWidth, rightWidth int
	// sideWidth ist die Breite der rechten Spalte mit „Benachrichtigungen“ und „Issues“, 0 ohne
	// Spalte.
	sideWidth int
	// notifHeight und issuesHeight sind die Höhen der Boxen in der rechten Spalte, 0 ohne Box.
	notifHeight, issuesHeight int
	topHeight, detailHeight   int
	footerLines               int
}

func (m Model) layout() layout {
	l := layout{width: m.width}
	l.leftWidth = min(max(24, m.width/4), 36)
	showNotif, showIss := m.showNotifications(), m.showIssues()
	if showNotif || showIss {
		l.sideWidth = l.leftWidth
	}
	l.rightWidth = max(0, m.width-l.leftWidth-l.sideWidth)
	l.footerLines = 2
	if len(m.candidates) > 0 {
		l.footerLines = 3
	}
	l.detailHeight = max(6, m.height/3)
	if m.logFull {
		l.detailHeight = max(6, m.height-l.footerLines)
	}
	l.topHeight = max(0, m.height-l.footerLines-l.detailHeight)
	switch {
	case showNotif && showIss:
		l.notifHeight = l.topHeight / 2
		l.issuesHeight = l.topHeight - l.notifHeight
	case showNotif:
		l.notifHeight = l.topHeight
	case showIss:
		l.issuesHeight = l.topHeight
	}
	return l
}

// logRows ist die Zahl der Log-Zeilen unter der Kopfzeile des Laufs.
func (l layout) logRows() int {
	return max(1, l.detailHeight-3)
}

// prdHeader liefert die Zeilen vor der ersten PRD-Zeile: Fehler, dann die Spaltenüberschrift.
func (m Model) prdHeader(p *project.Project, l layout) []string {
	var lines []string
	if p.Err != nil {
		lines = append(lines, styleError.Render(" "+firstLine(p.Err.Error())))
	}
	inner := l.rightWidth - 2
	colPRD := max(10, inner-2-colPhase-colPort-colCost-colBranch-4)
	lines = append(lines, styleMuted.Render("  "+padRight("PRD · "+m.prdSort.label(), colPRD)+" "+
		padRight("Phase", colPhase)+" "+padLeft("Port", colPort)+" "+padRight("Branch", colBranch)+" "+padLeft("Kosten", colCost)))
	return lines
}

// minPRDRows ist die Zahl der PRD-Zeilen, die der Docker-Block mindestens frei lässt.
const minPRDRows = 3

// prdFooter liefert die Zeilen unter der PRD-Liste: Trennlinie, ein Docker-Service pro Zeile und
// die Dev-Zeile. Bleiben dafür weniger als minPRDRows PRD-Zeilen, zeigt prdFooter die kurze
// Docker-Zeile.
func (m Model) prdFooter(p *project.Project, l layout) []string {
	inner := l.rightWidth - 2
	var lines []string
	for _, line := range m.docker.dockerBlock(p.Path, inner-1) {
		lines = append(lines, " "+line)
	}
	dev, hasDev := m.docker.devLine(p.Path)
	if hasDev {
		lines = append(lines, " "+dev)
	}
	if len(lines) == 0 {
		return nil
	}
	free := l.topHeight - 2 - len(m.prdHeader(p, l)) - 1
	if free-len(lines) < minPRDRows {
		lines = nil
		if line, ok := m.docker.dockerLine(p.Path, inner-1); ok {
			lines = append(lines, " "+line)
		}
		if hasDev {
			lines = append(lines, " "+dev)
		}
	}
	sep := styleMuted.Render(strings.Repeat("─", max(0, inner)))
	return append([]string{sep}, lines...)
}

func (m Model) prdHeaderRows(p *project.Project, l layout) int {
	return len(m.prdHeader(p, l))
}

func (m Model) prdRows(p *project.Project, l layout) int {
	return max(0, l.topHeight-2-m.prdHeaderRows(p, l)-len(m.prdFooter(p, l)))
}

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	if m.mode == modeEvent {
		return m.viewEvent()
	}
	if m.mode == modeDocker || (m.mode == modeConfirm && m.returnMode == modeDocker) {
		return m.viewDocker()
	}
	l := m.layout()
	parts := []string{}
	if l.topHeight >= 4 {
		top := []string{m.viewProjects(l), m.viewPRDs(l)}
		if l.sideWidth > 0 {
			var side []string
			if l.notifHeight > 0 {
				side = append(side, m.viewNotifications(l))
			}
			if l.issuesHeight > 0 {
				side = append(side, m.viewIssues(l))
			}
			top = append(top, lipgloss.JoinVertical(lipgloss.Left, side...))
		}
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, top...))
	}
	bottom := m.viewDetails(l)
	if m.selectedRun() != nil {
		bottom = m.viewLog(l)
	}
	parts = append(parts, bottom)
	body := strings.Join(parts, "\n")
	footer := m.viewFooter(l)
	var fg string
	switch {
	case m.mode == modeHelp:
		fg = m.viewHelp()
	case m.mode == modeLimits:
		fg = m.viewLimits(time.Now())
	case m.mode == modeTemplate:
		fg = m.viewTemplatePicker()
	case m.mode == modeNewPRD:
		fg = m.viewNewPRD()
	case m.mode == modePRDView && m.prdView != nil:
		fg = m.viewPRDView()
	case m.mode == modeIssueView && m.issueView != nil:
		fg = m.viewIssueView()
	case (m.mode == modeChat || (m.mode == modeConfirm && m.returnMode == modeChat)) && m.chat != nil:
		fg = m.chat.render(m.dialogSize())
	}
	if fg != "" {
		body = overlay(body, fg, m.width)
	}
	return body + "\n" + footer
}

func (m Model) viewProjects(l layout) string {
	rows := l.topHeight - 2
	var lines []string
	if len(m.projects) == 0 && !m.loading {
		lines = append(lines, styleMuted.Render(" keine Projekte"))
	}
	start, end := scrollWindow(m.projCursor, len(m.projects), rows)
	for i := start; i < end; i++ {
		p := m.projects[i]
		name := p.Name()
		if p.Err != nil {
			name = styleError.Render(name + " ✗")
		}
		suffix := ""
		if sym := m.docker.projectSymbol(p.Path); sym != "" {
			suffix += " " + sym
		}
		if n := m.runs.activeRuns(p.ID); n > 0 {
			suffix += fmt.Sprintf(" %d●", n)
		}
		if n := len(m.notifications(p.ID)); n > 0 {
			suffix += " " + lipgloss.NewStyle().Foreground(colorFocus).Render(fmt.Sprintf("%d✉", n))
		}
		if n := len(m.projectIssues(p.ID)); n > 0 {
			suffix += " " + styleMuted.Render(fmt.Sprintf("%d#", n))
		}
		name = padRight(name, max(4, l.leftWidth-4-ansi.StringWidth(suffix))) + suffix
		lines = append(lines, cursorLine(i == m.projCursor, m.focus == focusProjects, name))
	}
	return box("Projekte", lines, l.leftWidth, l.topHeight, m.focus == focusProjects)
}

func (m Model) viewPRDs(l layout) string {
	focused := m.focus == focusPRDs
	p := m.selectedProject()
	if p == nil {
		hint := " Um ein Projekt hinzuzufügen, drücke a."
		if m.loading {
			hint = " Lade Projekte …"
		}
		return box("PRDs", []string{styleMuted.Render(hint)}, l.rightWidth, l.topHeight, focused)
	}

	title := p.Name()
	if p.Branch != "" {
		title += " · " + p.Branch
	}
	total := 0.0
	for k, r := range m.runs {
		if k.projectID == p.ID {
			total += r.Cost()
		}
	}
	if total > 0 {
		title += fmt.Sprintf(" · %.2f USD", total)
	}

	inner := l.rightWidth - 2
	colPRD := max(10, inner-2-colPhase-colPort-colCost-colBranch-4)
	lines := m.prdHeader(p, l)

	if len(p.PRDs) == 0 {
		lines = append(lines, styleMuted.Render("  Keine PRDs unter "+project.PRDDir+"/"))
	}
	rows := m.prdList(p)
	start, end := scrollWindow(m.prdCursor, len(rows), m.prdRows(p, l))
	for i := start; i < end; i++ {
		d := rows[i].prd
		if d == nil {
			cost := ""
			if rows[i].cost > 0 {
				cost = fmt.Sprintf("%.2f $", rows[i].cost)
			}
			// Die Summe steht in der Kostenspalte, wie bei den einzelnen PRDs.
			group := padRight(m.groupLine(rows[i].done), colPRD+1+colPhase+1+colPort+1+colBranch) + " " + padLeft(cost, colCost)
			lines = append(lines, cursorLine(i == m.prdCursor, focused, styleMuted.Render(group)))
			continue
		}
		r := m.runs.get(p, d)
		phase := prdPhase(d, r)
		branch, port, cost := d.Branch, "–", ""
		if r != nil && phase != project.PhaseMerged {
			branch, port = r.Branch, fmt.Sprint(r.Port)
		}
		if r != nil && r.Cost() > 0 {
			cost = fmt.Sprintf("%.2f $", r.Cost())
		}
		if branch == "" {
			branch = "–"
		}
		row := padRight(d.Label(), colPRD) + " " +
			padRight(phaseCell(phase), colPhase) + " " +
			padLeft(port, colPort) + " " +
			padRight(branch, colBranch) + " " +
			padLeft(cost, colCost)
		lines = append(lines, cursorLine(i == m.prdCursor, focused, row))
	}
	// Der Docker-Block steht am unteren Rand der Box.
	if footer := m.prdFooter(p, l); len(footer) > 0 {
		for len(lines) < l.topHeight-2-len(footer) {
			lines = append(lines, "")
		}
		lines = append(lines, footer...)
	}
	return box(title, lines, l.rightWidth, l.topHeight, focused)
}

func (m Model) viewDetails(l layout) string {
	d := m.selectedPRD()
	if d == nil {
		return box("Details", nil, l.width, l.detailHeight, false)
	}
	lines := m.detailHeader(d)
	for _, line := range strings.Split(m.detail.View(), "\n") {
		lines = append(lines, " "+line)
	}
	title := "Details: " + d.Label()
	if m.detail.TotalLineCount() > m.detail.VisibleLineCount() {
		title += fmt.Sprintf(" · %d %%", int(m.detail.ScrollPercent()*100))
	}
	return box(title, lines, l.width, l.detailHeight, m.focus == focusLog)
}

// detailHeader liefert die feste Zeile über der Detailansicht: den Titel.
func (m Model) detailHeader(d *project.PRD) []string {
	if d.Title == "" {
		return nil
	}
	return []string{" " + styleSelected.Render(d.Title)}
}

// detailMeta liefert Status, Datei, Git-Zustand und Erstellzeit untereinander. Die Zeilen scrollen
// mit dem Detail-Text.
func detailMeta(d *project.PRD) []string {
	status := d.Status.Value
	if status == "" {
		status = styleError.Render("keine Statuszeile gefunden")
	}
	git := "getrackt"
	if !d.Tracked {
		git = "ungetrackt im Hauptordner"
	}
	lines := []string{
		metaLabel("Status") + status,
		metaLabel("Datei") + d.Path,
		metaLabel("Git") + git,
	}
	if !d.Created.IsZero() {
		lines = append(lines, metaLabel("Erstellt")+d.Created.Local().Format("2006-01-02 15:04"))
	}
	return lines
}

// metaLabel beschriftet eine Zeile von detailMeta.
func metaLabel(s string) string {
	return styleMuted.Render(padRight(s, 10))
}

func (m Model) viewFooter(l layout) string {
	var status string
	switch m.mode {
	case modeAddProject:
		status = m.input.View()
	case modeConfirmRemove:
		p := m.selectedProject()
		status = fmt.Sprintf(" %s aus der Liste entfernen? Die Dateien bleiben erhalten. (j/n)",
			styleSelected.Render(config.ContractPath(p.Path)))
	case modeConfirm:
		status = " " + styleSelected.Render(m.confirm.question)
	default:
		switch {
		case m.message != "" && m.messageErr:
			status = styleError.Render(" " + m.message)
		case m.message != "":
			status = styleOK.Render(" " + m.message)
		case m.loading:
			status = styleMuted.Render(" Lade Projekte …")
		}
	}
	lines := []string{ansi.Truncate(status, l.width, "…")}
	if len(m.candidates) > 0 && m.mode == modeAddProject {
		lines = append(lines, styleMuted.Render(ansi.Truncate(" "+strings.Join(m.candidates, "  "), l.width, "…")))
	}
	// Die Limits stehen rechts, die Tastenhilfe kürzt sich davor.
	limits := m.limitsBadge(time.Now())
	help := ansi.Truncate(m.helpLine(), max(0, l.width-ansi.StringWidth(limits)-1), "…")
	gap := max(1, l.width-ansi.StringWidth(help)-ansi.StringWidth(limits))
	lines = append(lines, help+strings.Repeat(" ", gap)+limits)
	return strings.Join(lines, "\n")
}

// viewTemplatePicker zeigt das Fenster „Template wählen“: die Templates untereinander, jedes mit
// seiner Beschreibung darunter.
func (m Model) viewTemplatePicker() string {
	var lines []string
	for i, t := range prdtemplate.All {
		if i > 0 {
			lines = append(lines, "")
		}
		if i == m.template {
			lines = append(lines, styleSelected.Render("> "+t.Label))
		} else {
			lines = append(lines, styleMuted.Render("  "+t.Label))
		}
		lines = append(lines, styleMuted.Render("    "+t.Description))
	}
	w, _ := m.dialogSize()
	return dialog("Template wählen", lines, w, len(lines)+4)
}

// viewNewPRD zeigt das Fenster „Neues PRD“ mit dem Eingabefeld für die Beschreibung.
func (m Model) viewNewPRD() string {
	lines := []string{styleMuted.Render("Beschreibe das Feature in einem Satz."), "", m.input.View()}
	w, _ := m.dialogSize()
	return dialog("Neues PRD · "+prdtemplate.All[m.template].Label, lines, w, len(lines)+4)
}

func (m Model) helpLine() string {
	var keys [][2]string
	switch m.mode {
	case modeAddProject:
		keys = [][2]string{{"Tab", "vervollständigen"}, {"Enter", "hinzufügen"}, {"Esc", "abbrechen"}}
	case modeConfirmRemove:
		keys = [][2]string{{"j", "entfernen"}, {"n", "abbrechen"}}
	case modeConfirm:
		keys = [][2]string{{"j", "ja"}, {"n", "nein"}}
	case modeTemplate:
		keys = [][2]string{{"↑ ↓", "Template wählen"}, {"Enter", "weiter"}, {"Esc", "abbrechen"}}
	case modeNewPRD:
		keys = [][2]string{{"Enter", "Chat mit Claude öffnen"}, {"Esc", "abbrechen"}}
	case modeChat:
		keys = [][2]string{{"Enter", "senden"}, {"Alt+Enter", "neue Zeile"}, {"PgUp/PgDn", "scrollen"}, {"Ctrl+R", "wiederholen"},
			{"Ctrl+X", "beenden"}, {"Esc", "ausblenden"}}
	case modePRDView:
		keys = [][2]string{{"j/k", "scrollen"}, {"PgUp/PgDn", "Seite"}, {"g/G", "Anfang/Ende"}, {"o", "Editor"}, {"Esc", "schließen"}}
	case modeIssueView:
		keys = [][2]string{{"j/k", "scrollen"}, {"PgUp/PgDn", "Seite"}, {"g/G", "Anfang/Ende"}, {"o", "Browser"}, {"n", "PRD"},
			{"Esc", "schließen"}}
	default:
		keys = m.normalKeys()
	}
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = styleKey.Render(k[0]) + " " + k[1]
	}
	return " " + strings.Join(parts, styleMuted.Render(" · "))
}

// normalKeys liefert die Tasten der Fußzeile im Normalmodus. Sie zeigt nur, was im aktiven Fokus
// und im Zustand des gewählten PRDs wirkt. Alle Tasten bleiben aus jedem Fokus erreichbar, die
// vollständige Liste steht in der Hilfe.
func (m Model) normalKeys() [][2]string {
	// Die Zeile wird bei schmalem Terminal gekürzt, Hilfe und Beenden stehen deshalb vorn.
	keys := [][2]string{{"?", "Hilfe"}, {"q", "beenden"}}
	switch m.focus {
	case focusLog:
		return append(keys, [][2]string{{"j/k", "scrollen"}, {"Enter", "Ereignis"}, {"G", "Ende"}, {"l", "Log groß"}}...)
	case focusNotifications:
		return append(keys, [][2]string{{"j/k", "wählen"}, {"Enter", "öffnen"}, {"X", "beenden"}}...)
	case focusIssues:
		return append(keys, [][2]string{{"j/k", "wählen"}, {"Enter", "ansehen"}, {"o", "Browser"}, {"n", "PRD"}}...)
	case focusPRDs:
		keys = append(keys, m.prdKeys()...)
	default:
		keys = append(keys, [2]string{"a", "Projekt"})
		if m.selectedProject() != nil {
			keys = append(keys, [][2]string{{"d", "entfernen"}, {"D", "Docker"}, {"n", "neues PRD"}}...)
		}
	}
	if m.hasNotifications() {
		keys = append(keys, [2]string{"N", "Rückfragen"})
	}
	return keys
}

// prdKeys liefert die Tasten für das gewählte PRD. Die Bedingungen sind dieselben wie in den
// Aktionen, damit die Fußzeile keine Taste zeigt, die nur eine Fehlermeldung bringt.
func (m Model) prdKeys() [][2]string {
	var keys [][2]string
	if m.selectedProject() != nil {
		keys = append(keys, [2]string{"n", "neues PRD"})
	}
	if d := m.selectedPRD(); d != nil {
		r := m.selectedRun()
		keys = append(keys, [2]string{"v", "ansehen"})
		if startBlocked(d, r) == "" {
			if r != nil {
				keys = append(keys, [2]string{"s", "fortsetzen"})
			} else {
				keys = append(keys, [2]string{"s", "starten"})
			}
		}
		if mergeBlocked(d, r) == "" {
			keys = append(keys, [2]string{"m", "mergen"})
		}
		if stopBlocked(d, r) == "" {
			keys = append(keys, [2]string{"x", "abbrechen"})
		}
		if discardBlocked(d, r) == "" {
			keys = append(keys, [2]string{"X", "verwerfen"})
		}
		if r, ok := m.worktreeRun(); ok {
			if lastSession(r) != "" {
				keys = append(keys, [2]string{"o", "Sitzung"})
			}
			keys = append(keys, [][2]string{{"p", "dev"}, {"w", "Shell"}}...)
		}
	}
	rows := m.prdList(m.selectedProject())
	for _, row := range rows {
		if row.prd == nil {
			keys = append(keys, [2]string{"e", "erledigte"})
		}
	}
	if len(rows) > 1 {
		keys = append(keys, [2]string{"S", "sortieren"})
	}
	return keys
}

// hasNotifications meldet, ob ein Projekt eine Benachrichtigung hat. N öffnet die älteste.
func (m Model) hasNotifications() bool {
	for _, p := range m.projects {
		if len(m.notifications(p.ID)) > 0 {
			return true
		}
	}
	return false
}

func cursorLine(selected, focused bool, text string) string {
	if !selected {
		return "  " + text
	}
	if focused {
		return selMark + lipgloss.NewStyle().Foreground(colorFocus).Render(">") + " " + styleSelected.Render(text)
	}
	return styleMuted.Render(">") + " " + text
}

func phaseCell(p project.Phase) string {
	cell := p.Symbol() + " " + p.Label()
	switch p {
	case project.PhaseMerged:
		return styleOK.Render(cell)
	case project.PhaseError, project.PhaseAborted:
		return styleError.Render(cell)
	case project.PhaseUnknown:
		return styleMuted.Render(cell)
	}
	return cell
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// viewLog zeigt Zustand und Log des gewählten Laufs.
func (m Model) viewLog(l layout) string {
	d := m.selectedPRD()
	r := m.selectedRun()
	focused := m.focus == focusLog
	summary := runSummary(runKey{m.selectedProject().ID, d.Name}, r)
	if pid := devserver.PID(m.selectedProject().ID, d.Name); pid != 0 {
		summary += styleMuted.Render("  ·  ") + styleOK.Render(fmt.Sprintf("dev :%d", r.Port))
	}
	lines := []string{" " + summary}
	rows := l.logRows()
	if len(m.logLines) == 0 {
		lines = append(lines, styleMuted.Render(" Noch keine Log-Zeilen"))
	}
	start, end := scrollWindow(m.logCursor, len(m.logLines), rows)
	if m.logFollow {
		start, end = max(0, len(m.logLines)-rows), len(m.logLines)
	}
	for i := start; i < end; i++ {
		ll := m.logLines[i]
		text := styleMuted.Render(ll.time) + " " + logStyle(ll.kind).Render(ll.text)
		lines = append(lines, cursorLine(focused && i == m.logCursor, focused, text))
	}
	return box("Log: "+d.Label(), lines, l.width, l.detailHeight, focused)
}

// runSummary fasst Phase, Port, Kosten und Hinweise eines Laufs in einer Zeile zusammen.
func runSummary(k runKey, r *state.Run) string {
	phase := r.Effective()
	parts := []string{phaseCell(phase)}
	if phase != project.PhaseMerged && r.Port > 0 {
		parts = append(parts, fmt.Sprintf("Port %d", r.Port))
	}
	if cost := r.Cost(); cost > 0 {
		parts = append(parts, fmt.Sprintf("%.2f USD", cost))
	}
	if r.Running() {
		parts = append(parts, fmt.Sprintf("PID %d", r.PID))
	}
	out := strings.Join(parts, styleMuted.Render("  ·  "))
	if r.Error != nil && phase == project.PhaseError {
		out += styleMuted.Render("  ·  ") + styleError.Render(firstLine(*r.Error))
	}
	if r.Note != "" {
		out += styleMuted.Render("  ·  ") + r.Note
	}
	// Ein Runner, der ohne x endet, hinterlässt den Grund nur in seiner Ausgabe.
	if phase == project.PhaseAborted && r.Phase != project.PhaseAborted {
		out += styleMuted.Render("  ·  ") + styleError.Render("Runner endete unerwartet, Ausgabe: "+
			config.ContractPath(state.RunnerLogPath(k.projectID, k.prdName)))
	}
	return out
}

func logStyle(k logKind) lipgloss.Style {
	switch k {
	case logRunner:
		return styleMuted
	case logError:
		return styleError
	case logStep:
		return styleSelected
	}
	return lipgloss.NewStyle()
}

// viewEvent zeigt ein Log-Ereignis über die ganze Höhe.
func (m Model) viewEvent() string {
	all := strings.Split(m.event, "\n")
	rows := max(1, m.height-3)
	top := min(m.eventTop, max(0, len(all)-rows))
	lines := all[top:min(len(all), top+rows)]
	for i, line := range lines {
		lines[i] = " " + line
	}
	help := " " + styleKey.Render("j/k") + " scrollen" + styleMuted.Render(" · ") + styleKey.Render("Esc") + " schließen"
	return box("Ereignis", lines, m.width, m.height-1, true) + "\n" + help
}

// padLeft richtet s rechtsbündig auf w Zellen aus.
func padLeft(s string, w int) string {
	s = ansi.Truncate(s, w, "…")
	return strings.Repeat(" ", max(0, w-ansi.StringWidth(s))) + s
}

// helpSections gruppiert die Tasten für die Hilfe.
var helpSections = []struct {
	title string
	keys  [][2]string
}{
	{"Navigation", [][2]string{
		{"↑ ↓ / j k", "Auswahl in der aktiven Liste"},
		{"Tab / ← →", "Fokus: Projekte, PRDs, Benachrichtigungen, Issues, Log oder Details"},
		{"PgUp PgDn", "seitenweise blättern"},
		{"r", "Projekte, PRDs und Issues neu einlesen"},
		{"q", "beenden, laufende Runner und Chats arbeiten weiter"},
	}},
	{"Projekte", [][2]string{
		{"a", "Projekt hinzufügen, Tab vervollständigt den Pfad"},
		{"d", "Projekt aus der Liste entfernen, die Dateien bleiben"},
		{"D", "Docker-Ansicht: U compose up, S compose stop, L Logs"},
	}},
	{"PRDs", [][2]string{
		{"n", "neues PRD im Chat (Opus), vorher Template wählen: Nuxt-Feature oder Einfach"},
		{"N", "älteste Rückfrage eines Chats öffnen"},
		{"v", "PRD vollständig ansehen, darin o für den Editor"},
		{"s", "Umsetzung starten oder beim gescheiterten Schritt fortsetzen"},
		{"m", "Merge freigeben"},
		{"x", "Runner abbrechen, der Worktree bleibt"},
		{"X", "verwerfen: Datenbanken, Worktree und Branch löschen"},
		{"o", "letzte Claude-Sitzung im Worktree öffnen"},
		{"p", "Dev-Server im Worktree starten oder stoppen"},
		{"w", "Shell im Worktree öffnen"},
		{"e / Enter", "erledigte PRDs auf- und zuklappen"},
		{"S", "sortieren nach Name oder Erstellzeit, neueste zuerst"},
	}},
	{"Chat", [][2]string{
		{"Esc", "ausblenden, die Runde läuft weiter"},
		{"Ctrl+X", "Chat beenden"},
		{"Ctrl+R", "nach einem Fehler wiederholen"},
		{"X", "in „Benachrichtigungen“: Chat beenden"},
	}},
	{"Issues", [][2]string{
		{"Enter", "Issue im Fenster ansehen"},
		{"o", "Issue im Browser öffnen"},
		{"n", "PRD-Chat aus dem Issue starten"},
	}},
	{"Claude", [][2]string{
		{"u", "Limits und Verbrauch, r aktualisiert mit einer kurzen Anfrage"},
	}},
	{"Log", [][2]string{
		{"l", "Log in voller Höhe ein- und ausschalten"},
		{"G", "ans Ende springen und neuen Zeilen folgen"},
		{"Enter", "vollständiges Ereignis anzeigen"},
	}},
}

func (m Model) viewHelp() string {
	var lines []string
	for i, sec := range helpSections {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, styleSelected.Render(sec.title))
		for _, k := range sec.keys {
			lines = append(lines, "  "+styleKey.Render(padRight(k[0], 12))+" "+k[1])
		}
	}
	lines = append(lines, "", styleMuted.Render("Eine beliebige Taste schließt die Hilfe."))
	width := min(m.width-4, 72)
	return dialog("Hilfe", lines, width, min(m.height-2, len(lines)+4))
}
