package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"

	"github.com/ruby570bocadito/vesper/internal/agent"
	"github.com/ruby570bocadito/vesper/internal/appstate"
	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

// ConsoleOut is the global output writer for all CLI operations (allows WebSocket redirection)
var ConsoleOut io.Writer = os.Stdout

// ErrOut is where errors go (stderr by default; the web terminal redirects
// it into the shared console buffer so browser users see errors too).
var ErrOut io.Writer = os.Stderr

// ─── Console types ────────────────────────────────────────────────────────────

type Console struct {
	reader     *bufio.Reader
	state      *appstate.AppState
	running    bool
	ctx        *ModuleContext
	hideBanner bool
	workspace  string // session-local workspace label
}

type ModuleContext struct {
	Name    string
	Options map[string]string
}

func NewConsole(state *appstate.AppState) *Console {
	return NewConsoleWithReader(state, os.Stdin)
}

func NewConsoleWithReader(state *appstate.AppState, r io.Reader) *Console {
	return &Console{
		reader: bufio.NewReader(r),
		state:  state,
		ctx:    &ModuleContext{Options: make(map[string]string)},
	}
}

// ─── Run loop ─────────────────────────────────────────────────────────────────

func (c *Console) Run() error {
	if !c.hideBanner {
		c.printBanner()
	}
	c.running = true

	// Ctrl+C exits the interactive console cleanly (the web terminal
	// console skips this — the dashboard process owns its own signals).
	if !c.hideBanner {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		go func() {
			if _, ok := <-sigCh; ok {
				c.running = false
				fmt.Fprint(os.Stderr, "\n")
			}
		}()
	}

	for c.running {
		c.PrintPrompt()
		input, err := c.reader.ReadString('\n')
		if err != nil {
			// EOF with a trailing partial line still executes it
			if trimmed := strings.TrimSpace(input); trimmed != "" {
				parts := strings.Fields(trimmed)
				c.dispatch(strings.ToLower(parts[0]), parts[1:])
			}
			break
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		parts := strings.Fields(input)
		cmd := strings.ToLower(parts[0])
		args := parts[1:]
		c.dispatch(cmd, args)
	}
	fmt.Fprintf(ConsoleOut, "\n  %s%s[*]%s Goodbye.\n\n", cPrimary, ansiB, ansiR)
	return nil
}

// ─── Prompt ───────────────────────────────────────────────────────────────────

// PrintPrompt writes the interactive prompt to ConsoleOut.
// Exported so the WebSocket handler can resend it to new clients.
func (c *Console) PrintPrompt() {
	module := ""
	if c.ctx.Name != "" {
		module = fmt.Sprintf(" %s(%s%s%s)", cMuted, cOrange, c.ctx.Name, cMuted) + ansiR
	}
	fmt.Fprintf(ConsoleOut, "\n%s%s[vesper]%s%s %s›%s ", cPrimary, ansiB, ansiR, module, cSuccess, ansiR)
}

// ─── Banner ───────────────────────────────────────────────────────────────────

func (c *Console) printBanner() {
	printBigBanner()
	// Safety posture always visible so the operator never forgets the mode.
	if authorized {
		fmt.Fprintf(ConsoleOut, "  %sPOSTURE%s  %s● AUTHORIZED ENGAGEMENT%s %s— config safety limits apply%s\n\n",
			cInfo, ansiR, cSuccess, ansiR, cMuted, ansiR)
	} else {
		fmt.Fprintf(ConsoleOut, "  %sPOSTURE%s  %s● LAB-ONLY%s %s— sandboxed file ops, loopback bind only%s\n\n",
			cInfo, ansiR, cWarn, ansiR, cMuted, ansiR)
	}
	printPanel("INTERACTIVE CONSOLE", fmt.Sprintf(
		`  Type %shelp%s for a list of commands.
  Use %suse <module>%s to load an exploit or auxiliary module.
  Use %ssuggest%s to get AI-powered attack recommendations.
  [Ctrl+C] or exit quits the console`,
		cSuccess, ansiR,
		cSuccess, ansiR,
		cSuccess, ansiR,
	))
	fmt.Fprintln(ConsoleOut)

	// Quick stats
	agents := c.state.GetAgents()
	hosts := c.state.GetHosts()
	camps := c.state.Orchestrator.ListCampaigns()
	online := 0
	for _, a := range agents {
		if a.Status == types.AgentStatusOnline || a.Status == types.AgentStatusActive {
			online++
		}
	}
	fmt.Fprintf(ConsoleOut, "  %sSessions%s %-4d  %sHosts%s %-4d  %sCampaigns%s %-4d  %sBridge%s %s\n\n",
		cInfo, ansiR, online,
		cInfo, ansiR, len(hosts),
		cInfo, ansiR, len(camps),
		cInfo, ansiR, bridgeStatus(c.state.Bridge.Connected()),
	)
}

// ─── Command dispatcher ───────────────────────────────────────────────────────

func (c *Console) dispatch(cmd string, args []string) {
	// safety: kill_switch is handled before any command routing
	joined := strings.TrimSpace(cmd + " " + strings.Join(args, " "))
	if checkConsoleKillSwitch(joined) {
		return
	}
	switch cmd {
	// Navigation
	case "help", "?":
		c.cmdHelp()
	case "exit", "quit", "q":
		c.running = false
	case "version":
		fmt.Fprintf(ConsoleOut, "  Vesper v%s  Go %s  %s%s\n", version, runtime.Version()[2:], cMuted, ansiR)
	case "clear", "cls":
		fmt.Fprint(ConsoleOut, "\033[H\033[2J")

	// Campaigns
	case "campaign", "campaigns":
		c.cmdCampaign(args)

	// Sessions
	case "sessions":
		c.cmdSessions(args)
	case "session":
		c.cmdSessions(args)

	// Modules
	case "use":
		c.cmdUse(args)
	case "search":
		c.cmdSearch(args)
	case "show":
		c.cmdShow(args)
	case "set":
		c.cmdSet(args)
	case "unset":
		c.cmdUnset(args)
	case "run", "exploit", "execute":
		c.cmdExploit(args)
	case "back":
		c.cmdBack()
	case "info":
		c.cmdInfo(args)
	case "options":
		c.cmdShow(nil)

	// AI
	case "suggest":
		c.cmdSuggest(args)
	case "ai":
		c.cmdAI(args)
	case "accept":
		c.cmdAccept(args)
	case "reject":
		c.cmdReject(args)
	case "auto":
		c.cmdAutoMode(args)

	// Data
	case "hosts":
		c.cmdHosts()
	case "services":
		c.cmdServices()
	case "creds", "credentials":
		c.cmdCreds()
	case "vulns", "vulnerabilities":
		c.cmdVulns()
	case "db_status":
		c.cmdDBStatus()

	// Operations
	case "killchain", "kill_chain", "kc":
		c.cmdKillChain(args)
	case "workspace", "ws":
		c.cmdWorkspace(args)
	case "listeners":
		c.cmdListeners(args)
	case "builder", "generate":
		c.cmdBuilder(args)
	case "lab":
		c.cmdLab(args)
	case "webhook":
		c.cmdWebhook(args)

	default:
		printErr("Unknown command: %s%s%s  — type %shelp%s", cWhite, cmd, ansiR, cSuccess, ansiR)
	}
}

// ─── help ─────────────────────────────────────────────────────────────────────

func (c *Console) cmdHelp() {
	fmt.Fprintf(ConsoleOut, "\n  %s%s━━  Vesper COMMAND REFERENCE  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n",
		cPrimary, ansiB, ansiR)

	groups := []struct {
		name     string
		commands [][2]string
	}{
		{"CAMPAIGN", [][2]string{
			{"campaign [start|list|pause|resume]", "Manage red team operations"},
			{"killchain", "Visual kill chain progress"},
			{"workspace [name]", "Session-local workspace label"},
		}},
		{"SESSIONS & AGENTS", [][2]string{
			{"sessions [-i <id>]", "List sessions (-i shows session info)"},
		}},
		{"MODULES", [][2]string{
			{"use <module>", "Load an exploit / auxiliary module"},
			{"search <term>", "Search module database"},
			{"show options", "Display current module options"},
			{"set <option> <value>", "Configure module parameter"},
			{"run / exploit", "Execute loaded module"},
			{"back", "Unload current module"},
			{"info <module>", "Show module details"},
		}},
		{"RECONNAISSANCE", [][2]string{
			{"hosts", "Show discovered hosts"},
			{"services", "Show discovered services"},
			{"vulns", "List identified vulnerabilities"},
			{"creds", "Show captured credentials"},
		}},
		{"AI ENGINE", [][2]string{
			{"suggest", "Get ranked attack recommendations"},
			{"ai <prompt>", "Query AI assistant"},
			{"auto [on|off]", "Toggle autonomous execution"},
			{"accept <#|id>", "Approve an AI recommendation"},
			{"reject <#|id>", "Dismiss an AI recommendation"},
		}},
		{"OPERATIONS", [][2]string{
			{"builder [options]", "Generate implant payloads"},
			{"listeners [add|list]", "Manage C2 transport listeners"},
		}},
		{"SYSTEM", [][2]string{
			{"lab [up|down|status]", "Docker lab environment"},
			{"db_status", "Database connection info"},
			{"webhook [on|off]", "Notification webhooks"},
			{"clear", "Clear screen"},
			{"exit / quit", "Exit console"},
		}},
	}

	for _, g := range groups {
		fmt.Fprintf(ConsoleOut, "  %s%s%s\n", cPrimary+ansiB, g.name, ansiR)
		for _, cmd := range g.commands {
			fmt.Fprintf(ConsoleOut, "    %s%-44s%s %s%s%s\n",
				cSuccess, cmd[0], ansiR,
				cMuted, cmd[1], ansiR)
		}
		fmt.Fprintln(ConsoleOut)
	}
}

// ─── campaign ─────────────────────────────────────────────────────────────────

func (c *Console) cmdCampaign(args []string) {
	if len(args) > 0 && args[0] == "start" {
		name, target, goal, profile := "default", "10.0.0.0/24", "domain_admin", "balanced"
		auto := false
		for i, a := range args {
			switch a {
			case "--name":
				if i+1 < len(args) {
					name = args[i+1]
				}
			case "--target":
				if i+1 < len(args) {
					target = args[i+1]
				}
			case "--goal":
				if i+1 < len(args) {
					goal = args[i+1]
				}
			case "--profile":
				if i+1 < len(args) {
					profile = args[i+1]
				}
			case "--auto":
				auto = true
			}
		}
		cam, err := c.state.Orchestrator.StartCampaign(context.Background(), name, target, goal, profile, auto)
		if err != nil {
			printErr("Failed to start campaign: %v", err)
			return
		}
		printOK("Campaign %s%s%s started  (id=%s)", cWhite+ansiB, cam.Name, ansiR, cMuted+cam.ID+ansiR)
		printInfo("Phase: %s  |  Target: %s%s%s", statusTag(string(cam.Phase)), cCyan, target, ansiR)
		return
	}

	campaigns := c.state.Orchestrator.ListCampaigns()
	printSection("CAMPAIGNS")
	if len(campaigns) == 0 {
		printInfo("No campaigns — use %scampaign start --name <name> --target <scope>%s", cSuccess, ansiR)
		return
	}
	tbl := newTable("ID", "Name", "Phase", "Status", "Agents", "Progress")
	for _, cam := range campaigns {
		tbl.addRow(
			cMuted+trunc(cam.ID, 26)+ansiR,
			cWhite+ansiB+cam.Name+ansiR,
			statusTag(string(cam.Phase)),
			statusTag(string(cam.Status)),
			fmt.Sprintf("%d", cam.AgentCount),
			pbar(cam.Progress, 14),
		)
	}
	tbl.render()
}

// ─── sessions ─────────────────────────────────────────────────────────────────

func (c *Console) cmdSessions(args []string) {
	if len(args) >= 2 && args[0] == "-i" {
		id := args[1]
		for _, a := range c.state.GetAgents() {
			if a.SessionID == id || a.ID == id {
				printPanel("SESSION "+id, fmt.Sprintf(
					`%sHost%s     %s@%s
%sOS%s       %s
%sIP%s       %s
%sStatus%s   %s`,
					cInfo, ansiR, cWhite+a.Username, a.LocalIP+ansiR,
					cInfo, ansiR, a.OS,
					cInfo, ansiR, cCyan+a.LocalIP+ansiR,
					cInfo, ansiR, statusTag(string(a.Status)),
				))
				fmt.Fprintf(ConsoleOut, "  %s[i]%s Interactive shells are not implemented yet — this is read-only session info.\n", cInfo, ansiR)
				return
			}
		}
		printErr("Session %s%s%s not found.", cWhite, id, ansiR)
		return
	}

	agents := c.state.GetAgents()
	printSection("SESSIONS")
	if len(agents) == 0 {
		printInfo("No active sessions.")
		return
	}
	tbl := newTable("ID", "SID", "Host", "OS", "User", "IP", "Status")
	for _, a := range agents {
		tbl.addRow(
			cMuted+trunc(a.ID, 10)+ansiR,
			cSuccess+ansiB+a.SessionID+ansiR,
			cWhite+trunc(a.Hostname, 14)+ansiR,
			trunc(a.OS, 10),
			trunc(a.Username, 12),
			cCyan+a.LocalIP+ansiR,
			statusTag(string(a.Status)),
		)
	}
	tbl.render()
}

// ─── modules ──────────────────────────────────────────────────────────────────

func (c *Console) cmdUse(args []string) {
	if len(args) == 0 {
		printErr("Usage: use <module>")
		return
	}
	modules := c.state.GetModules()
	for _, m := range modules {
		if m.Name == args[0] {
			c.ctx = &ModuleContext{Name: m.Name, Options: defaultOptions(m.Name)}
			fmt.Fprintf(ConsoleOut, "\n  %s%s[+]%s Module: %s%s%s\n", cSuccess, ansiB, ansiR, cWhite+ansiB, m.Name, ansiR)
			fmt.Fprintf(ConsoleOut, "      %s%s%s\n", cMuted, m.Description, ansiR)
			fmt.Fprintf(ConsoleOut, "      %sType:%s %-10s %sCVE:%s %-18s %sRank:%s %s\n",
				cInfo, ansiR, m.Type, cInfo, ansiR, m.CVE, cInfo, ansiR, m.Rank)
			fmt.Fprintf(ConsoleOut, "\n  %sUse %sshow options%s to configure.\n", cMuted, cSuccess+ansiR+cMuted, ansiR)
			c.state.LogAudit("", "", "module_load", "success", m.Name)
			return
		}
	}
	printErr("Module not found: %s%s%s", cWhite, args[0], ansiR)
}

func (c *Console) cmdSearch(args []string) {
	if len(args) == 0 {
		printErr("Usage: search <term>")
		return
	}
	term := strings.Join(args, " ")
	results := c.state.SearchModules(term)
	printSection("SEARCH: " + term)
	if len(results) == 0 {
		printInfo("No modules matching %q", term)
		return
	}
	tbl := newTable("Name", "Type", "CVE", "Rank", "OS")
	for _, m := range results {
		rankColor := cMuted
		switch strings.ToLower(m.Rank) {
		case "excellent", "great":
			rankColor = cSuccess
		case "good":
			rankColor = cInfo
		case "normal":
			rankColor = cWarn
		}
		tbl.addRow(
			cWhite+ansiB+m.Name+ansiR,
			cInfo+m.Type+ansiR,
			m.CVE,
			rankColor+m.Rank+ansiR,
			m.OS,
		)
	}
	tbl.render()
}

func (c *Console) cmdShow(args []string) {
	if c.ctx.Name == "" {
		printErr("No module loaded — use %suse <module>%s first.", cSuccess, ansiR)
		return
	}
	fmt.Fprintf(ConsoleOut, "\n  %s%sModule:%s %s%s%s\n\n", cPrimary, ansiB, ansiR, cWhite+ansiB, c.ctx.Name, ansiR)
	tbl := newTable("Option", "Value", "Description")
	for k, v := range c.ctx.Options {
		display := cSuccess + v + ansiR
		if v == "" {
			display = cMuted + "(not set)" + ansiR
		}
		desc := optionDesc(c.ctx.Name, k)
		tbl.addRow(cInfo+ansiB+k+ansiR, display, cMuted+desc+ansiR)
	}
	tbl.render()
}

func (c *Console) cmdSet(args []string) {
	if c.ctx.Name == "" {
		printErr("No module loaded.")
		return
	}
	if len(args) < 2 {
		printErr("Usage: set <option> <value>")
		return
	}
	key := strings.ToUpper(args[0])
	val := strings.Join(args[1:], " ")
	c.ctx.Options[key] = val
	fmt.Fprintf(ConsoleOut, "  %s%-12s%s → %s%s%s\n", cInfo+ansiB, key, ansiR, cSuccess, val, ansiR)
}

func (c *Console) cmdUnset(args []string) {
	if c.ctx.Name == "" {
		printErr("No module loaded.")
		return
	}
	if len(args) == 0 {
		printErr("Usage: unset <option>")
		return
	}
	key := strings.ToUpper(args[0])
	delete(c.ctx.Options, key)
	printInfo("%s%s%s cleared.", cInfo, key, ansiR)
}

func (c *Console) cmdBack() {
	if c.ctx.Name != "" {
		printInfo("Unloading %s%s%s.", cOrange, c.ctx.Name, ansiR)
		c.ctx = &ModuleContext{Options: make(map[string]string)}
	}
}

func (c *Console) cmdInfo(args []string) {
	name := c.ctx.Name
	if len(args) > 0 {
		name = args[0]
	}
	if name == "" {
		printErr("Usage: info <module>")
		return
	}
	for _, m := range c.state.GetModules() {
		if m.Name == name {
			printPanel("MODULE: "+m.Name, fmt.Sprintf(
				`%sDescription%s  %s
%sType%s         %s
%sCVE%s          %s
%sRank%s         %s
%sPlatform%s     %s`,
				cInfo, ansiR, m.Description,
				cInfo, ansiR, m.Type,
				cInfo, ansiR, m.CVE,
				cInfo, ansiR, m.Rank,
				cInfo, ansiR, m.OS,
			))
			return
		}
	}
	printErr("Module not found: %s", name)
}

// ─── exploit / run ────────────────────────────────────────────────────────────

func (c *Console) cmdExploit(args []string) {
	if c.ctx.Name == "" {
		printErr("No module loaded — use %suse <module>%s first.", cSuccess, ansiR)
		return
	}

	rh := c.targetOpt()

	fmt.Fprintf(ConsoleOut, "\n  %s%s[*]%s Executing %s%s%s\n", cPrimary, ansiB, ansiR, cWhite+ansiB, c.ctx.Name, ansiR)
	if rh != "" {
		printInfo("Target: %s%s%s", cCyan+ansiB, rh, ansiR)
	}

	if !c.state.Bridge.Connected() {
		printErr("Bridge offline — module NOT executed.")
		printInfo("Vesper never simulates results. Start the bridge with %svesper --dashboard%s and retry.", cSuccess, ansiR)
		return
	}

	ctx := context.Background()
	camps := c.state.Orchestrator.ListCampaigns()
	campaignID := ""
	if len(camps) > 0 {
		campaignID = camps[0].ID
	}

	params := map[string]interface{}{
		"target":  rh,
		"module":  c.ctx.Name,
		"options": c.ctx.Options,
	}

	// "group/function" references map onto handler groups; bare names
	// hit the bridge's inline module registry.
	var resp *agent.BridgeResponse
	var err error
	if i := strings.Index(c.ctx.Name, "/"); i > 0 {
		resp, err = c.state.Bridge.Call(ctx, c.ctx.Name[:i], c.ctx.Name[i+1:], params)
	} else {
		resp, err = c.state.Bridge.Call(ctx, c.ctx.Name, "", params)
	}

	if err != nil {
		printErr("Bridge error: %v", err)
		c.state.LogAudit("", campaignID, "exploit", "bridge_error", c.ctx.Name)
		return
	}
	if resp == nil || !resp.Success {
		msg := "module rejected the request"
		if resp != nil && resp.Error != "" {
			msg = resp.Error
		}
		printErr("Module failed: %s", msg)
		c.state.LogAudit("", campaignID, "exploit", "failed", c.ctx.Name)
		return
	}

	printOK("Module executed via bridge.")
	if result, ok := resp.Result["output"]; ok {
		fmt.Fprintf(ConsoleOut, "  %sOutput:%s %v\n", cMuted, ansiR, result)
	}
	if res, ok := resp.Result["result"]; ok {
		if out, err := json.MarshalIndent(res, "  ", "  "); err == nil {
			fmt.Fprintf(ConsoleOut, "  %s%s\n", cMuted, string(out))
		}
	}
	if sid, ok := resp.Result["session"]; ok {
		printOK("Session %s%v%s opened.", cSuccess+ansiB, sid, ansiR)
	}
	c.state.LogAudit("", campaignID, "exploit", "success", c.ctx.Name)
}

// targetOpt returns the configured target checking the canonical
// option and its msf-style aliases.
func (c *Console) targetOpt() string {
	for _, k := range []string{"TARGET", "RHOSTS", "RHOST"} {
		if v := strings.TrimSpace(c.ctx.Options[k]); v != "" {
			return v
		}
	}
	return ""
}

// ─── AI ───────────────────────────────────────────────────────────────────────

func (c *Console) cmdSuggest(args []string) {
	camps := c.state.Orchestrator.ListCampaigns()
	if len(camps) == 0 {
		printErr("No active campaigns — run %scampaign start%s first.", cSuccess, ansiR)
		return
	}
	decisions, err := c.state.Orchestrator.Decide(context.Background(), camps[0].ID)
	if err != nil {
		printErr("Decision engine error: %v", err)
		return
	}
	printSection("AI SUGGESTIONS — " + camps[0].Name)
	if len(decisions) == 0 {
		printInfo("No recommendations yet — expand reconnaissance first.")
		return
	}
	tbl := newTable("#", "Conf", "Tactic", "Technique", "Source", "Target")
	for i, d := range decisions {
		if i >= 10 {
			break
		}
		cc := cSuccess
		if d.Confidence < 0.7 {
			cc = cWarn
		}
		if d.Confidence < 0.5 {
			cc = cMuted
		}
		tbl.addRow(
			fmt.Sprintf("%d", i+1),
			fmt.Sprintf("%s%.0f%%%s", cc+ansiB, d.Confidence*100, ansiR),
			cInfo+trunc(d.Tactic, 18)+ansiR,
			cWhite+trunc(d.Technique, 20)+ansiR,
			cMuted+d.Source+ansiR,
			trunc(d.Target, 16),
		)
	}
	tbl.render()
	fmt.Fprintf(ConsoleOut, "\n  %s[*]%s Use %saccept <#>%s or %sreject <#>%s to act.\n", cMuted, ansiR, cSuccess, ansiR, cDanger, ansiR)
}
func (c *Console) cmdAutoMode(args []string) {
	state := c.state
	if state == nil || state.Auto == nil {
		printErr("AutoMode requires initialized state.")
		return
	}
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "on":
		if !state.Auto.IsEnabled() {
			state.Auto.Toggle()
		}
		printOK("AutoMode %sENABLED%s — decisions above %.2f confidence execute automatically.", cSuccess+ansiB, ansiR, state.Cfg.AI.MinConfidence)
	case "off":
		if state.Auto.IsEnabled() {
			state.Auto.Stop()
		}
		printWarn("AutoMode %sDISABLED%s — manual approval required.", ansiB, ansiR)
	default:
		status := cDanger + "disabled" + ansiR
		if state.Auto.IsEnabled() {
			status = cSuccess + "enabled" + ansiR
		}
		printInfo("AutoMode is %s — use %sauto on%s / %sauto off%s.", status, cSuccess, ansiR, cDanger, ansiR)
	}
}

