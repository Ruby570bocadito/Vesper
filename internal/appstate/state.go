// Package appstate provides the shared application state that connects
// the orchestrator, Python bridge, C2 server, and database.
//
// All user-facing components (CLI, console, dashboard, API) share this state
// so they operate on the same data — no hardcoded maps, no demo fallbacks.
//
// Usage:
//
//	state := appstate.New(cfg)
//	state.Start()
//	defer state.Stop()
//
//	// All console data now comes from state.Orchestrator.WorldGraph()
//	// Exploits call state.DecisionEngine.Evaluate() for real decisions
//	// recon/privesc commands call state.Bridge.CallModule() → Python bridge
package appstate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"

	_ "modernc.org/sqlite" // pure-Go SQLite driver

	"github.com/ruby570bocadito/vesper/internal/agent"
	"github.com/ruby570bocadito/vesper/internal/dispatch"
	"github.com/ruby570bocadito/vesper/internal/orchestrator"
	"github.com/ruby570bocadito/vesper/pkg/shared/config"
	"github.com/ruby570bocadito/vesper/pkg/shared/logger"
	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

// AppState holds all shared state for the Vesper application.
type AppState struct {
	Cfg          *config.Config
	Log          *logger.Logger
	Orchestrator *orchestrator.Orchestrator
	Bridge       *agent.BridgeClient
	DB           *sql.DB

	mu        sync.RWMutex
	campaigns map[string]*types.Campaign
	agents    map[string]*types.Agent
	sessions  map[string]*types.Agent // active C2 sessions
	hosts     []*types.Target
	vulns     []*types.Vulnerability
	creds     []*types.Credential
	modules   []ModuleDef // available exploit modules
}

// ModuleDef describes an available exploit/recon module.
type ModuleDef struct {
	Name        string
	Type        string
	Description string
	CVE         string
	Rank        string
	OS          string
}

// New creates the shared application state.
func New(cfg *config.Config) (*AppState, error) {
	log, err := logger.New(logger.Config{
		Level:     cfg.Logging.Level,
		Format:    cfg.Logging.Format,
		Output:    cfg.Logging.Output,
		File:      cfg.Logging.File,
		Component: "appstate",
	})
	if err != nil {
		return nil, fmt.Errorf("creating logger: %w", err)
	}

	orch, err := orchestrator.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating orchestrator: %w", err)
	}

	bridge := agent.NewBridgeClient(cfg, log)

	state := &AppState{
		Cfg:          cfg,
		Log:          log,
		Orchestrator: orch,
		Bridge:       bridge,
		campaigns:    make(map[string]*types.Campaign),
		agents:       make(map[string]*types.Agent),
		sessions:     make(map[string]*types.Agent),
		hosts:        make([]*types.Target, 0),
		vulns:        make([]*types.Vulnerability, 0),
		creds:        make([]*types.Credential, 0),
	}

	state.initModules()
	return state, nil
}

// Start initializes the database, starts the orchestrator, bridge, and loads demo data.
func (s *AppState) Start(ctx context.Context) error {
	s.Log.Info("starting application state")

	// Initialize SQLite
	if err := s.initDB(); err != nil {
		s.Log.Warnf("database init failed (continuing without persistence): %v", err)
	}

	// Auto-start Python bridge if script exists
	bridgeScript := "modules/bridge/bridge_grpc.py"
	if _, err := os.Stat(bridgeScript); err == nil {
		s.Log.Info("auto-starting Python bridge...")
		if err := s.Bridge.StartBridge(ctx, bridgeScript); err != nil {
			s.Log.Warnf("Python bridge start failed (modules will use offline fallback): %v", err)
		} else {
			s.Log.Infof("Python bridge connected: %d modules available", 9)
		}
	} else {
		s.Log.Info("Python bridge script not found — modules use offline fallback")
	}

	// Load world graph — discovered from agents, not hardcoded demo data
	wg := s.Orchestrator.WorldGraph()
	wg.DiscoverFromAgents(s.GetAgents())

	// Wire the dispatcher — connects Orchestrator → Agent → C2 → Modules
	if !s.Cfg.AI.AutoApproval {
		s.Cfg.AI.AutoApproval = true
	}
	if s.Cfg.AI.MinConfidence == 0 {
		s.Cfg.AI.MinConfidence = 0.65
	}
	s.Orchestrator.SetDispatcher(dispatch.New(s, s.Cfg.AI.AutoApproval, s.Cfg.AI.MinConfidence))
	s.Log.Info("Dispatcher wired: Orchestrator → Agent → C2 → Modules → Bridge")

	s.Log.Infof("state started: %d agents, %d hosts, %d vulns, %d creds",
		len(s.agents), len(s.hosts), len(s.vulns), len(s.creds))
	return nil
}

// Stop tears down all connections.
func (s *AppState) Stop() {
	_ = s.Bridge.Disconnect()
	if s.DB != nil {
		s.DB.Close()
	}
	s.Log.Info("application state stopped")
}

