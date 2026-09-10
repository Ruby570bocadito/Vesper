// Package c2server provides a lightweight C2 listener that agents connect to.
// It uses gRPC and implements the AgentService defined in agent.proto.
// In production, this is replaced by Pulse-C2. For the unified framework,
// it serves as the integrated C2 that the agent, orchestrator, and dashboard share.
package c2server

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/ruby570bocadito/vesper/internal/appstate"
	agentv1 "github.com/ruby570bocadito/vesper/pkg/proto/gen/agent"
	c2v1 "github.com/ruby570bocadito/vesper/pkg/proto/gen/c2"
	"github.com/ruby570bocadito/vesper/pkg/shared/config"
	"github.com/ruby570bocadito/vesper/pkg/shared/logger"
	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

// Server is the integrated C2 listener.
// It runs a gRPC server that implements AgentService (for agent comms)
// and C2Service (for management/monitoring).
type Server struct {
	cfg          *config.Config
	log          *logger.Logger
	state        *appstate.AppState
	grpcSrv      *grpc.Server
	agents       map[string]*AgentConnection
	agentStreams map[string]agentv1.AgentService_CommandStreamServer
	mu           sync.RWMutex
	running      bool
}

// AgentConnection represents a connected agent.
type AgentConnection struct {
	Agent       *types.Agent
	ConnectedAt time.Time
	LastSeen    time.Time
	SessionKey  []byte
	AgentID     string
}

// New creates a new C2 server.
func New(cfg *config.Config, log *logger.Logger) *Server {
	return &Server{
		cfg:          cfg,
		log:          log,
		agents:       make(map[string]*AgentConnection),
		agentStreams: make(map[string]agentv1.AgentService_CommandStreamServer),
	}
}

// SetAppState attaches the shared application state for C2 service impls.
func (s *Server) SetAppState(state *appstate.AppState) {
	s.state = state
}

// Start begins listening for agent connections via gRPC.
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	s.log.Infof("C2 server starting on %s (gRPC)", addr)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}

	s.grpcSrv = grpc.NewServer()

	// Register the AgentService (agent ↔ C2)
	agentSvc := &agentServiceServer{server: s}
	agentv1.RegisterAgentServiceServer(s.grpcSrv, agentSvc)

	// Register the C2Service (management/monitoring)
	c2Svc := &c2ServiceServer{server: s}
	c2v1.RegisterC2ServiceServer(s.grpcSrv, c2Svc)

	s.running = true

	go func() {
		<-ctx.Done()
		s.log.Info("C2 server shutting down")
		s.grpcSrv.GracefulStop()
		s.running = false
	}()

	go func() {
		if err := s.grpcSrv.Serve(listener); err != nil {
			if s.running {
				s.log.Errorf("gRPC serve error: %v", err)
			}
		}
	}()

	s.log.Infof("C2 server ready on %s", addr)
	return nil
}

// HandleCheckIn processes an agent check-in request.
func (s *Server) HandleCheckIn(agentID, hostname, os, arch, username, localIP string, privileges []string, publicKey []byte) *types.Agent {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	agent := &types.Agent{
		ID:          agentID,
		SessionID:   fmt.Sprintf("s%d", now.UnixMilli()),
		Hostname:    hostname,
		OS:          os,
		LocalIP:     localIP,
		Username:    username,
		Status:      types.AgentStatusOnline,
		FirstSeen:   now,
		LastCheckin: now,
		Privileges:  privileges,
	}

	ac := &AgentConnection{
		Agent:       agent,
		ConnectedAt: now,
		LastSeen:    now,
		AgentID:     agentID,
	}

	if len(publicKey) > 0 {
		ac.SessionKey = publicKey
	}

	s.agents[agentID] = ac
	s.log.Infof("agent checked in: %s (host=%s os=%s)", agentID, hostname, os)

	return agent
}

// GetAgents returns all connected agents.
func (s *Server) GetAgents() []*types.Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]*types.Agent, 0, len(s.agents))
	for _, ac := range s.agents {
		agents = append(agents, ac.Agent)
	}
	return agents
}

// AgentCount returns the number of connected agents.
func (s *Server) AgentCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.agents)
}

// Stop shuts down the C2 server.
func (s *Server) Stop() {
	s.running = false
	if s.grpcSrv != nil {
		s.grpcSrv.GracefulStop()
	}
	s.log.Info("C2 server stopped")
}

// RegisterStream stores the command stream for an agent.
func (s *Server) RegisterStream(agentID string, stream agentv1.AgentService_CommandStreamServer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentStreams[agentID] = stream
}

// AgentIDFromMetadata extracts the agent ID from gRPC metadata.
func AgentIDFromMetadata(ctx context.Context) string {
	// In production, extract from metadata via grpc metadata package
	return ""
}
