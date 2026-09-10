// Package agent provides the BridgeClient for Go↔Python gRPC IPC communication.
//
// The BridgeClient connects to a Python gRPC BridgeServer implementing
// BridgeService (ExecuteModule, AIAnalyze, ReconStream, HealthCheck).
// This enables the Go agent to invoke Python modules and 107+ ransomware
// handlers without embedding Python.
//
// Protocol: gRPC with protobuf schema (pkg/proto/bridge.proto).
// Python bridge: modules/bridge/bridge_grpc.py (gRPC server on :9100).
//
// JSON format for payloads (backward-compatible with handler expectations):
//
//	payload: {"simulation": true, "target": "10.0.0.1", ...}
//	result:  {"success": true, "encrypted": 42, ...}
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"time"

	bridgev1 "github.com/ruby570bocadito/vesper/pkg/proto/gen/bridge"
	"github.com/ruby570bocadito/vesper/pkg/shared/config"
	"github.com/ruby570bocadito/vesper/pkg/shared/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// BridgeRequest is a request to the Python bridge (kept for compatibility).
type BridgeRequest struct {
	Module    string                 `json:"module"`
	Function  string                 `json:"function"`
	Params    map[string]interface{} `json:"params"`
	TimeoutMS int64                  `json:"timeout_ms"`
}

// BridgeResponse is a response from the Python bridge (kept for compatibility).
type BridgeResponse struct {
	Success   bool                   `json:"success"`
	Result    map[string]interface{} `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
	ElapsedMS int64                  `json:"elapsed_ms"`
}

// BridgeClient communicates with the Python gRPC BridgeServer.
type BridgeClient struct {
	cfg     *config.Config
	log     *logger.Logger
	address string
	conn    *grpc.ClientConn
	stub    bridgev1.BridgeServiceClient
	mu      sync.Mutex
	cmd     *exec.Cmd
}

// NewBridgeClient creates a new bridge client.
// If bridgePath is empty, connects to an already-running bridge server.
// If bridgePath is set, starts the bridge as a subprocess.
func NewBridgeClient(cfg *config.Config, log *logger.Logger) *BridgeClient {
	port := 9100
	if cfg != nil && cfg.Agent.BridgePort > 0 {
		port = cfg.Agent.BridgePort
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	return &BridgeClient{
		cfg:     cfg,
		log:     log,
		address: addr,
	}
}

// Connect establishes a gRPC connection to the Python bridge server.
func (bc *BridgeClient) Connect(ctx context.Context) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if bc.conn != nil {
		return fmt.Errorf("already connected")
	}

	conn, err := grpc.DialContext(ctx, bc.address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return fmt.Errorf("connecting to gRPC bridge at %s: %w", bc.address, err)
	}

	bc.conn = conn
	bc.stub = bridgev1.NewBridgeServiceClient(conn)
	bc.log.Infof("gRPC bridge connected at %s", bc.address)

	return nil
}

// StartBridge starts the Python gRPC bridge as a subprocess and connects to it.
func (bc *BridgeClient) StartBridge(ctx context.Context, bridgeScript string) error {
	bc.log.Infof("starting Python gRPC bridge: %s", bridgeScript)

	port := 9100
	if bc.cfg != nil && bc.cfg.Agent.BridgePort > 0 {
		port = bc.cfg.Agent.BridgePort
	}

	bc.cmd = exec.CommandContext(ctx, "python3", bridgeScript,
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", port),
	)

	if err := bc.cmd.Start(); err != nil {
		return fmt.Errorf("starting bridge process: %w", err)
	}

	// Wait for bridge to be ready
	time.Sleep(500 * time.Millisecond)

	// Connect via gRPC
	if err := bc.Connect(ctx); err != nil {
		bc.cmd.Process.Kill()
		return fmt.Errorf("connecting to gRPC bridge after start: %w", err)
	}

	bc.log.Info("Python gRPC bridge started and connected")
	return nil
}

// Call invokes a module function on the Python bridge via gRPC.
func (bc *BridgeClient) Call(ctx context.Context, module, function string, params map[string]interface{}) (*BridgeResponse, error) {
	if bc.stub == nil {
		return nil, fmt.Errorf("not connected to gRPC bridge")
	}

	payload, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshaling params: %w", err)
	}

	req := &bridgev1.ModuleRequest{
		ModuleName:   module,
		FunctionName: function,
		Payload:      payload,
		TimeoutMs:    30000,
	}

	bc.log.Debugf("bridge call: %s.%s", module, function)

	resp, err := bc.stub.ExecuteModule(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("gRPC call %s.%s: %w", module, function, err)
	}

	br := &BridgeResponse{
		Success:   resp.Success,
		Error:     resp.Error,
		ElapsedMS: resp.ElapsedMs,
	}

	if len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, &br.Result); err != nil {
			// Non-JSON result — wrap it
			br.Result = map[string]interface{}{"raw": string(resp.Result)}
		}
	}

	if !br.Success {
		return br, fmt.Errorf("bridge error: %s", br.Error)
	}

	bc.log.Debugf("bridge response: %s.%s → elapsed=%dms", module, function, br.ElapsedMS)
	return br, nil
}

// CallModule is a convenience method that calls a module's default handler.
func (bc *BridgeClient) CallModule(ctx context.Context, module string, params map[string]interface{}) (*BridgeResponse, error) {
	return bc.Call(ctx, module, "execute", params)
}

// HealthCheck verifies the gRPC bridge is responsive.
func (bc *BridgeClient) HealthCheck(ctx context.Context) (*BridgeResponse, error) {
	if bc.stub == nil {
		return nil, fmt.Errorf("not connected to gRPC bridge")
	}

	resp, err := bc.stub.HealthCheck(ctx, &bridgev1.HealthCheckRequest{})
	if err != nil {
		return nil, fmt.Errorf("gRPC health check: %w", err)
	}

	return &BridgeResponse{
		Success: resp.Ok,
		Result: map[string]interface{}{
			"module":  resp.ModuleName,
			"version": resp.Version,
		},
	}, nil
}

// ListModules returns registered modules from the bridge.
func (bc *BridgeClient) ListModules(ctx context.Context) ([]string, error) {
	resp, err := bc.HealthCheck(ctx)
	if err != nil {
		return nil, err
	}

	if module, ok := resp.Result["module"].(string); ok {
		return []string{module}, nil
	}

	return nil, nil
}

// Disconnect closes the gRPC connection and stops the subprocess.
func (bc *BridgeClient) Disconnect() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if bc.conn != nil {
		bc.conn.Close()
		bc.conn = nil
		bc.stub = nil
	}

	if bc.cmd != nil && bc.cmd.Process != nil {
		bc.cmd.Process.Kill()
		bc.cmd = nil
	}

	bc.log.Info("gRPC bridge disconnected")
	return nil
}

// Connected returns whether the bridge is connected.
func (bc *BridgeClient) Connected() bool {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.conn != nil
}

// IsConnected is the interface-compatible alias for Connected().
func (bc *BridgeClient) IsConnected() bool {
	return bc.Connected()
}

// CallRaw calls the gRPC bridge and returns the raw result map for the dispatch interface.
func (bc *BridgeClient) CallRaw(ctx context.Context, module, function string, params map[string]interface{}) (map[string]interface{}, error) {
	resp, err := bc.Call(ctx, module, function, params)
	if err != nil {
		return nil, err
	}
	return resp.Result, nil
}
