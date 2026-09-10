// Vesper — Semi-Autonomous Red Team Platform
//
// Entry point. All modes are served by the cobra command tree in
// root.go (Execute); main() only fast-paths `version` so smoke tests
// stay instant.
//
//   vesper                 → interactive console (msfconsole-style)
//   vesper --dashboard     → REST API + WebSocket + C2 backend
//   vesper <command>       → subcommands (campaign, recon, agent, …)
//
// Safety: lab-only posture is the default (see safety.go); live use
// requires --yes-i-am-authorized or VESPER_AUTHORIZED=1.

package main

import (
	"fmt"
	"os"

	"github.com/ruby570bocadito/vesper/internal/appstate"
)

func main() {
	args := os.Args[1:]

	// fast path: version must never touch disk or config
	for _, a := range args {
		if a == "version" || a == "--version" || a == "-v" {
			fmt.Println("Vesper v1.0.0")
			return
		}
	}

	if len(args) > 0 && (args[0] == "console" || args[0] == "--console") {
		// legacy explicit console flag → map onto the cobra default
		args = []string{"--console"}
	}
	os.Args = append([]string{"vesper"}, args...)

	Execute() // console, dashboard and every subcommand live here
}

// globalState is shared by commands.go / dashboard_run.go / terminal_ws.go.
var globalState *appstate.AppState
