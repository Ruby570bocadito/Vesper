package c2server

import (
	"context"
	"io"

	c2v1 "github.com/ruby570bocadito/vesper/pkg/proto/gen/c2"
	"github.com/ruby570bocadito/vesper/pkg/shared/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// c2ServiceServer implements c2v1.C2ServiceServer for campaign/agent management.
type c2ServiceServer struct {
	c2v1.UnimplementedC2ServiceServer
	server *Server
}

// ListAgents returns all connected agents.
func (s *c2ServiceServer) ListAgents(ctx context.Context, req *c2v1.ListAgentsRequest) (*c2v1.ListAgentsResponse, error) {
	agents := s.server.GetAgents()
	pbAgents := make([]*c2v1.AgentInfo, 0, len(agents))

	for _, a := range agents {
		if req.StatusFilter != "" && string(a.Status) != req.StatusFilter {
			continue
		}
		pbAgents = append(pbAgents, agentToProto(a))
	}

	return &c2v1.ListAgentsResponse{Agents: pbAgents}, nil
}

// GetAgent returns details about a specific agent.
func (s *c2ServiceServer) GetAgent(ctx context.Context, req *c2v1.GetAgentRequest) (*c2v1.AgentInfo, error) {
	s.server.mu.RLock()
	defer s.server.mu.RUnlock()

	if ac, ok := s.server.agents[req.AgentId]; ok {
		return agentToProto(ac.Agent), nil
	}

	return nil, notFound("agent", req.AgentId)
}

// KillAgent marks an agent as dead.
func (s *c2ServiceServer) KillAgent(ctx context.Context, req *c2v1.KillAgentRequest) (*c2v1.KillAgentResponse, error) {
	s.server.mu.Lock()
	defer s.server.mu.Unlock()

	if ac, ok := s.server.agents[req.AgentId]; ok {
		ac.Agent.Status = types.AgentStatusDead
		delete(s.server.agents, req.AgentId)

		if s.server.state != nil {
			s.server.state.RemoveAgent(req.AgentId)
			s.server.state.LogAudit(req.AgentId, "", "kill", "success", req.Reason)
		}

		return &c2v1.KillAgentResponse{Success: true}, nil
	}

	return &c2v1.KillAgentResponse{Success: false}, nil
}

// CreateCampaign creates a new campaign via the orchestrator.
func (s *c2ServiceServer) CreateCampaign(ctx context.Context, req *c2v1.CreateCampaignRequest) (*c2v1.Campaign, error) {
	if s.server.state == nil {
		return nil, notAvailable("orchestrator")
	}

	campaign, err := s.server.state.Orchestrator.StartCampaign(
		ctx, req.Name, req.TargetScope, req.Goal, req.Profile, req.AutoApprove,
	)
	if err != nil {
		return nil, err
	}

	return campaignToProto(campaign), nil
}

// GetCampaign returns a specific campaign.
func (s *c2ServiceServer) GetCampaign(ctx context.Context, req *c2v1.GetCampaignRequest) (*c2v1.Campaign, error) {
	if s.server.state == nil {
		return nil, notAvailable("orchestrator")
	}

	campaigns := s.server.state.Orchestrator.ListCampaigns()
	for _, c := range campaigns {
		if c.ID == req.CampaignId {
			return campaignToProto(c), nil
		}
	}

	return nil, notFound("campaign", req.CampaignId)
}

// ListCampaigns returns all campaigns.
func (s *c2ServiceServer) ListCampaigns(ctx context.Context, req *c2v1.ListCampaignsRequest) (*c2v1.ListCampaignsResponse, error) {
	if s.server.state == nil {
		return &c2v1.ListCampaignsResponse{}, nil
	}

	campaigns := s.server.state.Orchestrator.ListCampaigns()
	pbCampaigns := make([]*c2v1.Campaign, 0, len(campaigns))

	for _, c := range campaigns {
		pbCampaigns = append(pbCampaigns, campaignToProto(c))
	}

	return &c2v1.ListCampaignsResponse{Campaigns: pbCampaigns}, nil
}

// PauseCampaign pauses a running campaign.
func (s *c2ServiceServer) PauseCampaign(ctx context.Context, req *c2v1.PauseCampaignRequest) (*c2v1.Campaign, error) {
	if s.server.state == nil {
		return nil, notAvailable("orchestrator")
	}

	campaigns := s.server.state.Orchestrator.ListCampaigns()
	for _, c := range campaigns {
		if c.ID == req.CampaignId {
			c.Status = types.CampaignStatusPaused
			return campaignToProto(c), nil
		}
	}

	return nil, notFound("campaign", req.CampaignId)
}

// ResumeCampaign resumes a paused campaign.
func (s *c2ServiceServer) ResumeCampaign(ctx context.Context, req *c2v1.ResumeCampaignRequest) (*c2v1.Campaign, error) {
	if s.server.state == nil {
		return nil, notAvailable("orchestrator")
	}

	campaigns := s.server.state.Orchestrator.ListCampaigns()
	for _, c := range campaigns {
		if c.ID == req.CampaignId {
			c.Status = types.CampaignStatusRunning
			return campaignToProto(c), nil
		}
	}

	return nil, notFound("campaign", req.CampaignId)
}

// DecisionFeed streams decision updates from the orchestrator.
func (s *c2ServiceServer) DecisionFeed(stream c2v1.C2Service_DecisionFeedServer) error {
	if s.server.state == nil {
		return notAvailable("orchestrator")
	}

	for {
		update, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// Decisions that do not require operator approval are
		// auto-approved; blocking ones stay unapproved until an
		// operator acts through the API (proto drift fix: the
		// DecisionUpdate message carries requires_approval, not
		// an approved flag).
		approved := !update.RequiresApproval

		s.server.log.Debugf("decision feed: campaign=%s decision=%s requires_approval=%v approved=%v",
			update.CampaignId, update.DecisionId, update.RequiresApproval, approved)

		// Acknowledge
		if err := stream.Send(&c2v1.DecisionAck{
			DecisionId: update.DecisionId,
			Approved:   approved,
		}); err != nil {
			return err
		}
	}
}

// GetMetrics returns operational metrics.
func (s *c2ServiceServer) GetMetrics(ctx context.Context, req *c2v1.MetricsRequest) (*c2v1.MetricsResponse, error) {
	agents := s.server.GetAgents()
	var active, total int32
	for _, a := range agents {
		total++
		if a.Status == types.AgentStatusOnline || a.Status == types.AgentStatusActive {
			active++
		}
	}

	return &c2v1.MetricsResponse{
		TotalAgents:  total,
		ActiveAgents: active,
	}, nil
}

// ============================================================
// Conversion helpers
// ============================================================

func agentToProto(a *types.Agent) *c2v1.AgentInfo {
	return &c2v1.AgentInfo{
		AgentId:     a.ID,
		Hostname:    a.Hostname,
		Os:          a.OS,
		Arch:        "",
		Username:    a.Username,
		LocalIp:     a.LocalIP,
		Status:      string(a.Status),
		LastCheckin: a.LastCheckin.UnixMilli(),
		Uptime:      int32(a.Uptime),
		CampaignId:  a.CampaignID,
	}
}

func campaignToProto(c *types.Campaign) *c2v1.Campaign {
	return &c2v1.Campaign{
		Id:          c.ID,
		Name:        c.Name,
		TargetScope: c.TargetScope,
		Goal:        c.Goal,
		Profile:     c.Profile,
		Status:      string(c.Status),
		Phase:       string(c.Phase),
		CreatedAt:   c.CreatedAt.UnixMilli(),
		StartedAt:   c.StartedAt.UnixMilli(),
		AgentCount:  int32(c.AgentCount),
		Progress:    float32(c.Progress),
	}
}

func notFound(entity, id string) error {
	return status.Errorf(codes.NotFound, "%s not found: %s", entity, id)
}

func notAvailable(component string) error {
	return status.Errorf(codes.Unavailable, "%s not available", component)
}