func (c *Console) cmdAI(args []string) {
	if len(args) == 0 {
		printErr("Usage: ai <prompt>")
		return
	}
	prompt := strings.Join(args, " ")
	printInfo("AI processing: %s\"%s\"%s", cMuted+ansiIt, prompt, ansiR)

	if c.state.Bridge.Connected() {
		resp, err := c.state.Bridge.Call(context.Background(), "ai_analyze", "chat", map[string]interface{}{"context": prompt})
		if err == nil && resp.Success {
			fmt.Fprintf(ConsoleOut, "\n  %s%s[AI]%s %v\n\n", cPrimary, ansiB, ansiR, resp.Result["response"])
			return
		}
	}

	camps := c.state.Orchestrator.ListCampaigns()
	if len(camps) > 0 {
		wg := c.state.Orchestrator.WorldGraph()
		fmt.Fprintf(ConsoleOut, "\n  %s%s[AI — context]%s\n  %s\n", cPrimary, ansiB, ansiR, wg.Summary())
	}
	fmt.Fprintf(ConsoleOut, "\n  %s%s[AI — offline]%s Recommendations available via %ssuggest%s.\n\n", cPrimary, ansiB, ansiR, cSuccess, ansiR)
}

func (c *Console) cmdAccept(args []string) {
	decisionID, ok := c.resolveDecisionArg(args, "accept")
	if !ok {
		return
	}
	if err := c.state.Orchestrator.ApproveDecision(decisionID); err != nil {
		printErr("Approve failed: %v", err)
		return
	}
	// honest: approval marks the decision reviewed; execution happens
	// through AutoMode or manual module runs, not by this command
	printOK("Decision %s%s%s approved (marked as reviewed).", cWhite+ansiB, decisionID, ansiR)
	printInfo("AutoMode executes approved decisions when enabled (ai auto on).")
}

