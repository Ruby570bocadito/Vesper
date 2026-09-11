// Package orchestrator implements the central coordination engine for Vesper.
//
// The Orchestrator is the brain of the framework. It:
//   - Manages campaigns and their kill chain progression
//   - Fuses decisions from three engines: Rules (25%), A* Planner (35%), AI (40%)
//   - Maintains a live world graph (NetworkX knowledge tree)
//   - Implements Human-in-the-Loop (HITL) oversight
//   - Tracks BlueForge detection metrics
//   - Communicates with the C2 server and agent fleet
//
// Architecture:
//
//	┌───────────────────────────────────────────────┐
//	│               ORCHESTRATOR                     │
//	│                                                  │
//	│  ┌──────────────┐  ┌──────────┐  ┌──────────┐ │
//	│  │ Campaign Mgr │  │ Decision │  │ Event    │ │
//	│  │              │  │ Engine   │  │ Bus      │ │
//	│  └──────────────┘  └────┬─────┘  └──────────┘ │
//	│                        │                        │
//	│          ┌─────────────┼─────────────┐         │
//	│          ▼             ▼             ▼         │
//	│  ┌──────────┐ ┌──────────────┐ ┌──────────┐  │
//	│  │ Rules    │ │ A* Planner   │ │ AI Engine│  │
//	│  │ 25%      │ │ 35%          │ │ 40%      │  │
//	│  └──────────┘ └──────────────┘ └────┬─────┘  │
//	│                                      │         │
//	│                        ┌─────────────▼──────┐ │
//	│                        │ Python Bridge      │ │
//	│                        │ (IPC → Specter+    │ │
//	│                        │  Apex + Ollama)    │ │
//	│                        └────────────────────┘ │
//	└───────────────────────────────────────────────┘
package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/ruby570bocadito/vesper/internal/dispatch"
	"github.com/ruby570bocadito/vesper/pkg/shared/config"
	"github.com/ruby570bocadito/vesper/pkg/shared/logger"
	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

const maxDecisionsPerCampaign = 1000

// Orchestrator is the central coordination engine.
type Orchestrator struct {
	cfg *config.Config
	log *logger.Logger

	campaigns map[string]*types.Campaign
	agents    map[string]*types.Agent
	decisions map[string][]*types.Decision
	mutex     sync.RWMutex

	eventBus    *EventBus
	decisionEng *DecisionEngine
	worldGraph  *WorldGraph
	dispatcher  *dispatch.Dispatcher

	stopCh chan struct{}
}

// New creates a new Orchestrator.
func New(cfg *config.Config) (*Orchestrator, error) {
	log, err := logger.New(logger.Config{
		Level:     cfg.Logging.Level,
		Format:    cfg.Logging.Format,
		Component: "orchestrator",
	})
	if err != nil {
		return nil, fmt.Errorf("creating logger: %w", err)
	}

	o := &Orchestrator{
		cfg:        cfg,
		log:        log,
		campaigns:  make(map[string]*types.Campaign),
		agents:     make(map[string]*types.Agent),
		decisions:  make(map[string][]*types.Decision),
		eventBus:   NewEventBus(),
		worldGraph: NewWorldGraph(),
		stopCh:     make(chan struct{}),
	}

	o.decisionEng = NewDecisionEngine(cfg, log, o.worldGraph)

	return o, nil
}

// SetDispatcher wires the unified module dispatcher into the orchestrator.
func (o *Orchestrator) SetDispatcher(d *dispatch.Dispatcher) {
	o.dispatcher = d
}

// StartCampaign creates and starts a new red team campaign.
func (o *Orchestrator) StartCampaign(ctx context.Context, name, targetScope, goal, profile string, autoApproval bool) (*types.Campaign, error) {
	campaign := &types.Campaign{
		ID:           generateID("camp"),
		Name:         name,
		TargetScope:  targetScope,
		Goal:         goal,
		Profile:      profile,
		Status:       types.CampaignStatusRunning,
		Phase:        types.PhaseRecon,
		CreatedAt:    time.Now(),
		StartedAt:    time.Now(),
		AutoApproval: autoApproval,
	}

	o.mutex.Lock()
	o.campaigns[campaign.ID] = campaign
	o.decisions[campaign.ID] = make([]*types.Decision, 0)
	o.mutex.Unlock()

	o.log.Infof("campaign started: %s (id=%s scope=%s goal=%s)", name, campaign.ID, targetScope, goal)

	// Publish event
	o.eventBus.Publish(Event{
		Type:       EventCampaignStarted,
		CampaignID: campaign.ID,
		Timestamp:  time.Now(),
		Data:       campaign,
	})

	return campaign, nil
}

