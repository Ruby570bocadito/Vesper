// Package recon implements Vesper's native TCP reconnaissance scanner.
//
// Unlike the pre-remodel flows it never invents results: a port is
// reported open only when a TCP connection actually succeeded, and
// banners are read from the service itself. When the Python bridge is
// offline this scanner keeps `vesper recon scan` honest and useful.
package recon

import (
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PortResult is the outcome of probing one TCP port.
type PortResult struct {
	Port   int
	Open   bool
	Banner string
}

// DefaultPorts is a curated list of common TCP service ports worth
// probing in a quick scan.
var DefaultPorts = []int{
	21, 22, 23, 25, 53, 80, 110, 111, 135, 139, 143, 443, 445,
	873, 1433, 1521, 2049, 2375, 3306, 3389, 4444, 5432, 5900,
	6379, 8000, 8080, 8443, 9000, 9100, 9200, 11211, 27017,
}

// ScanHost performs a concurrent TCP connect scan of a single host.
// When banner is true, up to 128 bytes are read (best effort) from
// each open port before closing. Results keep the input port order.
func ScanHost(ctx context.Context, host string, ports []int, timeout time.Duration, banner bool) []PortResult {
	results := make([]PortResult, len(ports))
	sem := make(chan struct{}, 64)
	var wg sync.WaitGroup

	for i, port := range ports {
		wg.Add(1)
		go func(idx, port int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			r := PortResult{Port: port}
			conn, err := net.DialTimeout("tcp",
				net.JoinHostPort(host, strconv.Itoa(port)), timeout)
			if err == nil {
				r.Open = true
				if banner {
					_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
					buf := make([]byte, 128)
					if n, _ := conn.Read(buf); n > 0 {
						r.Banner = strings.TrimSpace(string(buf[:n]))
					}
				}
				_ = conn.Close()
			}
			results[idx] = r
		}(i, port)
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
	}
	return results
}

// ServiceGuess maps a port to its conventional service name. The label
// is a hint based on the port number, not a fingerprint.
func ServiceGuess(port int) string {
	names := map[int]string{
		21: "ftp", 22: "ssh", 23: "telnet", 25: "smtp", 53: "dns",
		80: "http", 110: "pop3", 111: "rpcbind", 135: "msrpc",
		139: "netbios", 143: "imap", 443: "https", 445: "smb",
		873: "rsync", 1433: "mssql", 1521: "oracle", 2049: "nfs",
		2375: "docker", 3306: "mysql", 3389: "rdp", 4444: "metasploit",
		5432: "postgres", 5900: "vnc", 6379: "redis", 8000: "http-alt",
		8080: "http-proxy", 8443: "https-alt", 9000: "services",
		9100: "printer", 9200: "elasticsearch", 11211: "memcached",
		27017: "mongodb",
	}
	if n, ok := names[port]; ok {
		return n
	}
	return "unknown"
}
