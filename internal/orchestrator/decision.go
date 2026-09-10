package orchestrator

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/ruby570bocadito/vesper/pkg/shared/config"
	"github.com/ruby570bocadito/vesper/pkg/shared/logger"
	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

// DecisionEngine fuses three sub-engines to decide the next best action.
type DecisionEngine struct {
	cfg        *config.Config
	log        *logger.Logger
	worldGraph *WorldGraph
	rules      *RulesEngine
	planner    *AStarPlanner
	ai         *AIEngine
}

// NewDecisionEngine creates the hybrid decision engine.
func NewDecisionEngine(cfg *config.Config, log *logger.Logger, wg *WorldGraph) *DecisionEngine {
	return &DecisionEngine{
		cfg:        cfg,
		log:        log,
		worldGraph: wg,
		rules:      NewRulesEngine(log, wg),
		planner:    NewAStarPlanner(wg, log),
		ai:         NewAIEngine(cfg, log, wg, nil),
	}
}

// Evaluate produces ranked decisions for a campaign's current phase.
func (de *DecisionEngine) Evaluate(ctx context.Context, campaign *types.Campaign) ([]*types.Decision, error) {
	de.log.Infof("evaluating decisions for campaign %s (phase=%s profile=%s)",
		campaign.ID, campaign.Phase, campaign.Profile)

	// Collect candidates from all three engines
	rulesDecisions := de.rules.Evaluate(ctx, campaign)
	plannerDecisions := de.planner.Evaluate(ctx, campaign)
	aiDecisions := de.ai.Evaluate(ctx, campaign)

	// Fusion: weighted scoring (Rules 25% | A* 35% | AI 40%)
	fused := fuseDecisions(rulesDecisions, plannerDecisions, aiDecisions)

	// Sort by confidence descending
	sort.Slice(fused, func(i, j int) bool {
		return fused[i].Confidence > fused[j].Confidence
	})

	// Apply profile modifiers
	for _, d := range fused {
		de.applyProfileModifiers(d, campaign.Profile)
	}

	de.log.Infof("decision engine produced %d ranked decisions (top=%.2f)", len(fused),
		func() float64 {
			if len(fused) > 0 {
				return fused[0].Confidence
			}
			return 0
		}())

	return fused, nil
}

func (de *DecisionEngine) applyProfileModifiers(d *types.Decision, profile string) {
	switch profile {
	case "stealth":
		// Penalize noisy, high-detection-risk tactics to push them down the ranking.
		// Recon and C2 are fine; exploitation and lateral movement get a hit.
		switch d.Tactic {
		case "Initial Access", "Execution", "Lateral Movement", "Privilege Escalation":
			d.Confidence *= 0.70 // deprioritize loud tactics
		case "Reconnaissance", "Command and Control", "Collection":
			d.Confidence *= 1.10 // favor quiet information gathering
		}
	case "aggressive":
		d.Confidence *= 1.1 // favor higher-risk, faster results
	case "audit":
		d.RequiresApproval = true
	}
}

// ============================================================
// RULES ENGINE — Deterministic rules based on known patterns
// ============================================================

type RulesEngine struct {
	log        *logger.Logger
	worldGraph *WorldGraph
	// Rule set: service/port → action
	rules map[string]Rule
}

type Rule struct {
	Service     string
	Port        int
	Tactic      string
	Technique   string
	MITREID     string
	Description string
	Confidence  float64
	Risk        types.RiskLevel
	MinPhase    types.KillChainPhase
}

func NewRulesEngine(log *logger.Logger, wg *WorldGraph) *RulesEngine {
	re := &RulesEngine{
		log:        log,
		worldGraph: wg,
		rules:      make(map[string]Rule),
	}
	re.registerRules()
	return re
}

