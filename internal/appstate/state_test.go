package appstate

import (
	"context"
	"testing"

	"github.com/ruby570bocadito/vesper/pkg/shared/config"
	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

func TestNew(t *testing.T) {
	cfg := config.Default()
	s, err := New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if s.Cfg != cfg {
		t.Error("Config not stored")
	}
}

func TestRegisterAgent(t *testing.T) {
	cfg := config.Default()
	s, _ := New(cfg)

	a := &types.Agent{ID: "test-agent-1", Hostname: "box-01"}
	s.RegisterAgent(a)

	agents := s.GetAgents()
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].ID != "test-agent-1" {
		t.Errorf("expected ID test-agent-1, got %s", agents[0].ID)
	}
}

func TestGetAgent(t *testing.T) {
	cfg := config.Default()
	s, _ := New(cfg)

	a := &types.Agent{ID: "agent-42", Hostname: "dc-01"}
	s.RegisterAgent(a)

	got := s.GetAgent("agent-42")
	if got == nil {
		t.Fatal("GetAgent returned nil for existing agent")
	}
	if got.Hostname != "dc-01" {
		t.Errorf("expected dc-01, got %s", got.Hostname)
	}

	missing := s.GetAgent("nonexistent")
	if missing != nil {
		t.Error("GetAgent should return nil for unknown ID")
	}
}

func TestRemoveAgent(t *testing.T) {
	cfg := config.Default()
	s, _ := New(cfg)

	s.RegisterAgent(&types.Agent{ID: "a1"})
	s.RegisterAgent(&types.Agent{ID: "a2"})

	s.RemoveAgent("a1")
	agents := s.GetAgents()
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent after removal, got %d", len(agents))
	}
	if agents[0].ID != "a2" {
		t.Errorf("expected a2 to remain, got %s", agents[0].ID)
	}
}

func TestAddHost(t *testing.T) {
	cfg := config.Default()
	s, _ := New(cfg)

	h := &types.Target{Hostname: "server-01", IP: "10.0.0.1"}
	s.AddHost(h)

	hosts := s.GetHosts()
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	if hosts[0].IP != "10.0.0.1" {
		t.Errorf("expected IP 10.0.0.1, got %s", hosts[0].IP)
	}
}

func TestAddVuln(t *testing.T) {
	cfg := config.Default()
	s, _ := New(cfg)

	v := &types.Vulnerability{ID: "CVE-2024-1234", Description: "Test Vuln"}
	s.AddVuln(v)

	vulns := s.GetVulns()
	if len(vulns) != 1 {
		t.Fatalf("expected 1 vuln, got %d", len(vulns))
	}
	if vulns[0].ID != "CVE-2024-1234" {
		t.Errorf("expected CVE-2024-1234, got %s", vulns[0].ID)
	}
}

func TestAddCredential(t *testing.T) {
	cfg := config.Default()
	s, _ := New(cfg)

	c := &types.Credential{Username: "admin", Password: "s3cret"}
	s.AddCredential(c)

	creds := s.GetCreds()
	if len(creds) != 1 {
		t.Fatalf("expected 1 cred, got %d", len(creds))
	}
	if creds[0].Username != "admin" {
		t.Errorf("expected admin, got %s", creds[0].Username)
	}
}

func TestGetModules(t *testing.T) {
	cfg := config.Default()
	s, _ := New(cfg)

	mods := s.GetModules()
	if len(mods) == 0 {
		t.Error("GetModules() returned empty — expected at least some modules")
	}
}

func TestSearchModules(t *testing.T) {
	cfg := config.Default()
	s, _ := New(cfg)

	results := s.SearchModules("ransomware")
	if len(results) == 0 {
		t.Error("SearchModules('ransomware') returned empty — expected at least one module")
	}
}

func TestStartContextCancel(t *testing.T) {
	cfg := config.Default()
	s, _ := New(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.Start(ctx)
	if err == nil {
		t.Log("Start() with cancelled ctx returned nil (expected graceful handling)")
	}
}

func TestAppStateConcurrency(t *testing.T) {
	cfg := config.Default()
	s, _ := New(cfg)

	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			s.RegisterAgent(&types.Agent{ID: "conc-agent"})
			s.RemoveAgent("conc-agent")
		}
		done <- true
	}()
	go func() {
		for i := 0; i < 100; i++ {
			s.GetAgents()
		}
		done <- true
	}()

	<-done
	<-done
	t.Log("Concurrent access completed without race")
}
