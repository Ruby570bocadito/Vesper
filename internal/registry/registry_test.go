package registry

import (
	"context"
	"testing"

	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

type testModule struct {
	name    string
	phase   types.KillChainPhase
	desc    string
	risk    string
	require []string
	provide []string
}

func (m *testModule) Name() string                { return m.name }
func (m *testModule) Phase() types.KillChainPhase { return m.phase }
func (m *testModule) Description() string         { return m.desc }
func (m *testModule) Require() []string           { return m.require }
func (m *testModule) Provide() []string           { return m.provide }
func (m *testModule) Risk() string                { return m.risk }
func (m *testModule) Execute(ctx context.Context, target Target) (ModuleResult, error) {
	return ModuleResult{Success: true, Output: "ok"}, nil
}

func TestRegisterAndGet(t *testing.T) {
	GlobalRegistry = map[string]ModuleFactory{}
	defer func() { GlobalRegistry = map[string]ModuleFactory{} }()

	Register(ModuleFactory{
		Name:    "test/scanner",
		Phase:   types.PhaseRecon,
		Factory: func() Module { return &testModule{name: "test/scanner"} },
	})

	m, ok := GetModule("test/scanner")
	if !ok {
		t.Fatal("GetModule returned false for registered module")
	}
	if m.Name != "test/scanner" {
		t.Errorf("expected name test/scanner, got %s", m.Name)
	}
}

func TestGetModuleNotFound(t *testing.T) {
	GlobalRegistry = map[string]ModuleFactory{}
	defer func() { GlobalRegistry = map[string]ModuleFactory{} }()

	_, ok := GetModule("nonexistent")
	if ok {
		t.Error("GetModule should return false for unregistered module")
	}
}

func TestGetModulesForPhase(t *testing.T) {
	GlobalRegistry = map[string]ModuleFactory{}
	defer func() { GlobalRegistry = map[string]ModuleFactory{} }()

	Register(ModuleFactory{Name: "recon/scan1", Phase: types.PhaseRecon, Factory: func() Module { return &testModule{} }})
	Register(ModuleFactory{Name: "recon/scan2", Phase: types.PhaseRecon, Factory: func() Module { return &testModule{} }})
	Register(ModuleFactory{Name: "exploit/exec", Phase: types.PhaseExploitation, Factory: func() Module { return &testModule{} }})

	reconMods := GetModulesForPhase(types.PhaseRecon)
	if len(reconMods) != 2 {
		t.Errorf("expected 2 recon modules, got %d", len(reconMods))
	}
}

func TestGetModulesByRequirements(t *testing.T) {
	GlobalRegistry = map[string]ModuleFactory{}
	defer func() { GlobalRegistry = map[string]ModuleFactory{} }()

	Register(ModuleFactory{
		Name:    "exploit/eternal",
		Phase:   types.PhaseExploitation,
		Require: []string{"target_reachable", "smb_open"},
		Provide: []string{"shell"},
		Factory: func() Module { return &testModule{} },
	})
	Register(ModuleFactory{
		Name:    "exploit/web",
		Phase:   types.PhaseExploitation,
		Require: []string{"http_open"},
		Provide: []string{"shell"},
		Factory: func() Module { return &testModule{} },
	})

	// Should match eternal
	mods := GetModulesByRequirements([]string{"target_reachable", "smb_open"})
	if len(mods) != 1 {
		t.Errorf("expected 1 module with SMB requirements, got %d", len(mods))
	}

	// Should match none
	mods = GetModulesByRequirements([]string{"nothing"})
	if len(mods) != 0 {
		t.Errorf("expected 0 modules with no matching requirements, got %d", len(mods))
	}
}

func TestModuleFactoryExecute(t *testing.T) {
	GlobalRegistry = map[string]ModuleFactory{}
	defer func() { GlobalRegistry = map[string]ModuleFactory{} }()

	Register(ModuleFactory{
		Name:  "test/exec",
		Phase: types.PhaseRecon,
		Factory: func() Module {
			return &testModule{name: "test/exec"}
		},
	})

	mf, _ := GetModule("test/exec")
	mod := mf.Factory()
	result, err := mod.Execute(context.Background(), Target{IP: "10.0.0.1"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.Success {
		t.Error("expected success=true")
	}
}