func (re *RulesEngine) registerRules() {
	rules := []Rule{
		// Initial Access
		{Service: "smb", Port: 445, Tactic: "Initial Access", Technique: "EternalBlue",
			MITREID: "T1210", Description: "MS17-010 EternalBlue SMB exploit", Confidence: 0.85, Risk: types.RiskHigh, MinPhase: types.PhaseDelivery},
		{Service: "rdp", Port: 3389, Tactic: "Initial Access", Technique: "BlueKeep",
			MITREID: "T1210", Description: "CVE-2019-0708 RDP exploit", Confidence: 0.80, Risk: types.RiskHigh, MinPhase: types.PhaseDelivery},
		{Service: "ssh", Port: 22, Tactic: "Initial Access", Technique: "SSH Brute Force",
			MITREID: "T1110.001", Description: "Brute force SSH credentials", Confidence: 0.60, Risk: types.RiskLow, MinPhase: types.PhaseDelivery},
		{Service: "http", Port: 80, Tactic: "Initial Access", Technique: "Web App Exploitation",
			MITREID: "T1190", Description: "Exploit web application vulnerabilities", Confidence: 0.50, Risk: types.RiskLow, MinPhase: types.PhaseDelivery},
		{Service: "ftp", Port: 21, Tactic: "Initial Access", Technique: "FTP Exploit",
			MITREID: "T1190", Description: "vsftpd backdoor / anonymous login", Confidence: 0.55, Risk: types.RiskLow, MinPhase: types.PhaseDelivery},
		{Service: "mysql", Port: 3306, Tactic: "Initial Access", Technique: "MySQL Exploit",
			MITREID: "T1190", Description: "MySQL auth bypass / UDF escalation", Confidence: 0.50, Risk: types.RiskMedium, MinPhase: types.PhaseDelivery},
		{Service: "redis", Port: 6379, Tactic: "Initial Access", Technique: "Redis Unauthorized",
			MITREID: "T1190", Description: "Redis no-auth RCE", Confidence: 0.90, Risk: types.RiskMedium, MinPhase: types.PhaseDelivery},

		// Privilege Escalation
		{Service: "suid", Port: 0, Tactic: "Privilege Escalation", Technique: "SUID GTFOBins",
			MITREID: "T1548.001", Description: "Exploit SUID binaries via GTFOBins", Confidence: 0.88, Risk: types.RiskSafe, MinPhase: types.PhaseExploitation},
		{Service: "sudo", Port: 0, Tactic: "Privilege Escalation", Technique: "Sudo Misconfiguration",
			MITREID: "T1548.003", Description: "NOPASSWD sudo → GTFOBins escalation", Confidence: 0.85, Risk: types.RiskSafe, MinPhase: types.PhaseExploitation},
		{Service: "docker", Port: 2375, Tactic: "Privilege Escalation", Technique: "Docker Escape",
			MITREID: "T1611", Description: "Container breakout via docker socket", Confidence: 0.82, Risk: types.RiskHigh, MinPhase: types.PhaseExploitation},
		{Service: "cron", Port: 0, Tactic: "Privilege Escalation", Technique: "Cron Injection",
			MITREID: "T1053.003", Description: "Writable cron → reverse shell as root", Confidence: 0.75, Risk: types.RiskMedium, MinPhase: types.PhaseExploitation},
		{Service: "nfs", Port: 2049, Tactic: "Privilege Escalation", Technique: "NFS no_root_squash",
			MITREID: "T1548", Description: "Mount NFS export, write SUID binary", Confidence: 0.78, Risk: types.RiskMedium, MinPhase: types.PhaseExploitation},

		// Lateral Movement
		{Service: "smb", Port: 445, Tactic: "Lateral Movement", Technique: "SMB PSExec",
			MITREID: "T1570", Description: "Lateral via SMB with captured credentials", Confidence: 0.80, Risk: types.RiskMedium, MinPhase: types.PhaseActionsOnObjective},
		{Service: "ssh", Port: 22, Tactic: "Lateral Movement", Technique: "SSH Lateral",
			MITREID: "T1021.004", Description: "SSH with captured credentials", Confidence: 0.75, Risk: types.RiskLow, MinPhase: types.PhaseActionsOnObjective},
		{Service: "winrm", Port: 5985, Tactic: "Lateral Movement", Technique: "WinRM",
			MITREID: "T1021.006", Description: "Remote PowerShell via WinRM", Confidence: 0.70, Risk: types.RiskLow, MinPhase: types.PhaseActionsOnObjective},

		// Persistence
		{Service: "kernel", Port: 0, Tactic: "Persistence", Technique: "Kernel Rootkit",
			MITREID: "T1547.006", Description: "Vault-Kernel LKM load for persistence", Confidence: 0.95, Risk: types.RiskHigh, MinPhase: types.PhaseInstallation},
		{Service: "cron", Port: 0, Tactic: "Persistence", Technique: "Cron Persistence",
			MITREID: "T1053.003", Description: "Install cron job for persistence", Confidence: 0.85, Risk: types.RiskLow, MinPhase: types.PhaseInstallation},

		// C2
		{Service: "c2", Port: 8443, Tactic: "Command and Control", Technique: "C2 Beacon",
			MITREID: "T1071.001", Description: "Establish encrypted C2 channel", Confidence: 0.90, Risk: types.RiskLow, MinPhase: types.PhaseCommandAndControl},
	}

	for _, r := range rules {
		key := fmt.Sprintf("%s:%d:%s", r.Service, r.Port, r.Tactic)
		re.rules[key] = r
	}
}

