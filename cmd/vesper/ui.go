package main

// ui.go — ANSI palette, tables and console output helpers.
//
// Recreated during the Vesper remodel: the original palette file was
// lost in a repo-wide regex cleanup that mangled ANSI escape literals.
// Colors are raw ANSI strings so they interpolate inside format strings
// (e.g. printInfo("%stext%s", cWhite, ansiR)).

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// ─── Raw ANSI fragments ───────────────────────────────────────────────────────

const (
	ansiR  = "\033[0m" // reset
	ansiB  = "\033[1m" // bold
	ansiIt = "\033[3m" // italic
)

// ─── Palette (256-color foregrounds as raw ANSI strings) ─────────────────────

var (
	cSuccess = fg(42)  // green
	cInfo    = fg(80)  // cyan-blue
	cWhite   = fg(252) // near-white
	cMuted   = fg(243) // gray
	cCyan    = fg(51)  // bright cyan
	cPrimary = fg(135) // violet
	cWarn    = fg(214) // orange-yellow
	cOrange  = fg(208) // orange
	cDanger  = fg(203) // red
)

// banner gradient (red → orange → yellow and back)
var (
	g1 = fg(203)
	g2 = fg(209)
	g3 = fg(214)
	g4 = fg(214)
	g5 = fg(209)
	g6 = fg(203)
)

func fg(code int) string { return "\033[38;5;" + itoa(code) + "m" }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [4]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// ─── Line helpers ─────────────────────────────────────────────────────────────

// printInfo writes an informational line to the console output.
func printInfo(format string, args ...interface{}) {
	fmt.Fprintf(ConsoleOut, "  %s~%s %s\n", cInfo, ansiR, fmt.Sprintf(format, args...))
}

// printOK writes a success line to the console output.
func printOK(format string, args ...interface{}) {
	fmt.Fprintf(ConsoleOut, "  %s+%s %s\n", cSuccess, ansiR, fmt.Sprintf(format, args...))
}

// printWarn writes a warning line to the console output.
func printWarn(format string, args ...interface{}) {
	fmt.Fprintf(ConsoleOut, "  %s!%s %s\n", cWarn, ansiR, fmt.Sprintf(format, args...))
}

// printErr writes an error line to ErrOut (stderr, or the shared
// console buffer when running behind the web terminal).
func printErr(format string, args ...interface{}) {
	fmt.Fprintf(ErrOut, "  %s✗%s %s\n", cDanger, ansiR, fmt.Sprintf(format, args...))
}

// ─── Block helpers ────────────────────────────────────────────────────────────

// printSection writes a section header with a rule.
func printSection(title string) {
	rule := strings.Repeat("─", clampInt(80-visualLen(title)-4, 8, 80))
	fmt.Fprintf(ConsoleOut, "\n %s── %s %s\n", cPrimary, cWhite+ansiB+title+ansiR, cMuted+rule+ansiR+"\n")
}

// printPanel writes a titled, rounded panel with body text.
func printPanel(title, body string) {
	width := 0
	for _, l := range strings.Split(body, "\n") {
		if v := visualLen(l); v > width {
			width = v
		}
	}
	width = clampInt(width+2, 20, 100)
	fmt.Fprintln(ConsoleOut)
	fmt.Fprintf(ConsoleOut, "  %s╭─ %s %s╮%s\n",
		cMuted, cPrimary+ansiB+strings.ToUpper(title)+ansiR,
		cMuted, ansiR)
	for _, l := range strings.Split(body, "\n") {
		pad := strings.Repeat(" ", clampInt(width-visualLen(l), 0, 200))
		fmt.Fprintf(ConsoleOut, "  %s│%s %s%s %s│%s\n", cMuted, ansiR, l, pad, cMuted, ansiR)
	}
	fmt.Fprintf(ConsoleOut, "  %s╰%s╯%s\n", cMuted, strings.Repeat("─", width+1), ansiR)
}

// bannerLines returns the VESPER ASCII art (ANSI Shadow font), uncolored.
// Every surface (console, dashboard, --help, version) renders the same
// block so the identity stays consistent across modes.
func bannerLines() []string {
	return []string{
		`██╗   ██╗███████╗███████╗██████╗ ███████╗██████╗ `,
		`██║   ██║██╔════╝██╔════╝██╔══██╗██╔════╝██╔══██╗`,
		`██║   ██║█████╗  ███████╗██████╔╝█████╗  ██████╔╝`,
		`╚██╗ ██╔╝██╔══╝  ╚════██╗██╔═══╝ ██╔══╝  ██╔══██╗`,
		` ╚████╔╝ ███████╗███████╗██║     ███████╗██║  ██║`,
		`  ╚═══╝  ╚══════╝╚══════╝╚═╝     ╚══════╝╚═╝  ╚═╝`,
	}
}

