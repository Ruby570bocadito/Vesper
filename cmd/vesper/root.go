package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ruby570bocadito/vesper/pkg/shared/config"
)

var (
	cfg             *config.Config
	cfgPath         string
	launchConsole   bool
	launchDashboard bool
	authorizedFlag  bool
)

var rootCmd = &cobra.Command{
	Use:   "vesper",
	Short: "Vesper — Semi-Autonomous Red Team Platform",
	Long:  buildRootLong(),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load(cfgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [!] config %s unreadable (%v) — running with defaults\n", cfgPath, err)
			cfg = config.Default()
		}
		// Identity/help queries answer quietly; every operational
		// mode keeps the loud safety gate (see the demo GIF).
		name := cmd.Name()
		applySafety(cfg, name == "version" || name == "help" || name == "completion")
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		// one-shot commands: tear down the state they created (bridge
		// subprocess + DB) and any listeners, instead of leaking them
		if globalState != nil && cmd.Name() != "console" && !launchDashboard {
			ShutdownListeners()
			globalState.Stop()
			globalState = nil
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		switch {
		case launchDashboard:
			if err := startDashboard(cfg); err != nil {
				printErr("Dashboard error: %v", err)
				os.Exit(1)
			}
		case launchConsole:
			state := GetOrCreateState()
			if state != nil {
				if err := NewConsole(state).Run(); err != nil {
					printErr("Console error: %v", err)
					os.Exit(1)
				}
			}
		default:
			// No flags and no subcommand → launch console by default
			state := GetOrCreateState()
			if state != nil {
				if err := NewConsole(state).Run(); err != nil {
					printErr("Console error: %v", err)
					os.Exit(1)
				}
			}
		}
	},
}

// buildRootLong returns the long description shown by --help.
func buildRootLong() string {
	var sb strings.Builder

	// Same VESPER block the console and dashboard render.
	sb.WriteString("\n")
	gradient := []string{g1, g2, g3, g4, g5, g6}
	for i, l := range bannerLines() {
		sb.WriteString("  " + gradient[i] + l + ansiR + "\n")
	}
	fmt.Fprintf(&sb, "\n  %s%sSemi-Autonomous Red Team Platform%s  %sv%s%s\n",
		cPrimary, ansiB, ansiR, cMuted, version, ansiR)
	fmt.Fprintf(&sb, "  %s%s%s\n", cMuted, strings.Repeat("─", 52), ansiR)

	fmt.Fprintf(&sb, `
%sMODES%s
  %svesper%s              → Interactive console (msf-style shell)
  %svesper --dashboard%s  → REST API + WebSocket + C2 server
  %svesper <command>%s    → One-shot CLI mode (campaign, recon, …)

%sKILL CHAIN COVERAGE%s
  Recon → Weaponize → Deliver → Exploit → Install → C2 → Objectives
`,
		cPrimary+ansiB, ansiR,
		cSuccess, ansiR,
		cSuccess, ansiR,
		cSuccess, ansiR,
		cPrimary+ansiB, ansiR,
	)

	return sb.String()
}

func Execute() {
	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "config.yaml", "Path to config file")
	rootCmd.PersistentFlags().BoolVar(&authorizedFlag, "yes-i-am-authorized", false,
		"confirm written authorization for a live engagement (or set VESPER_AUTHORIZED=1)")
	rootCmd.Flags().BoolVar(&launchConsole, "console", false, "Launch interactive console (default when no subcommand)")
	rootCmd.Flags().BoolVar(&launchDashboard, "dashboard", false, "Start API server + WebSocket + C2 backend")
	rootCmd.AddCommand(campaignCmd())
	rootCmd.AddCommand(reconCmd())
	rootCmd.AddCommand(agentCmd())
	rootCmd.AddCommand(exploitCmd())
	rootCmd.AddCommand(aiCmd())
	rootCmd.AddCommand(dashboardCmd())
	rootCmd.AddCommand(dbCmd())
	rootCmd.AddCommand(labCmd())
	rootCmd.AddCommand(payloadCmd())
	rootCmd.AddCommand(listenersCmd())
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version and build info",
		Run: func(cmd *cobra.Command, args []string) {
			printVersion()
		},
	}
}

// printVersion renders the banner plus the build info table. It reads
// nothing from disk, so main.go's --version fast path reuses it.
func printVersion() {
	printBigBanner()
	printSection("BUILD INFO")
	tbl := newTable("Field", "Value")
	tbl.addRow(cInfo+"Version"+ansiR, cWhite+"v"+version+ansiR)
	tbl.addRow(cInfo+"Go"+ansiR, cWhite+runtime.Version()+ansiR)
	tbl.addRow(cInfo+"OS/Arch"+ansiR, cWhite+runtime.GOOS+"/"+runtime.GOARCH+ansiR)
	tbl.addRow(cInfo+"Author"+ansiR, "Rafael Gálvez  ·  Cisco NetAcad")
	tbl.addRow(cInfo+"TFG"+ansiR, "Autonomous Red Team Platform — 2025/2026")
	tbl.addRow(cInfo+"License"+ansiR, "MIT")
	tbl.render()
	fmt.Fprintln(ConsoleOut)
}