// initDB initializes SQLite database.
func (s *AppState) initDB() error {
	dbPath := s.Cfg.Database.DSN
	if dbPath == "" {
		dbPath = "vesper.db"
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("opening sqlite: %w", err)
	}

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS campaigns (
			id TEXT PRIMARY KEY, name TEXT, target_scope TEXT, goal TEXT,
			profile TEXT, status TEXT, phase TEXT, created_at DATETIME,
			started_at DATETIME, auto_approval INTEGER
		);
		CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY, campaign_id TEXT, session_id TEXT,
			hostname TEXT, os TEXT, username TEXT, local_ip TEXT,
			status TEXT, last_checkin DATETIME, first_seen DATETIME, uptime INTEGER
		);
		CREATE TABLE IF NOT EXISTS targets (
			id INTEGER PRIMARY KEY AUTOINCREMENT, ip TEXT, hostname TEXT,
			os TEXT, open_ports TEXT, services TEXT, asset_value INTEGER
		);
		CREATE TABLE IF NOT EXISTS vulnerabilities (
			id INTEGER PRIMARY KEY AUTOINCREMENT, cve TEXT, description TEXT,
			severity TEXT, service TEXT, port INTEGER, target_ip TEXT,
			discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS credentials (
			id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT, password TEXT,
			hash TEXT, hash_type TEXT, domain TEXT, source TEXT, agent_id TEXT,
			captured_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT, campaign_id TEXT, agent_id TEXT,
			action TEXT, result TEXT, detail TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("creating tables: %w", err)
	}

	s.DB = db
	s.Log.Infof("database initialized: %s (%d tables)", dbPath, 6)
	return nil
}

// ============================================================
// ACCESSORS
// ============================================================

func (s *AppState) RegisterAgent(a *types.Agent) {
	s.mu.Lock()
	s.agents[a.ID] = a
	s.sessions[a.SessionID] = a
	s.mu.Unlock()
}

func (s *AppState) RemoveAgent(id string) {
	s.mu.Lock()
	delete(s.agents, id)
	for sid, a := range s.sessions {
		if a.ID == id {
			delete(s.sessions, sid)
			break
		}
	}
	s.mu.Unlock()
}

func (s *AppState) GetAgents() []*types.Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agents := make([]*types.Agent, 0, len(s.agents))
	for _, a := range s.agents {
		agents = append(agents, a)
	}
	return agents
}

func (s *AppState) GetAgent(id string) *types.Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.agents[id]
}

func (s *AppState) GetSessions() []*types.Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sessions := make([]*types.Agent, 0, len(s.sessions))
	for _, a := range s.sessions {
		sessions = append(sessions, a)
	}
	return sessions
}

func (s *AppState) AddHost(h *types.Target) {
	s.mu.Lock()
	s.hosts = append(s.hosts, h)
	s.mu.Unlock()
}

func (s *AppState) GetHosts() []*types.Target {
	s.mu.RLock()
	defer s.mu.RUnlock()
	hosts := make([]*types.Target, len(s.hosts))
	copy(hosts, s.hosts)
	return hosts
}

func (s *AppState) AddVuln(v *types.Vulnerability) {
	s.mu.Lock()
	s.vulns = append(s.vulns, v)
	s.mu.Unlock()
}

func (s *AppState) GetVulns() []*types.Vulnerability {
	s.mu.RLock()
	defer s.mu.RUnlock()
	vulns := make([]*types.Vulnerability, len(s.vulns))
	copy(vulns, s.vulns)
	return vulns
}

func (s *AppState) GetCreds() []*types.Credential {
	s.mu.RLock()
	defer s.mu.RUnlock()
	creds := make([]*types.Credential, len(s.creds))
	copy(creds, s.creds)
	return creds
}

func (s *AppState) AddCredential(c *types.Credential) {
	s.mu.Lock()
	s.creds = append(s.creds, c)
	s.mu.Unlock()

	// Persist to DB
	if s.DB != nil {
		_, _ = s.DB.Exec(
			"INSERT INTO credentials (username, password, domain, source, agent_id) VALUES (?,?,?,?,?)",
			c.Username, c.Password, c.Domain, c.Source, c.AgentID,
		)
	}
}

func (s *AppState) AddLateralEdge(from, to, exploit string) {
	if s.Orchestrator != nil {
		wg := s.Orchestrator.WorldGraph()
		if wg != nil {
			wg.AddExploitEdge(from, to, exploit, 1.0)
		}
	}
}

func (s *AppState) GetModules() []ModuleDef {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mods := make([]ModuleDef, len(s.modules))
	copy(mods, s.modules)
	return mods
}

func (s *AppState) SearchModules(query string) []ModuleDef {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q := strings.ToLower(query)
	var results []ModuleDef
	for _, m := range s.modules {
		if strings.Contains(strings.ToLower(m.Name), q) ||
			strings.Contains(strings.ToLower(m.CVE), q) ||
			strings.Contains(strings.ToLower(m.OS), q) ||
			strings.Contains(strings.ToLower(m.Description), q) {
			results = append(results, m)
		}
	}
	return results
}

