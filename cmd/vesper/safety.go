package main

// safety.go — real authorization + kill-switch wiring.
//
// Before the Vesper remodel the `safety:` config block existed but had
// zero consumers. This file makes the safety posture executable:
//
//   - Destructive/active operations require explicit authorization:
//     --yes-i-am-authorized flag or VESPER_AUTHORIZED=1.
//   - Without authorization the CLI runs in LAB-ONLY posture: bridge
//     handlers are told (via VESPER_LAB_ONLY) to sandbox every file
//     operation inside the lab root, and the platform refuses to bind
//     non-loopback interfaces.
//   - VESPER_KILLSWITCH=1 (or `kill_switch <code>` in the console)
//     stops the engine and exits.

import (
	"fmt"
	"os"
	"strings"

	"github.com/ruby570bocadito/vesper/pkg/shared/config"
)

const killSwitchEnv = "VESPER_KILLSWITCH"

// authorized reports whether the operator explicitly authorized live use.
var authorized bool

// applySafety enforces the safety posture before any mode starts.
func applySafety(c *config.Config) {
	authorized = authorizedFlag || strings.EqualFold(os.Getenv("VESPER_AUTHORIZED"), "1")

	// Runtime kill switch: hard stop before anything else starts.
	if v := os.Getenv(killSwitchEnv); v != "" && !strings.EqualFold(v, "0") {
		fmt.Fprintln(os.Stderr, "  [!] Kill switch active (VESPER_KILLSWITCH set) — refusing to start.")
		os.Exit(130)
	}

	if authorized {
		printOK("Authorized engagement mode — safety limits from config still apply")
		printInfo("kill_switch=%v  geofence=%v  max_infections=%d  no_persistence=%v",
			c.Safety.KillSwitchEnabled, c.Safety.GeofenceEnabled,
			c.Safety.MaxInfections, c.Safety.NoPersistence)
	} else {
		printWarn("NOT authorized — running in LAB-ONLY posture")
		printInfo("Active operations are sandboxed to the lab root;")
		printInfo("to enable a live engagement: --yes-i-am-authorized (or VESPER_AUTHORIZED=1)")
		// Tell child components (Python bridge included) to stay in the lab.
		_ = os.Setenv("VESPER_LAB_ONLY", "1")
	}

	// Geofence: without authorization we never bind a public interface.
	if !authorized && c.Server.Host != "127.0.0.1" && c.Server.Host != "localhost" {
		printWarn("server.host %q overridden to 127.0.0.1 (unauthorized run)", c.Server.Host)
		c.Server.Host = "127.0.0.1"
	}

	// Dashboard auth must not be silently disabled.
	if c.Dashboard.Enabled && c.Dashboard.AuthToken == "" {
		printWarn("dashboard auth_token is empty — REST/WS API is unauthenticated (localhost only)")
	}
}

// RequireAuthorization returns an error when the operator has not
// explicitly authorized live (non-lab) operations.
func RequireAuthorization(action string) error {
	if authorized {
		return nil
	}
	return fmt.Errorf("%s requires authorization: rerun with --yes-i-am-authorized or set VESPER_AUTHORIZED=1", action)
}

// checkConsoleKillSwitch handles `kill_switch <code>` typed in the console.
func checkConsoleKillSwitch(input string) bool {
	fields := strings.Fields(strings.TrimSpace(input))
	if len(fields) < 2 || !strings.EqualFold(fields[0], "kill_switch") {
		return false
	}
	code := fields[1]
	expected := "EMERGENCY_STOP"
	if cfg != nil && cfg.Safety.KillSwitchCode != "" {
		expected = cfg.Safety.KillSwitchCode
	}
	if code == expected {
		printOK("Kill switch accepted — shutting down cleanly")
		_ = os.Setenv(killSwitchEnv, "1")
		os.Exit(130)
	}
	printErr("Invalid kill switch code")
	return true
}
