// Package agent provides the Kill Chain Engine.
//
// The killchain engine automates phase transitions during a campaign.
// When the agent completes a task, it reports to the orchestrator which
// automatically advances the campaign through the 7 kill chain phases.
//
// Phase transitions:
//
//	Phase 1: RECON      → Recon data gathered → advance to Weaponization
//	Phase 2: WEAPONIZE  → Payload ready       → advance to Delivery
//	Phase 3: DELIVERY   → Agent deployed      → advance to Exploitation
//	Phase 4: EXPLOIT    → Root obtained       → advance to Installation
//	Phase 5: INSTALL    → Persistence set     → advance to C2
//	Phase 6: C2         → Channel established → advance to Actions
//	Phase 7: ACTIONS    → Objective achieved  → campaign complete
//
// Each transition is gated by the Decision Engine:
// Rules (25%) + A* Planner (35%) + AI (40%) must agree to proceed.
package agent

import (
	"context"
	"fmt"

	"github.com/ruby570bocadito/vesper/pkg/shared/logger"
	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

// KillChainEngine manages automatic kill chain phase transitions.
type KillChainEngine struct {
	log   *logger.Logger
	phase types.KillChainPhase
}

// NewKillChainEngine creates a new kill chain engine.
func NewKillChainEngine(log *logger.Logger) *KillChainEngine {
	return &KillChainEngine{
		log:   log,
		phase: types.PhaseRecon,
	}
}

// CurrentPhase returns the current kill chain phase.
func (ke *KillChainEngine) CurrentPhase() types.KillChainPhase {
	return ke.phase
}

// CanAdvance checks if all conditions for the next phase are met.
func (ke *KillChainEngine) CanAdvance(conditions KillChainConditions) (bool, string) {
	switch ke.phase {
	case types.PhaseRecon:
		if conditions.HostsDiscovered > 0 {
			return true, fmt.Sprintf("%d hosts discovered — ready to weaponize", conditions.HostsDiscovered)
		}
		return false, "no hosts discovered — continue recon"

	case types.PhaseWeaponization:
		if conditions.PayloadReady {
			return true, "payload compiled — ready for delivery"
		}
		return false, "payload not ready — continue weaponization"

	case types.PhaseDelivery:
		if conditions.AgentDeployed {
			return true, fmt.Sprintf("agent deployed on %s — ready to exploit", conditions.TargetHost)
		}
		return false, "agent not deployed — continue delivery"

	case types.PhaseExploitation:
		if conditions.RootObtained {
			return true, fmt.Sprintf("root obtained via %s — ready to install persistence", conditions.PrivescVector)
		}
		return false, "root not obtained — continue exploitation"

	case types.PhaseInstallation:
		if conditions.PersistenceSet {
			return true, fmt.Sprintf("persistence set (%v) — ready for C2", conditions.PersistMethods)
		}
		return false, "persistence not set — continue installation"

	case types.PhaseCommandAndControl:
		if conditions.C2Established {
			return true, "C2 channel established — ready for actions on objective"
		}
		return false, "C2 not established — continue C2 setup"

	case types.PhaseActionsOnObjective:
		if conditions.ObjectiveAchieved {
			return true, "objective achieved — campaign complete"
		}
		return false, "objective not achieved — continue actions"

	default:
		return false, "unknown phase"
	}
}

// Advance transitions to the next phase if conditions are met.
func (ke *KillChainEngine) Advance(ctx context.Context, conditions KillChainConditions) (types.KillChainPhase, error) {
	can, reason := ke.CanAdvance(conditions)
	if !can {
		return ke.phase, fmt.Errorf("cannot advance: %s", reason)
	}

	oldPhase := ke.phase
	ke.phase = ke.nextPhase()
	ke.log.Infof("kill chain advanced: %s → %s (reason: %s)", oldPhase, ke.phase, reason)

	return ke.phase, nil
}

func (ke *KillChainEngine) nextPhase() types.KillChainPhase {
	switch ke.phase {
	case types.PhaseRecon:
		return types.PhaseWeaponization
	case types.PhaseWeaponization:
		return types.PhaseDelivery
	case types.PhaseDelivery:
		return types.PhaseExploitation
	case types.PhaseExploitation:
		return types.PhaseInstallation
	case types.PhaseInstallation:
		return types.PhaseCommandAndControl
	case types.PhaseCommandAndControl:
		return types.PhaseActionsOnObjective
	default:
		return ke.phase // stay at current
	}
}

// Reset restarts the kill chain from phase 1.
func (ke *KillChainEngine) Reset() {
	ke.phase = types.PhaseRecon
}

// KillChainConditions represent the requirements to advance to the next phase.
type KillChainConditions struct {
	// Phase 1 -> 2
	HostsDiscovered int

	// Phase 2 -> 3
	PayloadReady bool

	// Phase 3 -> 4
	AgentDeployed bool
	TargetHost    string

	// Phase 4 -> 5
	RootObtained  bool
	PrivescVector string

	// Phase 5 -> 6
	PersistenceSet bool
	PersistMethods []string

	// Phase 6 -> 7
	C2Established bool

	// Phase 7 -> complete
	ObjectiveAchieved bool
}
