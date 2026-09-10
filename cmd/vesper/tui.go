package main

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ruby570bocadito/vesper/internal/appstate"
	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

var (
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("63")).
			Bold(true)

	accentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	dangerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	phaseOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("✓")
	phaseActive  = lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Render("◉")
	phasePending = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("□")
)

type model struct {
	activeTab int
	tabs      []string
	width     int
	height    int
	state     *appstate.AppState
}

func initialModel(state *appstate.AppState) model {
	return model{
		activeTab: 0,
		tabs:      []string{"Dashboard", "Campaigns", "Agents", "AI", "Logs", "Lab"},
		state:     state,
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % len(m.tabs)
		case "shift+tab", "left", "h":
			m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
		}
	}
	return m, nil
}

func (m model) View() string {
	header := m.renderHeader()
	tabs := m.renderTabs()
	content := m.renderContent()

	return lipgloss.JoinVertical(lipgloss.Left, header, tabs, content)
}

func (m model) renderHeader() string {
	banner := titleStyle.Render(`
  ▄▄▄   ▄▄▄  ▄▄   ▄▄   ▄▄▄▄▄▄   ▄▄   ▄▄  ▄▄▄   ▄▄▄ 
  ▀██▄ ▄██▀  ██   ██  ██▀  ▀██  ██   ██  ▀██▄ ▄██▀ 
    ▀███▀    ███████  ██    ██  ███████    ▀███▀   
  ▄██▀ ▀██▄       ██  ██▄  ▄██       ██  ▄██▀ ▀██▄ 
  ██▀   ▀██       ██   ▀████▀        ██  ██▀   ▀██ 
  ▀▀     ▀▀       ▀▀                 ▀▀  ▀▀     ▀▀ `)

	info := mutedStyle.Render(fmt.Sprintf("v1.0 | %dx%d | [CTRL+C] Quit | [Tab] Switch", m.width, m.height))
	return lipgloss.JoinHorizontal(lipgloss.Top, banner, "   ", info)
}

func (m model) renderTabs() string {
	var rendered []string
	for i, tab := range m.tabs {
		if i == m.activeTab {
			rendered = append(rendered, accentStyle.Render(fmt.Sprintf("[%s]", tab)))
		} else {
			rendered = append(rendered, mutedStyle.Render(fmt.Sprintf(" %s ", tab)))
		}
	}
	return strings.Join(rendered, " │ ")
}

func (m model) renderContent() string {
	switch m.activeTab {
	case 0:
		return m.dashboardTab()
	case 1:
		return m.campaignsTab()
	case 2:
		return m.agentsTab()
	case 3:
		return m.aiTab()
	case 4:
		return m.logsTab()
	case 5:
		return m.labTab()
	default:
		return "No content"
	}
}

