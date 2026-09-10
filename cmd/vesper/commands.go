package main

import (
	"context"
	"fmt"
	"net"
	stdos "os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ruby570bocadito/vesper/internal/appstate"
	"github.com/ruby570bocadito/vesper/internal/recon"
)

// GetOrCreateState returns the shared AppState, creating one lazily if needed.
func GetOrCreateState() *appstate.AppState {
	if globalState != nil {
		return globalState
	}
	ctx := context.Background()
	s, err := appstate.New(cfg)
	if err != nil {
		printErr("failed to initialise state: %v", err)
		return nil
	}
	_ = s.Start(ctx)
	globalState = s
	return s
}

// ─── campaign ─────────────────────────────────────────────────────────────────

func campaignCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "campaign",
		Short: "Manage red team campaigns",
		Long: fmt.Sprintf(`%s%sCampaign Management%s

Create, monitor and control red team operations across the full kill chain.
Each campaign tracks phases, agents, decisions and progress independently.`,
			cPrimary, ansiB, ansiR),
	}

	startCmd := &cobra.Command{
		Use:     "start",
		Short:   "Launch a new campaign",
		Example: "  vesper campaign start --name demo --target 192.168.1.0/24 --profile stealth",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			target, _ := cmd.Flags().GetString("target")
			goal, _ := cmd.Flags().GetString("goal")
			profile, _ := cmd.Flags().GetString("profile")
			auto, _ := cmd.Flags().GetBool("auto")

			state := GetOrCreateState()
			if state == nil {
				return fmt.Errorf("state unavailable")
			}

			printInfo("Starting campaign %s%s%s …", cWhite+ansiB, name, ansiR)
			c, err := state.Orchestrator.StartCampaign(cmd.Context(), name, target, goal, profile, auto)
			if err != nil {
				printErr("Failed: %v", err)
				return err
			}

			fmt.Fprintln(ConsoleOut)
			printOK("Campaign launched!")
			printPanel("CAMPAIGN DETAILS", fmt.Sprintf(
				`%sID%s       %s
%sName%s     %s
%sTarget%s   %s
%sGoal%s     %s
%sProfile%s  %s
%sPhase%s    %s
%sAuto%s     %v`,
				cInfo, ansiR, cWhite+c.ID+ansiR,
				cInfo, ansiR, cWhite+ansiB+c.Name+ansiR,
				cInfo, ansiR, cCyan+target+ansiR,
				cInfo, ansiR, cOrange+goal+ansiR,
				cInfo, ansiR, c.Profile,
				cInfo, ansiR, statusTag(string(c.Phase)),
				cInfo, ansiR, auto,
			))
			fmt.Fprintln(ConsoleOut)
			return nil
		},
	}
	startCmd.Flags().StringP("name", "n", "default", "Campaign name")
	startCmd.Flags().StringP("target", "t", "10.0.0.0/24", "Target scope (CIDR or hostname)")
	startCmd.Flags().StringP("goal", "g", "domain_admin", "Objective (domain_admin, data_exfil, ransomware…)")
	startCmd.Flags().StringP("profile", "p", "balanced", "Aggression profile: stealth | balanced | aggressive")
	startCmd.Flags().Bool("auto", false, "Enable auto-approval of AI decisions")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all campaigns",
		Run: func(c *cobra.Command, a []string) {
			state := GetOrCreateState()
			if state == nil {
				return
			}
			campaigns := state.Orchestrator.ListCampaigns()
			printSection("CAMPAIGNS")
			if len(campaigns) == 0 {
				printInfo("No campaigns — run %svesper campaign start%s to begin.", cSuccess, ansiR)
				fmt.Fprintln(ConsoleOut)
				return
			}
			tbl := newTable("ID", "Name", "Phase", "Status", "Agents", "Progress")
			for _, cam := range campaigns {
				tbl.addRow(
					cMuted+trunc(cam.ID, 28)+ansiR,
					cWhite+ansiB+cam.Name+ansiR,
					statusTag(string(cam.Phase)),
					statusTag(string(cam.Status)),
					fmt.Sprintf("%d", cam.AgentCount),
					pbar(cam.Progress, 16),
				)
			}
			tbl.render()
			fmt.Fprintln(ConsoleOut)
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show detailed campaign status",
		Run: func(c *cobra.Command, a []string) {
			state := GetOrCreateState()
			if state == nil {
				return
			}
			campaigns := state.Orchestrator.ListCampaigns()
			if len(campaigns) == 0 {
				printInfo("No active campaigns.")
				return
			}
			for _, cam := range campaigns {
				printSection("CAMPAIGN: " + cam.Name)
				phases := []string{"Recon", "Weaponization", "Delivery", "Exploitation", "Installation", "C2", "Objectives"}
				cur := killChainOrder(string(cam.Phase))
				var chain []string
				for i, p := range phases {
					switch {
					case i < cur:
						chain = append(chain, cSuccess+"✓ "+p+ansiR)
					case i == cur:
						chain = append(chain, cInfo+ansiB+"◉ "+p+ansiR)
					default:
						chain = append(chain, cMuted+"○ "+p+ansiR)
					}
				}
				printPanel("KILL CHAIN", strings.Join(chain, "  →  "))
				fmt.Fprintf(ConsoleOut, "  %sProgress%s  %s\n", cInfo, ansiR, pbar(cam.Progress, 24))
				fmt.Fprintf(ConsoleOut, "  %sAgents%s    %d\n\n", cInfo, ansiR, cam.AgentCount)
			}
		},
	}

	pauseCmd := &cobra.Command{
		Use:   "pause",
		Short: "Pause running campaign",
		Run: func(c *cobra.Command, a []string) {
			state := GetOrCreateState()
			if state == nil {
				return
			}
			for _, cam := range state.Orchestrator.ListCampaigns() {
				if cam.Status == "running" {
					cam.Status = "paused"
					printWarn("Campaign %s%s%s paused at phase %s.", cWhite, cam.Name, ansiR, string(cam.Phase))
					return
				}
			}
			printInfo("No running campaigns to pause.")
		},
	}

	resumeCmd := &cobra.Command{
		Use:   "resume",
		Short: "Resume paused campaign",
		Run: func(c *cobra.Command, a []string) {
			state := GetOrCreateState()
			if state == nil {
				return
			}
			for _, cam := range state.Orchestrator.ListCampaigns() {
				if cam.Status == "paused" {
					cam.Status = "running"
					printOK("Campaign %s%s%s resumed.", cWhite, cam.Name, ansiR)
					return
				}
			}
			printInfo("No paused campaigns to resume.")
		},
	}

	cmd.AddCommand(startCmd, listCmd, statusCmd, pauseCmd, resumeCmd)
	return cmd
}