// GetCampaign returns a campaign by ID.
func (o *Orchestrator) GetCampaign(id string) (*types.Campaign, error) {
	o.mutex.RLock()
	defer o.mutex.RUnlock()

	c, ok := o.campaigns[id]
	if !ok {
		return nil, fmt.Errorf("campaign not found: %s", id)
	}
	return c, nil
}

// ListCampaigns returns all campaigns.
func (o *Orchestrator) ListCampaigns() []*types.Campaign {
	o.mutex.RLock()
	defer o.mutex.RUnlock()

	campaigns := make([]*types.Campaign, 0, len(o.campaigns))
	for _, c := range o.campaigns {
		campaigns = append(campaigns, c)
	}
	return campaigns
}

// GetDecisions returns the decisions recorded for a campaign
// (chronological). Used by the console to resolve suggestion row
// numbers into real decision IDs.
func (o *Orchestrator) GetDecisions(campaignID string) []*types.Decision {
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	out := make([]*types.Decision, len(o.decisions[campaignID]))
	copy(out, o.decisions[campaignID])
	return out
}

// PauseCampaign pauses a campaign without exposing the shared pointer
// for mutation (thread-safe).
func (o *Orchestrator) PauseCampaign(id string) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	c, ok := o.campaigns[id]
	if !ok {
		return fmt.Errorf("campaign not found: %s", id)
	}
	c.Status = types.CampaignStatusPaused
	o.log.Infof("campaign paused: %s", id)
	return nil
}

// ResumeCampaign resumes a paused campaign (thread-safe).
func (o *Orchestrator) ResumeCampaign(id string) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	c, ok := o.campaigns[id]
	if !ok {
		return fmt.Errorf("campaign not found: %s", id)
	}
	c.Status = types.CampaignStatusRunning
	o.log.Infof("campaign resumed: %s", id)
	return nil
}

// RegisterAgent registers a new agent with a campaign.
func (o *Orchestrator) RegisterAgent(agent *types.Agent) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	o.agents[agent.ID] = agent

	if agent.CampaignID != "" {
		campaign, ok := o.campaigns[agent.CampaignID]
		if ok {
			campaign.AgentCount++
		}
	}

	o.log.Infof("agent registered: %s@%s (campaign=%s)", agent.ID, agent.Hostname, agent.CampaignID)

	o.eventBus.Publish(Event{
		Type:       EventAgentRegistered,
		AgentID:    agent.ID,
		CampaignID: agent.CampaignID,
		Timestamp:  time.Now(),
		Data:       agent,
	})

	// Update world graph
	o.worldGraph.AddHost(agent.LocalIP, agent.Hostname, agent.OS)

	return nil
}

// Decide evaluates the current battlefield state and produces a list of
// recommended actions, ordered by confidence.
func (o *Orchestrator) Decide(ctx context.Context, campaignID string) ([]*types.Decision, error) {
	o.mutex.RLock()
	campaign, ok := o.campaigns[campaignID]
	o.mutex.RUnlock()

	if !ok {
		return nil, fmt.Errorf("campaign not found: %s", campaignID)
	}

	decisions, err := o.decisionEng.Evaluate(ctx, campaign)
	if err != nil {
		return nil, fmt.Errorf("decision engine: %w", err)
	}

	// Store decisions (capped to prevent unbounded growth)
	o.mutex.Lock()
	all := append(o.decisions[campaignID], decisions...)
	if len(all) > maxDecisionsPerCampaign {
		all = all[len(all)-maxDecisionsPerCampaign:]
	}
	o.decisions[campaignID] = all
	o.mutex.Unlock()

	// If auto-approval is enabled, approve all decisions above confidence threshold
	if campaign.AutoApproval {
		for _, d := range decisions {
			if d.Confidence >= o.cfg.AI.MinConfidence {
				approved := true
				d.Approved = &approved
				o.log.Infof("auto-approved decision %s (tactic=%s confidence=%.2f)", d.ID, d.Tactic, d.Confidence)
			}
		}
	}

	return decisions, nil
}