// resolveDecisionArg maps a suggestion row number (from the last
// `suggest` output) or a literal decision ID to the real ID.
func (c *Console) resolveDecisionArg(args []string, verb string) (string, bool) {
	if c.state == nil || c.state.Orchestrator == nil {
		printInfo("No active campaign. Start a campaign first with 'campaign start'.")
		return "", false
	}
	if len(args) < 1 {
		printInfo("Usage: %s <#|decision-id>", verb)
		return "", false
	}
	arg := args[0]
	if n, err := strconv.Atoi(arg); err == nil {
		camps := c.state.Orchestrator.ListCampaigns()
		if len(camps) == 0 {
			printErr("No campaigns — run %scampaign start%s first.", cSuccess, ansiR)
			return "", false
		}
		decisions := c.state.Orchestrator.GetDecisions(camps[0].ID)
		if n < 1 || n > len(decisions) {
			printErr("Row %d out of range — run %ssuggest%s to refresh the list.", n, cSuccess, ansiR)
			return "", false
		}
		return decisions[n-1].ID, true
	}
	return arg, true
}

func (c *Console) cmdReject(args []string) {
	decisionID, ok := c.resolveDecisionArg(args, "reject")
	if !ok {
		return
	}
	if err := c.state.Orchestrator.RejectDecision(decisionID); err != nil {
		printErr("Reject failed: %v", err)
		return
	}
	printOK("Decision %s%s%s rejected.", cWhite+ansiB, decisionID, ansiR)
}

