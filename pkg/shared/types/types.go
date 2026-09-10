// Package types defines shared domain types used across the full Vesper platform.
// These types are the canonical representation of core concepts
// (agents, campaigns, kill chain phases, etc.) and are used by gRPC, database,
// and internal logic.
package types

import "time"

// AgentStatus represents the current state of an implant/agent.
type AgentStatus string

const (
	AgentStatusOnline AgentStatus = "online"
	AgentStatusIdle   AgentStatus = "idle"
	AgentStatusActive AgentStatus = "active"
	AgentStatusDead   AgentStatus = "dead"
)

// CampaignStatus represents the current phase of a campaign.
type CampaignStatus string

const (
	CampaignStatusDraft     CampaignStatus = "draft"
	CampaignStatusRunning   CampaignStatus = "running"
	CampaignStatusPaused    CampaignStatus = "paused"
	CampaignStatusCompleted CampaignStatus = "completed"
	CampaignStatusFailed    CampaignStatus = "failed"
)

// KillChainPhase represents a phase in the cyber kill chain.
type KillChainPhase string

const (
	PhaseRecon              KillChainPhase = "recon"
	PhaseWeaponization      KillChainPhase = "weaponization"
	PhaseDelivery           KillChainPhase = "delivery"
	PhaseExploitation       KillChainPhase = "exploitation"
	PhaseInstallation       KillChainPhase = "installation"
	PhaseCommandAndControl  KillChainPhase = "c2"
	PhaseActionsOnObjective KillChainPhase = "actions_on_objective"
	PhaseExfiltration       KillChainPhase = "exfiltration"
)

// Order returns the sequential order of a kill chain phase.
func (p KillChainPhase) Order() int {
	switch p {
	case PhaseRecon:
		return 1
	case PhaseWeaponization:
		return 2
	case PhaseDelivery:
		return 3
	case PhaseExploitation:
		return 4
	case PhaseInstallation:
		return 5
	case PhaseCommandAndControl:
		return 6
	case PhaseActionsOnObjective:
		return 7
	case PhaseExfiltration:
		return 8
	default:
		return 0
	}
}

// RiskLevel represents the risk associated with an action or exploit.
type RiskLevel string

const (
	RiskSafe   RiskLevel = "safe"
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
	RiskDanger RiskLevel = "danger"
)

// Agent represents an implant deployed on a target machine.
type Agent struct {
	ID          string            `json:"id"`
	SessionID   string            `json:"session_id"`
	CampaignID  string            `json:"campaign_id"`
	Hostname    string            `json:"hostname"`
	OS          string            `json:"os"`
	Arch        string            `json:"arch"`
	Username    string            `json:"username"`
	LocalIP     string            `json:"local_ip"`
	Privileges  []string          `json:"privileges"`
	Status      AgentStatus       `json:"status"`
	LastCheckin time.Time         `json:"last_checkin"`
	FirstSeen   time.Time         `json:"first_seen"`
	Uptime      int32             `json:"uptime"`
	Metadata    map[string]string `json:"metadata"`
	PublicKey   []byte            `json:"public_key"`
}

// Campaign represents a red team operation.
type Campaign struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	TargetScope  string         `json:"target_scope"`
	Goal         string         `json:"goal"`
	Profile      string         `json:"profile"`
	Status       CampaignStatus `json:"status"`
	Phase        KillChainPhase `json:"phase"`
	CreatedAt    time.Time      `json:"created_at"`
	StartedAt    time.Time      `json:"started_at"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	AgentCount   int32          `json:"agent_count"`
	Progress     float64        `json:"progress"`
	AutoApproval bool           `json:"auto_approval"`
}

// Mission represents a specific action within a campaign.
type Mission struct {
	ID           string     `json:"id"`
	CampaignID   string     `json:"campaign_id"`
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Tactic       string     `json:"tactic"`
	Technique    string     `json:"technique"`
	MITREID      string     `json:"mitre_id"`
	Status       string     `json:"status"`
	Order        int        `json:"order"`
	Dependencies []string   `json:"dependencies"`
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// Target represents a discovered host in the target environment.
type Target struct {
	IP         string   `json:"ip"`
	Hostname   string   `json:"hostname"`
	OS         string   `json:"os"`
	OpenPorts  []int    `json:"open_ports"`
	Services   []string `json:"services"`
	AssetValue int      `json:"asset_value"`
}

// Vulnerability represents a discovered vulnerability.
type Vulnerability struct {
	ID          string `json:"id"`
	CVE         string `json:"cve"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Service     string `json:"service"`
	Port        int    `json:"port"`
	TargetIP    string `json:"target_ip"`
}

// Credential represents a captured credential.
type Credential struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Hash     string `json:"hash"`
	HashType string `json:"hash_type"`
	Domain   string `json:"domain"`
	Source   string `json:"source"`
	AgentID  string `json:"agent_id"`
}

// KillChainEntry logs a specific action in the kill chain.
type KillChainEntry struct {
	ID         string         `json:"id"`
	CampaignID string         `json:"campaign_id"`
	AgentID    string         `json:"agent_id"`
	Phase      KillChainPhase `json:"phase"`
	Tactic     string         `json:"tactic"`
	Technique  string         `json:"technique"`
	MITREID    string         `json:"mitre_id"`
	Success    bool           `json:"success"`
	Detail     string         `json:"detail"`
	Timestamp  time.Time      `json:"timestamp"`
}

// Decision represents an AI-suggested or rule-generated action.
type Decision struct {
	ID               string    `json:"id"`
	CampaignID       string    `json:"campaign_id"`
	AgentID          string    `json:"agent_id"`
	Tactic           string    `json:"tactic"`
	Technique        string    `json:"technique"`
	MITREID          string    `json:"mitre_id"`
	Target           string    `json:"target"`
	Confidence       float64   `json:"confidence"`
	Reasoning        string    `json:"reasoning"`
	RequiresApproval bool      `json:"requires_approval"`
	Approved         *bool     `json:"approved,omitempty"`
	Source           string    `json:"source"`
	Timestamp        time.Time `json:"timestamp"`
}

// PrivescResult represents the outcome of a privilege escalation attempt.
type PrivescResult struct {
	Success    bool   `json:"success"`
	Vector     string `json:"vector"`
	Technique  string `json:"technique"`
	Output     string `json:"output"`
	GainedRoot bool   `json:"gained_root"`
}

// ReconReport is the result of a reconnaissance operation.
type ReconReport struct {
	Target string          `json:"target"`
	Hosts  []Target        `json:"hosts"`
	Vulns  []Vulnerability `json:"vulns"`
}

// ExfilData represents data to be exfiltrated from a target.
type ExfilData struct {
	SessionID string `json:"session_id"`
	Filename  string `json:"filename"`
	TotalSize int64  `json:"total_size"`
	ChunkSize int32  `json:"chunk_size"`
}

// BlueMetric records detection metrics from BlueForge-Suite.
type BlueMetric struct {
	Tool      string `json:"tool"`
	Detected  bool   `json:"detected"`
	AlertType string `json:"alert_type"`
	Timestamp string `json:"timestamp"`
}

// AuditEntry is a complete log of an action for auditing.
type AuditEntry struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	CampaignID string    `json:"campaign_id"`
	AgentID    string    `json:"agent_id"`
	Action     string    `json:"action"`
	Result     string    `json:"result"`
	Detail     string    `json:"detail"`
}
