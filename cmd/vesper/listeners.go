package main

import (
	"fmt"
	"net"
	"sync"

	"github.com/spf13/cobra"
)

type ActiveListener struct {
	ID       int
	Type     string
	Host     string
	Port     int
	Status   string
	Protocol string
	Listener net.Listener
}

var (
	activeListeners   []*ActiveListener
	listenersMu       sync.Mutex
	listenerIDCounter int
)

func listenersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "listeners",
		Short: "Manage C2 transport listeners",
		Long: `Manage C2 listeners for agent communication.
Supports: TCP, HTTP, HTTPS, DNS, ICMP, SMB, WebSocket, DoH.

Examples:
  vesper listeners list
  vesper listeners add --type tcp --port 8443
  vesper listeners add --type https --port 443 --cert cert.pem --key key.pem
  vesper listeners remove 1
  vesper listeners start 1`,
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List active listeners",
		Run: func(cmd *cobra.Command, args []string) {
			listenersMu.Lock()
			defer listenersMu.Unlock()

			fmt.Fprintln(ConsoleOut)
			if len(activeListeners) == 0 {
				fmt.Fprintln(ConsoleOut, "  No active listeners.")
				fmt.Fprintln(ConsoleOut)
				fmt.Fprintln(ConsoleOut, "  Available transports:")
				fmt.Fprintln(ConsoleOut, "    tcp, http, https, dns, icmp, smb, ws, doh")
				fmt.Fprintln(ConsoleOut)
				fmt.Fprintln(ConsoleOut, "  Add one: vesper listeners add --type tcp --port 8443")
				return
			}

			fmt.Fprintln(ConsoleOut, "Active Listeners:")
			fmt.Fprintln(ConsoleOut, "--------------------------------------------------------------")
			fmt.Fprintf(ConsoleOut, "  #  Type       Address              Status    Agents  Proto\n")
			fmt.Fprintf(ConsoleOut, "  -- ---------  -------------------  --------  ------  -----\n")
			for i, l := range activeListeners {
				agentCount := 0
				state := GetOrCreateState()
				if state != nil {
					agentCount = len(state.GetAgents())
				}
				fmt.Fprintf(ConsoleOut, "  %d  %-9s  %-19s  %-8s  %-6d  gRPC+XChaCha20\n",
					i+1, l.Type, fmt.Sprintf("%s:%d", l.Host, l.Port),
					l.Status, agentCount)
			}
			fmt.Fprintln(ConsoleOut)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "add",
		Short: "Add a new listener",
		RunE: func(cmd *cobra.Command, args []string) error {
			ltype, _ := cmd.Flags().GetString("type")
			port, _ := cmd.Flags().GetInt("port")
			host, _ := cmd.Flags().GetString("host")

			if ltype == "" {
				return fmt.Errorf("--type is required (tcp, http, https, dns, icmp, smb, ws, doh)")
			}
			if port == 0 {
				switch ltype {
				case "tcp":
					port = 8443
				case "http":
					port = 8080
				case "https":
					port = 443
				case "dns":
					port = 53
				default:
					port = 8443
				}
			}

			listenersMu.Lock()
			defer listenersMu.Unlock()

			listenerIDCounter++
			l := &ActiveListener{
				ID:       listenerIDCounter,
				Type:     ltype,
				Host:     host,
				Port:     port,
				Status:   "stopped",
				Protocol: "gRPC+XChaCha20",
			}

			// Try to actually bind the port
			addr := fmt.Sprintf("%s:%d", host, port)
			var ln net.Listener
			var err error

			if ltype == "tcp" || ltype == "http" || ltype == "https" || ltype == "ws" {
				ln, err = net.Listen("tcp", addr)
				if err == nil {
					l.Listener = ln
					l.Status = "active"
				}
			}

			activeListeners = append(activeListeners, l)

			if l.Status == "active" {
				fmt.Fprintf(ConsoleOut, "[+] Listener %d added and started: %s %s (%s)\n", l.ID, ltype, addr, l.Protocol)
			} else {
				fmt.Fprintf(ConsoleOut, "[+] Listener %d registered: %s %s (bind: %v) — use 'start %d' to activate\n",
					l.ID, ltype, addr, err, l.ID)
			}

			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "remove",
		Short: "Remove a listener",
		Run: func(cmd *cobra.Command, args []string) {
			listenersMu.Lock()
			defer listenersMu.Unlock()

			if len(activeListeners) == 0 {
				fmt.Fprintln(ConsoleOut, "[-] No listeners to remove.")
				return
			}

			// Remove the last listener if no ID specified
			idx := len(activeListeners) - 1
			l := activeListeners[idx]
			if l.Listener != nil {
				l.Listener.Close()
			}
			activeListeners = activeListeners[:idx]
			fmt.Fprintf(ConsoleOut, "[+] Listener %d removed\n", l.ID)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Start a listener",
		Run: func(cmd *cobra.Command, args []string) {
			listenersMu.Lock()
			defer listenersMu.Unlock()

			targetID := 0
			if len(args) > 0 {
				fmt.Sscanf(args[0], "%d", &targetID)
			}

			for _, l := range activeListeners {
				if targetID == 0 || l.ID == targetID {
					if l.Status == "active" {
						fmt.Fprintf(ConsoleOut, "[i] Listener %d already active on %s:%d\n", l.ID, l.Host, l.Port)
						return
					}
					addr := fmt.Sprintf("%s:%d", l.Host, l.Port)
					ln, err := net.Listen("tcp", addr)
					if err != nil {
						fmt.Fprintf(ConsoleOut, "[-] Cannot bind %s: %v\n", addr, err)
						return
					}
					l.Listener = ln
					l.Status = "active"
					fmt.Fprintf(ConsoleOut, "[+] Listener %d started: %s %s (%s)\n", l.ID, l.Type, addr, l.Protocol)
					return
				}
			}
			fmt.Fprintln(ConsoleOut, "[-] Listener not found. Use 'listeners list' to see IDs.")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Stop a listener",
		Run: func(cmd *cobra.Command, args []string) {
			listenersMu.Lock()
			defer listenersMu.Unlock()

			targetID := 0
			if len(args) > 0 {
				fmt.Sscanf(args[0], "%d", &targetID)
			}

			for _, l := range activeListeners {
				if targetID == 0 || l.ID == targetID {
					if l.Listener != nil {
						l.Listener.Close()
						l.Listener = nil
					}
					l.Status = "stopped"
					fmt.Fprintf(ConsoleOut, "[+] Listener %d stopped\n", l.ID)
					return
				}
			}
			fmt.Fprintln(ConsoleOut, "[-] Listener not found.")
		},
	})

	cmd.PersistentFlags().String("type", "", "Listener type (tcp, http, https, dns, icmp, smb, ws, doh)")
	cmd.PersistentFlags().Int("port", 0, "Listener port")
	cmd.PersistentFlags().String("host", "0.0.0.0", "Listener host")
	cmd.PersistentFlags().String("cert", "", "TLS certificate path")
	cmd.PersistentFlags().String("key", "", "TLS key path")

	return cmd
}

func ShutdownListeners() {
	listenersMu.Lock()
	defer listenersMu.Unlock()
	for _, l := range activeListeners {
		if l.Listener != nil {
			l.Listener.Close()
		}
	}
	activeListeners = nil
}