// ─── Data views ───────────────────────────────────────────────────────────────

func (c *Console) cmdDBStatus() {
	printSection("DATABASE")
	if c.state.DB != nil {
		tbl := newTable("Field", "Value")
		tbl.addRow(cInfo+"Engine"+ansiR, cSuccess+ansiB+"SQLite"+ansiR)
		tbl.addRow(cInfo+"File"+ansiR, "vesper.db")
		tbl.addRow(cInfo+"Tables"+ansiR, "6")
		tbl.addRow(cInfo+"Agents"+ansiR, fmt.Sprintf("%d", len(c.state.GetAgents())))
		tbl.addRow(cInfo+"Hosts"+ansiR, fmt.Sprintf("%d", len(c.state.GetHosts())))
		tbl.render()
	} else {
		printWarn("SQLite unavailable — running in-memory.")
	}
}

func (c *Console) cmdHosts() {
	hosts := c.state.GetHosts()
	printSection("DISCOVERED HOSTS")
	if len(hosts) == 0 {
		printInfo("No hosts discovered yet — run a recon scan.")
		return
	}
	tbl := newTable("IP", "Hostname", "OS", "Value", "Ports", "Status")
	for _, h := range hosts {
		ports := ""
		for _, p := range h.OpenPorts {
			ports += fmt.Sprintf("%d ", p)
		}
		hostStatus := cMuted + "○ scanned" + ansiR
		for _, a := range c.state.GetAgents() {
			if a.LocalIP == h.IP && (a.Status == types.AgentStatusOnline || a.Status == types.AgentStatusActive) {
				hostStatus = cSuccess + ansiB + "● compromised" + ansiR
				break
			}
		}
		tbl.addRow(
			cCyan+ansiB+h.IP+ansiR,
			cWhite+h.Hostname+ansiR,
			trunc(h.OS, 14),
			fmt.Sprintf("%d", h.AssetValue),
			cMuted+strings.TrimSpace(ports)+ansiR,
			hostStatus,
		)
	}
	tbl.render()
}

