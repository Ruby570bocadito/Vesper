// Package orchestrator provides integration tests for the Vesper core.
// These tests verify the decision engine, world graph, and kill chain
// orchestrator produce correct results.
package orchestrator

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ruby570bocadito/vesper/pkg/shared/config"
	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

func TestDecisionEngineEvaluate(t *testing.T) {
	cfg := config.Default()
	orch, err := New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Populate world graph with demo data
	wg := orch.WorldGraph()
	wg.GenerateDemoData()

	// Start a campaign
	ctx := context.Background()
	campaign, err := orch.StartCampaign(ctx, "test-campaign", "10.0.0.0/24", "domain_admin", "balanced", false)
	if err != nil {
		t.Fatalf("StartCampaign failed: %v", err)
	}

	// Evaluate decisions
	decisions, err := orch.Decide(ctx, campaign.ID)
	if err != nil {
		t.Fatalf("Decide() failed: %v", err)
	}

	if len(decisions) == 0 {
		t.Error("Decide() returned empty decisions — expected at least some rules/AI output")
	}

	t.Logf("Decide() returned %d decisions", len(decisions))
	for _, d := range decisions {
		t.Logf("  [%s] %s → %s (conf=%.2f)", d.Source, d.Tactic, d.Technique, d.Confidence)
	}
}

func TestWorldGraphDemoData(t *testing.T) {
	wg := NewWorldGraph()
	wg.GenerateDemoData()

	nodes := wg.GetAllNodes()
	if len(nodes) == 0 {
		t.Fatal("GenerateDemoData produced zero nodes")
	}

	if len(nodes) < 3 {
		t.Errorf("expected at least 3 nodes in demo data, got %d", len(nodes))
	}

	dc, err := wg.GetNode("10.0.0.10")
	if err != nil {
		t.Fatal("DC node (10.0.0.10) not found")
	}
	if !dc.Compromised {
		t.Log("DC node not marked as compromised in demo data")
	}

	services := wg.GetServices("10.0.0.10")
	if len(services) < 2 {
		t.Errorf("DC should have at least 2 services, got %d", len(services))
	}
}

func TestKillChainPhaseAdvance(t *testing.T) {
	cfg := config.Default()
	orch, err := New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx := context.Background()
	campaign, err := orch.StartCampaign(ctx, "kc-test", "10.0.0.0/24", "domain_admin", "balanced", false)
	if err != nil {
		t.Fatalf("StartCampaign failed: %v", err)
	}

	if campaign.Phase != types.PhaseRecon {
		t.Errorf("new campaign should start at Recon phase, got %s", campaign.Phase)
	}

	// Advance through phases
	kc := NewKillChainOrchestrator(orch.log, orch)

	if err := kc.ReconComplete(campaign.ID, 23); err != nil {
		t.Fatalf("ReconComplete failed: %v", err)
	}

	c, _ := orch.GetCampaign(campaign.ID)
	if c.Phase != types.PhaseWeaponization {
		t.Errorf("expected Weaponization after recon, got %s", c.Phase)
	}

	if err := kc.WeaponizeComplete(campaign.ID); err != nil {
		t.Fatalf("WeaponizeComplete failed: %v", err)
	}

	c, _ = orch.GetCampaign(campaign.ID)
	if c.Phase != types.PhaseDelivery {
		t.Errorf("expected Delivery after weaponize, got %s", c.Phase)
	}
}

func TestEventBusWildcard(t *testing.T) {
	eb := NewEventBus()

	var mu sync.Mutex
	received := 0
	var wg sync.WaitGroup
	wg.Add(3) // one per published event

	eb.Subscribe(EventWildcard, func(event Event) {
		defer wg.Done()
		mu.Lock()
		received++
		mu.Unlock()
	})

	eb.Publish(Event{Type: EventCampaignStarted, CampaignID: "test"})
	eb.Publish(Event{Type: EventAgentCheckin, AgentID: "agent1"})
	eb.Publish(Event{Type: EventExploitSuccess, CampaignID: "test"})

	// handlers run in goroutines — wait (bounded) for all three deliveries
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for wildcard handlers")
	}

	mu.Lock()
	defer mu.Unlock()
	if received != 3 {
		t.Fatalf("received %d events via wildcard, want 3", received)
	}
}

func TestAutoModeToggle(t *testing.T) {
	cfg := config.Default()
	orch, _ := New(cfg)
	kc := NewKillChainOrchestrator(orch.log, orch)
	am := NewAutoMode(cfg, orch.log, orch, kc)

	if am.IsEnabled() {
		t.Error("AutoMode should be disabled by default")
	}

	enabled := am.Toggle()
	if !enabled {
		t.Error("AutoMode should be enabled after toggle")
	}

	am.Stop()
}
