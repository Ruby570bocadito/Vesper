// Package orchestrator provides the AutoMode engine.
//
// AutoMode enables fully autonomous operation: the Decision Engine evaluates
// the battlefield continuously, and any decision with confidence above the
// threshold is automatically approved and dispatched to the C2 without
// waiting for human approval.
//
// This is the "AI commander" mode — Specter + Apex + Rules + A* collaborate
// to progress through the kill chain with zero human interaction.
//
// Safety: AutoMode respects risk levels. Only SAFE and LOW risk decisions
// are auto-approved by default. MEDIUM requires explicit config. HIGH/DANGER
// are never auto-approved even in AutoMode.
package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ruby570bocadito/vesper/pkg/shared/config"
	"github.com/ruby570bocadito/vesper/pkg/shared/logger"
	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

// AutoMode manages autonomous decision execution.
type AutoMode struct {
	cfg         *config.Config
	log         *logger.Logger
	orch        *Orchestrator
	killChain   *KillChainOrchestrator
	enabled     bool
	interval    time.Duration
	mu          sync.RWMutex
	stopCh      chan struct{}
	executedLog []string
}

// NewAutoMode creates the autonomous operation engine.
func NewAutoMode(cfg *config.Config, log *logger.Logger, orch *Orchestrator, kc *KillChainOrchestrator) *AutoMode {
	return &AutoMode{
		cfg:       cfg,
		log:       log,
		orch:      orch,
		killChain: kc,
		interval:  10 * time.Second,
		stopCh:    make(chan struct{}),
	}
}

func (am *AutoMode) closeStopCh() {
	select {
	case <-am.stopCh:
	default:
		close(am.stopCh)
	}
}

// Start begins autonomous decision evaluation and execution.
func (am *AutoMode) Start(ctx context.Context) {
	am.mu.Lock()
	am.enabled = true
	am.mu.Unlock()

	am.log.Infof("AutoMode started (interval=%v, threshold=%.2f)", am.interval, am.cfg.AI.MinConfidence)

	go am.loop(ctx)
}

// Stop halts autonomous operation.
func (am *AutoMode) Stop() {
	am.mu.Lock()
	am.enabled = false
	am.mu.Unlock()
	am.closeStopCh()
	am.log.Info("AutoMode stopped")
}

// IsEnabled returns whether auto-mode is active.
func (am *AutoMode) IsEnabled() bool {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.enabled
}

// Toggle switches auto-mode on/off.
func (am *AutoMode) Toggle() bool {
	am.mu.Lock()
	am.enabled = !am.enabled
	enabled := am.enabled
	if enabled {
		am.closeStopCh()
		am.stopCh = make(chan struct{})
	}
	am.mu.Unlock()

	if enabled {
		go am.loop(context.Background())
	}
	am.log.Infof("AutoMode toggled: %v", enabled)
	return enabled
}

func (am *AutoMode) loop(ctx context.Context) {
	ticker := time.NewTicker(am.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-am.stopCh:
			return
		case <-ticker.C:
			am.mu.RLock()
			if !am.enabled {
				am.mu.RUnlock()
				return
			}
			am.mu.RUnlock()

			am.evaluateAndExecute(ctx)
		}
	}
}

func (am *AutoMode) evaluateAndExecute(ctx context.Context) {
	campaigns := am.orch.ListCampaigns()
	if len(campaigns) == 0 {
		return
	}

	for _, campaign := range campaigns {
		if campaign.Status != types.CampaignStatusRunning {
			continue
		}

		decisions, err := am.orch.Decide(ctx, campaign.ID)
		if err != nil {
			am.log.Debugf("AutoMode: decision error for %s: %v", campaign.ID, err)
			continue
		}

		for _, d := range decisions {
			if d.Approved != nil {
				continue
			}

			threshold := am.cfg.AI.MinConfidence
			if campaign.AutoApproval {
				threshold = 0.65
			}

			if d.Confidence < threshold {
				continue
			}

			if !am.isRiskAcceptable(d) {
				am.log.Infof("AutoMode: skipping %s (tactic=%s, confidence=%.2f) — risk too high",
					campaign.ID, d.Tactic, d.Confidence)
				continue
			}

			if err := am.orch.ApproveDecision(d.ID); err != nil {
				am.log.Warnf("AutoMode: approve error: %v", err)
				continue
			}

			am.log.Infof("AutoMode: auto-approved %s → %s:%s (conf=%.2f, source=%s)",
				campaign.ID, d.Tactic, d.Technique, d.Confidence, d.Source)

			am.mu.Lock()
			am.executedLog = append(am.executedLog,
				fmt.Sprintf("[%s] %s → %s:%s (conf=%.2f)",
					time.Now().Format("15:04:05"), campaign.ID, d.Tactic, d.Technique, d.Confidence))
			if len(am.executedLog) > 100 {
				am.executedLog = am.executedLog[len(am.executedLog)-100:]
			}
			am.mu.Unlock()

			am.executeDecision(ctx, campaign, d)

			break
		}
	}
}