func (c *Console) cmdServices() {
	hosts := c.state.GetHosts()
	printSection("SERVICES")
	if len(hosts) == 0 {
		printInfo("No hosts discovered.")
		return
	}
	tbl := newTable("IP", "Port", "Service", "Banner")
	for _, h := range hosts {
		for i, svc := range h.Services {
			port := 0
			if i < len(h.OpenPorts) {
				port = h.OpenPorts[i]
			}
			tbl.addRow(
				cCyan+h.IP+ansiR,
				fmt.Sprintf("%d", port),
				cWhite+ansiB+svc+ansiR,
				cMuted+"—"+ansiR,
			)
		}
	}
	tbl.render()
}

func (c *Console) cmdCreds() {
	creds := c.state.GetCreds()
	printSection("CAPTURED CREDENTIALS")
	if len(creds) == 0 {
		printInfo("No credentials captured yet.")
		return
	}
	tbl := newTable("Username", "Password", "Domain", "Source", "Agent")
	for _, cr := range creds {
		tbl.addRow(
			cSuccess+ansiB+cr.Username+ansiR,
			cWarn+maskPassword(cr.Password)+ansiR,
			cr.Domain,
			cMuted+cr.Source+ansiR,
			cMuted+trunc(cr.AgentID, 12)+ansiR,
		)
	}
	tbl.render()
}