func (m model) dashboardTab() string {
	campaignName := "TFG-Demo"
	campaignPhase := "Exploitation"

	campaigns := m.state.Orchestrator.ListCampaigns()
	if len(campaigns) > 0 {
		c := campaigns[0]
		campaignName = c.Name
		campaignPhase = string(c.Phase)
		_ = float32(c.Progress)
	}

	title := titleStyle.Render(fmt.Sprintf("╭─ DASHBOARD ─ Campaign: %q · Phase: %s ──────╮",
		campaignName, campaignPhase))

	killChain := m.renderKillChain()

	// Build network map from AppState hosts
	hosts := m.state.GetHosts()
	netLines := " NETWORK MAP\n"
	for _, h := range hosts {
		status := "scanned"
		for _, a := range m.state.GetAgents() {
			if a.LocalIP == h.IP {
				if a.Status == types.AgentStatusOnline || a.Status == types.AgentStatusActive {
					status = "compromised"
				}
				break
			}
		}
		netLines += fmt.Sprintf(" ● %s (%s) [%s]\n", h.Hostname, h.IP, status)
	}
	if len(hosts) == 0 {
		netLines += " (no hosts discovered)\n"
	}
	networkMap := borderStyle.Copy().Width(34).Height(10).Render(netLines)

	// AI panel from first decision
	aiLines := " AI CONSOLE\n"
	if len(campaigns) > 0 {
		decisions, err := m.state.Orchestrator.Decide(context.Background(), campaigns[0].ID)
		if err == nil && len(decisions) > 0 {
			for i, d := range decisions {
				if i >= 4 {
					break
				}
				aiLines += fmt.Sprintf(" [%d] %s → %s [%.2f]\n", i+1, d.Tactic, d.Technique, d.Confidence)
			}
		} else {
			aiLines += " (no recommendations yet)\n"
		}
	}
	aiPanel := borderStyle.Copy().Width(34).Height(10).Render(aiLines)

	// Events from audit log
	events := borderStyle.Copy().Width(70).Height(6).Render(
		" LIVE FEED\n" + strings.Repeat("─", 68) + "\n" +
			"  Dashboard connected to live AppState\n" +
			fmt.Sprintf("  Agents: %d | Hosts: %d | Vulns: %d | Campaigns: %d\n",
				len(m.state.GetAgents()), len(hosts), len(m.state.GetVulns()), len(campaigns)))

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, networkMap, "  ", aiPanel)
	bottomRow := events

	return lipgloss.JoinVertical(lipgloss.Left, title, killChain, " ", topRow, " ", bottomRow)
}

func (m model) renderKillChain() string {
	phases := []struct {
		name string
		done bool
		act  bool
	}{
		{"Recon", false, false},
		{"Weaponize", false, false},
		{"Deliver", false, false},
		{"Exploit", false, false},
		{"Install", false, false},
		{"C2", false, false},
	}

	// Determine current phase from active campaign
	campaigns := m.state.Orchestrator.ListCampaigns()
	currentPhase := ""
	if len(campaigns) > 0 {
		currentPhase = string(campaigns[0].Phase)
	}
	phaseNames := []string{"recon", "weaponization", "delivery", "exploitation", "installation", "c2"}
	found := false
	for i, pn := range phaseNames {
		if strings.EqualFold(currentPhase, pn) {
			phases[i].act = true
			found = true
			break
		}
	}
	if found {
		for i := range phases {
			if phases[i].act {
				break
			}
			phases[i].done = true
		}
	}

	var items []string
	for _, p := range phases {
		switch {
		case p.done:
			items = append(items, fmt.Sprintf(" %s %s", p.name, phaseOK))
		case p.act:
			items = append(items, fmt.Sprintf(" %s %s", p.name, phaseActive))
		default:
			items = append(items, fmt.Sprintf(" %s %s", p.name, phasePending))
		}
	}

	progress := 0.0
	if len(campaigns) > 0 {
		progress = campaigns[0].Progress
	}
	barLen := 20
	filled := int(float64(barLen) * progress)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)
	pc := fmt.Sprintf(" %.0f%%", progress*100)

	return borderStyle.Copy().Width(70).Render(
		" KILL CHAIN " + strings.Join(items, " │ ") + "\n" +
			" " + accentStyle.Render(bar) + mutedStyle.Render(pc))
}

func (m model) campaignsTab() string {
	campaigns := m.state.Orchestrator.ListCampaigns()
	content := titleStyle.Render(" CAMPAIGNS") + "\n\n"

	if len(campaigns) > 0 {
		content += "  Active:\n"
		for _, c := range campaigns {
			statusColor := accentStyle
			if c.Status == types.CampaignStatusPaused {
				statusColor = mutedStyle
			}
			content += fmt.Sprintf("  ▸ %s  | %s | %s | %d agents | %.0f%%\n",
				c.Name, statusColor.Render(string(c.Status)), c.Phase, c.AgentCount, c.Progress*100)
		}
	} else {
		content += "  No active campaigns.\n"
	}

	return borderStyle.Copy().Width(70).Height(10).Render(content)
}

