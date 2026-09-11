package main

// listeners.go — real TCP-family C2 transport listeners.
//
// A listener here is a bound socket with an Accept loop: connections
// are counted, never ignored. Non-TCP transports (dns/icmp/smb/doh)
// are explicitly rejected as not implemented instead of pretending to
// listen. Without authorization (LAB-ONLY posture) listeners are
// forced onto loopback like every other bind in the platform.

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/spf13/cobra"
)

type ActiveListener struct {
	ID         int
	Type       string
	Host       string
	Port       int
	Status     string
	Listener   net.Listener
	Conns      atomic.Int64
	Accepted   atomic.Int64
	closedCh   chan struct{}
	closedOnce sync.Once
}

var (
	activeListeners   []*ActiveListener
	listenersMu       sync.Mutex
	listenerIDCounter int
)

// supportedListenerTypes are the transports with a real implementation.
// Everything else in the help text is a roadmap item, not a feature.
var supportedListenerTypes = map[string]bool{
	"tcp":   true,
	"http":  true, // plain TCP carrying HTTP; no HTTP multiplexing yet
	"https": true,
}

func listenerHelpTypes() string {
	return "tcp, http, https (implemented) | dns, icmp, smb, ws, doh (roadmap)"
}

// serveAccept runs the Accept loop; each connection is counted and
// drained until close so peers never hang on a deaf socket.
func serveAccept(l *ActiveListener) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			select {
			case <-l.closedCh:
				return // intentional close
			default:
				l.Status = "error: " + err.Error()
				return
			}
		}
		l.Accepted.Add(1)
		l.Conns.Add(1)
		go func(c net.Conn) {
			defer l.Conns.Add(-1)
			// drain until the peer or the listener goes away; agent
			// protocol handling lands with the implant channel (v1.2)
			buf := make([]byte, 4096)
			for {
				if _, err := c.Read(buf); err != nil {
					break
				}
			}
			_ = c.Close()
		}(conn)
	}
}