func (c *Console) cmdVulns() {
	vulns := c.state.GetVulns()
	printSection("VULNERABILITIES")
	if len(vulns) == 0 {
		printInfo("No vulnerabilities identified.")
		return
	}
	tbl := newTable("CVE", "Severity", "Service", "Port", "Target")
	for _, v := range vulns {
		sevColor := cMuted
		switch strings.ToLower(v.Severity) {
		case "critical":
			sevColor = cDanger + ansiB
		case "high":
			sevColor = cDanger
		case "medium":
			sevColor = cWarn
		case "low":
			sevColor = cInfo
		}
		tbl.addRow(
			cWhite+ansiB+v.CVE+ansiR,
			sevColor+v.Severity+ansiR,
			v.Service,
			fmt.Sprintf("%d", v.Port),
			cCyan+v.TargetIP+ansiR,
		)
	}
	tbl.render()
}

// ─── Kill chain ────────────────────────────────────────────────────────────────

func (c *Console) cmdKillChain(args []string) {
	camps := c.state.Orchestrator.ListCampaigns()
	printSection("KILL CHAIN")
	if len(camps) == 0 {
		printInfo("No active campaigns.")
		return
	}
	for _, cam := range camps {
		phases := []string{"Recon", "Weaponization", "Delivery", "Exploitation", "Installation", "C2", "Objectives"}
		curOrder := cam.Phase.Order() - 1 // Order() is 1-based

		fmt.Fprintf(ConsoleOut, "\n  %s%s%s  %s%s%s  phase=%s  agents=%d\n",
			cWhite+ansiB, cam.Name, ansiR,
			cMuted, cam.Profile, ansiR,
			statusTag(string(cam.Phase)), cam.AgentCount)
		fmt.Fprintf(ConsoleOut, "  Progress: %s\n\n", pbar(cam.Progress, 24))

		for i, p := range phases {
			var marker string
			switch {
			case i < curOrder:
				marker = cSuccess + ansiB + " ✓ " + ansiR + cSuccess
			case i == curOrder:
				marker = cInfo + ansiB + " ▶ " + ansiR + cInfo + ansiB
			default:
				marker = cMuted + " ○ " + ansiR + cMuted
			}
			fmt.Fprintf(ConsoleOut, "  %s%s%s\n", marker, p, ansiR)
		}
		fmt.Fprintln(ConsoleOut)
	}
}

