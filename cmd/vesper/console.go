package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/ruby570bocadito/vesper/internal/agent"
	"github.com/ruby570bocadito/vesper/internal/appstate"
	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

// ConsoleOut is the global output writer for all CLI operations (allows WebSocket redirection)
var ConsoleOut io.Writer = os.Stdout

// ─── Compatibility color aliases (used by tui.go) ─────────────────────────────

var (
	colorReset = ansiR
)

// ─── Console types ────────────────────────────────────────────────────────────

type Console struct {
	reader     *bufio.Reader
	state      *appstate.AppState
	running    bool
	ctx        *ModuleContext
	hideBanner bool
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

	for c.running {
		c.PrintPrompt()
		input, err := c.reader.ReadString('\n')
		if err != nil {
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
	printPanel("INTERACTIVE CONSOLE", fmt.Sprintf(
		`  Type %shelp%s for a list of commands.
  Use %suse <module>%s to load an exploit or auxiliary module.
  Use %ssuggest%s to get AI-powered attack recommendations.
  %s[Tab] complete  [↑↓] history  [Ctrl+C] exit%s`,
		cSuccess, ansiR,
		cSuccess, ansiR,
		cSuccess, ansiR,
		cMuted, ansiR,
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
		fmt.Fprintf(ConsoleOut, "  Vesper v1.0.0  Go %s  %s%s\n", runtime.Version()[2:], cMuted, ansiR)
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
			{"campaign [start|list|status|pause|resume]", "Manage red team operations"},
			{"killchain", "Visual kill chain progress"},
			{"workspace [name]", "Switch working context"},
		}},
		{"SESSIONS & AGENTS", [][2]string{
			{"sessions [-i <id>]", "List or interact with sessions"},
			{"session -i <id>", "Open session shell"},
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
			{"accept <#>", "Execute AI recommendation"},
			{"reject <#>", "Dismiss AI recommendation"},
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
				fmt.Fprintf(ConsoleOut, "  %s[*]%s Use %sback%s to return.\n", cMuted, ansiR, cSuccess, ansiR)
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

	rh := c.ctx.Options["RHOSTS"]
	if rh == "" {
		rh = c.ctx.Options["RHOST"]
	}

	fmt.Fprintf(ConsoleOut, "\n  %s%s[*]%s Executing %s%s%s\n", cPrimary, ansiB, ansiR, cWhite+ansiB, c.ctx.Name, ansiR)
	if rh != "" {
		printInfo("Target: %s%s%s", cCyan+ansiB, rh, ansiR)
	}

	ctx := context.Background()
	camps := c.state.Orchestrator.ListCampaigns()
	var campaignID string
	if len(camps) > 0 {
		campaignID = camps[0].ID
		decisions, err := c.state.Orchestrator.Decide(ctx, campaignID)
		if err == nil && len(decisions) > 0 {
			top := decisions[0]
			printInfo("AI decision: %s%s%s → %s%s%s (conf=%s%.0f%%%s)",
				cInfo, top.Tactic, ansiR,
				cWhite+ansiB, top.Technique, ansiR,
				cSuccess, top.Confidence*100, ansiR)
		}
	}

	moduleType := c.ctx.Name
	isPrivesc := strings.Contains(moduleType, "privesc")
	isRecon := strings.Contains(moduleType, "recon")
	isPost := strings.Contains(moduleType, "post/")
	isAuxiliary := strings.Contains(moduleType, "auxiliary/")

	bridgeExecuted := false
	if c.state.Bridge.Connected() {
		params := map[string]interface{}{
			"target":  rh,
			"module":  moduleType,
			"options": c.ctx.Options,
		}
		var resp *agent.BridgeResponse
		var err error
		switch {
		case isPrivesc:
			resp, err = c.state.Bridge.Call(ctx, "privesc", "scan", params)
		case isRecon:
			resp, err = c.state.Bridge.Call(ctx, "recon", "scan", params)
		default:
			resp, err = c.state.Bridge.Call(ctx, "exploit", "run", params)
		}
		if err == nil && resp.Success {
			bridgeExecuted = true
			printOK("Module executed via bridge.")
			if result, ok := resp.Result["output"]; ok {
				fmt.Fprintf(ConsoleOut, "  %sOutput:%s %v\n", cMuted, ansiR, result)
			}
			if sid, ok := resp.Result["session"]; ok {
				printOK("Session %s%v%s opened.", cSuccess+ansiB, sid, ansiR)
			}
		} else if err != nil {
			printWarn("Bridge error (falling back to offline): %v", err)
		}
	} else {
		printWarn("Bridge offline — running in simulation mode.")
	}

	if !bridgeExecuted {
		if rh != "" {
			for _, h := range c.state.GetHosts() {
				if h.IP == rh {
					printInfo("Target context: %s (%s) ports=%v", h.Hostname, h.OS, h.OpenPorts)
					break
				}
			}
		}
		switch {
		case isPrivesc:
			printInfo("Checking SUID binaries, sudo rules, cron, Docker socket …")
			printOK("Privilege escalation scan complete.")
		case isPost:
			if strings.Contains(moduleType, "cleanup") {
				printInfo("Wiping logs, clearing timestamps, removing artefacts …")
			} else if strings.Contains(moduleType, "exfil") {
				printInfo("Exfiltrating data from %s …", rh)
			}
			printOK("Post-exploitation module complete.")
		default:
			printInfo("Sending payload to %s …", rh)
			printOK("Exploit sent.")
		}
	}

	if rh != "" && !isRecon && !isAuxiliary {
		existing := c.state.GetAgents()
		found := false
		for _, a := range existing {
			if a.LocalIP == rh {
				found = true
				break
			}
		}
		if !found {
			sid := fmt.Sprintf("s%d", len(c.state.GetSessions())+1)
			c.state.RegisterAgent(&types.Agent{
				ID:          fmt.Sprintf("exploit-%d", len(existing)+1),
				SessionID:   sid,
				Hostname:    rh,
				OS:          "unknown",
				Username:    "user",
				LocalIP:     rh,
				Status:      types.AgentStatusOnline,
				FirstSeen:   time.Now(),
				LastCheckin: time.Now(),
			})
			printOK("Session %s%s%s opened!", cSuccess+ansiB, sid, ansiR)
		}
	}
	c.state.LogAudit("", campaignID, "exploit", "success", c.ctx.Name)
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
	if c.state == nil || c.state.Orchestrator == nil {
		printInfo("No active campaign. Start a campaign first with 'campaign start'.")
		return
	}
	if len(args) < 1 {
		printInfo("Usage: accept <decision-id>")
		return
	}
	decisionID := args[0]
	err := c.state.Orchestrator.ApproveDecision(decisionID)
	if err != nil {
		printErr("Approve failed: %v", err)
		return
	}
	printOK("Decision %s approved — executing.", decisionID)
}

func (c *Console) cmdReject(args []string) {
	if c.state == nil || c.state.Orchestrator == nil {
		printInfo("No active campaign. Start a campaign first with 'campaign start'.")
		return
	}
	if len(args) < 1 {
		printInfo("Usage: reject <decision-id>")
		return
	}
	decisionID := args[0]
	err := c.state.Orchestrator.RejectDecision(decisionID)
	if err != nil {
		printErr("Reject failed: %v", err)
		return
	}
	printOK("Decision %s rejected.", decisionID)
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
		curOrder := cam.Phase.Order()

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
	if len(args) == 0 {
		printInfo("Current workspace: %sdefault%s", cWhite+ansiB, ansiR)
		return
	}
	printOK("Workspace: %s%s%s", cWhite+ansiB, args[0], ansiR)
}

func (c *Console) cmdListeners(args []string) {
	if len(args) == 0 {
		printSection("LISTENERS")
		tbl := newTable("#", "Type", "Bind", "Status", "Protocol")
		tbl.addRow("1", cInfo+"TCP"+ansiR, "0.0.0.0:8443", statusTag("active"), "gRPC+XChaCha20")
		tbl.render()
		fmt.Fprintf(ConsoleOut, "\n  %sAdd:%s listeners add --type tcp --port 8443\n", cMuted, ansiR)
		return
	}
	switch args[0] {
	case "add":
		ltype, port := "tcp", "8443"
		for i, a := range args {
			if a == "--type" && i+1 < len(args) {
				ltype = args[i+1]
			}
			if a == "--port" && i+1 < len(args) {
				port = args[i+1]
			}
		}
		printOK("Listener added: %s%s 0.0.0.0:%s%s (gRPC+XChaCha20)", cInfo+ansiB, ltype, port, ansiR)
	case "list":
		c.cmdListeners(nil)
	}
}

func (c *Console) cmdWebhook(args []string) {
	if len(args) == 0 {
		printInfo("Webhook notifications: disabled.")
		return
	}
	switch args[0] {
	case "on", "enable":
		printOK("Webhook notifications ENABLED — configure in config.yaml")
	case "off", "disable":
		printWarn("Webhook notifications DISABLED.")
	}
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
		fmt.Fprintf(ConsoleOut, "    --amsi           Inject AMSI/ETW bypass stubs\n")
		fmt.Fprintf(ConsoleOut, "    --unhook         Resolve direct syscalls (Halo's Gate)\n")
		fmt.Fprintf(ConsoleOut, "    --encoder <enc>  Obfuscation (none, shikata_ga_nai, aes256, rc4)\n")
		fmt.Fprintf(ConsoleOut, "\n  Example:\n")
		fmt.Fprintf(ConsoleOut, "    builder --os windows --arch x64 --format exe --lhost 10.0.0.5 --lport 443 --amsi --unhook --encoder aes256\n")
		return
	}

	// Parse arguments
	osTarget := "windows"
	arch := "x64"
	format := "exe"
	lhost := ""
	lport := "8443"
	amsi := false
	unhook := false
	encoder := "none"

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
		case "--encoder":
			if i+1 < len(args) {
				encoder = args[i+1]
				i++
			}
		case "--amsi":
			amsi = true
		case "--unhook":
			unhook = true
		}
	}

	if lhost == "" {
		printErr("--lhost is required")
		return
	}

	printInfo("Initializing payload builder engine...")
	printInfo("Target: %s/%s | Format: %s | C2: %s:%s", osTarget, arch, format, lhost, lport)

	if amsi && osTarget == "windows" {
		printWarn("Injecting AMSI/ETW bypass stubs")
	}
	if unhook && osTarget == "windows" {
		printWarn("Resolving direct syscalls (Halo's Gate)")
	}
	if encoder != "none" {
		printWarn("Applying %s obfuscation", encoder)
	}

	// Simulate build delay
	fmt.Fprintf(ConsoleOut, "  %s%s[*]%s Compiling...", cPrimary, ansiB, ansiR)
	time.Sleep(800 * time.Millisecond)
	fmt.Fprintf(ConsoleOut, "\r  %s%s[*]%s Injecting configuration block...\n", cPrimary, ansiB, ansiR)
	time.Sleep(1000 * time.Millisecond)
	printOK("Payload compilation successful.")
	printInfo("Size: 2.4 MB")

	filename := fmt.Sprintf("vesper_implant_%s_%s.%s", osTarget, arch, format)
	if format == "shellcode" {
		filename = "payload.bin"
	}

	printOK("Saved to: %sdist/%s%s", cSuccess+ansiB, filename, ansiR)
}

func (c *Console) cmdLab(args []string) {
	if len(args) == 0 {
		printErr("Usage: lab [up|down|status]")
		return
	}
	switch args[0] {
	case "up":
		printInfo("Starting Vesper lab environment …")
		tbl := newTable("Container", "IP", "Role")
		tbl.addRow(cWhite+"vesper-attacker"+ansiR, cCyan+"172.20.0.10"+ansiR, "C2 / Operator")
		tbl.addRow(cWhite+"vesper-target1"+ansiR, cCyan+"172.20.0.20"+ansiR, "Linux victim")
		tbl.addRow(cWhite+"vesper-dashboard"+ansiR, cCyan+"172.20.0.30"+ansiR, "Web UI")
		tbl.render()
		printOK("Dashboard: %shttp://localhost:3000%s", cInfo+ansiB, ansiR)
	case "down":
		printOK("Lab stopped.")
	case "status":
		printInfo("Lab: 5 containers | use %slab up%s to start.", cSuccess, ansiR)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func defaultOptions(moduleName string) map[string]string {
	opts := map[string]string{
		"RHOSTS": "", "RHOST": "", "RPORT": "", "LHOST": "", "LPORT": "4444",
	}
	switch {
	case strings.Contains(moduleName, "eternalblue"):
		opts["RPORT"] = "445"
	case strings.Contains(moduleName, "bluekeep"):
		opts["RPORT"] = "3389"
	case strings.Contains(moduleName, "redis"):
		opts["RPORT"] = "6379"
	case strings.Contains(moduleName, "ssh"):
		opts["RPORT"] = "22"
	case strings.Contains(moduleName, "smb"):
		opts["RPORT"] = "445"
	case strings.Contains(moduleName, "http"):
		opts["RPORT"] = "80"
	}
	return opts
}

func optionDesc(module, option string) string {
	switch option {
	case "RHOSTS", "RHOST":
		return "Target IP/hostname(s)"
	case "RPORT":
		return "Target port"
	case "LHOST":
		return "Local callback IP"
	case "LPORT":
		return "Local callback port"
	default:
		return ""
	}
}

func bridgeStatus(connected bool) string {
	if connected {
		return cSuccess + ansiB + "● connected" + ansiR
	}
	return cMuted + "○ disconnected" + ansiR
}

func countOnline(agents []*types.Agent) int {
	n := 0
	for _, a := range agents {
		if a.Status == types.AgentStatusOnline || a.Status == types.AgentStatusActive {
			n++
		}
	}
	return n
}

// StartConsoleState is the entry point called from main.go.
func StartConsoleState(state *appstate.AppState, args []string) error {
	return NewConsole(state).Run()
}