func bindListener(l *ActiveListener) error {
	if l.Type == "https" {
		return fmt.Errorf("https listener is pending TLS cert wiring — use tcp for now")
	}
	addr := net.JoinHostPort(l.Host, strconv.Itoa(l.Port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	l.Listener = ln
	l.Status = "active"
	l.closedCh = make(chan struct{})
	go serveAccept(l)
	return nil
}

func closeListener(l *ActiveListener) {
	if l.Listener != nil {
		if l.closedCh != nil {
			l.closedOnce.Do(func() { close(l.closedCh) })
		}
		_ = l.Listener.Close()
		l.Listener = nil
	}
	l.Status = "stopped"
}

func listenersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "listeners",
		Short: "Manage C2 transport listeners",
		Long: `Manage C2 listeners for agent communication.

Implemented transports: tcp, http, https (with TLS pending).
Not implemented (roadmap): dns, icmp, smb, ws, doh — asking for them
returns an error instead of a fake listener.

Note: a listener lives inside this process. Start it from an
interactive context (vesper console / vesper --dashboard); a one-shot
` + "`vesper listeners add`" + ` closes it again when the process exits.

Examples:
  vesper listeners list
  vesper listeners add --type tcp --port 8443
  vesper listeners remove 1
  vesper listeners stop 1`,
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List active listeners",
		Run: func(cmd *cobra.Command, args []string) {
			listenersMu.Lock()
			defer listenersMu.Unlock()

			fmt.Fprintln(ConsoleOut)
			if len(activeListeners) == 0 {
				fmt.Fprintln(ConsoleOut, "  No listeners configured.")
				fmt.Fprintln(ConsoleOut)
				fmt.Fprintf(ConsoleOut, "  Transports: %s\n", listenerHelpTypes())
				fmt.Fprintln(ConsoleOut, "  Add one:    vesper listeners add --type tcp --port 8443")
				fmt.Fprintln(ConsoleOut)
				return
			}

			fmt.Fprintln(ConsoleOut, "Listeners:")
			fmt.Fprintln(ConsoleOut, "----------------------------------------------------------------")
			fmt.Fprintf(ConsoleOut, "  ID  Type       Address              Status      CurConn  Accepted\n")
			fmt.Fprintf(ConsoleOut, "  --  ---------  -------------------  ----------  -------  --------\n")
			for _, l := range activeListeners {
				fmt.Fprintf(ConsoleOut, "  %-2d  %-9s  %-19s  %-10s  %-7d  %d\n",
					l.ID, l.Type, fmt.Sprintf("%s:%d", l.Host, l.Port),
					l.Status, l.Conns.Load(), l.Accepted.Load())
			}
			fmt.Fprintln(ConsoleOut)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "add",
		Short: "Add and start a listener",
		RunE: func(cmd *cobra.Command, args []string) error {
			ltype, _ := cmd.Flags().GetString("type")
			port, _ := cmd.Flags().GetInt("port")
			host, _ := cmd.Flags().GetString("host")

			if ltype == "" {
				return fmt.Errorf("--type is required (%s)", listenerHelpTypes())
			}
			if !supportedListenerTypes[ltype] {
				return fmt.Errorf("listener type %q is not implemented yet (roadmap) — implemented: %s",
					ltype, "tcp, http, https")
			}
			if port == 0 {
				port = 8443
			}

			// geofence: unauthorized runs never bind a public interface
			if !authorized && host != "127.0.0.1" && host != "localhost" {
				printWarn("listener host %q overridden to 127.0.0.1 (LAB-ONLY posture)", host)
				host = "127.0.0.1"
			}

			listenersMu.Lock()
			defer listenersMu.Unlock()

			listenerIDCounter++
			l := &ActiveListener{
				ID:   listenerIDCounter,
				Type: ltype,
				Host: host,
				Port: port,
			}
			if err := bindListener(l); err != nil {
				return fmt.Errorf("binding %s: %w", net.JoinHostPort(host, strconv.Itoa(port)), err)
			}
			activeListeners = append(activeListeners, l)
			fmt.Fprintf(ConsoleOut, "[+] Listener %d active: %s %s (accepting, connections counted)\n",
				l.ID, ltype, net.JoinHostPort(host, strconv.Itoa(port)))
			fmt.Fprintf(ConsoleOut, "    note: lives in this process — it stops when the process exits\n")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a listener by ID",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			targetID, err := strconv.Atoi(strings.TrimSpace(args[0]))
			if err != nil {
				fmt.Fprintf(ConsoleOut, "[-] Invalid listener ID %q\n", args[0])
				return
			}
			listenersMu.Lock()
			defer listenersMu.Unlock()

			for i, l := range activeListeners {
				if l.ID == targetID {
					closeListener(l)
					activeListeners = append(activeListeners[:i], activeListeners[i+1:]...)
					fmt.Fprintf(ConsoleOut, "[+] Listener %d removed\n", l.ID)
					return
				}
			}
			fmt.Fprintf(ConsoleOut, "[-] Listener %d not found. Use 'listeners list' to see IDs.\n", targetID)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "start <id>",
		Short: "Start a stopped listener",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			targetID, err := strconv.Atoi(strings.TrimSpace(args[0]))
			if err != nil {
				fmt.Fprintf(ConsoleOut, "[-] Invalid listener ID %q\n", args[0])
				return
			}
			listenersMu.Lock()
			defer listenersMu.Unlock()

			for _, l := range activeListeners {
				if l.ID == targetID {
					if l.Status == "active" {
						fmt.Fprintf(ConsoleOut, "[i] Listener %d already active on %s:%d\n", l.ID, l.Host, l.Port)
						return
					}
					if err := bindListener(l); err != nil {
						fmt.Fprintf(ConsoleOut, "[-] Cannot bind %s: %v\n",
							net.JoinHostPort(l.Host, strconv.Itoa(l.Port)), err)
						return
					}
					fmt.Fprintf(ConsoleOut, "[+] Listener %d started: %s %s\n", l.ID, l.Type,
						net.JoinHostPort(l.Host, strconv.Itoa(l.Port)))
					return
				}
			}
			fmt.Fprintf(ConsoleOut, "[-] Listener %d not found. Use 'listeners list' to see IDs.\n", targetID)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "stop <id>",
		Short: "Stop a listener",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			targetID, err := strconv.Atoi(strings.TrimSpace(args[0]))
			if err != nil {
				fmt.Fprintf(ConsoleOut, "[-] Invalid listener ID %q\n", args[0])
				return
			}
			listenersMu.Lock()
			defer listenersMu.Unlock()

			for _, l := range activeListeners {
				if l.ID == targetID {
					if l.Status != "active" {
						fmt.Fprintf(ConsoleOut, "[i] Listener %d is not active\n", l.ID)
						return
					}
					closeListener(l)
					fmt.Fprintf(ConsoleOut, "[+] Listener %d stopped (accepted: %d)\n", l.ID, l.Accepted.Load())
					return
				}
			}
			fmt.Fprintf(ConsoleOut, "[-] Listener %d not found.\n", targetID)
		},
	})

	cmd.PersistentFlags().String("type", "", "Listener type ("+listenerHelpTypes()+")")
	cmd.PersistentFlags().Int("port", 0, "Listener port")
	cmd.PersistentFlags().String("host", "127.0.0.1", "Listener bind host (loopback unless authorized)")

	return cmd
}

// ShutdownListeners closes every listener (called on clean exits).
func ShutdownListeners() {
	listenersMu.Lock()
	defer listenersMu.Unlock()
	for _, l := range activeListeners {
		closeListener(l)
	}
	activeListeners = nil
}