// ─── Misc commands ────────────────────────────────────────────────────────────

func (c *Console) cmdWorkspace(args []string) {
	if c.workspace == "" {
		c.workspace = "default"
	}
	if len(args) == 0 {
		printInfo("Current workspace: %s%s%s", cWhite+ansiB, c.workspace, ansiR)
		return
	}
	c.workspace = args[0]
	printOK("Workspace: %s%s%s (session-local label)", cWhite+ansiB, c.workspace, ansiR)
}

func (c *Console) cmdListeners(args []string) {
	// delegates to the REAL listener manager (accept loops, live state)
	lc := listenersCmd()
	if len(args) == 0 {
		args = []string{"list"}
	}
	lc.SetArgs(args)
	_ = lc.Execute()
}

func (c *Console) cmdWebhook(args []string) {
	printInfo("Webhooks are configured in config.yaml (notifications section).")
	printInfo("The console toggle is not implemented — nothing was changed.")
}

func (c *Console) cmdBuilder(args []string) {
	if len(args) == 0 {
		printSection("PAYLOAD BUILDER")
		fmt.Fprintf(ConsoleOut, "  Usage: %sbuilder%s [options]\n\n", cSuccess, ansiR)
		fmt.Fprintf(ConsoleOut, "  Options:\n")
		fmt.Fprintf(ConsoleOut, "    --os <target>    Target OS (windows, linux, macos) [default: windows]\n")
		fmt.Fprintf(ConsoleOut, "    --arch <arch>    Architecture (x64, x86, arm64) [default: x64]\n")
		fmt.Fprintf(ConsoleOut, "    --format <fmt>   Format (exe, dll, ps1, elf, sh, macho, shellcode) [default: exe]\n")
		fmt.Fprintf(ConsoleOut, "    --lhost <ip>     C2 Listener IP / Domain\n")
		fmt.Fprintf(ConsoleOut, "    --lport <port>   C2 Listener Port [default: 8443]\n")
		fmt.Fprintf(ConsoleOut, "\n  Example:\n")
		fmt.Fprintf(ConsoleOut, "    builder --os windows --arch x64 --format exe --lhost 10.0.0.5 --lport 443\n")
		return
	}

	// Parse arguments
	osTarget := "windows"
	arch := "x64"
	format := "exe"
	lhost := ""
	lport := "8443"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--os":
			if i+1 < len(args) {
				osTarget = args[i+1]
				i++
			}
		case "--arch":
			if i+1 < len(args) {
				arch = args[i+1]
				i++
			}
		case "--format":
			if i+1 < len(args) {
				format = args[i+1]
				i++
			}
		case "--lhost":
			if i+1 < len(args) {
				lhost = args[i+1]
				i++
			}
		case "--lport":
			if i+1 < len(args) {
				lport = args[i+1]
				i++
			}
		}
	}

	if lhost == "" {
		printErr("--lhost is required")
		return
	}

	// Honest guidance: the console does not compile payloads — the
	// real builder is `vesper payload generate` (cobra), which runs an
	// actual `go build` of the agent. The pre-remodel flow here only
	// slept, printed a fake size and claimed a file it never wrote.
	printInfo("Target: %s/%s | Format: %s | C2: %s:%s", osTarget, arch, format, lhost, lport)
	printErr("The console cannot compile payloads.")
	fmt.Fprintf(ConsoleOut, "  Use: %svesper payload generate --os %s --arch %s --c2 %s:%s%s\n\n",
		cSuccess+ansiB, osTarget, arch, lhost, lport, ansiR)
}

