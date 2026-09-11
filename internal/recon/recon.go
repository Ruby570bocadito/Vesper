// Package recon implements Vesper's native TCP reconnaissance scanner.
//
// Unlike the pre-remodel flows it never invents results: a port is
// reported open only when a TCP connection actually succeeded, and
// banners are read from the service itself. When the Python bridge is
// offline this scanner keeps `vesper recon scan` honest and useful.
// Targets can be single IPs, hostnames or CIDR blocks (capped).
package recon

import (
	"context"
	"fmt"
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

// MaxScanHosts caps CIDR expansion so a /8 cannot explode into
// millions of probes. /24 and smaller blocks always fit.
const MaxScanHosts = 256

// ExpandTargets resolves a scan target expression into a list of host
// addresses. Accepted forms: single IP, hostname (resolved via DNS),
// CIDR block (expanded up to MaxScanHosts hosts) and comma-separated
// combinations of those.
func ExpandTargets(target string) ([]string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("empty target")
	}
	var out []string
	truncated := false
	for _, part := range strings.Split(target, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		switch {
		case strings.Contains(part, "/"):
			ip, ipnet, err := net.ParseCIDR(part)
			if err != nil {
				return nil, fmt.Errorf("invalid CIDR %q: %w", part, err)
			}
			for cur := ip.Mask(ipnet.Mask); ipnet.Contains(cur); incIP(cur) {
				if len(out) >= MaxScanHosts {
					truncated = true
					break
				}
				out = append(out, cur.String())
			}
		case net.ParseIP(part) != nil:
			if len(out) < MaxScanHosts {
				out = append(out, part)
			} else {
				truncated = true
			}
		default:
			ips, err := net.LookupIP(part)
			if err != nil || len(ips) == 0 {
				return nil, fmt.Errorf("cannot resolve host %q", part)
			}
			if len(out) < MaxScanHosts {
				out = append(out, ips[0].String())
			} else {
				truncated = true
			}
		}
		if truncated {
			break
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid targets in %q", target)
	}
	return out, nil
}

// incIP advances an IP by one (IPv4/IPv6 safe for this use).
func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// ScanHost performs a concurrent TCP connect scan of a single host.
// When banner is true, up to 128 bytes are read (best effort) from
// each open port before closing. Results keep the input port order.
// Cancellation propagates into the dials; ScanHost always waits for
// every probe goroutine to finish before returning, so the returned
// slice is never written concurrently.
func ScanHost(ctx context.Context, host string, ports []int, timeout time.Duration, banner bool) []PortResult {
	results := make([]PortResult, len(ports))
	sem := make(chan struct{}, 64)
	var wg sync.WaitGroup

	for i, port := range ports {
		wg.Add(1)
		go func(idx, port int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			r := PortResult{Port: port}
			dialer := &net.Dialer{Timeout: timeout}
			conn, err := dialer.DialContext(ctx, "tcp",
				net.JoinHostPort(host, strconv.Itoa(port)))
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

	wg.Wait()
	return results
}

// ScanTargets scans every host sequentially (each host scan is already
// concurrent) and returns per-host results. A cancelled context stops
// the sweep; hosts scanned so far are still returned.
func ScanTargets(ctx context.Context, hosts []string, ports []int, timeout time.Duration, banner bool) map[string][]PortResult {
	out := make(map[string][]PortResult, len(hosts))
	for _, h := range hosts {
		select {
		case <-ctx.Done():
			return out
		default:
		}
		out[h] = ScanHost(ctx, h, ports, timeout, banner)
	}
	return out
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