// ─── recon ────────────────────────────────────────────────────────────────────

func reconCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recon",
		Short: "Reconnaissance operations",
	}
	cmd.PersistentFlags().StringP("target", "t", "10.0.0.0/24", "Target IP/CIDR")
	cmd.PersistentFlags().StringP("mode", "m", "basic", "Scan mode: basic | stealth | full")
	cmd.PersistentFlags().StringP("domain", "d", "example.com", "Domain for DNS/OSINT")

	scanCmd := &cobra.Command{
		Use:     "scan",
		Short:   "Discover hosts, ports and services",
		Example: "  vesper recon scan -t 10.0.0.0/24 -m stealth",
		Run: func(c *cobra.Command, a []string) {
			target, _ := c.Flags().GetString("target")
			mode, _ := c.Flags().GetString("mode")
			printInfo("Launching %s%s%s scan → %s%s%s", cCyan, mode, ansiR, cWhite+ansiB, target, ansiR)

			state := GetOrCreateState()
			if state != nil && state.Bridge != nil && state.Bridge.Connected() {
				result, err := state.Bridge.CallRaw(c.Context(), "recon", "scan", map[string]interface{}{
					"target": target, "mode": mode,
				})
				if err == nil && result != nil {
					if hosts, ok := result["hosts_found"]; ok {
						printOK("Scan complete — %s%v%s hosts discovered", cWhite+ansiB, hosts, ansiR)
					}
					if ports, ok := result["ports_open"]; ok {
						if portList, ok := ports.([]interface{}); ok {
							tbl := newTable("IP", "Port", "Service")
							for _, p := range portList {
								if pm, ok := p.(map[string]interface{}); ok {
									tbl.addRow(
										fmt.Sprintf("%v", pm["ip"]),
										fmt.Sprintf("%v", pm["port"]),
										fmt.Sprintf("%v", pm["service"]),
									)
								}
							}
							tbl.render()
						}
					}
					return
				}
			}
			printWarn("Bridge offline — using the native TCP scanner")
			host := strings.TrimSpace(target)
			if host == "" || strings.Contains(host, "/") {
				printErr("native scanner supports single IPs only; CIDR sweeps land in v1.2")
				return
			}
			printInfo("Scanning %d common TCP ports on %s%s%s …", len(recon.DefaultPorts), cWhite+ansiB, host, ansiR)
			res := recon.ScanHost(c.Context(), host, recon.DefaultPorts, 1500*time.Millisecond, true)
			tbl := newTable("Port", "State", "Service", "Banner")
			open := 0
			for _, r := range res {
				if r.Open {
					open++
					tbl.addRow(fmt.Sprintf("%d", r.Port), cSuccess+"open"+ansiR, recon.ServiceGuess(r.Port), trunc(r.Banner, 28))
				}
			}
			printOK("Scan complete — %d/%d ports open", open, len(res))
			if open > 0 {
				tbl.render()
			}
			fmt.Fprintln(ConsoleOut)
		},
	}

	osintCmd := &cobra.Command{
		Use:     "osint",
		Short:   "OSINT gathering (emails, subdomains, leaks)",
		Example: "  vesper recon osint -d target.com",
		Run: func(c *cobra.Command, a []string) {
			domain, _ := c.Flags().GetString("domain")
			printInfo("OSINT for %s%s%s …", cWhite+ansiB, domain, ansiR)
			printWarn("OSINT gathering is not implemented yet (roadmap v1.2) — no results were produced.")
		},
	}

	dnsCmd := &cobra.Command{
		Use:     "dns",
		Short:   "DNS enumeration (A, MX, NS, TXT)",
		Example: "  vesper recon dns -d target.com",
		Run: func(c *cobra.Command, a []string) {
			domain, _ := c.Flags().GetString("domain")
			if domain == "" {
				printErr("--domain is required")
				return
			}
			printInfo("Enumerating DNS for %s%s%s …", cCyan+ansiB, domain, ansiR)

			resolver := &net.Resolver{}
			ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
			defer cancel()

			tbl := newTable("Type", "Value")
			rows := 0
			if addrs, err := resolver.LookupHost(ctx, domain); err == nil {
				for _, addr := range addrs {
					tbl.addRow("A/AAAA", addr)
					rows++
				}
			}
			if mxs, err := resolver.LookupMX(ctx, domain); err == nil {
				for _, mx := range mxs {
					tbl.addRow("MX", fmt.Sprintf("%s %d", mx.Host, mx.Pref))
					rows++
				}
			}
			if nss, err := resolver.LookupNS(ctx, domain); err == nil {
				for _, ns := range nss {
					tbl.addRow("NS", ns.Host)
					rows++
				}
			}
			if txts, err := resolver.LookupTXT(ctx, domain); err == nil {
				for _, txt := range txts {
					tbl.addRow("TXT", trunc(txt, 48))
					rows++
				}
			}
			if rows == 0 {
				printWarn("No DNS records found for %s (check the domain and your resolver)", domain)
				return
			}
			printOK("DNS enumeration — %d records", rows)
			tbl.render()
		},
	}

	cmd.AddCommand(scanCmd, osintCmd, dnsCmd)
	return cmd
}

