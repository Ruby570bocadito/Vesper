package recon

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestScanHostDetectsOpenPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Write([]byte("TEST-BANNER\n"))
			conn.Close()
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	res := ScanHost(context.Background(), "127.0.0.1", []int{port}, 500*time.Millisecond, true)

	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if !res[0].Open {
		t.Fatalf("port %d should be open", port)
	}
	if res[0].Banner == "" || !contains(res[0].Banner, "TEST-BANNER") {
		t.Fatalf("expected banner, got %q", res[0].Banner)
	}
}

func TestScanHostClosedPort(t *testing.T) {
	// Port 9 is almost never open locally (discard); if it somehow is,
	// pick another unlikely one. We assert NOT-open on a listener we
	// never start: bind one to RESERVE the port, close it, then scan.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	time.Sleep(50 * time.Millisecond)

	res := ScanHost(context.Background(), "127.0.0.1", []int{port}, 500*time.Millisecond, true)
	if res[0].Open {
		t.Fatalf("port %d should be closed", port)
	}
}

func TestScanHostHonorsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := ScanHost(ctx, "127.0.0.1", DefaultPorts, 10*time.Millisecond, false)
	// With a cancelled context the scan returns without blocking.
	// Results may or may not be populated — just don't hang/panic.
	_ = res
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