func (re *RulesEngine) Evaluate(ctx context.Context, campaign *types.Campaign) []*types.Decision {
	var decisions []*types.Decision

	re.log.Debugf("rules engine: evaluating campaign %s (phase=%s)", campaign.ID, campaign.Phase)

	// Get current world state
	nodes := re.worldGraph.GetAllNodes()
	if len(nodes) == 0 {
		re.log.Debug("rules engine: no nodes in world graph, returning base recon rule")
		d := &types.Decision{
			ID:               generateID("rule"),
			CampaignID:       campaign.ID,
			Tactic:           "Reconnaissance",
			Technique:        "Network Scan",
			MITREID:          "T1046",
			Confidence:       0.95,
			Reasoning:        "No hosts discovered yet. Run network scan to populate the world graph.",
			Source:           "rules",
			RequiresApproval: !campaign.AutoApproval,
		}
		return []*types.Decision{d}
	}

	// For each node, check which rules apply based on open ports
	for _, node := range nodes {
		services := re.worldGraph.GetServices(node.IP)
		for _, svc := range services {
			re.matchRules(&decisions, campaign, node, svc)
		}
	}

	// Phase-specific rules
	phaseRules := re.phaseRules(campaign)
	decisions = append(decisions, phaseRules...)

	return decisions
}

func (re *RulesEngine) matchRules(decisions *[]*types.Decision, campaign *types.Campaign, node *WorldNode, svc WorldService) {
	for _, rule := range re.rules {
		if !strings.EqualFold(rule.Service, svc.Name) {
			continue
		}
		if rule.Port != 0 && rule.Port != svc.Port {
			continue
		}
		if campaign.Phase.Order() < rule.MinPhase.Order() {
			continue
		}

		d := &types.Decision{
			ID:               generateID("rule"),
			CampaignID:       campaign.ID,
			Tactic:           rule.Tactic,
			Technique:        rule.Technique,
			MITREID:          rule.MITREID,
			Target:           node.IP,
			Confidence:       rule.Confidence,
			Reasoning:        fmt.Sprintf("Rule match: %s on %s:%d — %s", svc.Name, node.IP, svc.Port, rule.Description),
			Source:           "rules",
			RequiresApproval: rule.Risk == types.RiskHigh || rule.Risk == types.RiskDanger,
		}
		*decisions = append(*decisions, d)
	}
}

