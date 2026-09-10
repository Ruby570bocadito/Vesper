package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ruby570bocadito/vesper/pkg/shared/config"
	"github.com/ruby570bocadito/vesper/pkg/shared/logger"
)

var (
	cfg             *config.Config
	log             *logger.Logger
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
			cfg = config.Default()
		}
		log, _ = logger.New(logger.Config{
			Level:     cfg.Logging.Level,
			Format:    cfg.Logging.Format,
			Component: "cli",
		})
		applySafety(cfg)
		return nil
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

	// Gradient banner
	rows := []struct{ c, t string }{
		{g1, `  ▄▄▄   ▄▄▄  ▄▄   ▄▄   ▄▄▄▄▄▄   ▄▄   ▄▄  ▄▄▄   ▄▄▄ `},
		{g2, `  ▀██▄ ▄██▀  ██   ██  ██▀  ▀██  ██   ██  ▀██▄ ▄██▀ `},
		{g3, `    ▀███▀    ███████  ██    ██  ███████    ▀███▀   `},
		{g4, `  ▄██▀ ▀██▄       ██  ██▄  ▄██       ██  ▄██▀ ▀██▄ `},
		{g5, `  ██▀   ▀██       ██   ▀████▀        ██  ██▀   ▀██ `},
		{g6, `  ▀▀     ▀▀       ▀▀                 ▀▀  ▀▀     ▀▀ `},
	}
	sb.WriteString("\n")
	for _, r := range rows {
		sb.WriteString(r.c + r.t + ansiR + "\n")
	}
	sb.WriteString(fmt.Sprintf("\n  %s%sSemi-Autonomous Red Team Platform%s  %sv1.0.0%s\n",
		cPrimary, ansiB, ansiR, cMuted, ansiR))
	sb.WriteString(fmt.Sprintf("  %s%s%s\n", cMuted, strings.Repeat("─", 52), ansiR))

	sb.WriteString(fmt.Sprintf(`
%sMODES%s
  %svesper%s              → Interactive TUI (BubbleTea)
  %svesper console%s      → msfconsole-style interactive shell
  %svesper dashboard%s    → REST API + WebSocket + C2 server
  %svesper <command>%s    → Traditional CLI mode

%sKILL CHAIN COVERAGE%s
  Recon → Weaponize → Deliver → Exploit → Install → C2 → Objectives
`,
		cPrimary+ansiB, ansiR,
		cSuccess, ansiR,
		cSuccess, ansiR,
		cSuccess, ansiR,
		cSuccess, ansiR,
		cPrimary+ansiB, ansiR,
	))

	return sb.String()
}

func printUsageCard() {
	printPanel("QUICK START", fmt.Sprintf(
		`%svesper console%s          Launch interactive shell
%svesper dashboard%s         Start API server (port 8443)
%svesper campaign start%s    Begin a new red team operation
%svesper recon scan%s        Discover hosts & services
%svesper ai suggest%s        Get AI-powered recommendations`,
		cSuccess, ansiR,
		cSuccess, ansiR,
		cSuccess, ansiR,
		cSuccess, ansiR,
		cSuccess, ansiR,
	))
	fmt.Fprintf(ConsoleOut, "\n  %s[TAB]%s Navigate  %s[q/ctrl+c]%s Quit  %s--help%s Any command\n\n",
		cInfo, ansiR, cInfo, ansiR, cInfo, ansiR)
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
	rootCmd.AddCommand(lateralCmd())
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
			printBigBanner()
			printSection("BUILD INFO")
			tbl := newTable("Field", "Value")
			tbl.addRow(cInfo+"Version"+ansiR, cWhite+"v1.0.0"+ansiR)
			tbl.addRow(cInfo+"Go"+ansiR, cWhite+runtime.Version()+ansiR)
			tbl.addRow(cInfo+"OS/Arch"+ansiR, cWhite+runtime.GOOS+"/"+runtime.GOARCH+ansiR)
			tbl.addRow(cInfo+"Author"+ansiR, "Rafael Gálvez  ·  Cisco NetAcad")
			tbl.addRow(cInfo+"TFG"+ansiR, "Autonomous Red Team Platform — 2025/2026")
			tbl.addRow(cInfo+"License"+ansiR, "MIT")
			tbl.render()
			fmt.Fprintln(ConsoleOut)
		},
	}
}
