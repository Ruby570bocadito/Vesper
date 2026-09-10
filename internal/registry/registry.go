package registry

import (
	"context"
	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

type ModuleFactory struct {
	Name        string
	Phase       types.KillChainPhase
	Description string
	Risk        string
	Require     []string
	Provide     []string
	Factory     func() Module
}

type Target struct {
	Hostname string
	IP       string
	OS       string
	Ports    []int
	Services []string
}

type ModuleResult struct {
	Success  bool
	Output   string
	NewHosts []Target
	NewCreds []string
	Error    error
}

type Module interface {
	Name() string
	Phase() types.KillChainPhase
	Description() string
	Require() []string
	Provide() []string
	Risk() string
	Execute(ctx context.Context, target Target) (ModuleResult, error)
}

var GlobalRegistry = map[string]ModuleFactory{}

func Register(f ModuleFactory) { GlobalRegistry[f.Name] = f }

func GetModule(name string) (ModuleFactory, bool) { m, ok := GlobalRegistry[name]; return m, ok }

func GetModulesForPhase(phase types.KillChainPhase) []ModuleFactory {
	var mods []ModuleFactory
	for _, m := range GlobalRegistry {
		if m.Phase == phase {
			mods = append(mods, m)
		}
	}
	return mods
}

func GetModulesByRequirements(available []string) []ModuleFactory {
	var mods []ModuleFactory
	set := make(map[string]bool)
	for _, a := range available {
		set[a] = true
	}
	for _, m := range GlobalRegistry {
		satisfied := true
		for _, req := range m.Require {
			if !set[req] {
				satisfied = false
				break
			}
		}
		if satisfied {
			mods = append(mods, m)
		}
	}
	return mods
}
