package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ruby570bocadito/vesper/pkg/shared/logger"
	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

// KillChainOrchestrator manages automatic phase transitions for campaigns.
// It listens to events from the agent and advances phases when conditions are met.
type KillChainOrchestrator struct {
	log         *logger.Logger
	orch        *Orchestrator
	mu          sync.Mutex
	transitions map[string]bool
	maxRetries  int
	execTimeout time.Duration
}

// NewKillChainOrchestrator creates a kill chain orchestrator.
func NewKillChainOrchestrator(log *logger.Logger, orch *Orchestrator) *KillChainOrchestrator {
	return &KillChainOrchestrator{
		log:         log,
		orch:        orch,
		transitions: make(map[string]bool),
		maxRetries:  3,
		execTimeout: 120 * time.Second,
	}
}

// ReconComplete should be called when recon data has been gathered.
// Automatically advances the campaign from Recon → Weaponization.
func (ko *KillChainOrchestrator) ReconComplete(campaignID string, hostsDiscovered int) error {
	return ko.tryAdvance(campaignID, types.PhaseRecon, types.PhaseWeaponization,
		fmt.Sprintf("recon complete: %d hosts discovered", hostsDiscovered))
}

// WeaponizeComplete should be called when a payload is built.
// Automatically advances Weaponization → Delivery.
func (ko *KillChainOrchestrator) WeaponizeComplete(campaignID string) error {
	return ko.tryAdvance(campaignID, types.PhaseWeaponization, types.PhaseDelivery,
		"payload compiled and ready")
}

// DeliveryComplete should be called when the agent is deployed on target.
// Automatically advances Delivery → Exploitation.
func (ko *KillChainOrchestrator) DeliveryComplete(campaignID, targetHost string) error {
	return ko.tryAdvance(campaignID, types.PhaseDelivery, types.PhaseExploitation,
		fmt.Sprintf("agent deployed on %s", targetHost))
}

// ExploitComplete should be called when root is obtained.
// Automatically advances Exploitation → Installation.
func (ko *KillChainOrchestrator) ExploitComplete(campaignID, privescVector string) error {
	return ko.tryAdvance(campaignID, types.PhaseExploitation, types.PhaseInstallation,
		fmt.Sprintf("privilege escalation: %s", privescVector))
}

// InstallComplete should be called when persistence is set.
// Automatically advances Installation → C2.
func (ko *KillChainOrchestrator) InstallComplete(campaignID string, methods []string) error {
	return ko.tryAdvance(campaignID, types.PhaseInstallation, types.PhaseCommandAndControl,
		fmt.Sprintf("persistence installed: %v", methods))
}

// C2Complete should be called when C2 channel is established.
// Automatically advances C2 → Actions on Objective.
func (ko *KillChainOrchestrator) C2Complete(campaignID string) error {
	return ko.tryAdvance(campaignID, types.PhaseCommandAndControl, types.PhaseActionsOnObjective,
		"encrypted C2 channel established")
}

// ObjectiveComplete marks the campaign as complete.
func (ko *KillChainOrchestrator) ObjectiveComplete(campaignID string) error {
	campaign, err := ko.orch.GetCampaign(campaignID)
	if err != nil {
		return err
	}

	campaign.Status = types.CampaignStatusCompleted
	campaign.Progress = 1.0
	campaign.CompletedAt = now()

	ko.log.Infof("campaign %s completed! goal: %s", campaignID, campaign.Goal)

	ko.orch.eventBus.Publish(Event{
		Type:       EventCampaignCompleted,
		CampaignID: campaignID,
		Data:       campaign,
	})

	return nil
}