// printBigBanner renders the VESPER ASCII banner with its dusk gradient
// (red → orange → amber and back) and the version tag.
func printBigBanner() {
	gradient := []string{cDanger, cOrange, cWarn, cWarn, cOrange, cDanger}
	fmt.Fprintln(ConsoleOut)
	for i, l := range bannerLines() {
		fmt.Fprintln(ConsoleOut, "  "+gradient[i]+l+ansiR)
	}
	fmt.Fprintf(ConsoleOut, "  %s  semi-autonomous red team platform%s  %sv%s%s\n\n",
		cMuted, ansiR, cPrimary, version, ansiR)
}

// ─── Widgets ──────────────────────────────────────────────────────────────────

// statusTag renders a colored bracket tag for phases/statuses.
func statusTag(s string) string {
	var color string
	switch strings.ToLower(s) {
	case "running", "active", "online", "infected", "success", "completed":
		color = cSuccess
	case "pending", "planning", "queued", "created", "partial":
		color = cWarn
	case "failed", "error", "offline", "lost", "dead":
		color = cDanger
	case "paused", "idle", "unknown", "":
		color = cMuted
	default:
		color = cInfo
	}
	return color + "[" + strings.ToUpper(s) + "]" + ansiR
}

// killChainOrder returns the index of phase within the canonical kill
// chain (Recon → Weaponization → Delivery → Exploitation → Installation
// → C2 → Objectives); unknown phases sort last.
func killChainOrder(phase string) int {
	order := []string{"recon", "weaponization", "delivery", "exploitation",
		"installation", "c2", "objectives"}
	p := strings.ToLower(strings.TrimSpace(phase))
	for i, o := range order {
		if p == o || strings.HasPrefix(p, o) {
			return i
		}
	}
	return len(order)
}

// maskPassword hides all but the first and last character of a secret.
func maskPassword(pw string) string {
	switch n := len(pw); {
	case n == 0:
		return "(empty)"
	case n <= 4:
		return strings.Repeat("•", n)
	default:
		return pw[:1] + strings.Repeat("•", n-2) + pw[n-1:]
	}
}

// pbar renders a progress bar of the given visual width.
func pbar(progress float64, width int) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	filled := int(progress * float64(width))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	pct := fmt.Sprintf("%3.0f%%", progress*100)
	color := cWarn
	if progress >= 1 {
		color = cSuccess
	}
	return color + bar + ansiR + cMuted + " " + pct + ansiR
}

// trunc shortens s to at most n runes, appending an ellipsis when cut.
func trunc(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

// table renders aligned columns with a header row.
type table struct {
	headers []string
	rows    [][]string
	out     io.Writer
}

func newTable(headers ...string) *table {
	return &table{headers: headers, out: ConsoleOut}
}

func (t *table) addRow(cells ...string) { t.rows = append(t.rows, cells) }

func (t *table) render() {
	cols := len(t.headers)
	for _, r := range t.rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	widths := make([]int, cols)
	for i, h := range t.headers {
		widths[i] = visualLen(h)
	}
	for _, r := range t.rows {
		for i, c := range r {
			if v := visualLen(c); v > widths[i] {
				widths[i] = v
			}
		}
	}
	// header
	head := make([]string, cols)
	for i, h := range t.headers {
		head[i] = cPrimary + ansiB + h + strings.Repeat(" ", widths[i]-visualLen(h)) + ansiR
	}
	fmt.Fprintln(t.out, "  "+strings.Join(head, "  "))
	// rule
	rule := make([]string, cols)
	for i := range rule {
		rule[i] = cMuted + strings.Repeat("─", widths[i]) + ansiR
	}
	fmt.Fprintln(t.out, "  "+strings.Join(rule, "  "))
	// rows
	for _, r := range t.rows {
		cells := make([]string, cols)
		for i := range cells {
			cell := ""
			visible := ""
			if i < len(r) {
				cell = r[i]
				visible = stripANSI(cell)
			}
			cells[i] = cell + strings.Repeat(" ", clampInt(widths[i]-visualLen(visible), 0, 200))
		}
		fmt.Fprintln(t.out, "  "+strings.Join(cells, "  "))
	}
	fmt.Fprintln(t.out)
}

// ─── Text utilities ───────────────────────────────────────────────────────────

// visualLen counts printable runes (ANSI-aware approximation).
func visualLen(s string) int {
	return utf8.RuneCountInString(stripANSI(s))
}

// stripANSI removes ANSI escape sequences from s.
func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		if r == '\033' {
			inEsc = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