func (c *Console) cmdLab(args []string) {
	if len(args) == 0 {
		printErr("Usage: lab [up|down|status]")
		return
	}
	// Real docker compose — the pre-remodel console version printed a
	// hardcoded container table without touching Docker.
	var sub string
	switch args[0] {
	case "up":
		sub = "up -d"
	case "down":
		sub = "down"
	case "status":
		sub = "ps"
	default:
		printErr("Unknown lab command: %s (use up|down|status)", args[0])
		return
	}
	cmdArgs := strings.Fields(sub)
	out, err := exec.Command("docker", append([]string{"compose", "-f", "lab/docker-compose.yml"}, cmdArgs...)...).CombinedOutput()
	if err != nil {
		printErr("Docker compose failed: %v", err)
		if len(out) > 0 {
			fmt.Fprintln(ConsoleOut, cMuted+string(out)+ansiR)
		}
		return
	}
	if len(out) > 0 {
		fmt.Fprintln(ConsoleOut, cMuted+string(out)+ansiR)
	}
	printOK("Lab %s complete.", args[0])
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func defaultOptions(moduleName string) map[string]string {
	// Canonical option set: TARGET is what every bridge module reads.
	// Anything else the operator sets with `set` is passed through to
	// the module as-is (see cmdExploit params).
	opts := map[string]string{"TARGET": ""}
	if strings.Contains(moduleName, "recon") {
		opts["MODE"] = "basic"
	}
	return opts
}

func optionDesc(module, option string) string {
	switch option {
	case "TARGET", "RHOSTS", "RHOST":
		return "Target IP/hostname"
	case "MODE":
		return "Scan mode: basic | stealth"
	case "RPORT":
		return "Target port"
	default:
		return "module option"
	}
}

func bridgeStatus(connected bool) string {
	if connected {
		return cSuccess + ansiB + "● connected" + ansiR
	}
	return cMuted + "○ disconnected" + ansiR
}

// StartConsoleState is the entry point called from main.go.
func StartConsoleState(state *appstate.AppState, args []string) error {
	return NewConsole(state).Run()
}