// ─── agent ────────────────────────────────────────────────────────────────────

func agentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Agent management",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List connected agents",
		Run: func(c *cobra.Command, a []string) {
			state := GetOrCreateState()
			if state == nil {
				return
			}
			agents := state.GetAgents()
			printSection("ACTIVE AGENTS")
			if len(agents) == 0 {
				printInfo("No agents connected. Deploy one with %svesper agent generate%s.", cSuccess, ansiR)
				fmt.Fprintln(ConsoleOut)
				return
			}
			tbl := newTable("ID", "Hostname", "OS", "User", "IP", "Status", "Last Seen")
			for _, ag := range agents {
				tbl.addRow(
					cMuted+trunc(ag.ID, 12)+ansiR,
					cWhite+ansiB+trunc(ag.Hostname, 14)+ansiR,
					trunc(ag.OS, 10),
					trunc(ag.Username, 12),
					cCyan+ag.LocalIP+ansiR,
					statusTag(string(ag.Status)),
					cMuted+ag.LastCheckin.Format("15:04:05")+ansiR,
				)
			}
			tbl.render()
			fmt.Fprintf(ConsoleOut, "\n  %sTotal:%s %s%d%s agents\n\n",
				cMuted, ansiR, cWhite+ansiB, len(agents), ansiR)
		},
	}

	generateCmd := &cobra.Command{
		Use:     "generate",
		Short:   "Cross-compile a new implant",
		Example: "  vesper agent generate --os linux --arch amd64",
		Run: func(c *cobra.Command, a []string) {
			targetOS, _ := c.Flags().GetString("os")
			arch, _ := c.Flags().GetString("arch")
			stdos.MkdirAll("dist", 0755)
			output := fmt.Sprintf("dist/agent-%s-%s", targetOS, arch)
			if targetOS == "windows" {
				output += ".exe"
			}
			printInfo("Cross-compiling implant → %s%s%s (%s/%s)", cWhite+ansiB, output, ansiR, targetOS, arch)
			buildCmd := exec.Command("go", "build", "-ldflags", "-s -w", "-o", output, "./internal/agent/cmd/agent/")
			buildCmd.Env = append(stdos.Environ(), "GOOS="+targetOS, "GOARCH="+arch, "CGO_ENABLED=0")
			if out, err := buildCmd.CombinedOutput(); err != nil {
				printErr("Build failed: %v\n%s", err, string(out))
				return
			}
			printOK("Implant ready: %s%s%s", cSuccess+ansiB, output, ansiR)
		},
	}
	generateCmd.Flags().String("os", "linux", "Target OS (linux|windows|darwin)")
	generateCmd.Flags().String("arch", "amd64", "Architecture (amd64|arm64|386)")

	interactCmd := &cobra.Command{
		Use:   "interact <session-id>",
		Short: "Open interactive session",
		Run: func(c *cobra.Command, a []string) {
			if len(a) == 0 {
				printErr("Usage: vesper agent interact <session-id>")
				return
			}
			printInfo("Opening session %s%s%s …", cCyan+ansiB, a[0], ansiR)
			printOK("Session active — use %svesper console%s for full interaction.", cSuccess, ansiR)
		},
	}

	cmd.AddCommand(listCmd, generateCmd, interactCmd)
	return cmd
}