func (am *AutoMode) isRiskAcceptable(d *types.Decision) bool {
	technique := d.Technique
	riskStr := "low"

	if d.RequiresApproval {
		riskStr = "medium"
	}

	highRiskTechniques := []string{
		"passwd injection", "shadow cracking", "kernel rootkit",
		"docker escape", "nfs no_root_squash",
	}
	for _, hrt := range highRiskTechniques {
		if strings.Contains(strings.ToLower(d.Technique), hrt) || strings.Contains(strings.ToLower(technique), hrt) {
			riskStr = "high"
			break
		}
	}

	switch riskStr {
	case "safe", "low":
		return true
	case "medium":
		return am.cfg.AI.AutoApproval
	default:
		return false
	}
}

func (am *AutoMode) executeDecision(ctx context.Context, campaign *types.Campaign, d *types.Decision) {
	am.log.Infof("AutoMode: executing decision %s/%s on campaign %s (conf=%.2f)",
		d.Tactic, d.Technique, campaign.ID, d.Confidence)

	if am.orch.dispatcher != nil {
		dispatchCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()

		result, err := am.orch.dispatcher.DispatchDecisionSync(dispatchCtx, campaign, d)
		if err != nil {
			am.log.Warnf("dispatch failed for %s: %v — falling back to phase advance", d.ID, err)
		} else if result != nil && result.Success {
			am.log.Infof("dispatched %s → success", d.ID)

			hasNewData := len(result.NewHosts) > 0 ||
				len(result.NewVulnerabilities) > 0 ||
				len(result.NewCredentials) > 0 ||
				result.ExploitSucceeded

			if hasNewData {
				am.log.Infof("AutoMode: new data from %s, retriggering evaluation", d.ID)
				freshDecisions, evalErr := am.orch.Decide(ctx, campaign.ID)
				if evalErr == nil && len(freshDecisions) > 0 {
					am.log.Infof("AutoMode: got %d fresh decisions after data update", len(freshDecisions))
				}
			}

			if am.isPhaseComplete(campaign, d) {
				am.advanceViaKillchain(campaign, d)
			}
			return
		}
	}

	am.advanceViaKillchain(campaign, d)

	am.log.Infof("AutoMode: campaign %s now at phase %s (progress %.0f%%)",
		campaign.ID, campaign.Phase, campaign.Progress*100)
}

func (am *AutoMode) isPhaseComplete(campaign *types.Campaign, d *types.Decision) bool {
	phaseForTactic := map[string]types.KillChainPhase{
		"Reconnaissance":       types.PhaseRecon,
		"Initial Access":       types.PhaseDelivery,
		"Privilege Escalation": types.PhaseExploitation,
		"Persistence":          types.PhaseInstallation,
		"Command and Control":  types.PhaseCommandAndControl,
		"Lateral Movement":     types.PhaseActionsOnObjective,
		"Collection":           types.PhaseActionsOnObjective,
		"Exfiltration":         types.PhaseExfiltration,
		"Actions on Objective": types.PhaseActionsOnObjective,
	}

	expectedPhase, ok := phaseForTactic[d.Tactic]
	if !ok {
		return true
	}
	return campaign.Phase == expectedPhase
}

func (am *AutoMode) advanceViaKillchain(campaign *types.Campaign, d *types.Decision) {
	switch d.Tactic {
	case "Reconnaissance":
		am.killChain.ReconComplete(campaign.ID, 1)
	case "Initial Access":
		am.killChain.DeliveryComplete(campaign.ID, d.Target)
	case "Privilege Escalation":
		am.killChain.ExploitComplete(campaign.ID, d.Technique)
	case "Persistence":
		am.killChain.InstallComplete(campaign.ID, []string{d.Technique})
	case "Command and Control":
		am.killChain.C2Complete(campaign.ID)
	case "Actions on Objective", "Lateral Movement", "Collection", "Exfiltration":
		am.killChain.ObjectiveComplete(campaign.ID)
	}
}

// RecentActions returns the log of auto-executed decisions.
func (am *AutoMode) RecentActions() []string {
	am.mu.RLock()
	defer am.mu.RUnlock()
	log := make([]string, len(am.executedLog))
	copy(log, am.executedLog)
	return log
}