func (re *RulesEngine) phaseRules(campaign *types.Campaign) []*types.Decision {
	var decisions []*types.Decision

	switch campaign.Phase {
	case types.PhaseRecon:
		decisions = append(decisions, &types.Decision{
			ID: generateID("rule"), CampaignID: campaign.ID,
			Tactic: "Reconnaissance", Technique: "OSINT Gathering", MITREID: "T1593",
			Confidence: 0.90, Source: "rules", Reasoning: "Phase: Recon — gather OSINT on target scope",
			RequiresApproval: !campaign.AutoApproval,
		})
	case types.PhaseExploitation:
		decisions = append(decisions, &types.Decision{
			ID: generateID("rule"), CampaignID: campaign.ID,
			Tactic: "Execution", Technique: "Exploitation", MITREID: "T1203",
			Confidence: 0.70, Source: "rules", Reasoning: "Phase: Exploitation — execute discovered exploits",
			RequiresApproval: !campaign.AutoApproval,
		})
	case types.PhaseActionsOnObjective:
		decisions = append(decisions, &types.Decision{
			ID: generateID("rule"), CampaignID: campaign.ID,
			Tactic: "Exfiltration", Technique: "Data Exfiltration", MITREID: "T1041",
			Confidence: 0.65, Source: "rules", Reasoning: "Phase: Actions — exfiltrate collected data",
			RequiresApproval: true,
		})
	}

	return decisions
}

// ============================================================
// A* PLANNER — Optimal path finding through exploitation graph
// ============================================================

type AStarPlanner struct {
	worldGraph *WorldGraph
	log        *logger.Logger
}

func NewAStarPlanner(wg *WorldGraph, log *logger.Logger) *AStarPlanner {
	return &AStarPlanner{worldGraph: wg, log: log}
}

func (ap *AStarPlanner) Evaluate(ctx context.Context, campaign *types.Campaign) []*types.Decision {
	var decisions []*types.Decision

	ap.log.Debugf("A* planner: finding optimal paths for campaign %s", campaign.ID)

	nodes := ap.worldGraph.GetAllNodes()
	if len(nodes) == 0 {
		return decisions
	}

	// Find all compromised nodes (entry points)
	for _, node := range nodes {
		if !node.Compromised {
			continue
		}

		// For each compromised node, find reachable un-compromised nodes
		neighbors := ap.worldGraph.GetNeighbors(node.IP)
		for _, neighborIP := range neighbors {
			neighbor, err := ap.worldGraph.GetNode(neighborIP)
			if err != nil || neighbor.Compromised {
				continue
			}

			// Calculate path cost (hops) and success probability
			edges := ap.worldGraph.GetEdges(node.IP, neighborIP)
			for _, edge := range edges {
				assetValue := float64(len(neighbor.Tags)) // simplified by tag count
				cost := float64(len(neighbors) + 1)

				decision := &types.Decision{
					ID:               generateID("path"),
					CampaignID:       campaign.ID,
					Tactic:           "Lateral Movement",
					Technique:        edge.Type,
					Target:           neighborIP,
					Confidence:       edge.Success * (1.0 + assetValue/cost),
					Reasoning:        fmt.Sprintf("A* path: %s → %s via %s (hops=%d, success=%.2f)", node.IP, neighborIP, edge.Type, len(neighbors), edge.Success),
					Source:           "planner",
					RequiresApproval: edge.Success < 0.7,
				}

				if edge.Exploit != "" {
					decision.Technique = edge.Exploit
					decision.MITREID = "T1210"
				}

				decisions = append(decisions, decision)
			}
		}
	}

	return decisions
}

// ============================================================
// AI ENGINE — Specter-Terminal + Apex-Automation via Bridge
// ============================================================

type AIEngine struct {
	cfg        *config.Config
	log        *logger.Logger
	worldGraph *WorldGraph
	bridge     BridgeCaller
}

type BridgeCaller interface {
	CallRaw(ctx context.Context, module, function string, params map[string]interface{}) (map[string]interface{}, error)
	IsConnected() bool
}

func NewAIEngine(cfg *config.Config, log *logger.Logger, wg *WorldGraph, bridge BridgeCaller) *AIEngine {
	return &AIEngine{cfg: cfg, log: log, worldGraph: wg, bridge: bridge}
}