func (m model) agentsTab() string {
	agents := m.state.GetAgents()
	content := titleStyle.Render(" AGENTS") + "\n\n"

	if len(agents) > 0 {
		content += "  ID       Hostname    OS          User     Status\n"
		content += "  ──────── ─────────── ─────────── ──────── ──────\n"
		for _, a := range agents {
			statusColor := accentStyle
			if a.Status == types.AgentStatusDead {
				statusColor = dangerStyle
			} else if a.Status == types.AgentStatusIdle {
				statusColor = mutedStyle
			}
			content += fmt.Sprintf("  %-8s %-11s %-11s %-8s %s\n",
				trunc(a.ID, 8), trunc(a.Hostname, 11), trunc(a.OS, 11),
				trunc(a.Username, 8), statusColor.Render(string(a.Status)))
		}
	} else {
		content += "  No active agents.\n"
	}

	return borderStyle.Copy().Width(70).Height(10).Render(content)
}

func (m model) aiTab() string {
	campaigns := m.state.Orchestrator.ListCampaigns()
	content := titleStyle.Render(" AI CONSOLE (Specter + Apex + Ollama)") + "\n\n"

	if len(campaigns) > 0 {
		decisions, err := m.state.Orchestrator.Decide(context.Background(), campaigns[0].ID)
		if err == nil && len(decisions) > 0 {
			for i, d := range decisions {
				if i >= 6 {
					break
				}
				confColor := accentStyle
				if d.Confidence < 0.6 {
					confColor = mutedStyle
				}
				content += fmt.Sprintf("  [%d] %s → %s %s[%.2f]%s\n",
					i+1, d.Tactic, d.Technique, confColor, d.Confidence, colorReset)
			}
		} else {
			content += "  No recommendations available yet.\n"
		}
	} else {
		content += "  No active campaign.\n"
	}
	content += "\n  Bridge: "
	if m.state.Bridge.Connected() {
		content += accentStyle.Render("connected")
	} else {
		content += mutedStyle.Render("disconnected")
	}

	return borderStyle.Copy().Width(70).Height(10).Render(content)
}

func (m model) logsTab() string {
	agents := m.state.GetAgents()
	hosts := m.state.GetHosts()
	vulns := m.state.GetVulns()

	content := titleStyle.Render(" EVENT LOG") + "\n"
	content += strings.Repeat("─", 68) + "\n"
	content += fmt.Sprintf("  Agents: %d online/%d total\n",
		countOnline(agents), len(agents))
	content += fmt.Sprintf("  Hosts:  %d discovered\n", len(hosts))
	content += fmt.Sprintf("  Vulns:  %d found\n", len(vulns))
	content += fmt.Sprintf("  Creds:  %d captured\n", len(m.state.GetCreds()))
	content += fmt.Sprintf("  Bridge: %s\n", bridgeStatus(m.state.Bridge.Connected()))
	content += strings.Repeat("─", 68) + "\n"
	content += mutedStyle.Render(" [F] Filter [S] Search [/] Find [R] Refresh")

	return borderStyle.Copy().Width(70).Height(12).Render(content)
}

func (m model) labTab() string {
	content := titleStyle.Render(" LAB ENVIRONMENT") + "\n\n"
	content += "  Container          Status    IP\n"
	content += "  ─────────────────  ────────  ───────────\n"
	content += "  vesper-attacker     running  172.20.0.10\n"
	content += "  vesper-target1      running  172.20.0.20\n"
	content += "  vesper-target2      running  172.20.0.21\n"
	content += "  vesper-dashboard    running  172.20.0.30\n"
	content += "  vesper-ollama       running  172.20.0.40\n"
	content += "\n"
	content += fmt.Sprintf("  Agents deployed: %d\n", len(m.state.GetAgents()))
	content += fmt.Sprintf("  Hosts discovered: %d\n", len(m.state.GetHosts()))
	content += "\n"
	content += "  Dashboard: " + accentStyle.Render("http://localhost:3000") + "\n"
	content += mutedStyle.Render("  [U] Start  [D] Stop  [S] Status")

	return borderStyle.Copy().Width(70).Height(10).Render(content)
}

func StartTUI(state *appstate.AppState) error {
	p := tea.NewProgram(initialModel(state), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
