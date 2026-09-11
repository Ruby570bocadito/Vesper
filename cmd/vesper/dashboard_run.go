package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ruby570bocadito/vesper/internal/api"
	"github.com/ruby570bocadito/vesper/internal/appstate"
	"github.com/ruby570bocadito/vesper/pkg/shared/config"
)

func startDashboard(cfg *config.Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Redirect all structured logs to file — stdout stays clean ───────────
	cfg.Logging.Output = "file"
	if cfg.Logging.File == "" {
		cfg.Logging.File = "vesper.log"
	}

	port := cfg.Server.Port
	if port == 0 {
		port = 9090
	}

	// ── Banner ───────────────────────────────────────────────────────────────
	printDashboardBanner()

	// ── State (starts bridge internally via state.Start) ─────────────────────
	state, err := appstate.New(cfg)
	if err != nil {
		return fmt.Errorf("creating state: %w", err)
	}
	globalState = state

	if err := state.Start(ctx); err != nil {
		// Non-fatal — continue without bridge
		fmt.Printf("\033[38;5;220m  [~]\033[0m State start warning: %v\n", err)
	}
	defer state.Stop()

	// ── Bridge status (already started by state.Start) ───────────────────────
	if state.Bridge != nil && state.Bridge.Connected() {
		mods := len(state.GetModules())
		fmt.Printf("\033[38;5;46m  [+]\033[0m Python bridge \033[38;5;46m●\033[0m connected — %d modules\n", mods)
	} else {
		fmt.Printf("\033[38;5;240m  [~]\033[0m Python bridge \033[38;5;240m○\033[0m offline (local fallback active)\n")
	}

	// ── API server ───────────────────────────────────────────────────────────
	apiServer, err := api.NewWithState(cfg, state)
	if err != nil {
		return fmt.Errorf("creating API: %w", err)
	}
	apiServer.SetPort(port)
	apiServer.ServeStatic("web/dist")

	// Single shared WebSocket terminal (one Console for all browser tabs).
	// Gated by dashboard.auth_token when one is configured.
	apiServer.Mux().HandleFunc("/ws/terminal", handleTerminalWS(state, cfg.Dashboard.AuthToken))

	// ── Signal handler ───────────────────────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Printf("\n\033[38;5;196m  [!]\033[0m Shutting down Vesper...\n")
		_ = apiServer.Shutdown(context.Background())
		state.Stop()
		cancel()
		os.Exit(0)
	}()

	// ── Endpoint summary ─────────────────────────────────────────────────────
	d := "\033[38;5;240m"
	p := "\033[38;5;99m"
	w := "\033[38;5;255m"
	r := "\033[0m"
	fmt.Println()
	fmt.Printf("%s  ────────────────────────────────────────────────%s\n", d, r)
	fmt.Printf("%s  [*]%s Dashboard  %shttp://localhost:%d%s\n", p, r, w, port, r)
	fmt.Printf("%s  [*]%s API        %shttp://localhost:%d/api%s\n", p, r, w, port, r)
	fmt.Printf("%s  [*]%s Terminal   %sws://localhost:%d/ws/terminal%s\n", p, r, w, port, r)
	fmt.Printf("%s  ────────────────────────────────────────────────%s\n", d, r)
	fmt.Printf("  %sCtrl+C to stop%s\n\n", d, r)

	if err := apiServer.Start(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("API server: %w", err)
	}
	return nil
}

func printDashboardBanner() {
	printBigBanner()
	fmt.Printf("  %s%sDASHBOARD%s  %s·%s  %sv%s%s\n",
		cPrimary, ansiB, ansiR, cMuted, ansiR, cMuted, version, ansiR)
	fmt.Printf("  %s%s\n\n", cMuted, strings.Repeat("─", 46))
}