// ApproveDecision approves a pending decision (HITL).
func (o *Orchestrator) ApproveDecision(decisionID string) error {
	approved := true
	return o.setDecisionApproval(decisionID, &approved)
}

// RejectDecision rejects a pending decision (HITL).
func (o *Orchestrator) RejectDecision(decisionID string) error {
	rejected := false
	return o.setDecisionApproval(decisionID, &rejected)
}

// AdvancePhase moves a campaign to the next kill chain phase.
func (o *Orchestrator) AdvancePhase(campaignID string, phase types.KillChainPhase) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	c, ok := o.campaigns[campaignID]
	if !ok {
		return fmt.Errorf("campaign not found: %s", campaignID)
	}

	c.Phase = phase
	c.Progress = float64(phase.Order()) / 8.0

	o.log.Infof("campaign %s advanced to phase: %s (progress: %.0f%%)", campaignID, phase, c.Progress*100)

	o.eventBus.Publish(Event{
		Type:       EventPhaseChanged,
		CampaignID: campaignID,
		Timestamp:  time.Now(),
		Data:       phase,
	})

	// Log kill chain entry
	entry := types.KillChainEntry{
		ID:         generateID("kc"),
		CampaignID: campaignID,
		Phase:      phase,
		Tactic:     getTacticForPhase(phase),
		Timestamp:  time.Now(),
	}

	o.eventBus.Publish(Event{
		Type:       EventKillChainUpdate,
		CampaignID: campaignID,
		Timestamp:  time.Now(),
		Data:       entry,
	})

	return nil
}

// GetMetrics returns current metrics for a campaign.
func (o *Orchestrator) GetMetrics(campaignID string) map[string]interface{} {
	o.mutex.RLock()
	defer o.mutex.RUnlock()

	c, ok := o.campaigns[campaignID]
	if !ok {
		return nil
	}

	return map[string]interface{}{
		"campaign_name":     c.Name,
		"phase":             c.Phase,
		"progress":          c.Progress,
		"agent_count":       c.AgentCount,
		"pending_decisions": len(o.decisions[campaignID]),
		"world_graph_nodes": o.worldGraph.NodeCount(),
		"world_graph_edges": o.worldGraph.EdgeCount(),
	}
}

// GetEventBus returns the orchestrator's event bus for external subscribers.
func (o *Orchestrator) GetEventBus() *EventBus {
	return o.eventBus
}

// WorldGraph returns the orchestrator's world graph.
func (o *Orchestrator) WorldGraph() *WorldGraph {
	return o.worldGraph
}

// Stop gracefully stops the orchestrator.
func (o *Orchestrator) Stop() {
	o.log.Info("orchestrator stopping")
	close(o.stopCh)
}

func (o *Orchestrator) setDecisionApproval(decisionID string, approved *bool) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	for campaignID, decisions := range o.decisions {
		for _, d := range decisions {
			if d.ID == decisionID {
				d.Approved = approved
				if *approved {
					o.log.Infof("decision %s approved (campaign=%s)", decisionID, campaignID)
				} else {
					o.log.Infof("decision %s rejected (campaign=%s)", decisionID, campaignID)
				}
				return nil
			}
		}
	}

	return fmt.Errorf("decision not found: %s", decisionID)
}

func getTacticForPhase(phase types.KillChainPhase) string {
	switch phase {
	case types.PhaseRecon:
		return "Reconnaissance"
	case types.PhaseWeaponization:
		return "Resource Development"
	case types.PhaseDelivery:
		return "Initial Access"
	case types.PhaseExploitation:
		return "Execution"
	case types.PhaseInstallation:
		return "Persistence"
	case types.PhaseCommandAndControl:
		return "Command and Control"
	case types.PhaseActionsOnObjective:
		return "Collection"
	default:
		return "Unknown"
	}
}

func generateID(prefix string) string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%s-%d-%s", prefix, time.Now().UnixNano(), hex.EncodeToString(b))
}
