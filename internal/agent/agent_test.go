// Package agent provides integration tests for the agent modules,
// bridge client, and post-exploitation pipeline.
package agent

import (
	"testing"

	"github.com/ruby570bocadito/vesper/pkg/shared/config"
	"github.com/ruby570bocadito/vesper/pkg/shared/logger"
)

func TestPostExploitPipelineCreation(t *testing.T) {
	cfg := config.Default()
	log := logger.NewDefault("test")
	bridge := NewBridgeClient(cfg, log)

	pipeline := NewPostExploitPipeline(cfg, log, bridge)
	if pipeline == nil {
		t.Fatal("NewPostExploitPipeline returned nil")
	}
	if pipeline.hostname == "" {
		t.Error("pipeline hostname should not be empty")
	}
}

func TestVaultIOCTLDeviceCheck(t *testing.T) {
	vault := NewVaultIOCTL()
	if vault.IsAvailable() {
		t.Log("Vault-Kernel device found — IOCTL available")
	} else {
		t.Log("Vault-Kernel device not found — expected in non-kernel environment")
	}
	vault.Close()
}

func TestRiseWrapperAvailability(t *testing.T) {
	log := logger.NewDefault("test")
	rw := NewRiseWrapper(log)
	if rw.IsAvailable() {
		t.Logf("Rise-Privilege binary found at: %s", rw.BinPath())
	} else {
		t.Log("Rise-Privilege binary not found — expected in dev environment")
	}
}

func TestBridgeClientCreation(t *testing.T) {
	cfg := config.Default()
	log := logger.NewDefault("test")
	bc := NewBridgeClient(cfg, log)

	if bc == nil {
		t.Fatal("NewBridgeClient returned nil")
	}

	if bc.Connected() {
		t.Error("bridge should not be connected before Connect() call")
	}
}

func TestKillChainEnginePhases(t *testing.T) {
	log := logger.NewDefault("test")
	ke := NewKillChainEngine(log)

	if ke.CurrentPhase().Order() == 0 {
		t.Error("initial phase should be Recon (order > 0)")
	}

	conditions := KillChainConditions{HostsDiscovered: 23}
	can, reason := ke.CanAdvance(conditions)
	if !can {
		t.Errorf("should be able to advance from Recon with 23 hosts, got: %s", reason)
	}
}

func TestExfilManager(t *testing.T) {
	log := logger.NewDefault("test")
	manager := NewExfilManager(log, nil)

	if manager == nil {
		t.Fatal("NewExfilManager returned nil")
	}

	targets := CommonTargets()
	if len(targets) == 0 {
		t.Error("CommonTargets should return at least some paths")
	}
}
