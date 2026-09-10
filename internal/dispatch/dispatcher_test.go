package dispatch

import (
	"context"
	"testing"

	"github.com/ruby570bocadito/vesper/internal/registry"
	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

type mockState struct {
	agents []*types.Agent
	hosts  []*types.Target
	vulns  []*types.Vulnerability
	creds  []*types.Credential
	edges  []string
	bridge BridgeCaller
}

func (m *mockState) GetAgents() []*types.Agent { return m.agents }
func (m *mockState) GetAgent(id string) *types.Agent {
	for _, a := range m.agents {
		if a.ID == id {
			return a
		}
	}
	return nil
}
func (m *mockState) AddHost(h *types.Target)                 { m.hosts = append(m.hosts, h) }
func (m *mockState) AddVulnerability(v *types.Vulnerability) { m.vulns = append(m.vulns, v) }
func (m *mockState) AddCredential(c *types.Credential)       { m.creds = append(m.creds, c) }
func (m *mockState) AddLateralEdge(from, to, exploit string) { m.edges = append(m.edges, from+"->"+to) }
func (m *mockState) GetBridgeClient() BridgeCaller           { return m.bridge }

type mockBridge struct{ connected bool }

func (m *mockBridge) CallRaw(ctx context.Context, module, function string, params map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"success": true, "result": "mock"}, nil
}
func (m *mockBridge) IsConnected() bool { return m.connected }

func TestNewDispatcher(t *testing.T) {
	mState := &mockState{agents: []*types.Agent{{ID: "agent-1"}}}
	d := New(mState, true, 0.5)
	if d == nil {
		t.Fatal("New() returned nil")
	}
}

func TestDispatchDecisionNoAgents(t *testing.T) {
	mState := &mockState{}
	mBridge := &mockBridge{connected: false}
	mState.bridge = mBridge

	d := New(mState, true, 0.5)

	decision := &types.Decision{
		ID:         "dec-1",
		Tactic:     "recon",
		Technique:  "port_scan",
		Confidence: 0.9,
	}
	campaign := &types.Campaign{
		ID:          "camp-1",
		Phase:       types.PhaseRecon,
		TargetScope: "10.0.0.0/24",
	}

	err := d.DispatchDecision(context.Background(), campaign, decision)
	if err != nil {
		t.Fatalf("DispatchDecision with no agents should not error: %v", err)
	}
}

func TestNewWithOptions(t *testing.T) {
	tests := []struct {
		name        string
		autoApprove bool
		minConf     float64
	}{
		{"auto approve high conf", true, 0.9},
		{"manual low conf", false, 0.1},
		{"default", true, 0.65},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New(&mockState{}, tt.autoApprove, tt.minConf)
			if d.autoApprove != tt.autoApprove {
				t.Errorf("autoApprove = %v, want %v", d.autoApprove, tt.autoApprove)
			}
			if d.minConfidence != tt.minConf {
				t.Errorf("minConfidence = %v, want %v", d.minConfidence, tt.minConf)
			}
		})
	}
}

func TestDispatchDecisionSync(t *testing.T) {
	mState := &mockState{
		agents: []*types.Agent{{ID: "agent-1", Hostname: "box-01"}},
	}
	mBridge := &mockBridge{connected: true}
	mState.bridge = mBridge

	// Register a mock module for testing
	registry.Register(registry.ModuleFactory{
		Name:        "test/recon_scan",
		Phase:       types.PhaseRecon,
		Description: "Mock recon module",
		Risk:        "low",
		Factory: func() registry.Module {
			return &mockModule{}
		},
	})

	d := New(mState, true, 0.5)

	decision := &types.Decision{
		ID:         "dec-sync-1",
		Tactic:     "recon",
		Technique:  "port_scan",
		Confidence: 0.85,
	}
	campaign := &types.Campaign{
		ID:          "camp-sync-1",
		Phase:       types.PhaseRecon,
		TargetScope: "10.0.0.5",
	}

	result, err := d.DispatchDecisionSync(context.Background(), campaign, decision)
	if err != nil {
		t.Fatalf("DispatchDecisionSync failed: %v", err)
	}
	if result == nil {
		t.Fatal("DispatchDecisionSync returned nil result")
	}
}

type mockModule struct{}

func (m *mockModule) Name() string                { return "test/recon_scan" }
func (m *mockModule) Phase() types.KillChainPhase { return types.PhaseRecon }
func (m *mockModule) Description() string         { return "mock" }
func (m *mockModule) Require() []string           { return nil }
func (m *mockModule) Provide() []string           { return []string{"hosts"} }
func (m *mockModule) Risk() string                { return "low" }
func (m *mockModule) Execute(ctx context.Context, target registry.Target) (registry.ModuleResult, error) {
	return registry.ModuleResult{
		Success: true,
		Output:  "mock scan complete",
		NewHosts: []registry.Target{
			{Hostname: "found-host", IP: "10.0.0.10", OS: "linux"},
		},
	}, nil
}

func TestMockModuleInterface(t *testing.T) {
	m := &mockModule{}
	if m.Name() != "test/recon_scan" {
		t.Errorf("Name() = %s, want test/recon_scan", m.Name())
	}
	if m.Phase() != types.PhaseRecon {
		t.Errorf("Phase() = %v, want PhaseRecon", m.Phase())
	}
}

func TestBridgeCallerInterface(t *testing.T) {
	m := &mockBridge{connected: true}
	if !m.IsConnected() {
		t.Error("mockBridge should be connected")
	}
	m.connected = false
	if m.IsConnected() {
		t.Error("mockBridge should not be connected")
	}
	result, err := m.CallRaw(context.Background(), "test", "ping", nil)
	if err != nil {
		t.Fatalf("CallRaw failed: %v", err)
	}
	if result["success"] != true {
		t.Error("expected success=true")
	}
}