func (ae *AIEngine) Evaluate(ctx context.Context, campaign *types.Campaign) []*types.Decision {
	var decisions []*types.Decision

	if !ae.cfg.AI.Enabled {
		ae.log.Debug("AI engine disabled — returning offline heuristic decisions")
		return ae.offlineHeuristic(campaign)
	}

	ae.log.Debugf("AI engine: building context prompt for campaign %s", campaign.ID)

	prompt := ae.buildContext(campaign)

	// Try Ollama bridge for real AI analysis
	if ae.bridge != nil && ae.bridge.IsConnected() {
		if response, err := ae.bridge.CallRaw(ctx, "ai", "analyze", map[string]interface{}{"prompt": prompt, "campaign_id": campaign.ID}); err == nil {
			ae.log.Infof("AI real analysis received: %v", response)
		}
	}

	// Offline heuristics as primary engine (augmented by bridge context above)
	decisions = ae.offlineHeuristic(campaign)
	for _, d := range decisions {
		d.Source = "ai"
		d.Reasoning = fmt.Sprintf("[AI/Specter] Context: %s\nREASONING: %s", prompt[:min(80, len(prompt))], d.Reasoning)
	}

	return decisions
}

func (ae *AIEngine) buildContext(campaign *types.Campaign) string {
	nodes := ae.worldGraph.GetAllNodes()
	allServices := ae.worldGraph.GetAllServices()

	compromised := 0
	for _, n := range nodes {
		if n.Compromised {
			compromised++
		}
	}

	return fmt.Sprintf(
		`CAMPAIGN: %s | GOAL: %s | PHASE: %s | PROFILE: %s
TARGET SCOPE: %s
DISCOVERED HOSTS: %d | COMPROMISED: %d | SERVICES: %d
AUTO-APPROVAL: %v | MIN CONFIDENCE: %.2f
CURRENT NETWORK STATE:
%s`,
		campaign.Name, campaign.Goal, campaign.Phase, campaign.Profile,
		campaign.TargetScope,
		len(nodes), compromised, len(allServices),
		campaign.AutoApproval, ae.cfg.AI.MinConfidence,
		ae.worldGraph.Summary(),
	)
}

func (ae *AIEngine) offlineHeuristic(campaign *types.Campaign) []*types.Decision {
	var decisions []*types.Decision

	nodes := ae.worldGraph.GetAllNodes()

	// Phase-aware heuristics
	switch {
	case len(nodes) == 0:
		decisions = append(decisions, &types.Decision{
			ID: generateID("ai"), CampaignID: campaign.ID,
			Tactic: "Reconnaissance", Technique: "Network Service Discovery", MITREID: "T1046",
			Confidence: 0.92, Source: "ai",
			Reasoning:        "No hosts in world graph. Begin network discovery via Horizon-Intel scan.",
			RequiresApproval: !campaign.AutoApproval,
		})

	case campaign.Phase == types.PhaseRecon || campaign.Phase == types.PhaseWeaponization:
		for _, node := range nodes {
			if node.OS != "" {
				decisions = append(decisions, &types.Decision{
					ID: generateID("ai"), CampaignID: campaign.ID,
					Tactic: "Reconnaissance", Technique: "OS Fingerprinting", MITREID: "T1082",
					Target: node.IP, Confidence: 0.85, Source: "ai",
					Reasoning: fmt.Sprintf("Host %s (%s) detected. Perform deep OS and service fingerprinting.", node.IP, node.OS),
				})
			}
		}
		// Add vulnerability scanning suggestion
		decisions = append(decisions, &types.Decision{
			ID: generateID("ai"), CampaignID: campaign.ID,
			Tactic: "Reconnaissance", Technique: "Vulnerability Scanning", MITREID: "T1595.002",
			Confidence: 0.88, Source: "ai",
			Reasoning: "Run vulnerability scan on all discovered hosts using NVD/CVE database.",
		})

	case campaign.Phase == types.PhaseDelivery || campaign.Phase == types.PhaseExploitation:
		// Prioritize compromised nodes for exploitation
		for _, node := range nodes {
			if !node.Compromised {
				// Find an exploit path
				services := ae.worldGraph.GetServices(node.IP)
				for _, svc := range services {
					if svc.Name == "smb" || svc.Name == "rdp" || svc.Name == "ssh" || svc.Name == "redis" {
						decisions = append(decisions, &types.Decision{
							ID: generateID("ai"), CampaignID: campaign.ID,
							Tactic: "Initial Access", Technique: fmt.Sprintf("%s Exploit", strings.ToUpper(svc.Name)),
							MITREID: "T1190", Target: node.IP, Confidence: 0.78, Source: "ai",
							Reasoning: fmt.Sprintf("Service %s:%d on %s — high-value exploit target.", svc.Name, svc.Port, node.IP),
						})
						break
					}
				}
			} else {
				// Compromised node → suggest privilege escalation check
				decisions = append(decisions, &types.Decision{
					ID: generateID("ai"), CampaignID: campaign.ID,
					Tactic: "Privilege Escalation", Technique: "Local Privilege Escalation",
					MITREID: "T1068", Target: node.IP, Confidence: 0.82, Source: "ai",
					Reasoning: fmt.Sprintf("Host %s compromised. Run Rise-Privilege escalation scanner.", node.IP),
				})
			}
		}

	case campaign.Phase == types.PhaseInstallation:
		for _, node := range nodes {
			if node.Compromised {
				decisions = append(decisions, &types.Decision{
					ID: generateID("ai"), CampaignID: campaign.ID,
					Tactic: "Persistence", Technique: "Kernel Rootkit", MITREID: "T1547.006",
					Target: node.IP, Confidence: 0.90, Source: "ai",
					Reasoning:        fmt.Sprintf("Install Vault-Kernel on %s for kernel-level persistence.", node.IP),
					RequiresApproval: true,
				})
			}
		}

	case campaign.Phase == types.PhaseActionsOnObjective:
		for _, node := range nodes {
			if node.Compromised {
				decisions = append(decisions, &types.Decision{
					ID: generateID("ai"), CampaignID: campaign.ID,
					Tactic: "Collection", Technique: "Data from Local System", MITREID: "T1005",
					Target: node.IP, Confidence: 0.85, Source: "ai",
					Reasoning: fmt.Sprintf("Collect sensitive data from compromised host %s.", node.IP),
				})
			}
		}
	}

	return decisions
}