// ─── exploit ──────────────────────────────────────────────────────────────────

func exploitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exploit",
		Short: "Exploitation & privilege escalation",
	}
	cmd.PersistentFlags().StringP("target", "t", "", "Target IP/hostname")
	cmd.PersistentFlags().String("cve", "", "CVE identifier")

	scanCmd := &cobra.Command{
		Use:   "scan",
		Short: "Enumerate privilege escalation vectors",
		Run: func(c *cobra.Command, a []string) {
			target, _ := c.Flags().GetString("target")
			if target == "" {
				target = "active session"
			}
			printInfo("Scanning privesc vectors on %s%s%s …", cWhite+ansiB, target, ansiR)
			state := GetOrCreateState()
			if state != nil && state.Bridge != nil && state.Bridge.Connected() {
				result, err := state.Bridge.CallRaw(c.Context(), "privesc", "scan", map[string]interface{}{})
				if err == nil && result != nil {
					if v, ok := result["findings"]; ok {
						printOK("%v vectors identified", v)
						return
					}
				}
			}
			printErr("Bridge offline — privesc checks require the bridge; no results are invented.")
			fmt.Fprintln(ConsoleOut)
		},
	}

	runCmd := &cobra.Command{
		Use:     "run",
		Short:   "Execute exploit module",
		Example: "  vesper exploit run --target 10.0.0.5 --cve CVE-2021-44228",
		Run: func(c *cobra.Command, a []string) {
			target, _ := c.Flags().GetString("target")
			cve, _ := c.Flags().GetString("cve")
			if target == "" {
				printErr("--target is required")
				return
			}
			printInfo("Executing %s%s%s against %s%s%s …", cOrange+ansiB, cve, ansiR, cWhite+ansiB, target, ansiR)
			printWarn("Vesper does not auto-exploit from the CLI.")
			printInfo("Load a module in the console (%suse <module> → run%s) or let a campaign dispatch decisions.", cSuccess, ansiR)
		},
	}

	cmd.AddCommand(scanCmd, runCmd)
	return cmd
}

// ─── ai ───────────────────────────────────────────────────────────────────────

func aiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai",
		Short: "AI-powered decision engine",
	}
	cmd.PersistentFlags().Bool("on", false, "Enable")
	cmd.PersistentFlags().Bool("off", false, "Disable")

	chatCmd := &cobra.Command{
		Use:     "chat <prompt>",
		Short:   "Query the AI assistant",
		Example: `  vesper ai chat "What is the best vector to escalate on a misconfigured sudoers?"`,
		Run: func(c *cobra.Command, a []string) {
			prompt := strings.Join(a, " ")
			if prompt == "" {
				if p, _ := c.Flags().GetString("prompt"); p != "" {
					prompt = p
				}
			}
			if prompt == "" {
				printErr("Provide a prompt: vesper ai chat \"<question>\"")
				return
			}
			printInfo("AI analyzing: %s\"%s\"%s", cWhite+ansiIt, prompt, ansiR)
			state := GetOrCreateState()
			if state != nil && state.Bridge != nil && state.Bridge.Connected() {
				result, err := state.Bridge.CallRaw(c.Context(), "ai", "analyze", map[string]interface{}{"prompt": prompt})
				if err == nil && result != nil {
					if resp, ok := result["response"].(string); ok {
						fmt.Fprintf(ConsoleOut, "\n  %s%s[AI]%s %s\n\n", cPrimary, ansiB, ansiR, resp)
						return
					}
				}
			}
			fmt.Fprintf(ConsoleOut, "\n  %s%s[AI — offline]%s Start with port/service enumeration. Match open services to CVEs.\n"+
				"  Escalate via SUID, sudo misconfigurations or kernel exploits. Persist via cron/systemd.\n\n",
				cPrimary, ansiB, ansiR)
		},
	}
	chatCmd.Flags().String("prompt", "", "Query string (alternative to positional arg)")

	suggestCmd := &cobra.Command{
		Use:   "suggest",
		Short: "Get ranked attack recommendations for active campaign",
		Run: func(c *cobra.Command, a []string) {
			state := GetOrCreateState()
			if state == nil {
				return
			}
			camps := state.Orchestrator.ListCampaigns()
			if len(camps) == 0 {
				printErr("No active campaigns — run %svesper campaign start%s first.", cSuccess, ansiR)
				return
			}
			decisions, err := state.Orchestrator.Decide(c.Context(), camps[0].ID)
			if err != nil {
				printErr("Decision engine error: %v", err)
				return
			}
			printSection("AI RECOMMENDATIONS — " + camps[0].Name)
			if len(decisions) == 0 {
				printInfo("No recommendations yet — expand reconnaissance first.")
				return
			}
			tbl := newTable("#", "Confidence", "Tactic", "Technique", "Source", "Target")
			for i, d := range decisions {
				if i >= 10 {
					break
				}
				confColor := cSuccess
				if d.Confidence < 0.7 {
					confColor = cWarn
				}
				if d.Confidence < 0.5 {
					confColor = cMuted
				}
				tbl.addRow(
					fmt.Sprintf("%d", i+1),
					fmt.Sprintf("%s%.0f%%%s", confColor+ansiB, d.Confidence*100, ansiR),
					cInfo+d.Tactic+ansiR,
					cWhite+d.Technique+ansiR,
					cMuted+d.Source+ansiR,
					d.Target,
				)
			}
			tbl.render()
			fmt.Fprintf(ConsoleOut, "\n  %s[TIP]%s Use %svesper console%s → %saccept <#>%s to execute a recommendation.\n\n",
				cInfo, ansiR, cSuccess, ansiR, cSuccess, ansiR)
		},
	}

	autoCmd := &cobra.Command{
		Use:   "auto",
		Short: "Toggle autonomous attack mode",
		Run: func(c *cobra.Command, a []string) {
			on, _ := c.Flags().GetBool("on")
			off, _ := c.Flags().GetBool("off")
			switch {
			case on:
				printOK("AutoMode %sENABLED%s — AI will approve and execute decisions automatically.", cSuccess+ansiB, ansiR)
			case off:
				printWarn("AutoMode %sDISABLED%s — Manual approval required.", ansiB, ansiR)
			default:
				printInfo("AutoMode status: use %s--on%s or %s--off%s flags.", cSuccess, ansiR, cDanger, ansiR)
			}
		},
	}

	cmd.AddCommand(chatCmd, suggestCmd, autoCmd)
	return cmd
}

// ─── dashboard ────────────────────────────────────────────────────────────────

func dashboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dashboard",
		Short: "Start REST API + WebSocket + C2 backend",
		Long: fmt.Sprintf(`%s%sDashboard Server%s

Starts the Vesper backend services:
  %s●%s REST API    → http://localhost:8443/api
  %s●%s WebSocket   → ws://localhost:8443/ws
  %s●%s Health      → http://localhost:8443/api/health
  %s●%s Metrics     → http://localhost:8443/api/metrics`,
			cPrimary, ansiB, ansiR,
			cSuccess, ansiR, cSuccess, ansiR, cSuccess, ansiR, cSuccess, ansiR),
		RunE: func(cmd *cobra.Command, args []string) error {
			return startDashboard(cfg)
		},
	}
}