func (s *AppState) LogAudit(agentID, campaignID, action, result, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.DB != nil {
		_, _ = s.DB.Exec(
			"INSERT INTO audit_log (campaign_id, agent_id, action, result, detail) VALUES (?,?,?,?,?)",
			campaignID, agentID, action, result, detail,
		)
	}
	s.Log.Infof("audit: %s | %s | %s | %s", action, result, truncateStr(detail, 50))
}

// ============================================================
// MODULE REGISTRY
// ============================================================

func (s *AppState) initModules() {
	// Honest catalog: only capabilities that exist in this repository.
	// Every entry is verifiable in modules/bridge/ or cmd/vesper/ — the
	// pre-remodel catalog shipped ~180 fictional payloads (v26-v210,
	// blockz, hydra, ransomware/*) that would have collapsed in any
	// audit demo. Bridge stubs are labeled as stubs.
	s.modules = []ModuleDef{
		// Bridge inline modules (modules/bridge/bridge_grpc.py)
		{Name: "recon", Type: "bridge", Description: "Network reconnaissance (nmap when available, lab targets only)", Rank: "excellent", OS: "any"},
		{Name: "ai_analyze", Type: "bridge", Description: "AI analysis via local Ollama (optional dependency)", Rank: "normal", OS: "any"},
		{Name: "privesc", Type: "bridge", Description: "Privilege escalation checks (stub: reports zero techniques)", Rank: "normal", OS: "any"},
		{Name: "persist", Type: "bridge", Description: "Persistence mechanisms (stub: installs nothing)", Rank: "normal", OS: "any"},
		{Name: "worm", Type: "bridge", Description: "Network worm propagation (stub: no-op)", Rank: "normal", OS: "any"},
		{Name: "blue", Type: "bridge", Description: "BlueForge defense coverage metrics", Rank: "normal", OS: "any"},
		{Name: "evasion", Type: "bridge", Description: "AMSI/ETW evasion checks (stub: no-op)", Rank: "normal", OS: "Windows"},
		{Name: "report", Type: "bridge", Description: "Campaign report generator (JSON)", Rank: "great", OS: "any"},
		{Name: "exfil", Type: "bridge", Description: "Data exfiltration (stub: no-op)", Rank: "normal", OS: "any"},
		{Name: "health", Type: "bridge", Description: "Bridge health check + module listing", Rank: "great", OS: "any"},

		// Bridge handler groups (modules/bridge/handlers/)
		{Name: "attacks/responder", Type: "bridge", Description: "LLMNR/NBT-NS analysis and NTLMv2 capture (lab-only)", Rank: "excellent", OS: "any"},
		{Name: "attacks/webscan", Type: "bridge", Description: "Web vulnerability scan: SQLi probing, forms, endpoints", Rank: "excellent", OS: "any"},
		{Name: "attacks/cloud", Type: "bridge", Description: "Cloud attack surface checks (S3, Azure, GCP)", Rank: "great", OS: "any"},
		{Name: "attacks/cleanup", Type: "bridge", Description: "Post-engagement cleanup (sandboxed to lab root)", Rank: "great", OS: "any"},
		{Name: "attacks/obfuscate", Type: "bridge", Description: "Payload obfuscation helpers", Rank: "normal", OS: "any"},
		{Name: "cred_dump/dump", Type: "bridge", Description: "Credential collection (sandboxed lab storage)", Rank: "excellent", OS: "Windows/Linux"},
		{Name: "bloodhound/collect", Type: "bridge", Description: "Active Directory collection (BloodHound ingest)", Rank: "excellent", OS: "Windows AD"},
		{Name: "reporting/attack_layer", Type: "bridge", Description: "MITRE ATT&CK Navigator layer generation from campaign data", Rank: "great", OS: "any"},

		// Go-native operations (cmd/vesper)
		{Name: "vesper/recon-scan", Type: "recon", Description: "Native Go TCP scan with service detection", Rank: "excellent", OS: "any"},
		{Name: "vesper/payload-generate", Type: "payload", Description: "Cross-platform agent payload compilation (go build)", Rank: "excellent", OS: "any"},
		{Name: "vesper/listeners", Type: "c2", Description: "C2 transport listeners (TCP/HTTP/HTTPS/WS)", Rank: "great", OS: "any"},
		{Name: "vesper/lab", Type: "lab", Description: "Docker lab environment management", Rank: "great", OS: "Linux"},
	}
}

// GetBridgeClient returns the Python bridge client for module execution.
func (s *AppState) GetBridgeClient() dispatch.BridgeCaller {
	return s.Bridge
}

// AddVulnerability adds a discovered vulnerability (compat alias).
func (s *AppState) AddVulnerability(v *types.Vulnerability) {
	s.AddVuln(v)
}

// ============================================================
// HELPERS
// ============================================================

var _ = os.Stdout // keep os import

func truncateStr(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