func (ko *KillChainOrchestrator) tryAdvance(campaignID string, from, to types.KillChainPhase, reason string) error {
	ko.mu.Lock()
	if ko.transitions[campaignID] {
		ko.mu.Unlock()
		return fmt.Errorf("transition already in progress for campaign %s", campaignID)
	}
	ko.transitions[campaignID] = true
	ko.mu.Unlock()

	defer func() {
		ko.mu.Lock()
		delete(ko.transitions, campaignID)
		ko.mu.Unlock()
	}()

	campaign, err := ko.orch.GetCampaign(campaignID)
	if err != nil {
		return err
	}

	if campaign.Phase != from {
		return fmt.Errorf("expected phase %s, got %s", from, campaign.Phase)
	}

	if ko.orch.dispatcher != nil {
		succeeded := ko.executeNextPhaseModule(campaignID, campaign, to)
		if !succeeded {
			ko.log.Warnf("kill chain: module execution for next phase %s failed after retries, staying at %s", to, from)
			return fmt.Errorf("module execution failed for phase %s", to)
		}
	}

	if err := ko.orch.AdvancePhase(campaignID, to); err != nil {
		return fmt.Errorf("advancing phase: %w", err)
	}

	ko.log.Infof("kill chain auto-advance: %s → %s (campaign=%s, %s)", from, to, campaignID, reason)
	return nil
}

func (ko *KillChainOrchestrator) executeNextPhaseModule(campaignID string, campaign *types.Campaign, nextPhase types.KillChainPhase) bool {
	tactic := ko.phaseToTactic(nextPhase)
	if tactic == "" {
		return true
	}

	for attempt := 0; attempt < ko.maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), ko.execTimeout)

		decision := ko.getTopDecisionForTactic(ctx, campaignID, campaign, tactic)
		if decision == nil {
			cancel()
			ko.log.Debugf("kill chain: no decision available for tactic %s (attempt %d)", tactic, attempt+1)
			time.Sleep(2 * time.Second)
			continue
		}

		result, err := ko.orch.dispatcher.DispatchDecisionSync(ctx, campaign, decision)
		cancel()

		if err != nil {
			ko.log.Warnf("kill chain: dispatch attempt %d failed for %s: %v", attempt+1, tactic, err)
			time.Sleep(2 * time.Second)
			continue
		}

		if result != nil && result.Success {
			ko.log.Infof("kill chain: module succeeded for phase %s (attempt %d)", nextPhase, attempt+1)
			return true
		}

		ko.log.Warnf("kill chain: module returned failure for phase %s (attempt %d)", nextPhase, attempt+1)
		time.Sleep(2 * time.Second)
	}

	return false
}

func (ko *KillChainOrchestrator) getTopDecisionForTactic(ctx context.Context, campaignID string, campaign *types.Campaign, tactic string) *types.Decision {
	decisions, err := ko.orch.Decide(ctx, campaignID)
	if err != nil || len(decisions) == 0 {
		return &types.Decision{
			ID:         fmt.Sprintf("kc-%s-%d", tactic, time.Now().UnixNano()),
			CampaignID: campaignID,
			Tactic:     tactic,
			Technique:  "auto",
			Target:     campaign.TargetScope,
			Confidence: 0.9,
			Source:     "killchain",
			Timestamp:  time.Now(),
		}
	}

	for _, d := range decisions {
		if d.Tactic == tactic {
			return d
		}
	}

	if len(decisions) > 0 {
		top := decisions[0]
		top.Tactic = tactic
		return top
	}

	return &types.Decision{
		ID:         fmt.Sprintf("kc-%s-%d", tactic, time.Now().UnixNano()),
		CampaignID: campaignID,
		Tactic:     tactic,
		Technique:  "auto",
		Target:     campaign.TargetScope,
		Confidence: 0.9,
		Source:     "killchain",
		Timestamp:  time.Now(),
	}
}

func (ko *KillChainOrchestrator) phaseToTactic(phase types.KillChainPhase) string {
	mapping := map[types.KillChainPhase]string{
		types.PhaseRecon:              "Reconnaissance",
		types.PhaseWeaponization:      "Reconnaissance",
		types.PhaseDelivery:           "Initial Access",
		types.PhaseExploitation:       "Privilege Escalation",
		types.PhaseInstallation:       "Persistence",
		types.PhaseCommandAndControl:  "Command and Control",
		types.PhaseActionsOnObjective: "Actions on Objective",
		types.PhaseExfiltration:       "Exfiltration",
	}
	return mapping[phase]
}

func now() *time.Time {
	t := time.Now()
	return &t
}