// ─── db ───────────────────────────────────────────────────────────────────────

func dbCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Database management",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show database status",
		Run: func(c *cobra.Command, a []string) {
			state := GetOrCreateState()
			if state == nil {
				return
			}
			printSection("DATABASE STATUS")
			tbl := newTable("Field", "Value")
			if state.DB != nil {
				tbl.addRow(cInfo+"Engine"+ansiR, cSuccess+ansiB+"SQLite"+ansiR)
				tbl.addRow(cInfo+"File"+ansiR, cWhite+"vesper.db"+ansiR)
				tbl.addRow(cInfo+"Tables"+ansiR, "6")
				tbl.addRow(cInfo+"Agents"+ansiR, fmt.Sprintf("%d", len(state.GetAgents())))
				tbl.addRow(cInfo+"Hosts"+ansiR, fmt.Sprintf("%d", len(state.GetHosts())))
				tbl.addRow(cInfo+"Vulns"+ansiR, fmt.Sprintf("%d", len(state.GetVulns())))
				tbl.addRow(cInfo+"Creds"+ansiR, fmt.Sprintf("%d", len(state.GetCreds())))
			} else {
				tbl.addRow(cInfo+"Engine"+ansiR, cWarn+"in-memory (SQLite unavailable)"+ansiR)
			}
			tbl.render()
			fmt.Fprintln(ConsoleOut)
		},
	})
	return cmd
}

// ─── lab ──────────────────────────────────────────────────────────────────────

func labCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lab",
		Short: "Docker lab environment management",
	}
	cmd.PersistentFlags().String("scenario", "default", "Lab scenario name")

	upCmd := &cobra.Command{
		Use:   "up",
		Short: "Start lab containers",
		Run: func(c *cobra.Command, a []string) {
			scenario, _ := c.Flags().GetString("scenario")
			printInfo("Starting lab (scenario=%s%s%s) …", cCyan, scenario, ansiR)
			out, _ := exec.Command("docker", "compose", "-f", "lab/docker-compose.yml", "up", "-d").CombinedOutput()
			if len(strings.TrimSpace(string(out))) > 0 {
				fmt.Fprintln(ConsoleOut, cMuted+string(out)+ansiR)
			}
			fmt.Fprintln(ConsoleOut)
			tbl := newTable("Container", "IP", "Role")
			tbl.addRow(cWhite+"vesper-attacker"+ansiR, cCyan+"172.20.0.10"+ansiR, "C2 / Operator")
			tbl.addRow(cWhite+"vesper-target1"+ansiR, cCyan+"172.20.0.20"+ansiR, "Victim (Linux)")
			tbl.addRow(cWhite+"vesper-target2"+ansiR, cCyan+"172.20.0.21"+ansiR, "Victim (Windows)")
			tbl.addRow(cWhite+"vesper-dashboard"+ansiR, cCyan+"172.20.0.30"+ansiR, "Web UI")
			tbl.addRow(cWhite+"vesper-ollama"+ansiR, cCyan+"172.20.0.40"+ansiR, "Local AI")
			tbl.render()
			fmt.Fprintf(ConsoleOut, "\n  %s●%s Dashboard: %shttp://localhost:3000%s\n\n", cSuccess, ansiR, cInfo+ansiB, ansiR)
		},
	}

	downCmd := &cobra.Command{
		Use:   "down",
		Short: "Stop and remove lab containers",
		Run: func(c *cobra.Command, a []string) {
			_ = exec.Command("docker", "compose", "-f", "lab/docker-compose.yml", "down").Run()
			printOK("Lab stopped.")
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show lab container status",
		Run: func(c *cobra.Command, a []string) {
			out, err := exec.Command("docker", "compose", "-f", "lab/docker-compose.yml", "ps").CombinedOutput()
			printSection("LAB STATUS")
			if err != nil {
				printWarn("Docker not available: %v", err)
				return
			}
			fmt.Fprintln(ConsoleOut, cMuted+string(out)+ansiR)
		},
	}

	cmd.AddCommand(upCmd, downCmd, statusCmd)
	return cmd
}