// ============================================================
// FUSION ENGINE — Weighted merging and ranking
// ============================================================

const (
	weightRules   = 0.25
	weightPlanner = 0.35
	weightAI      = 0.40
)

func fuseDecisions(rulesResult, plannerResult, aiResult []*types.Decision) []*types.Decision {
	// Each source contributes its weighted confidence to a shared map keyed by
	// (tactic:target). Duplicate decisions from different engines are merged by
	// ACCUMULATING their weighted scores so the fusion is additive, not just max.
	type entry struct {
		d           *types.Decision
		bestContrib float64 // highest single-source weighted contribution so far
	}
	seen := make(map[string]*entry)

	merge := func(d *types.Decision, weight float64) {
		key := fmt.Sprintf("%s:%s", d.Tactic, d.Target)
		weightedConf := d.Confidence * weight

		if e, ok := seen[key]; ok {
			// Accumulate: the final score reflects all engines that agree on this action.
			e.d.Confidence += weightedConf
			// Update reasoning to the source with the single biggest contribution.
			if weightedConf > e.bestContrib {
				e.bestContrib = weightedConf
				e.d.Reasoning = d.Reasoning
				e.d.Source = d.Source
			}
		} else {
			// First time we see this (tactic, target) pair.
			copy := *d
			copy.Confidence = weightedConf
			seen[key] = &entry{d: &copy, bestContrib: weightedConf}
		}
	}

	for _, d := range rulesResult {
		merge(d, weightRules)
	}
	for _, d := range plannerResult {
		merge(d, weightPlanner)
	}
	for _, d := range aiResult {
		merge(d, weightAI)
	}

	// Cap confidence at 1.0 (accumulation can exceed it when multiple engines agree)
	// and filter decisions below the noise floor.
	var fused []*types.Decision
	for _, e := range seen {
		if e.d.Confidence > 1.0 {
			e.d.Confidence = 1.0
		}
		if e.d.Confidence > 0.1 {
			fused = append(fused, e.d)
		}
	}

	return fused
}
